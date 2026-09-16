package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

type ChatBuddy struct {
	cfg         *Config
	ctxManager  *ContextManager
	llm         *LLMClient
	audio       AudioManager
	tts         *TTS
	brain       *Brain
	personality *Personality

	isActive         bool
	lastActivityTime time.Time
}

func NewChatBuddy() (*ChatBuddy, error) {
	fmt.Println("\n==================================================")
	fmt.Println("🐱 English Buddy - Starting up...")
	fmt.Println("==================================================")

	cfg := LoadConfig()

	personality := LoadPersonality(cfg.PersonalityFile)
	ctxManager := NewContextManager(cfg.MemoryDir)
	llm := NewLLMClient(cfg, ctxManager, personality)
	tts := NewTTS(cfg)

	audio, err := NewAudioManager(cfg)
	if err != nil {
		return nil, err
	}

	buddy := &ChatBuddy{
		cfg:              cfg,
		ctxManager:       ctxManager,
		llm:              llm,
		audio:            audio,
		tts:              tts,
		brain:            NewBrain(),
		personality:      personality,
		lastActivityTime: time.Now(),
	}

	fmt.Println("✅ Miaou initialized!")
	fmt.Printf("   LLM API: %s\n", cfg.LLMURL)
	fmt.Printf("   Timeout: %ds\n", cfg.InactivityTimeout)
	if cfg.Debug {
		fmt.Println("   DEBUG: ON")
	}
	fmt.Println("\nPress CTRL+C to quit")

	return buddy, nil
}

// runConversationLoop owns the wake-word/listen/process cycle. It runs in
// its own goroutine so the ebiten render loop (Update/Draw) never blocks on
// I/O or the LLM/TTS calls — the two sides only share Brain, which is
// mutex-protected.
func (b *ChatBuddy) runConversationLoop() {
	fmt.Println("😴 Idle mode - waiting for 'miaou'...")

	for {
		if !b.isActive {
			b.brain.SetState(StateIdle)

			text, ok := b.audio.Listen()
			if !ok {
				return
			}
			if text == "" || !containsWakeWord(text, b.cfg.WakeWord) {
				continue
			}

			fmt.Printf("🐱 '%s' detected!\n", b.cfg.WakeWord)
			b.isActive = true
			b.lastActivityTime = time.Now()
			b.ctxManager.StartSession()

			remaining := extractAfterWakeWord(text, b.cfg.WakeWord)
			if remaining != "" {
				fmt.Printf("Processing: %s\n", remaining)
				b.processInput(remaining)
			}
			continue
		}

		if time.Since(b.lastActivityTime) > time.Duration(b.cfg.InactivityTimeout)*time.Second {
			fmt.Printf("💤 Timeout - Going to sleep...\n")
			b.ctxManager.EndSession()
			b.isActive = false
			continue
		}

		b.brain.SetState(StateListening)
		text, ok := b.audio.Listen()
		b.lastActivityTime = time.Now()
		if !ok {
			return
		}
		if text != "" {
			b.processInput(text)
		}
	}
}

func (b *ChatBuddy) processInput(userText string) {
	b.brain.SetState(StateProcessing)

	response := b.llm.Chat(userText)

	start := time.Now()
	b.tts.Speak(response, func() { b.brain.SetState(StateSpeaking) })
	duration := time.Since(start).Seconds()

	b.ctxManager.AddExchange(userText, response, duration)

	b.brain.SetState(StateListening)
}

func (b *ChatBuddy) shutdown() {
	if b.isActive {
		b.ctxManager.EndSession()
	}
	fmt.Println("✅ Goodbye! 👋")
}

func main() {
	buddy, err := NewChatBuddy()
	if err != nil {
		fmt.Printf("❌ Startup error: %v\n", err)
		os.Exit(1)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n\n⏹️  Shutting down...")
		buddy.shutdown()
		os.Exit(0)
	}()

	face, err := NewFace(buddy.cfg, buddy.brain)
	if err != nil {
		fmt.Printf("❌ Startup error: %v\n", err)
		os.Exit(1)
	}

	go buddy.runConversationLoop()

	ebiten.SetWindowSize(buddy.cfg.ScreenWidth, buddy.cfg.ScreenHeight)
	ebiten.SetWindowTitle("Miaou - English Buddy")
	ebiten.SetFullscreen(true)
	if err := ebiten.RunGame(face); err != nil {
		fmt.Printf("❌ Display error: %v\n", err)
	}

	buddy.shutdown()
}
