package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

var emojiPattern = regexp.MustCompile(
	"[" +
		"\U0001F300-\U0001FAFF" +
		"\U00002600-\U000027BF" +
		"\U0001F1E0-\U0001F1FF" +
		"]+",
)

// stripEmojis removes emojis before sending text to TTS (they sound odd read
// aloud); the emoji stay in printed/logged text.
func stripEmojis(text string) string {
	return strings.TrimSpace(emojiPattern.ReplaceAllString(text, ""))
}

type TTS struct {
	apiURL     string
	apiKey     string
	model      string
	voice      string
	volume     float64
	httpClient *http.Client
}

func NewTTS(cfg *Config) *TTS {
	return &TTS{
		apiURL:     cfg.LLMURL + "/audio/speech",
		apiKey:     cfg.OpenAIAPIKey,
		model:      cfg.TTSModel,
		voice:      cfg.TTSVoice,
		volume:     cfg.TTSVolume,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// playbackCommand picks the OS's built-in WAV player. aplay (Linux/ALSA) has
// no per-invocation gain flag, so on Linux `volume` is a no-op; use `alsamixer`
// or `amixer` on the Pi to set the output level instead.
func playbackCommand(volume float64, path string) *exec.Cmd {
	if runtime.GOOS == "darwin" {
		return exec.Command("afplay", "-v", strconv.FormatFloat(volume, 'f', 2, 64), path)
	}
	return exec.Command("aplay", "-q", path)
}

type speechRequest struct {
	Model          string `json:"model"`
	Input          string `json:"input"`
	Voice          string `json:"voice"`
	ResponseFormat string `json:"response_format"`
}

// synthesize calls OpenAI's /audio/speech endpoint and returns a WAV file's
// bytes (response_format=wav keeps it directly playable by afplay/aplay,
// no mp3 decoder needed on the Pi).
func (t *TTS) synthesize(text string) ([]byte, error) {
	reqBody, _ := json.Marshal(speechRequest{
		Model:          t.model,
		Input:          text,
		Voice:          t.voice,
		ResponseFormat: "wav",
	})

	req, err := http.NewRequest(http.MethodPost, t.apiURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call OpenAI speech API: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read speech response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OpenAI speech API: %d %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// Speak synthesizes text via OpenAI's TTS API and plays it back.
// onPlaybackStart, if non-nil, fires right as audio playback begins (not
// before synthesis, which involves a network round-trip) so callers can
// sync mouth animation to the voice.
func (t *TTS) Speak(text string, onPlaybackStart func()) {
	fmt.Printf("\n🤖 Miaou: %s\n\n", text)

	clean := stripEmojis(text)
	if clean == "" {
		return
	}

	audio, err := t.synthesize(clean)
	if err != nil {
		fmt.Printf("❌ TTS error: %v\n", err)
		return
	}

	tmpFile, err := os.CreateTemp("", "miaou-*.wav")
	if err != nil {
		fmt.Printf("❌ TTS error: %v\n", err)
		return
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(audio); err != nil {
		tmpFile.Close()
		fmt.Printf("❌ TTS error: %v\n", err)
		return
	}
	tmpFile.Close()

	play := playbackCommand(t.volume, tmpFile.Name())
	if err := play.Start(); err != nil {
		fmt.Printf("❌ Playback error: %v\n", err)
		return
	}
	if onPlaybackStart != nil {
		onPlaybackStart()
	}
	if err := play.Wait(); err != nil {
		fmt.Printf("❌ Playback error: %v\n", err)
	}
}
