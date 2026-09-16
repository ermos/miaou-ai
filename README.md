# 🐱 English Buddy - Miaou

A cute English learning chat buddy written in Go, powered by the OpenAI API and featuring:
- Wake word detection ("miaou")
- Context memory (7 days)
- Editable personality config
- Animated cat face (image-based)
- French→English learning assistance
- Pronunciation help
- Local, realistic text-to-speech via Piper (no cloud TTS)

> This project was originally written in Python; that version is kept for reference in `python_legacy/` but is no longer maintained.

---

## 📋 Requirements

### Software
- Go 1.22+
- Docker (only for cross-compiling to the Raspberry Pi via `make package`, see below)
- macOS or Linux (audio playback auto-picks `afplay`/`aplay`)
- An OpenAI API key

### Hardware (for a physical device build)
- Raspberry Pi 4 (2GB+ RAM)
- USB microphone (for `AUDIO_MODE=whisper`)
- Mini 3.5" display (320x240 SPI)
- USB speakers

---

## 🚀 Installation

### 1. Set up the Piper TTS engine (local, one-time)

Piper runs as a native binary (no Python) via `make fetch-piper`, which pulls
the Linux/arm64 build into `assets/piper/`:

```bash
make fetch-piper

# Download the voice model (female, US English, medium quality)
curl -sL "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx" -o assets/piper/en_US-amy-medium.onnx
curl -sL "https://huggingface.co/rhasspy/piper-voices/resolve/main/en/en_US/amy/medium/en_US-amy-medium.onnx.json" -o assets/piper/en_US-amy-medium.onnx.json
```

### 2. Configure

```bash
nano .env
# Set OPENAI_API_KEY, and AUDIO_MODE (text_input to test from the keyboard,
# whisper for a real microphone — sends audio to OpenAI's transcription
# API using the same key, no separate setup needed)
```

### 3. Build

```bash
go build -o miaou-ai .
```

Or cross-compile and package for the Pi from your dev machine with `make package` (see `Makefile`).

### 4. Auto-start on boot (kiosk mode, crash-resistant)

The app needs a graphical session (ebiten/GLFW, built against X11) — install a
minimal X server, no desktop environment required:

```bash
sudo apt install --no-install-recommends xserver-xorg xinit
```

Then let systemd own the whole X session: it starts on boot and restarts
automatically if the app (or X) crashes. Free up tty1 first (the service
takes it over), and replace `pi` below with your actual Linux username
(check with `whoami`) — a wrong `User=` fails with `status=217/USER`.

```bash
sudo systemctl disable --now getty@tty1.service
sudo nano /etc/systemd/system/miaou-ai.service
```

```ini
[Unit]
Description=Miaou AI - kiosk display
After=network-online.target getty@tty1.service
Wants=network-online.target
Conflicts=getty@tty1.service

[Service]
Type=simple
User=pi
# PAMName+TTYPath register a real logind session on this tty, which is what
# grants X permission to open the console (xf86OpenConsole) — a plain
# ExecStart never gets that permission, no matter the user.
PAMName=login
TTYPath=/dev/tty1
TTYReset=yes
TTYVHangup=yes
TTYVTDisallocate=yes
# AUDIO_MODE=text_input reads from stdin: with StandardInput=tty, a background
# read on this tty gets SIGTTIN and freezes the *whole* process silently (no
# crash, no log). Use whisper mode here, or set StandardInput=null to test
# text_input without a controlling terminal (no keyboard input either way).
StandardInput=tty
StandardOutput=journal
WorkingDirectory=/home/pi/miaou-ai
ExecStart=/usr/bin/xinit /home/pi/miaou-ai/miaou-ai -- :0 vt1 -nocursor
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now miaou-ai.service

# Check status / live logs
sudo systemctl status miaou-ai.service
journalctl -u miaou-ai.service -f
```

---

## 💬 Usage

### Start the Chat

```bash
./miaou-ai
```

The app will:
1. Connect to the OpenAI API
2. Show an animated cat face in a window
3. Wait for you to say (or type) "miaou"
4. Start a conversation in English

### Talking to Miaou

1. **Say "miaou"** to wake it up
2. **Speak in English** - if you speak French, Miaou will help
3. **Chat naturally** - ask questions, tell about your day
4. **Stop talking** - after 5 minutes of inactivity, Miaou goes to sleep

### What Miaou Does

✅ **Learns about you** - Asks about your day, friends, interests
✅ **Helps pronunciation** - Detects speech errors and helps gently
✅ **Encourages English** - Translates French to English
✅ **Remembers context** - Keeps 7 days of conversation history
✅ **Shows emotion** - Animated eyes and mouth react to mood

---

## 🎨 Customizing Personality

All behavior is defined in `PERSONALITY.md` - **no coding needed!**

### Edit Behavior

```bash
nano PERSONALITY.md
```

### Available Settings

- **Correction Level**: How strict with grammar mistakes
- **English Enforcement**: How much to push English
- **Question Frequency**: How often to ask engaging questions
- **Emoji Level**: How many emojis to use

### Example Modifications

**Make Miaou stricter:**
```markdown
CORRECTION_LEVEL: 60  # Was 30
```

