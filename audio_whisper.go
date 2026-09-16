package main

// ponytail: this file could not be exercised in the sandbox this was
// written in — there was no microphone or whisper.cpp binary available.
// Test on real hardware before relying on it.

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
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

// transcribe runs whisper.cpp on the captured PCM. whisper-cli only reads
// audio files, so the raw S16LE mono capture is wrapped in a minimal WAV
// header first. Language is left on "auto" — whisper.cpp detects it itself,
// unlike Vosk's fixed per-model vocabulary, so this one binary handles both
// English and French without running separate servers per language.
func (w *WhisperAudio) transcribe(pcm []byte) (string, error) {
	wavPath, err := writeWAV(pcm, w.sampleRate)
	if err != nil {
		return "", fmt.Errorf("write wav: %w", err)
	}
	defer func() { _ = os.Remove(wavPath) }()

	cmd := exec.Command(w.whisperBin, "-m", w.modelPath, "-f", wavPath, "-l", "auto", "-nt", "-np")
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("whisper-cli: %w: %s", err, exitErr.Stderr)
		}
		return "", fmt.Errorf("whisper-cli: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
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
