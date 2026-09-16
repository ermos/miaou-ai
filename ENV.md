# 🔧 Environment Variables Configuration

All configuration is managed through `.env` file. No need to edit code!

## 📝 .env File Location

```
miaou-ai/
├── .env              ← Configuration file (edit this!)
├── main.py
└── ...
```

## 🚀 Quick Setup

### 1. For macOS (local testing with Vosk Server)

```bash
# .env content
VOSK_SERVER_URL=ws://localhost:2700
OLLAMA_SERVER_IP=127.0.0.1
AUDIO_MODE=vosk_server
```

Then:
```bash
# Terminal 1: Start Vosk Server
docker run -d -p 2700:2700 alphacep/kaldi-en:latest

# Terminal 2: Start Ollama
ollama serve

# Terminal 3: Run Miaou
python3 main.py
```

### 2. For macOS (text input only, no mic)

```bash
# .env content
AUDIO_MODE=text_input
OLLAMA_SERVER_IP=127.0.0.1
```

Then:
```bash
# Terminal 1: Start Ollama
ollama serve

# Terminal 2: Run Miaou
python3 main.py
```

### 3. For Raspberry Pi (client + network servers)

```bash
# .env content
VOSK_SERVER_URL=ws://192.168.1.100:2700
OLLAMA_SERVER_IP=192.168.1.100
AUDIO_MODE=vosk_server
```

## 📋 All Variables

### Servers

```env
# Vosk Server (Speech Recognition) - WebSocket URL
VOSK_SERVER_URL=ws://localhost:2700

# Ollama Server (LLM) - IP and Port
OLLAMA_SERVER_IP=127.0.0.1
OLLAMA_SERVER_PORT=11434

# LLM Model to use
OLLAMA_MODEL=mistral:7b
# Options: mistral:7b, neural-chat:7b, llama2:13b, phi:2.5
```

### Audio

```env
# Audio input mode
AUDIO_MODE=vosk_server
# Options:
#  - vosk_server   : Use Vosk Server (Docker) - better quality
#  - text_input    : Use keyboard input - for testing

# Audio settings
SAMPLE_RATE=16000          # Hz
TTS_RATE=150              # Words per minute
TTS_VOLUME=0.9            # 0-1
LISTENING_TIMEOUT=30      # Seconds to listen
```

### Wake Word

```env
WAKE_WORD=miaou
WAKE_WORD_SENSITIVITY=0.8
INACTIVITY_TIMEOUT=300     # 5 minutes
```

### Display

```env
SCREEN_WIDTH=320
SCREEN_HEIGHT=240
FPS=15
```

### LLM

```env
LLM_TIMEOUT=60              # Seconds to wait for response
LLM_TEMPERATURE=0.7         # 0=deterministic, 1=creative
```

### Paths

```env
PERSONALITY_FILE=./PERSONALITY.md
MEMORY_DIR=./memory
```

### Debug

```env
DEBUG=False
# Set to True to see detailed logging
```

## 🎯 Common Configurations

### Production (RPi + Vosk Server)

```env
VOSK_SERVER_URL=ws://192.168.1.100:2700
OLLAMA_SERVER_IP=192.168.1.100
OLLAMA_SERVER_PORT=11434
OLLAMA_MODEL=mistral:7b
AUDIO_MODE=vosk_server
DEBUG=False
```

### Development (macOS with Vosk Server)

```env
VOSK_SERVER_URL=ws://localhost:2700
OLLAMA_SERVER_IP=127.0.0.1
AUDIO_MODE=vosk_server
DEBUG=True
```

### Testing (No servers needed)

```env
AUDIO_MODE=text_input
OLLAMA_SERVER_IP=127.0.0.1
DEBUG=True
```

### Fast Testing (Small model)

```env
OLLAMA_MODEL=phi:2.5  # Faster but less accurate
AUDIO_MODE=text_input
```

## 🔧 How to Change Settings

### Edit .env

```bash
nano .env
```

Change any values:

```env
OLLAMA_MODEL=neural-chat:7b    # Use different model
AUDIO_MODE=text_input          # Switch to text input
WAKE_WORD=hello                # Change wake word
```

Save (Ctrl+X, then Y)

### Restart Application

```bash
# Stop current instance (Ctrl+C)
# Restart
python3 main.py
```

Changes take effect immediately! ✅

## 📊 Model Comparison

| Model | Size | Speed | Quality | RAM | Best For |
|-------|------|-------|---------|-----|----------|
| phi:2.5 | 1.6GB | ⚡⚡⚡ | Good | 2GB | Testing |
| neural-chat:7b | 6GB | ⚡⚡ | Good | 4GB | Balanced |
| mistral:7b | 7GB | ⚡⚡ | Excellent | 4GB | **Default** |
| llama2:13b | 13GB | ⚡ | Excellent | 6GB | Best quality |

## 🧪 Troubleshooting

### "Cannot connect to Vosk Server"

Check:
```bash
# Is Docker running?
docker ps

# Is Vosk server started?
docker run -d -p 2700:2700 alphacep/kaldi-en:latest

# Test connection
curl http://localhost:2700/
```

### "Cannot connect to Ollama"

Check:
```bash
# Is Ollama running?
ollama serve

# Is server accessible?
curl http://127.0.0.1:11434/api/tags

# Test with IP address
curl http://192.168.1.YOUR_IP:11434/api/tags
```

### Text Input Not Working

Check `.env`:
```env
AUDIO_MODE=text_input
```

Restart:
```bash
python3 main.py
```

### Want to See Debug Logs

Edit `.env`:
```env
DEBUG=True
```

Restart to see detailed logging.

## 💡 Tips

1. **Keep one .env** - Don't create multiple config files
2. **Restart to apply** - Changes only take effect on restart
3. **Use text_input** - For quick testing without Vosk Server
4. **Start small** - Use `phi:2.5` model for testing first
5. **Check logs** - Enable DEBUG=True if something fails

## 📚 Related Files

- `config.py` - Loads .env and manages config
- `.env` - Your configuration (edit this!)
- `main.py` - Uses config from config.py
- `audio_handler.py` - Uses AUDIO_MODE from config
- `llm_client.py` - Uses OLLAMA_* from config
