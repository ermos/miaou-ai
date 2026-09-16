package main

// ponytail: this file could not be exercised in the sandbox this was
// written in — there was no microphone available. Test on real hardware
// before relying on it.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

// WhisperAudio captures the mic and sends it to OpenAI's hosted transcription
// API. Local speech recognition (Vosk, then whisper.cpp) kept hitting real
// walls on a Pi 4: Vosk's small models are single-language, and whisper.cpp's
// tiny model can't reliably auto-detect language on a short/noisy clip
// (observed live: Russian gibberish for a French sentence) — the app already
// depends on OpenAI being reachable for chat, so offloading STT there too
// removes a whole local model/build/language-guessing problem for free.
type WhisperAudio struct {
	apiURL     string
	apiKey     string
	model      string
	httpClient *http.Client

	sampleRate int
	timeout    time.Duration

	silenceThreshold int32
	silenceHangover  time.Duration
	debug            bool

	malgoCtx *malgo.AllocatedContext
}

func NewWhisperAudio(cfg *Config) (*WhisperAudio, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	fmt.Printf("✅ Audio Manager ready (OpenAI transcription model: %s)\n", cfg.WhisperModel)
	return &WhisperAudio{
		apiURL:           cfg.LLMURL + "/audio/transcriptions",
		apiKey:           cfg.OpenAIAPIKey,
		model:            cfg.WhisperModel,
		httpClient:       &http.Client{Timeout: 30 * time.Second},
		sampleRate:       cfg.SampleRate,
		timeout:          time.Duration(cfg.ListeningTimeout) * time.Second,
		silenceThreshold: int32(cfg.VADSilenceThreshold),
		silenceHangover:  time.Duration(cfg.VADSilenceMs) * time.Millisecond,
		debug:            cfg.Debug,
		malgoCtx:         ctx,
	}, nil
}

func (w *WhisperAudio) Listen() (string, bool) {
	audio, err := w.recordAudio()
	if err != nil {
		fmt.Printf("❌ Recording error: %v\n", err)
		// ponytail: a broken mic/ALSA config fails here instantly and
		// unconditionally on every call; without a pause the caller's loop
		// retries at full speed and pegs the CPU. A real capture starts by
		// blocking for at least some audio, so this only slows down the
		// broken-device case.
		time.Sleep(time.Second)
		return "", true
	}

	text, err := w.transcribe(audio)
	if err != nil {
		fmt.Printf("❌ Whisper error: %v\n", err)
		time.Sleep(time.Second)
		return "", true
	}
	return text, true
}

// peakAmplitude returns the largest sample magnitude in a chunk of
// little-endian S16 PCM, used as a cheap voice-activity signal.
func peakAmplitude(pcm []byte) int32 {
	var peak int32
	for i := 0; i+1 < len(pcm); i += 2 {
		s := int32(int16(binary.LittleEndian.Uint16(pcm[i:])))
		if s < 0 {
			s = -s
		}
		if s > peak {
			peak = s
		}
	}
	return peak
}

func (w *WhisperAudio) recordAudio() ([]byte, error) {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(w.sampleRate)

	var (
		mu          sync.Mutex
		captured    []byte
		lastVoiceAt = time.Now()
		voiceHeard  bool
		lastPeak    int32
	)

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pSample []byte, _ uint32) {
			peak := peakAmplitude(pSample)
			loud := peak > w.silenceThreshold

			mu.Lock()
			captured = append(captured, pSample...)
			lastPeak = peak
			if loud {
				lastVoiceAt = time.Now()
				voiceHeard = true
			}
			mu.Unlock()
		},
	}

	device, err := malgo.InitDevice(w.malgoCtx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, fmt.Errorf("init capture device: %w", err)
	}
	defer device.Uninit()

	fmt.Println("🎤 Listening (speak now)...")
	if err := device.Start(); err != nil {
		return nil, fmt.Errorf("start capture: %w", err)
	}

	// Stop as soon as speech is followed by enough silence, instead of
	// always recording the full ListeningTimeout — that timeout stays as
	// the hard cap for someone who just keeps talking.
	deadline := time.Now().Add(w.timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	for tick := 0; ; tick++ {
		now := <-ticker.C
		mu.Lock()
		spoke, silentFor, peak := voiceHeard, now.Sub(lastVoiceAt), lastPeak
		mu.Unlock()

		// ~once/second: confirms whether spoken audio is actually crossing
		// VAD_SILENCE_THRESHOLD at all, vs. always waiting out the timeout.
		if w.debug && tick%10 == 0 {
			fmt.Printf("🔊 peak=%d threshold=%d spoke=%v\n", peak, w.silenceThreshold, spoke)
		}

		if now.After(deadline) || (spoke && silentFor > w.silenceHangover) {
			break
		}
	}
	ticker.Stop()

	if err := device.Stop(); err != nil {
		return nil, fmt.Errorf("stop capture: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	return captured, nil
}

// transcribe uploads the captured PCM (wrapped as a WAV file) to OpenAI's
// /audio/transcriptions endpoint. No language is forced: this model's
// language auto-detection is far more reliable than a local tiny model's,
// so there's no need to race English/French candidates against each other.
func (w *WhisperAudio) transcribe(pcm []byte) (string, error) {
	wavPath, err := writeWAV(pcm, w.sampleRate)
	if err != nil {
		return "", fmt.Errorf("write wav: %w", err)
	}
	defer func() { _ = os.Remove(wavPath) }()

	f, err := os.Open(wavPath)
	if err != nil {
		return "", fmt.Errorf("open wav: %w", err)
	}
	defer func() { _ = f.Close() }()

	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if _, err := io.Copy(part, f); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if err := writer.WriteField("model", w.model); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, w.apiURL, body)
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+w.apiKey)

	resp, err := w.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("call OpenAI transcription API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read transcription response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI transcription API: %d %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("parse transcription response: %w", err)
	}
	return strings.TrimSpace(parsed.Text), nil
}

// writeWAV wraps raw S16LE mono PCM in a WAV container and returns the
// temp file path; the transcription API (like most audio tools) won't
// accept bare PCM.
func writeWAV(pcm []byte, sampleRate int) (string, error) {
	f, err := os.CreateTemp("", "miaou-*.wav")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	const (
		numChannels   = 1
		bitsPerSample = 16
	)
	byteRate := sampleRate * numChannels * bitsPerSample / 8
	blockAlign := numChannels * bitsPerSample / 8
	dataSize := uint32(len(pcm))

	// bytes.Buffer.Write never errors, so binary.Write can't fail here.
	header := new(bytes.Buffer)
	header.WriteString("RIFF")
	_ = binary.Write(header, binary.LittleEndian, uint32(36+dataSize))
	header.WriteString("WAVE")
	header.WriteString("fmt ")
	_ = binary.Write(header, binary.LittleEndian, uint32(16)) // PCM fmt chunk size
	_ = binary.Write(header, binary.LittleEndian, uint16(1))  // PCM format
	_ = binary.Write(header, binary.LittleEndian, uint16(numChannels))
	_ = binary.Write(header, binary.LittleEndian, uint32(sampleRate))
	_ = binary.Write(header, binary.LittleEndian, uint32(byteRate))
	_ = binary.Write(header, binary.LittleEndian, uint16(blockAlign))
	_ = binary.Write(header, binary.LittleEndian, uint16(bitsPerSample))
	header.WriteString("data")
	_ = binary.Write(header, binary.LittleEndian, dataSize)

	if _, err := f.Write(header.Bytes()); err != nil {
		return "", err
	}
	if _, err := f.Write(pcm); err != nil {
		return "", err
	}
	return f.Name(), nil
}
