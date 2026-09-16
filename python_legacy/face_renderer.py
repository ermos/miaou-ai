import pygame
from config import SCREEN_WIDTH, SCREEN_HEIGHT, ASSETS_DIR

FACE_IMAGES = {
    'sleeping': 'face_sleeping.jpg',
    'idle': 'face_idle.jpg',
    'idle_blink': 'face_idle_blink.jpg',
    'talking': 'face_talking.jpg',
    'talking_blink': 'face_talking_blink.jpg',
}


class PixelArtFace:
    def __init__(self, width=SCREEN_WIDTH, height=SCREEN_HEIGHT):
        self.width = width
        self.height = height
        has_display = pygame.display.get_surface() is not None
        self.images = {}
        for name, filename in FACE_IMAGES.items():
            img = pygame.image.load(str(ASSETS_DIR / filename))
            if has_display:
                img = img.convert()
            self.images[name] = pygame.transform.smoothscale(img, (width, height))

    def render_frame(self, eye_state='open', mouth_state='closed', pupil_dir='center'):
        """Choisit l'image du chat selon l'état et la retourne"""
        talking = mouth_state in ('open_1', 'open_2', 'open_3')
        blinking = eye_state == 'closed'

        if eye_state == 'sleeping':
            key = 'sleeping'
        elif talking:
            key = 'talking_blink' if blinking else 'talking'
        else:
            key = 'idle_blink' if blinking else 'idle'

        return self.images[key]
