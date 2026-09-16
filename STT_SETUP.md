# 🎙️ Speech Recognition Setup (whisper.cpp)

This sets up the speech-to-text engine that `AUDIO_MODE=whisper` uses for
real microphone input: [whisper.cpp](https://github.com/ggml-org/whisper.cpp),
a native C++ binary, no Python, no separate server. It detects the spoken
language itself, so the same model handles English and French — no need to
run one server per language the way Vosk would.

---

## 🚀 Build it

whisper.cpp ships no prebuilt binaries, so this builds from source. Run it
**on the Raspberry Pi itself** (native compile — cross-compiling C++ for
arm64 from another machine via Docker/QEMU is impractically slow):

```bash
sudo apt install --no-install-recommends git cmake build-essential
cd ~/miaou-ai
make build-whisper
```

This clones whisper.cpp, builds `whisper-cli`, downloads the small
multilingual model (`tiny-q5_1`, quantized — ~30MB, light enough for a Pi's
RAM), and drops both into `assets/whisper/`. It skips the build entirely if
`assets/whisper/whisper-cli` already exists.

To use a different (larger, more accurate, heavier) model:

```bash
make build-whisper WHISPER_MODEL=base
```

Then set `WHISPER_MODEL_FILE=ggml-base.bin` in `.env` to match — Miaou reads
the model filename from there, it doesn't rebuild anything itself.

---

## ✅ Point Miaou at it

In `.env`:

```env
AUDIO_MODE=whisper
```

That's it — no server URL to configure, `WhisperBin`/`WhisperModel` in
`config.go` already point at `assets/whisper/`.

---

## 🔧 Troubleshooting

### `make build-whisper` fails to compile
Make sure `cmake` and a C++ compiler are installed (`build-essential` on
Debian/Raspberry Pi OS). Check `cmake --version` (3.x+ needed).

### Transcription is empty or clearly wrong
- Test the mic independently: `arecord -d 3 test.wav && aplay test.wav`.
- Try a bigger model (`base` or `small`) — `tiny` is fast but the least
  accurate, especially with background noise or a strong accent.
- Run whisper-cli directly on a known-good WAV to isolate the model from the
  mic/VAD pipeline: `assets/whisper/whisper-cli -m assets/whisper/ggml-tiny-q5_1.bin -f test.wav -l auto`

### Slow responses
The Pi's CPU is already busy with ebiten's software rendering (no GPU
acceleration with the `fbdev` X driver used for SPI displays) — a bigger
Whisper model adds real decode time on top of that. Stick to `tiny`/`base`
unless accuracy is a real problem.
