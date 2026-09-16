package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AudioManager captures user speech/text. Listen blocks until an utterance
// is available (or the source closes), returning ok=false in the latter case.
type AudioManager interface {
	Listen() (text string, ok bool)
}

type TextAudio struct {
	scanner *bufio.Scanner
}

func NewTextAudio() *TextAudio {
	fmt.Println("✅ Audio Manager ready (Text input mode)")
	return &TextAudio{scanner: bufio.NewScanner(os.Stdin)}
}

func (t *TextAudio) Listen() (string, bool) {
	fmt.Print("\n🎤 You: ")
	if !t.scanner.Scan() {
		return "", false
	}
	text := strings.TrimSpace(t.scanner.Text())
	if text == "" {
		return "", true
	}
	return text, true
}

func NewAudioManager(cfg *Config) (AudioManager, error) {
	switch cfg.AudioMode {
	case "text_input":
		return NewTextAudio(), nil
	case "whisper":
		return NewWhisperAudio(cfg)
	default:
		return nil, fmt.Errorf("unknown AUDIO_MODE: %s (use 'whisper' or 'text_input')", cfg.AudioMode)
	}
}
