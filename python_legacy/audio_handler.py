import pyttsx3
import json
import re
from config import AUDIO_MODE, SAMPLE_RATE, TTS_RATE, TTS_VOLUME, TTS_VOICE_ID, LISTENING_TIMEOUT, VOSK_SERVER_URL

print(f"ℹ️  Audio mode: {AUDIO_MODE}")

_EMOJI_PATTERN = re.compile(
    "["
    "\U0001F300-\U0001FAFF"
    "\U00002600-\U000027BF"
    "\U0001F1E0-\U0001F1FF"
    "]+",
    flags=re.UNICODE
)


def _strip_emojis(text):
    """Retire les emojis avant envoi au TTS (bizarre à l'oral sinon)"""
    return _EMOJI_PATTERN.sub('', text).strip()

if AUDIO_MODE == 'vosk_server':
    import sounddevice as sd
    import numpy as np
    import websocket
    import threading
    
    class AudioManager:
        def __init__(self):
            self.tts = pyttsx3.init()
            self.tts.setProperty('rate', TTS_RATE)
            self.tts.setProperty('volume', TTS_VOLUME)
            self.tts.setProperty('voice', TTS_VOICE_ID)
            self.ws = None
            self.result = None
            print(f"✅ Audio Manager ready (Vosk Server: {VOSK_SERVER_URL})")
        
        def listen(self, timeout=LISTENING_TIMEOUT):
            """Listen using Vosk Server via WebSocket"""
            try:
                self.ws = websocket.WebSocketApp(
                    VOSK_SERVER_URL,
                    on_message=self._on_message,
                    on_error=self._on_error,
                    on_close=self._on_close
                )
                
                wst = threading.Thread(target=self.ws.run_forever)
                wst.daemon = True
                wst.start()
                
                print("🎤 Listening (speak now)...")
                audio_data = sd.rec(int(SAMPLE_RATE * timeout), samplerate=SAMPLE_RATE, channels=1, dtype=np.int16)
                sd.wait()
                
                self.ws.send(audio_data.tobytes())
                self.ws.send('{"eof" : 1}')
                
                import time
                time.sleep(1)
                self.ws.close()
                
                if self.result:
                    text = self.result
                    self.result = None
                    return text.strip()
                
                return None
            
            except Exception as e:
                print(f"❌ Vosk error: {e}")
                return None
        
        def _on_message(self, ws, message):
            """Handle Vosk response"""
            try:
                data = json.loads(message)
                if 'result' in data and data['result']:
                    text = ' '.join([r['conf'] for r in data['result']])
                    self.result = text
            except:
                pass
        
        def _on_error(self, ws, error):
            print(f"❌ WebSocket error: {error}")
        
        def _on_close(self, ws, close_status_code, close_msg):
            pass
        
        def speak(self, text):
            """Text to speech"""
            try:
                print(f"\n🤖 Miaou: {text}\n")
                self.tts.say(_strip_emojis(text))
                self.tts.runAndWait()
            except Exception as e:
                print(f"❌ TTS error: {e}")
        
        def is_french(self, text):
            """Detect French"""
            french_words = ['je', 'tu', 'il', 'elle', 'nous', 'vous', 'ils', 'elles',
                           'un', 'une', 'des', 'le', 'la', 'les']
            return sum(1 for w in french_words if w in text.lower()) >= 2
        
        def has_common_errors(self, text):
            """Detect grammar errors"""
            import re
            errors = [r'\bi\s+(?!am|have|\'m)', r'\bno\s+(?!one|doubt)']
            return any(re.search(pattern, text.lower()) for pattern in errors)

elif AUDIO_MODE == 'text_input':
    class AudioManager:
        def __init__(self):
            self.tts = pyttsx3.init()
            self.tts.setProperty('rate', TTS_RATE)
            self.tts.setProperty('volume', TTS_VOLUME)
            self.tts.setProperty('voice', TTS_VOICE_ID)
            print("✅ Audio Manager ready (Text input mode)")
        
        def listen(self, timeout=LISTENING_TIMEOUT):
            """Read text input"""
            try:
                text = input("\n🎤 You: ")
            except EOFError:
                return None
            return text.strip() if text.strip() else None
        
        def speak(self, text):
            """Text to speech"""
            try:
                print(f"\n🤖 Miaou: {text}\n")
                self.tts.say(_strip_emojis(text))
                self.tts.runAndWait()
            except Exception as e:
                print(f"❌ TTS error: {e}")
        
        def is_french(self, text):
            """Detect French"""
            french_words = ['je', 'tu', 'il', 'elle', 'nous', 'vous', 'ils', 'elles',
                           'un', 'une', 'des', 'le', 'la', 'les']
            return sum(1 for w in french_words if w in text.lower()) >= 2
        
        def has_common_errors(self, text):
            """Detect grammar errors"""
            import re
            errors = [r'\bi\s+(?!am|have|\'m)', r'\bno\s+(?!one|doubt)']
            return any(re.search(pattern, text.lower()) for pattern in errors)

else:
    raise ValueError(f"Unknown AUDIO_MODE: {AUDIO_MODE}. Use 'vosk_server' or 'text_input'")
