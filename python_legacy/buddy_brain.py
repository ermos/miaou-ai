from config import ChatState, BLINK_FREQUENCY


class BuddyBrain:
    def __init__(self):
        self.state = ChatState.IDLE
        self.frame_count = 0
        self.state_frame_count = 0
        self.blink_counter = 0
        self.eyes_closed = False
    
    def update_state(self, new_state):
        """Change l'état du chat"""
        if new_state != self.state:
            self.state = new_state
            self.state_frame_count = 0
            print(f"🧠 State: {new_state}")
    
    def get_animation(self):
        """Retourne eye_state, mouth_state, pupil_dir selon state"""
        if self.state == ChatState.IDLE:
            return self.idle_animation()
        elif self.state == ChatState.LISTENING:
            return self.listening_animation()
        elif self.state == ChatState.PROCESSING:
            return self.processing_animation()
        elif self.state == ChatState.SPEAKING:
            return self.speaking_animation()
        else:
            return 'open', 'closed', 'center'
    
    def idle_animation(self):
        """Animation sommeil: yeux fermés (endormi, en attente du wake word)"""
        return 'sleeping', 'sleeping', 'center'
    
    def listening_animation(self):
        """Animation écoute: yeux ouverts, clignement occasionnel"""
        eye_state = 'closed' if self.eyes_closed else 'open'
        mouth_state = 'closed'

        return eye_state, mouth_state, 'center'

    def processing_animation(self):
        """Animation réflexion: yeux ouverts, clignement occasionnel"""
        eye_state = 'closed' if self.eyes_closed else 'open'
        mouth_state = 'closed'

        return eye_state, mouth_state, 'center'

    def speaking_animation(self):
        """Animation parole: bouche ouverte, clignement occasionnel"""
        mouth_state = 'open_2'
        eye_state = 'closed' if self.eyes_closed else 'open'

        return eye_state, mouth_state, 'center'
    
    def tick(self):
        """Incrémente les frame counters"""
        self.frame_count += 1
        self.state_frame_count += 1
        
        # Clignement bref: yeux ouverts la plupart du temps, fermés 2 frames seulement
        self.blink_counter += 1
        if self.eyes_closed:
            if self.blink_counter > 2:
                self.eyes_closed = False
                self.blink_counter = 0
        elif self.blink_counter > BLINK_FREQUENCY:
            self.eyes_closed = True
            self.blink_counter = 0
