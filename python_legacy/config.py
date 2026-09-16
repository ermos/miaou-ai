# English Buddy - Global Config

import os
from pathlib import Path
from dotenv import load_dotenv

# Charger .env
load_dotenv()

# === SERVERS ===
VOSK_SERVER_URL = os.getenv('VOSK_SERVER_URL', 'ws://localhost:2700')
OPENAI_API_KEY = os.getenv('OPENAI_API_KEY', '')
OPENAI_MODEL = os.getenv('OPENAI_MODEL', 'gpt-4o-mini')
LLM_URL = "https://api.openai.com/v1"

# === DISPLAY ===
SCREEN_WIDTH = int(os.getenv('SCREEN_WIDTH', '320'))
SCREEN_HEIGHT = int(os.getenv('SCREEN_HEIGHT', '240'))
FPS = int(os.getenv('FPS', '15'))

# === COLORS ===
COLORS = {
    'WHITE': (255, 255, 255),     # Fond blanc pur
    'BLACK': (0, 0, 0),           # Yeux contour, bouche, pupille
    'BLUE': (100, 150, 255),      # Iris bleu
    'PUPIL': (0, 0, 0),           # Pupille noir
    'HIGHLIGHT': (200, 220, 255), # Reflet bleu clair
}

# === EYE POSITIONS ===
EYE_LEFT_X = 80
EYE_LEFT_Y = 60
EYE_RIGHT_X = 190
EYE_RIGHT_Y = 60
EYE_WIDTH = 50
EYE_HEIGHT = 50
IRIS_SIZE = 30
PUPIL_SIZE = 8

# === MOUTH POSITIONS ===
MOUTH_X = 120
MOUTH_Y = 150
MOUTH_WIDTH = 80
MOUTH_HEIGHT = 30

# === AUDIO ===
SAMPLE_RATE = int(os.getenv('SAMPLE_RATE', '16000'))
FRAMES_PER_BUFFER = 4096
TTS_RATE = int(os.getenv('TTS_RATE', '175'))
TTS_VOLUME = float(os.getenv('TTS_VOLUME', '0.9'))
TTS_VOICE_ID = os.getenv('TTS_VOICE_ID', 'com.apple.voice.compact.en-US.Samantha')
AUDIO_MODE = os.getenv('AUDIO_MODE', 'vosk_server')  # vosk_server, text_input

# === LLM ===
LLM_TIMEOUT = int(os.getenv('LLM_TIMEOUT', '60'))
LLM_TEMPERATURE = float(os.getenv('LLM_TEMPERATURE', '0.7'))

# === WAKE WORD ===
WAKE_WORD = os.getenv('WAKE_WORD', 'miaou')
WAKE_WORD_SENSITIVITY = float(os.getenv('WAKE_WORD_SENSITIVITY', '0.8'))

# === TIMEOUTS ===
INACTIVITY_TIMEOUT = int(os.getenv('INACTIVITY_TIMEOUT', '300'))  # 5 minutes
LISTENING_TIMEOUT = int(os.getenv('LISTENING_TIMEOUT', '30'))    # 30 secondes

# === PATHS ===
PROJECT_ROOT = Path(__file__).parent
MEMORY_DIR = PROJECT_ROOT / os.getenv('MEMORY_DIR', 'memory')
PERSONALITY_FILE = PROJECT_ROOT / os.getenv('PERSONALITY_FILE', 'PERSONALITY.md')
ASSETS_DIR = PROJECT_ROOT / 'assets'

# Créer répertoires s'ils n'existent pas
MEMORY_DIR.mkdir(exist_ok=True)

# === STATES ===
class ChatState:
    IDLE = 'idle'
    LISTENING = 'listening'
    PROCESSING = 'processing'
    SPEAKING = 'speaking'
    THINKING = 'thinking'

# === ANIMATIONS ===
IDLE_CYCLE = 60  # frames
LISTENING_CYCLE = 30
SPEAK_CYCLE = 8
BLINK_FREQUENCY = 30  # frames between blinks

# === DEBUG ===
DEBUG = os.getenv('DEBUG', 'False').lower() == 'true'

if DEBUG:
    print("=" * 60)
    print("🐱 CONFIGURATION LOADED")
    print("=" * 60)
    print(f"Vosk Server: {VOSK_SERVER_URL}")
    print(f"OpenAI API: {LLM_URL}")
    print(f"Model: {OPENAI_MODEL}")
    print(f"Audio Mode: {AUDIO_MODE}")
    print(f"Wake Word: {WAKE_WORD}")
    print("=" * 60)
