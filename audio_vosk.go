package main

// ponytail: this file is implemented against the same protocol as the
// original Python vosk_server client (raw PCM over a websocket to a Vosk
// server, no local libvosk needed), but it could not be exercised in the
// sandbox this was written in — there was no microphone or running Vosk
// server available. Test on real hardware before relying on it.

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gen2brain/malgo"
	"github.com/gorilla/websocket"
)

type VoskAudio struct {
	serverURL  string
	sampleRate int
	timeout    time.Duration

	malgoCtx *malgo.AllocatedContext
}

func NewVoskAudio(cfg *Config) (*VoskAudio, error) {
	ctx, err := malgo.InitContext(nil, malgo.ContextConfig{}, func(string) {})
	if err != nil {
		return nil, fmt.Errorf("malgo init: %w", err)
	}

	fmt.Printf("✅ Audio Manager ready (Vosk Server: %s)\n", cfg.VoskServerURL)
	return &VoskAudio{
		serverURL:  cfg.VoskServerURL,
		sampleRate: cfg.SampleRate,
		timeout:    time.Duration(cfg.ListeningTimeout) * time.Second,
		malgoCtx:   ctx,
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

func (v *VoskAudio) recordAudio() ([]byte, error) {
	deviceConfig := malgo.DefaultDeviceConfig(malgo.Capture)
	deviceConfig.Capture.Format = malgo.FormatS16
	deviceConfig.Capture.Channels = 1
	deviceConfig.SampleRate = uint32(v.sampleRate)

	var captured []byte
	callbacks := malgo.DeviceCallbacks{
		Data: func(_, pSample []byte, _ uint32) {
			captured = append(captured, pSample...)
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
	time.Sleep(v.timeout)
	if err := device.Stop(); err != nil {
		return nil, fmt.Errorf("stop capture: %w", err)
	}

	return captured, nil
}

func (v *VoskAudio) transcribe(audio []byte) (string, error) {
	conn, _, err := websocket.DefaultDialer.Dial(v.serverURL, nil)
	if err != nil {
		return "", fmt.Errorf("connect to vosk server: %w", err)
	}
	defer conn.Close()

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
	case <-time.After(5 * time.Second):
		return "", nil
	}
}
