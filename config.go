package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	OpenAIAPIKey string
	OpenAIModel  string
	LLMURL       string

	ScreenWidth  int
	ScreenHeight int

	SampleRate int
	TTSVolume  float64
	TTSVoice   string // path to the piper .onnx voice model

	WhisperModel string // OpenAI transcription model name, e.g. "whisper-1"

	AudioMode string // "text_input" or "whisper"

	LLMTemperature float64

	WakeWord string

	InactivityTimeout int // seconds
	ListeningTimeout  int // seconds

	// Voice-activity detection: stop recording once speech has been heard
	// followed by this much silence, instead of always waiting the full
	// ListeningTimeout. VADSilenceThreshold is a peak PCM amplitude
	// (0-32767) below which a chunk counts as silence — mic gain and
	// ambient noise vary per device, so this needs to be tunable rather
	// than a fixed constant.
	VADSilenceThreshold int
	VADSilenceMs        int

	MemoryDir       string
	PersonalityFile string
	AssetsDir       string
	PiperBin        string // native piper executable (assets/piper/piper)
	PiperEspeakData string // assets/piper/espeak-ng-data

	Debug bool
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getenvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func getenvFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func LoadConfig() *Config {
	_ = godotenv.Load()

	root, err := os.Getwd()
	if err != nil {
		root = "."
	}

	audioMode := strings.TrimSpace(getenv("AUDIO_MODE", "text_input"))

	cfg := &Config{
		OpenAIAPIKey: getenv("OPENAI_API_KEY", ""),
		OpenAIModel:  getenv("OPENAI_MODEL", "gpt-4o-mini"),
		LLMURL:       "https://api.openai.com/v1",

		ScreenWidth:  getenvInt("SCREEN_WIDTH", 320),
		ScreenHeight: getenvInt("SCREEN_HEIGHT", 240),

		SampleRate: getenvInt("SAMPLE_RATE", 16000),
		TTSVolume:  getenvFloat("TTS_VOLUME", 0.9),

		AudioMode: audioMode,

		LLMTemperature: getenvFloat("LLM_TEMPERATURE", 0.7),

		WakeWord: getenv("WAKE_WORD", "miaou"),

		InactivityTimeout: getenvInt("INACTIVITY_TIMEOUT", 300),
		ListeningTimeout:  getenvInt("LISTENING_TIMEOUT", 30),

		VADSilenceThreshold: getenvInt("VAD_SILENCE_THRESHOLD", 500),
		VADSilenceMs:        getenvInt("VAD_SILENCE_MS", 1200),

		WhisperModel: getenv("OPENAI_WHISPER_MODEL", "whisper-1"),

		MemoryDir:       filepath.Join(root, getenv("MEMORY_DIR", "memory")),
		PersonalityFile: filepath.Join(root, getenv("PERSONALITY_FILE", "PERSONALITY.md")),
		AssetsDir:       filepath.Join(root, "assets"),

		Debug: strings.EqualFold(getenv("DEBUG", "False"), "true"),
	}

	cfg.TTSVoice = filepath.Join(cfg.AssetsDir, "piper", "en_US-amy-medium.onnx")
	cfg.PiperBin = filepath.Join(cfg.AssetsDir, "piper", "piper")
	cfg.PiperEspeakData = filepath.Join(cfg.AssetsDir, "piper", "espeak-ng-data")

	_ = os.MkdirAll(cfg.MemoryDir, 0o755)

	return cfg
}
