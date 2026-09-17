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
	"math"
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
	language   string // ISO-639-1 hint, e.g. "en"; empty lets the API guess
	httpClient *http.Client

	sampleRate int
	timeout    time.Duration

	silenceThreshold int32
	silenceHangover  time.Duration
	minVoiceDuration time.Duration
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
		language:         cfg.WhisperLanguage,
		httpClient:       &http.Client{Timeout: 60 * time.Second},
		sampleRate:       cfg.SampleRate,
		timeout:          time.Duration(cfg.ListeningTimeout) * time.Second,
		silenceThreshold: int32(cfg.VADSilenceThreshold),
		silenceHangover:  time.Duration(cfg.VADSilenceMs) * time.Millisecond,
		minVoiceDuration: time.Duration(cfg.VADMinVoiceMs) * time.Millisecond,
		debug:            cfg.Debug,
		malgoCtx:         ctx,
	}, nil
}

func (w *WhisperAudio) Listen() (string, bool) {
	audio, spoke, err := w.recordAudio()
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
	if !spoke {
		// Nothing crossed VAD_SILENCE_THRESHOLD for the whole listening
		// window (the idle/no-one-talking case) — sending silence to a
		// paid transcription API on every cycle would bill for nothing.
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

// rmsLevel returns the root-mean-square amplitude of a chunk of
// little-endian S16 PCM. Unlike a single-sample peak, it reflects energy
// sustained across the whole chunk: a keyboard click or a clap is a brief
// spike surrounded by silence within the same chunk, so it reads much
// lower on RMS than continuous speech does at the same peak amplitude.
func rmsLevel(pcm []byte) int32 {
	if len(pcm) < 2 {
		return 0
	}
	var sumSq float64
	n := 0
	for i := 0; i+1 < len(pcm); i += 2 {
		s := float64(int16(binary.LittleEndian.Uint16(pcm[i:])))
		sumSq += s * s
		n++
	}
	return int32(math.Sqrt(sumSq / float64(n)))
}

func (w *WhisperAudio) recordAudio() ([]byte, bool, error) {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(w.sampleRate)

	var (
		mu              sync.Mutex
		captured        []byte
		lastVoiceAt     = time.Now()
		voiceHeard      bool
		lastLevel       int32
		firstVoiceAt    int       // byte offset into captured of the first loud chunk
		lastVoiceIdx    int       // byte offset just past the most recent loud chunk
		loudStreakStart time.Time // zero when not currently in a loud streak
		loudStreakFrom  int       // byte offset where the current loud streak began
	)

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pSample []byte, _ uint32) {
			level := rmsLevel(pSample)
			loud := level > w.silenceThreshold

			mu.Lock()
			chunkStart := len(captured)
			captured = append(captured, pSample...)
			lastLevel = level

			switch {
			case !loud && !voiceHeard:
				// A brief loud blip (click, clap) not sustained long enough
				// to clear minVoiceDuration - drop the streak instead of
				// starting a recording for it.
				loudStreakStart = time.Time{}
			case loud && !voiceHeard:
				if loudStreakStart.IsZero() {
					loudStreakStart = time.Now()
					loudStreakFrom = chunkStart
				}
				if time.Since(loudStreakStart) >= w.minVoiceDuration {
					voiceHeard = true
					firstVoiceAt = loudStreakFrom
					lastVoiceAt = time.Now()
					lastVoiceIdx = len(captured)
				}
			case loud && voiceHeard:
				lastVoiceAt = time.Now()
				lastVoiceIdx = len(captured)
			}
			mu.Unlock()
		},
	}

	device, err := malgo.InitDevice(w.malgoCtx.Context, deviceConfig, callbacks)
	if err != nil {
		return nil, false, fmt.Errorf("init capture device: %w", err)
	}
	defer device.Uninit()

	fmt.Println("🎤 Listening (speak now)...")
	if err := device.Start(); err != nil {
		return nil, false, fmt.Errorf("start capture: %w", err)
	}

	// Stop as soon as speech is followed by enough silence, instead of
	// always recording the full ListeningTimeout — that timeout stays as
	// the hard cap for someone who just keeps talking.
	deadline := time.Now().Add(w.timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	for tick := 0; ; tick++ {
		now := <-ticker.C
		mu.Lock()
		spoke, silentFor, level := voiceHeard, now.Sub(lastVoiceAt), lastLevel
		mu.Unlock()

		// ~once/second: confirms whether spoken audio is actually crossing
		// VAD_SILENCE_THRESHOLD at all, vs. always waiting out the timeout.
		if w.debug && tick%10 == 0 {
			fmt.Printf("🔊 level=%d threshold=%d spoke=%v\n", level, w.silenceThreshold, spoke)
		}

		if now.After(deadline) || (spoke && silentFor > w.silenceHangover) {
			break
		}
	}
	ticker.Stop()

	if err := device.Stop(); err != nil {
		return nil, false, fmt.Errorf("stop capture: %w", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !voiceHeard {
		return captured, false, nil
	}

	// Trim to just the speech, with a small margin on each side so the
	// first/last word isn't clipped — sending the whole buffer including
	// silence before the user started talking wastes upload time and bills
	// for audio that isn't speech.
	const margin = 300 * time.Millisecond
	marginBytes := int(margin.Seconds() * float64(w.sampleRate) * 2) // S16 mono = 2 bytes/sample
	start := firstVoiceAt - marginBytes
	if start < 0 {
		start = 0
	}
	end := lastVoiceIdx + marginBytes
	if end > len(captured) {
		end = len(captured)
	}
	return captured[start:end], true, nil
}

// transcribe uploads the captured PCM (wrapped as a WAV file) to OpenAI's
// /audio/transcriptions endpoint.
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
	if w.language != "" {
		// Without this hint, the API occasionally drifts into translating
		// the audio instead of transcribing it (see the language field's
		// doc comment on Config) — forcing a language keeps it faithful.
		if err := writer.WriteField("language", w.language); err != nil {
			return "", fmt.Errorf("build request: %w", err)
		}
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
