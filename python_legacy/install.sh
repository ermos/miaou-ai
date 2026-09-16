#!/bin/bash

# English Buddy - Installation Script

echo "================================"
echo "🐱 English Buddy - Installation"
echo "================================"
echo ""

# Check if running on Raspberry Pi
if ! grep -q "Raspberry" /proc/cpuinfo 2>/dev/null; then
    echo "⚠️  Warning: This doesn't look like a Raspberry Pi"
    echo "   (Ollama client should run on RPi 4 2GB+)"
    echo ""
fi

# Update system
echo "📦 Updating system packages..."
sudo apt-get update
sudo apt-get install -y python3 python3-pip

# Install system dependencies for audio
echo "🔊 Installing audio dependencies..."
sudo apt-get install -y \
    alsa-utils \
    python3-dev \
    portaudio19-dev \
    libatlas-base-dev \
    libjasper-dev \
    libtiff5 \
    libjasper1 \
    libharfbuzz0b \
    libwebp6 \
    libtiff5 \
    libjasper1 \
    libharfbuzz0b \
    libwebp6

# Install Python dependencies
echo "🐍 Installing Python packages..."
pip3 install -r requirements.txt --break-system-packages

# Download Vosk model
echo "🎤 Downloading Vosk model (this may take a minute)..."
python3 -c "from vosk import Model; m = Model('en-us'); print('✅ Vosk model ready')" 2>/dev/null || {
    echo "ℹ️  Note: Vosk model will be downloaded on first run"
}

# Create memory directory
echo "📝 Creating memory directory..."
mkdir -p memory

# Ask for server IP
echo ""
echo "🖥️  Server Configuration"
echo "========================"
read -p "Enter Ollama server IP (default: 192.168.1.100): " SERVER_IP
SERVER_IP=${SERVER_IP:-192.168.1.100}

# Update main.py with server IP
echo "⚙️  Configuring server IP..."
sed -i "s/SERVER_IP = .*/SERVER_IP = \"$SERVER_IP\"/" main.py

# Make main.py executable
chmod +x main.py

# Optional: Setup systemd service
echo ""
read -p "Setup auto-start service? (y/n): " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "📌 Creating systemd service..."
    sudo tee /etc/systemd/system/miaou.service > /dev/null << EOF
[Unit]
Description=Miaou - English Buddy
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=$USER
WorkingDirectory=$(pwd)
ExecStart=/usr/bin/python3 $(pwd)/main.py
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

    sudo systemctl daemon-reload
    sudo systemctl enable miaou.service
    echo "✅ Service installed!"
    echo "   Start: sudo systemctl start miaou"
    echo "   Stop:  sudo systemctl stop miaou"
    echo "   Logs:  sudo systemctl status miaou"
fi

echo ""
echo "================================"
echo "✅ Installation Complete!"
echo "================================"
echo ""
echo "🚀 To start Miaou:"
echo "   python3 main.py"
echo ""
echo "📝 To customize behavior:"
echo "   nano PERSONALITY.md"
echo ""
echo "📖 For help:"
echo "   cat README.md"
echo ""
