# 🖥️ Ollama Server Setup Guide

This guide is for setting up the **Ollama server** on your second Raspberry Pi (8GB RAM).

The client (Miaou chat buddy) will connect to this server.

---

## 🚀 Server Installation

### Prerequisites
- Raspberry Pi 3 or 4 with **8GB+ RAM**
- Wired internet (recommended for stability)
- Raspberry Pi OS (any version)

### Step 1: Install Ollama

```bash
# Download and install Ollama
curl https://ollama.ai/install.sh | sh

# Start the service
ollama serve
```

Keep this terminal open! The server is now running on `localhost:11434`

### Step 2: Make it Accessible from Network

To access from another RPi, you need to make Ollama listen on all interfaces:

```bash
# In a new terminal:
sudo nano /etc/systemd/system/ollama.service
```

Find the `[Service]` section and add:
```
Environment="OLLAMA_HOST=0.0.0.0:11434"
```

Full example:
```ini
[Unit]
Description=Ollama
After=network-online.target

[Service]
Type=simple
ExecStart=/usr/local/bin/ollama serve
Restart=always
RestartSec=5s

Environment="OLLAMA_HOST=0.0.0.0:11434"

[Install]
WantedBy=multi-user.target
```

Then restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart ollama
```

### Step 3: Download a Model

Choose a model based on your needs:

**Fast (6GB):**
```bash
ollama pull neural-chat:7b
```

**Balanced (7GB):**
```bash
ollama pull mistral:7b
```

**Best Quality (13GB - slower):**
```bash
ollama pull llama2:13b
```

**Ultra-light (1.6GB - good for 2GB client):**
```bash
ollama pull phi:2.5
```

This downloads the model and is ready to use.

### Step 4: Verify it's Working

```bash
# Check available models
curl http://localhost:11434/api/tags

# Test a simple query
curl http://localhost:11434/api/generate -d '{
  "model": "mistral:7b",
  "prompt": "Hello, how are you?",
  "stream": false
}'
```

---

## 🔍 Finding Your Server IP

From the **server** terminal:
```bash
hostname -I
```

Example output: `192.168.1.100 192.168.1.101`

Use this IP when configuring the client.

---

## ✅ Testing from Client

From the **client** RPi (Miaou):

```bash
# Replace with your actual server IP
curl http://192.168.1.100:11434/api/tags

# Should return: {"models": [...]}
```

If this works, your client can connect!

---

## 🎯 Model Selection Tips

| Model | Size | Speed | Quality | Memory |
|-------|------|-------|---------|--------|
| phi:2.5 | 1.6GB | Fast | Good | 2GB |
| neural-chat:7b | 6GB | Good | Good | 4GB |
| mistral:7b | 7GB | Good | Good | 4GB |
| llama2:13b | 13GB | Slow | Great | 6GB |

**Recommended for Miaou:** `mistral:7b` or `neural-chat:7b`

---

## 🔧 Troubleshooting Server

### Ollama not starting
```bash
# Check logs
journalctl -u ollama -n 20

# Try manual start
ollama serve
```

### Connection refused on port 11434
```bash
# Check if Ollama is running
ps aux | grep ollama

# Kill and restart
pkill -f ollama
sudo systemctl restart ollama
```

### Slow responses
- Check available disk space: `df -h`
- Check memory: `free -h`
- Close other apps
- Use smaller model

### Can't reach from client
```bash
# On server: Check listening ports
netstat -tlnp | grep 11434

# On client: Test ping
ping 192.168.1.100

# On client: Test port
curl -v http://192.168.1.100:11434/api/tags
```

---

## 🛠️ Useful Commands

### Start/stop Ollama
```bash
# Start
sudo systemctl start ollama

# Stop
sudo systemctl stop ollama

# Status
sudo systemctl status ollama

# Auto-start on boot
sudo systemctl enable ollama
```

### Model Management
```bash
# List models
ollama list

# Delete model (free space)
ollama rm mistral:7b

# Show model info
ollama show mistral:7b

# Pull specific version
ollama pull mistral:7b-q4_0  # Quantized version
```

### Monitor Server
```bash
# Watch logs in real-time
journalctl -u ollama -f

# Check memory usage
free -h

# Check GPU usage (if applicable)
nvidia-smi
```

---

## 📊 Performance Optimization

### For 8GB RAM Server

**Use this model:**
```bash
ollama pull mistral:7b
```

**Config for better speed:**
Create `~/.ollama/config.json`:
```json
{
  "num_gpu": -1,
  "thread": 4,
  "threads": 4
}
```

### If Server Gets Slow

```bash
# Free memory
sync && echo 3 > /proc/sys/vm/drop_caches

# Kill other processes
kill -9 $(pgrep -f "unrelated_app")

# Monitor in real-time
watch -n 1 free -h
```

---

## 🚀 Quick Setup Script

Combine all steps:

```bash
#!/bin/bash
curl https://ollama.ai/install.sh | sh
sudo systemctl restart ollama
ollama pull mistral:7b
curl http://localhost:11434/api/tags
echo "✅ Server ready!"
hostname -I | awk '{print "IP:", $1}'
```

Save as `setup_server.sh` and run:
```bash
bash setup_server.sh
```

---

## 🔗 Next Steps

Once your server is running:

1. Note your server IP address
2. Go to client setup in `install.sh`
3. Enter server IP when prompted
4. Start Miaou!

---

## 📞 Support

### Check everything is working

```bash
# On server
curl http://localhost:11434/api/tags

# On client (replace IP)
curl http://192.168.1.100:11434/api/tags
```

Both should show your installed models.

---

## 🎓 Learning Resources

- [Ollama Docs](https://ollama.ai)
- [Model Library](https://ollama.ai/library)
- [API Reference](https://github.com/ollama/ollama/blob/main/docs/api.md)
