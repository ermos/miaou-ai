package main

// ponytail: this file could not be exercised in the sandbox this was
// written in — there was no microphone or whisper.cpp binary available.
// Test on real hardware before relying on it.

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
)

type WhisperAudio struct {
	whisperBin string
	modelPath  string
	sampleRate int
	timeout    time.Duration

	silenceThreshold int32
	silenceHangover  time.Duration

	malgoCtx *malgo.AllocatedContext
}

func NewWhisperAudio(cfg *Config) (*WhisperAudio, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	fmt.Printf("✅ Audio Manager ready (whisper.cpp: %s)\n", cfg.WhisperModel)
	return &WhisperAudio{
		whisperBin:       cfg.WhisperBin,
		modelPath:        cfg.WhisperModel,
		sampleRate:       cfg.SampleRate,
		timeout:          time.Duration(cfg.ListeningTimeout) * time.Second,
		silenceThreshold: int32(cfg.VADSilenceThreshold),
		silenceHangover:  time.Duration(cfg.VADSilenceMs) * time.Millisecond,
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
	)

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pSample []byte, _ uint32) {
			loud := peakAmplitude(pSample) > w.silenceThreshold

			mu.Lock()
			captured = append(captured, pSample...)
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
	for now := range ticker.C {
		mu.Lock()
		spoke, silentFor := voiceHeard, now.Sub(lastVoiceAt)
		mu.Unlock()

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

// whisperOutput mirrors the subset of whisper-cli's -ojf (full JSON) output
// this cares about: the recognized text and, per token, its probability
// ("p"), used as a confidence signal. Unlisted fields (timestamps, model
// info, ...) are simply ignored by json.Unmarshal.
type whisperOutput struct {
	Transcription []struct {
		Text   string `json:"text"`
		Tokens []struct {
			P float64 `json:"p"`
		} `json:"tokens"`
	} `json:"transcription"`
}

// transcribe runs whisper.cpp twice on the captured PCM, once forced to
// English and once to French, and keeps whichever whisper itself was more
// confident about (mean per-token probability). whisper.cpp's own "auto"
// language detection picks from ~100 languages off a single short/noisy
// clip and a tiny model — observed live returning Russian gibberish for a
// French sentence. Since only these two languages actually matter here,
// forcing both candidates and picking the better score is far more robust
// than trusting one open-ended guess.
func (w *WhisperAudio) transcribe(pcm []byte) (string, error) {
	wavPath, err := writeWAV(pcm, w.sampleRate)
	if err != nil {
		return "", fmt.Errorf("write wav: %w", err)
	}
	defer func() { _ = os.Remove(wavPath) }()

	type result struct {
		text string
		conf float64
		err  error
	}
	run := func(lang string) <-chan result {
		ch := make(chan result, 1)
		go func() {
			text, conf, err := w.transcribeLang(wavPath, lang)
			ch <- result{text, conf, err}
		}()
		return ch
	}

	en, fr := <-run("en"), <-run("fr")

	switch {
	case en.err != nil && fr.err != nil:
		return "", en.err
	case en.err != nil:
		return fr.text, nil
	case fr.err != nil:
		return en.text, nil
	case en.text == "":
		return fr.text, nil
	case fr.text == "":
		return en.text, nil
	case fr.conf > en.conf:
		return fr.text, nil
	default:
		return en.text, nil
	}
}

func (w *WhisperAudio) transcribeLang(wavPath, lang string) (string, float64, error) {
	outDir, err := os.MkdirTemp("", "miaou-whisper-*")
	if err != nil {
		return "", 0, err
	}
	defer func() { _ = os.RemoveAll(outDir) }()
	outBase := filepath.Join(outDir, "out")

	cmd := exec.Command(w.whisperBin, "-m", w.modelPath, "-f", wavPath, "-l", lang, "-ojf", "-of", outBase, "-np", "-nt")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", 0, fmt.Errorf("whisper-cli (%s): %w: %s", lang, err, stderr.String())
	}

	data, err := os.ReadFile(outBase + ".json")
	if err != nil {
		return "", 0, fmt.Errorf("read whisper output (%s): %w", lang, err)
	}
	var parsed whisperOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		return "", 0, fmt.Errorf("parse whisper output (%s): %w", lang, err)
	}

	var text strings.Builder
	var probSum float64
	var probCount int
	for _, seg := range parsed.Transcription {
		text.WriteString(seg.Text)
		for _, tok := range seg.Tokens {
			probSum += tok.P
			probCount++
		}
	}

	var conf float64
	if probCount > 0 {
		conf = probSum / float64(probCount)
	}
	return strings.TrimSpace(text.String()), conf, nil
}

// writeWAV wraps raw S16LE mono PCM in a WAV container and returns the
// temp file path; whisper-cli (and most audio tools) won't accept bare PCM.
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
