import time
from config import WAKE_WORD, INACTIVITY_TIMEOUT, AUDIO_MODE


class WakeWordDetector:
    def __init__(self):
        self.is_active = False
        self.last_activity_time = time.time()
        self.activity_timeout = INACTIVITY_TIMEOUT
        self.wake_word = WAKE_WORD.lower()
        
        # Si text_input, on n'a pas besoin du modèle Vosk
        if AUDIO_MODE == 'text_input':
            print(f"✅ Wake word detector ready (text mode)")
        else:
            try:
                from vosk import Model
                self.model = Model("en-us")
                print(f"✅ Wake word detector ready (listening for '{self.wake_word}')")
            except Exception as e:
                print(f"❌ Error loading model: {e}")
                self.model = None
    
    def start_listening_for_wake_word(self):
        """
        Écoute en mode basse consommation pour wake word
        Bloque jusqu'à détection
        """
        if AUDIO_MODE == 'text_input':
            # Text mode: demander l'input
            print("😴 Idle mode - waiting for 'miaou'...")
            while True:
                text = input("🎤 You: ")
                if self.wake_word in text.lower():
                    print(f"🐱 '{self.wake_word}' detected!")
                    self.is_active = True
                    self.last_activity_time = time.time()
                    return True
        else:
            # Vosk mode: utiliser le modèle
            try:
                import pyaudio
                import json
                from vosk import KaldiRecognizer
                
                p = pyaudio.PyAudio()
                stream = p.open(
                    format=pyaudio.paFloat32,
                    channels=1,
                    rate=16000,
                    input=True,
                    frames_per_buffer=4096
                )
                
                rec = KaldiRecognizer(self.model, 16000)
                
                print("😴 Idle mode - waiting for 'miaou'...")
                
                while not self.is_active:
                    try:
                        data = stream.read(4096, exception_on_overflow=False)
                        
                        if rec.AcceptWaveform(data):
                            result = json.loads(rec.Result())
                            
                            if result.get('result'):
                                text = ' '.join([r['conf'] for r in result['result']])
                                print(f"Heard: {text}")
                                
                                if self.wake_word in text.lower():
                                    print(f"🐱 '{self.wake_word}' detected!")
                                    self.is_active = True
                                    self.last_activity_time = time.time()
                                    
                                    stream.stop_stream()
                                    stream.close()
                                    p.terminate()
                                    return True
                    
                    except Exception as e:
                        print(f"⚠️  Error: {e}")
                        continue
            
            except Exception as e:
                print(f"❌ Listening error: {e}")
                return False
    
    def check_timeout(self):
        """Vérifie si timeout (5 min sans activité)"""
        if not self.is_active:
            return False
        
        elapsed = time.time() - self.last_activity_time
        if elapsed > self.activity_timeout:
            print(f"💤 Timeout ({elapsed:.0f}s) - Going to sleep...")
            self.is_active = False
            return True
        
        return False
    
    def update_activity(self):
        """Met à jour le timestamp de dernière activité"""
        self.last_activity_time = time.time()
    
    def get_remaining_time(self):
        """Retourne le temps restant avant timeout"""
        elapsed = time.time() - self.last_activity_time
        remaining = max(0, self.activity_timeout - elapsed)
        return remaining
