package main

import (
	"fmt"
	"math/rand"
	"sync"
	"time"
)

type ChatState int

const (
	StateIdle ChatState = iota
	StateListening
	StateProcessing
	StateSpeaking
)

func (s ChatState) String() string {
	switch s {
	case StateIdle:
		return "idle"
	case StateListening:
		return "listening"
	case StateProcessing:
		return "processing"
	case StateSpeaking:
		return "speaking"
	default:
		return "unknown"
	}
}

const blinkDuration = 150 * time.Millisecond

// ponytail: fixed flap rate, not audio-driven amplitude
const mouthFlapInterval = 120 * time.Millisecond

// Brain owns the chat state and the blink timer. It is read from the ebiten
// render loop (Update/Draw, main thread) and written from the goroutines
// handling listening/LLM/TTS, so every access goes through the mutex.
type Brain struct {
	mu sync.Mutex

	state       ChatState
	eyesClosed  bool
	nextBlinkAt time.Time
	blinkUntil  time.Time

	mouthOpen     bool
	nextMouthFlap time.Time
}

func NewBrain() *Brain {
	b := &Brain{state: StateIdle}
	b.scheduleNextBlink()
	return b
}

func (b *Brain) scheduleNextBlink() {
	// ponytail: real eyes blink every few seconds, not on a 50/50 duty cycle
	delay := time.Duration(2000+rand.Intn(2000)) * time.Millisecond
	b.nextBlinkAt = time.Now().Add(delay)
}

func (b *Brain) SetState(s ChatState) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.state != s {
		b.state = s
		fmt.Printf("🧠 State: %s\n", s)
	}
}

func (b *Brain) State() ChatState {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.state
}

// Tick advances the blink timer. Call it once per Update().
func (b *Brain) Tick() {
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.eyesClosed {
		if now.After(b.blinkUntil) {
			b.eyesClosed = false
			b.scheduleNextBlink()
		}
		return
	}
	if now.After(b.nextBlinkAt) {
		b.eyesClosed = true
		b.blinkUntil = now.Add(blinkDuration)
	}

	if b.state == StateSpeaking {
		if now.After(b.nextMouthFlap) {
			b.mouthOpen = !b.mouthOpen
			b.nextMouthFlap = now.Add(mouthFlapInterval)
		}
	} else {
		b.mouthOpen = false
	}
}

// FaceKey returns which of the 5 static face images to show.
func (b *Brain) FaceKey() string {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateIdle:
		return "sleeping"
	case StateSpeaking:
		if !b.mouthOpen {
			if b.eyesClosed {
				return "idle_blink"
			}
			return "idle"
		}
		if b.eyesClosed {
			return "talking_blink"
		}
		return "talking"
	default: // listening, processing
		if b.eyesClosed {
			return "idle_blink"
		}
		return "idle"
	}
}
