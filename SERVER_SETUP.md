# 🎙️ Vosk Speech Recognition Server Setup

This sets up the Vosk server that `AUDIO_MODE=vosk_server` talks to for
real microphone input. It can run on the same Raspberry Pi as Miaou, or on
a separate machine on your network.

> The official Vosk server Docker image (`alphacep/kaldi-en`) is **amd64-only**
> — it won't run natively on a Raspberry Pi. This guide installs the Python
> `vosk` package directly instead, which ships an `aarch64` wheel and needs
> no Docker/emulation.

---

## 🚀 Installation

### 1. Install Python deps

```bash
sudo apt install python3-pip python3-venv
python3 -m venv ~/vosk-server/venv
source ~/vosk-server/venv/bin/activate
pip install vosk websockets
```

### 2. Get the server script

This is Alphacep's reference websocket server (the protocol `audio_vosk.go`
speaks: raw PCM over a websocket, ending with an `{"eof" : 1}` message):

```bash
curl -o ~/vosk-server/asr_server.py \
  https://raw.githubusercontent.com/alphacep/vosk-server/master/websocket/asr_server.py
```

### 3. Download a model

The small English model is enough for this use case and light on RAM:

```bash
cd ~/vosk-server
curl -LO https://alphacephei.com/vosk/models/vosk-model-small-en-us-0.15.zip
unzip vosk-model-small-en-us-0.15.zip
mv vosk-model-small-en-us-0.15 model
rm vosk-model-small-en-us-0.15.zip
```

### 4. Run it

The client records at 16kHz (`SAMPLE_RATE` in `.env`) — the server must be
told to expect that, its own default is 8kHz:

```bash
cd ~/vosk-server
source venv/bin/activate
VOSK_SAMPLE_RATE=16000 python3 asr_server.py model
```

You should see `INFO:root:Listening on 0.0.0.0:2700`. Leave this running,
or set it up as a systemd service below.

---

## 🔁 Auto-start on boot (crash-resistant)

Replace `pi` below with your actual Linux username (check with `whoami`):

```bash
sudo nano /etc/systemd/system/vosk-server.service
```

```ini
[Unit]
Description=Vosk speech recognition server
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/vosk-server
Environment=VOSK_SAMPLE_RATE=16000
ExecStart=/home/pi/vosk-server/venv/bin/python3 asr_server.py model
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now vosk-server.service
journalctl -u vosk-server.service -f
```

---

## ✅ Point Miaou at it

In Miaou's `.env`:

```env
AUDIO_MODE=vosk_server
# Same Pi:
VOSK_SERVER_URL=ws://localhost:2700
# Separate machine:
VOSK_SERVER_URL=ws://<server-ip>:2700
```

---

## 🔧 Troubleshooting

### Server won't start / `ModuleNotFoundError`
Make sure you activated the venv before running it (`source venv/bin/activate`),
or use the venv's full python path (as the systemd unit above does).

### Miaou gets no transcription (always empty after ~5s)
- Check `VOSK_SAMPLE_RATE` on the server matches `SAMPLE_RATE` in Miaou's `.env` (both 16000).
- Check the server is reachable: `curl -v telnet://<server-ip>:2700` should connect (then hang, that's normal — it's a websocket, not HTTP).

### Find the server's IP (if running on a separate machine)
```bash
hostname -I
```

### Test the microphone independently
```bash
arecord -d 3 test.wav && aplay test.wav
```
