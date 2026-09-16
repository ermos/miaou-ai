package main

import (
	"fmt"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"

	"github.com/hajimehoshi/ebiten/v2"
)

var faceImageFiles = map[string]string{
	"sleeping":      "face_sleeping.jpg",
	"idle":          "face_idle.jpg",
	"idle_blink":    "face_idle_blink.jpg",
	"talking":       "face_talking.jpg",
	"talking_blink": "face_talking_blink.jpg",
}

type Face struct {
	brain  *Brain
	images map[string]*ebiten.Image
	width  int
	height int
}

func NewFace(cfg *Config, brain *Brain) (*Face, error) {
	images := make(map[string]*ebiten.Image, len(faceImageFiles))
	for key, filename := range faceImageFiles {
		path := filepath.Join(cfg.AssetsDir, filename)
		f, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", path, err)
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		images[key] = ebiten.NewImageFromImage(img)
	}

	return &Face{
		brain:  brain,
		images: images,
		width:  cfg.ScreenWidth,
		height: cfg.ScreenHeight,
	}, nil
}

func (f *Face) Update() error {
	f.brain.Tick()
	return nil
}

func (f *Face) Draw(screen *ebiten.Image) {
	img := f.images[f.brain.FaceKey()]

	op := &ebiten.DrawImageOptions{}
	bounds := img.Bounds()
	sx := float64(f.width) / float64(bounds.Dx())
	sy := float64(f.height) / float64(bounds.Dy())
	op.GeoM.Scale(sx, sy)
	screen.DrawImage(img, op)
}

func (f *Face) Layout(_, _ int) (int, int) {
	return f.width, f.height
}
