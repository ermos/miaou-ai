package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	VoskServerURL string
	OpenAIAPIKey  string
	OpenAIModel   string
	LLMURL        string

	ScreenWidth  int
	ScreenHeight int

	SampleRate int
	TTSVolume  float64
	TTSVoice   string // path to the piper .onnx voice model

	AudioMode string // "text_input" or "vosk_server"

	LLMTemperature float64

	WakeWord string

	InactivityTimeout int // seconds
	ListeningTimeout  int // seconds

	MemoryDir       string
	PersonalityFile string
	AssetsDir       string
	PiperVenvPython string

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
		VoskServerURL: getenv("VOSK_SERVER_URL", "ws://localhost:2700"),
		OpenAIAPIKey:  getenv("OPENAI_API_KEY", ""),
		OpenAIModel:   getenv("OPENAI_MODEL", "gpt-4o-mini"),
		LLMURL:        "https://api.openai.com/v1",

		ScreenWidth:  getenvInt("SCREEN_WIDTH", 320),
		ScreenHeight: getenvInt("SCREEN_HEIGHT", 240),

		SampleRate: getenvInt("SAMPLE_RATE", 16000),
		TTSVolume:  getenvFloat("TTS_VOLUME", 0.9),

		AudioMode: audioMode,

		LLMTemperature: getenvFloat("LLM_TEMPERATURE", 0.7),

		WakeWord: getenv("WAKE_WORD", "miaou"),

		InactivityTimeout: getenvInt("INACTIVITY_TIMEOUT", 300),
		ListeningTimeout:  getenvInt("LISTENING_TIMEOUT", 30),

		MemoryDir:       filepath.Join(root, getenv("MEMORY_DIR", "memory")),
		PersonalityFile: filepath.Join(root, getenv("PERSONALITY_FILE", "PERSONALITY.md")),
		AssetsDir:       filepath.Join(root, "assets"),
		PiperVenvPython: filepath.Join(root, "tts_engine", "venv", "bin", "python3"),

		Debug: strings.EqualFold(getenv("DEBUG", "False"), "true"),
	}

	cfg.TTSVoice = filepath.Join(cfg.AssetsDir, "piper", "en_US-amy-medium.onnx")

	_ = os.MkdirAll(cfg.MemoryDir, 0o755)

	return cfg
}
