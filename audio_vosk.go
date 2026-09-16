package main

// ponytail: this file is implemented against the same protocol as the
// original Python vosk_server client (raw PCM over a websocket to a Vosk
// server, no local libvosk needed), but it could not be exercised in the
// sandbox this was written in — there was no microphone or running Vosk
// server available. Test on real hardware before relying on it.

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gorilla/websocket"
)

type VoskAudio struct {
	serverURL  string
	sampleRate int
	timeout    time.Duration

	silenceThreshold int32
	silenceHangover  time.Duration

	malgoCtx *malgo.AllocatedContext
}

func NewVoskAudio(cfg *Config) (*VoskAudio, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	fmt.Printf("✅ Audio Manager ready (Vosk Server: %s)\n", cfg.VoskServerURL)
	return &VoskAudio{
		serverURL:        cfg.VoskServerURL,
		sampleRate:       cfg.SampleRate,
		timeout:          time.Duration(cfg.ListeningTimeout) * time.Second,
		silenceThreshold: int32(cfg.VADSilenceThreshold),
		silenceHangover:  time.Duration(cfg.VADSilenceMs) * time.Millisecond,
		malgoCtx:         ctx,
	}, nil
}

type voskWord struct {
	Word string `json:"word"`
}

type voskResult struct {
	Result []voskWord `json:"result"`
	Text   string     `json:"text"`
}

func (v *VoskAudio) Listen() (string, bool) {
	audio, err := v.recordAudio()
	if err != nil {
		fmt.Printf("❌ Recording error: %v\n", err)
		// ponytail: a broken mic/ALSA config fails here instantly and
		// unconditionally on every call; without a pause the caller's loop
		// retries at full speed and pegs the CPU (seen: 300%+, starving
		// the render loop). A real capture starts by blocking on
		// device.Start() for the full listening timeout, so this only
		// slows down the broken-device case.
		time.Sleep(time.Second)
		return "", true
	}

	text, err := v.transcribe(audio)
	if err != nil {
		fmt.Printf("❌ Vosk error: %v\n", err)
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

func (v *VoskAudio) recordAudio() ([]byte, error) {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(v.sampleRate)

	var (
		mu          sync.Mutex
		captured    []byte
		lastVoiceAt = time.Now()
		voiceHeard  bool
	)

	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pSample []byte, _ uint32) {
			loud := peakAmplitude(pSample) > v.silenceThreshold

			mu.Lock()
			captured = append(captured, pSample...)
			if loud {
				lastVoiceAt = time.Now()
				voiceHeard = true
			}
			mu.Unlock()
		},
	}

	device, err := malgo.InitDevice(v.malgoCtx.Context, deviceConfig, callbacks)
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
	deadline := time.Now().Add(v.timeout)
	ticker := time.NewTicker(100 * time.Millisecond)
	for now := range ticker.C {
		mu.Lock()
		spoke, silentFor := voiceHeard, now.Sub(lastVoiceAt)
		mu.Unlock()

		if now.After(deadline) || (spoke && silentFor > v.silenceHangover) {
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

func (v *VoskAudio) transcribe(audio []byte) (string, error) {
	conn, _, err := websocket.DefaultDialer.Dial(v.serverURL, nil)
	if err != nil {
		return "", fmt.Errorf("connect to vosk server: %w", err)
	}
	defer func() {
		// A clean close handshake (vs. a bare conn.Close(), which just drops
		// the TCP connection) avoids "no close frame received" errors on the
		// server if it's still mid-response when we're done with it.
		_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		conn.Close()
	}()

	resultCh := make(chan string, 1)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var res voskResult
			if err := json.Unmarshal(msg, &res); err != nil {
				continue
			}
			if len(res.Result) > 0 {
				text := ""
				for i, w := range res.Result {
					if i > 0 {
						text += " "
					}
					text += w.Word
				}
				resultCh <- text
				return
			}
		}
	}()

	if err := conn.WriteMessage(websocket.BinaryMessage, audio); err != nil {
		return "", fmt.Errorf("send audio: %w", err)
	}
	// alphacep/vosk-server's reference asr_server.py checks this exact string
	// (with spaces around ':'), not parsed JSON — don't "clean up" the spacing.
	if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"eof" : 1}`)); err != nil {
		return "", fmt.Errorf("send eof: %w", err)
	}

	select {
	case text := <-resultCh:
		return text, nil
	case <-time.After(15 * time.Second):
		return "", nil
	}
}