**Add new question:**
```markdown
- "What's your favorite sport?"
```

**Change personality:**
Edit sections in PERSONALITY.md directly

**Restart to apply changes:**
```bash
./miaou-ai
```

---

## 📁 Project Structure

```
miaou-ai/
├── main.go                    # Start here! Wires everything together
├── config.go                  # Configuration constants (.env)
├── personality.go             # Load PERSONALITY.md
├── face.go                    # ebiten window + face rendering
├── brain.go                   # Animation state machine (blink + mouth timing)
├── audio.go / audio_whisper.go # Text input / mic + OpenAI transcription API
├── llm.go                     # OpenAI integration
├── context.go                 # Memory + sessions
├── wakeword.go                # "Miaou" detection/extraction
├── tts.go                     # Native Piper binary + afplay/aplay playback
├── assets/                    # Cat face images + Piper binary/voice model (make fetch-piper)
├── PERSONALITY.md             # ← Edit this! (no code)
├── memory/                    # Session storage (auto-created)
│   ├── 2026-09-13.json
│   ├── 2026-09-12.json
│   └── memory_condensed.txt
├── python_legacy/             # Original Python implementation (reference only)
└── README.md                  # This file
```

---

## 🧠 Memory System

Miaou remembers conversations:

### Daily Sessions
Each day's conversations saved in `memory/YYYY-MM-DD.json`

### 7-Day Memory
Last 7 days kept automatically (older sessions deleted)

### Condensed Memory
Compact summary (1000 chars) used for LLM context

### Ask About Memory
```
User: "What did we talk about yesterday?"
Miaou: [Retrieves and summarizes from memory]
```

---

## 🔧 Troubleshooting

### "Cannot connect to OpenAI"
- Check `OPENAI_API_KEY` in `.env`
- Test: `curl https://api.openai.com/v1/models -H "Authorization: Bearer $OPENAI_API_KEY"`

### No audio output / TTS errors
- Check speakers are connected, and check volume in `.env` (`TTS_VOLUME`, macOS only — on Linux, set the level with `alsamixer`)
- Test Piper directly: `echo "hello" | LD_LIBRARY_PATH=assets/piper assets/piper/piper -m assets/piper/en_US-amy-medium.onnx -f /tmp/test.wav --espeak_data assets/piper/espeak-ng-data && aplay /tmp/test.wav` (use `afplay` instead of `aplay` on macOS)

### Micro not working (whisper mode)
- Test: `arecord -d 3 test.wav` then `aplay test.wav`
- Check `OPENAI_API_KEY` in `.env` — transcription uses the same key as the LLM chat

### High latency/slow responses
- Check network: `ping api.openai.com`
- Try a smaller/faster model in `.env` (`OPENAI_MODEL=gpt-4o-mini`)

### Wake word not detected
- Speak clearly: "MIIIIAOOOUUU"
- Check microphone input: `arecord -d 3 test.wav`
- Try louder or closer to mic

---

## 📊 Performance Tips

### Reduce latency
- Reduce `LLM_TEMPERATURE` in `.env` (doesn't affect latency much, but keep it tuned)
- Try `OPENAI_MODEL=gpt-4o-mini` (faster/cheaper than larger models)
- Optimize network (wired connection if possible)

### Reduce RAM usage
- Close other apps on RPi
- Disable X11/GUI: run `sudo raspi-config`
- Use Lite version of Raspberry Pi OS

### Better responses
- Try a stronger `OPENAI_MODEL`
- Fine-tune PERSONALITY.md for your needs

---

## 🎯 Example Conversations

### Learning English
```
You: "Miaou"
Miaou: "Hello! How are you today? 😊"

You: "I'm happy, I played football"
Miaou: "That's amazing! ⚽ Did you have fun?
Tell me about the game! 🎮"

You: "We win three zero"
Miaou: "Great! We SAY 'We won three-zero' 🎉
Can you try: 'We won 3-0'?
Who scored the goals? 😊"
```

### Pronunciation Help
```
You: (bad accent) "I hab a khat"
Miaou: "Good try! 🐱
'Have' sounds like 'hav' - and 'cat' is 'kat'
So: 'I HAVE a CAT'
Can you try again? 😊"
```

### Following Up
```
You: "I like pizza"
Miaou: "Yum! Pizza is delicious! 🍕
What's your favorite topping?
Do you like cheese? Pepperoni?"
```

---

## 🐛 Debugging

### Enable debug logging
Set `DEBUG=True` in `.env`.

### Check logs
```bash
./miaou-ai > debug.log 2>&1
tail -f debug.log
```

### Run the test suite
```bash
go test ./...
```

---

## 📝 License

Free to use and modify!

---

## 🤝 Contributing

Found a bug? Have an idea? Edit and improve!

---

## 🙏 Thanks

Built with:
- OpenAI API (LLM + speech recognition)
- Ebiten (graphics)
- Piper (local text-to-speech)

---

## 🐱 Have fun learning English with Miaou!

Questions? Problems? Check PERSONALITY.md for tips on how to customize Miaou to your needs.

Happy chatting! 😊
