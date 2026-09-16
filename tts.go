package main

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
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
	pythonBin string
	voice     string
	volume    float64
}

func NewTTS(cfg *Config) *TTS {
	return &TTS{
		pythonBin: cfg.PiperVenvPython,
		voice:     cfg.TTSVoice,
		volume:    cfg.TTSVolume,
	}
}

// Speak synthesizes text with Piper and plays it back. onPlaybackStart, if
// non-nil, fires right as audio playback begins (not before synthesis,
// which can take a while) so callers can sync mouth animation to the voice.
// ponytail: playback is macOS-only via `afplay`; swap for `aplay` on a Raspberry Pi/Linux target.
func (t *TTS) Speak(text string, onPlaybackStart func()) {
	fmt.Printf("\n🤖 Miaou: %s\n\n", text)

	clean := stripEmojis(text)
	if clean == "" {
		return
	}

	tmpFile, err := os.CreateTemp("", "miaou-*.wav")
	if err != nil {
		fmt.Printf("❌ TTS error: %v\n", err)
		return
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	synth := exec.Command(t.pythonBin, "-m", "piper", "-m", t.voice, "-f", tmpFile.Name())
	synth.Stdin = strings.NewReader(clean)
	if out, err := synth.CombinedOutput(); err != nil {
		fmt.Printf("❌ TTS error: %v\n%s\n", err, out)
		return
	}

	play := exec.Command("afplay", "-v", strconv.FormatFloat(t.volume, 'f', 2, 64), tmpFile.Name())
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
