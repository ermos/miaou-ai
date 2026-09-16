#!/usr/bin/env python3

import pygame
import re
import threading
import time
import sys

from config import (
    SCREEN_WIDTH, SCREEN_HEIGHT, FPS, ChatState,
    INACTIVITY_TIMEOUT, DEBUG, LLM_URL, AUDIO_MODE, WAKE_WORD
)
from buddy_brain import BuddyBrain
from face_renderer import PixelArtFace
from audio_handler import AudioManager
from llm_client import ContextualLLMClient
from context_manager import ContextManager
from wake_word_detector import WakeWordDetector


class ChatBuddy:
    def __init__(self):
        print("\n" + "="*50)
        print("🐱 English Buddy - Starting up...")
        print("="*50 + "\n")
        
        self.wake_detector = WakeWordDetector()
        self.context_manager = ContextManager()
        self.llm_client = ContextualLLMClient(self.context_manager)
        
        self.audio = AudioManager()
        self.brain = BuddyBrain()
        self.renderer = PixelArtFace(SCREEN_WIDTH, SCREEN_HEIGHT)
        
        self.is_active = False
        self.last_activity_time = time.time()
        self.should_quit = False
        
        print("✅ Miaou initialized!")
        print(f"   LLM API: {LLM_URL}")
        print(f"   Timeout: {INACTIVITY_TIMEOUT}s")
        if DEBUG:
            print("   DEBUG: ON")
        print("\nPress CTRL+C to quit\n")
    
    def run(self):
        """Main loop"""
        pygame.init()
        screen = pygame.display.set_mode((SCREEN_WIDTH, SCREEN_HEIGHT))
        pygame.display.set_caption("Miaou - English Buddy")
        clock = pygame.time.Clock()
        
        try:
            while not self.should_quit:
                clock.tick(FPS)
                
                # Event handling
                for event in pygame.event.get():
                    if event.type == pygame.QUIT:
                        self.should_quit = True
                    
                    if event.type == pygame.KEYDOWN:
                        if event.key == pygame.K_ESCAPE:
                            self.should_quit = True
                
                # Mode IDLE: En attente du wake word
                if not self.is_active:
                    self.brain.update_state(ChatState.IDLE)
                    
                    # Lancer écoute dans un thread
                    listening_thread = threading.Thread(
                        target=self._listen_for_wake_word,
                        daemon=True
                    )
                    listening_thread.start()
                    
                    # Attendre que le thread trouve le wake word
                    while listening_thread.is_alive() and not self.should_quit:
                        self._render_frame(screen)
                        pygame.display.flip()
                        clock.tick(FPS)
                
                # Mode ACTIVE: Chat normal
                else:
                    # Checker timeout
                    if self.wake_detector.check_timeout():
                        self.context_manager.end_session()
                        self.is_active = False
                        continue
                    
                    # Écouter l'utilisateur (dans un thread pour garder l'animation vivante)
                    self.brain.update_state(ChatState.LISTENING)

                    listen_result = {}
                    listen_thread = threading.Thread(
                        target=lambda: listen_result.update(text=self.audio.listen()),
                        daemon=True
                    )
                    listen_thread.start()

                    while listen_thread.is_alive() and not self.should_quit:
                        self._render_frame(screen)
                        pygame.display.flip()
                        clock.tick(FPS)

                    user_text = listen_result.get('text')
                    self.last_activity_time = time.time()
                    self.wake_detector.update_activity()
                    
                    if user_text and user_text.strip():
                        # Traiter dans un thread pour garder l'animation (parole) et les events vivants
                        process_thread = threading.Thread(
                            target=self._process_input,
                            args=(user_text,),
                            daemon=True
                        )
                        process_thread.start()

                        while process_thread.is_alive() and not self.should_quit:
                            self._render_frame(screen)
                            pygame.display.flip()
                            clock.tick(FPS)
                
                # Render frame
                self._render_frame(screen)
                pygame.display.flip()
        
        except KeyboardInterrupt:
            print("\n\n⏹️  Shutting down...")
        
        finally:
            # Cleanup
            if self.is_active:
                self.context_manager.end_session()
            
            pygame.quit()
            print("✅ Goodbye! 👋")
    
    def _listen_for_wake_word(self):
        """Thread: Écoute wake word et capture texte après"""
        if AUDIO_MODE == 'text_input':
            print("😴 Idle mode - waiting for 'miaou'...")
            while True:
                text = input("🎤 You: ").strip()
                if not text:
                    continue
                
                if WAKE_WORD in text.lower():
                    print(f"🐱 '{WAKE_WORD}' detected!")
                    self.is_active = True
                    self.context_manager.start_session()
                    
                    # Extraire le reste du message (avant et/ou après le wake word)
                    remaining = re.sub(re.escape(WAKE_WORD), '', text, count=1, flags=re.IGNORECASE).strip()
                    
                    # Si y a du texte après, le traiter comme première input
                    if remaining:
                        print(f"Processing: {remaining}")
                        self._process_input(remaining)
                    
                    break
        else:
            if self.wake_detector.start_listening_for_wake_word():
                self.is_active = True
                self.context_manager.start_session()
    
    def _process_input(self, user_text):
        """Traite un input utilisateur"""
        self.brain.update_state(ChatState.PROCESSING)
        self._render_frame_no_display()
        
        # Appeler LLM
        response = self.llm_client.chat(user_text)
        
        # Parler
        self.brain.update_state(ChatState.SPEAKING)
        
        duration_start = time.time()
        self.audio.speak(response)
        duration = time.time() - duration_start
        
        # Sauvegarder
        self.context_manager.add_exchange(user_text, response, duration)
        
        # Retour listening
        self.brain.update_state(ChatState.LISTENING)
    
    def _render_frame(self, screen):
        """Rend une frame"""
        try:
            # Get animation state
            eye_state, mouth_state, pupil_dir = self.brain.get_animation()
            
            # Render face
            face_surface = self.renderer.render_frame(eye_state, mouth_state, pupil_dir)
            screen.blit(face_surface, (0, 0))
            
            # Update brain
            self.brain.tick()
        except Exception as e:
            print(f"Rendering error: {e}")
    
    def _render_frame_no_display(self):
        """Rend une frame sans afficher (pour threading)"""
        try:
            eye_state, mouth_state, pupil_dir = self.brain.get_animation()
            self.renderer.render_frame(eye_state, mouth_state, pupil_dir)
            self.brain.tick()
        except Exception as e:
            print(f"Rendering error: {e}")


def main():
    # Start app
    buddy = ChatBuddy()
    buddy.run()


if __name__ == "__main__":
    main()
