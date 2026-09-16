package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
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
	piperBin   string
	espeakData string
	voice      string
	volume     float64
}

func NewTTS(cfg *Config) *TTS {
	return &TTS{
		piperBin:   cfg.PiperBin,
		espeakData: cfg.PiperEspeakData,
		voice:      cfg.TTSVoice,
		volume:     cfg.TTSVolume,
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

// Speak synthesizes text with Piper and plays it back. onPlaybackStart, if
// non-nil, fires right as audio playback begins (not before synthesis,
// which can take a while) so callers can sync mouth animation to the voice.
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

	synth := exec.Command(t.piperBin, "-m", t.voice, "-f", tmpFile.Name(), "--espeak_data", t.espeakData)
	synth.Stdin = strings.NewReader(clean)
	// Linux .so's ship next to the piper binary with no rpath set.
	synth.Env = append(os.Environ(), "LD_LIBRARY_PATH="+filepath.Dir(t.piperBin))
	if out, err := synth.CombinedOutput(); err != nil {
		fmt.Printf("❌ TTS error: %v\n%s\n", err, out)
		return
	}

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
