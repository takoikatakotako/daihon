package render

import (
	"image"
	"image/color"
	"testing"
)

func TestTreatBackdropDimScalesTowardBlack(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	fill(img, img.Bounds(), color.RGBA{200, 100, 50, 255})
	treatBackdrop(img, 0.5, 0)
	got := img.RGBAAt(1, 1)
	if got.R != 100 || got.G != 50 || got.B != 25 || got.A != 255 {
		t.Errorf("dim 0.5 of (200,100,50) = %v, want (100,50,25,255)", got)
	}
}

func TestTreatBackdropBlurSpreadsAnEdge(t *testing.T) {
	// Left half white, right half black. After blurring, the pixels next to
	// the seam should be grey, and the far edges untouched.
	img := image.NewRGBA(image.Rect(0, 0, 64, 8))
	fill(img, image.Rect(0, 0, 32, 8), color.White)
	fill(img, image.Rect(32, 0, 64, 8), color.Black)
	treatBackdrop(img, 0, 4)

	if l, r := img.RGBAAt(31, 4).R, img.RGBAAt(32, 4).R; l < 100 || l > 220 || r < 35 || r > 155 {
		t.Errorf("seam not softened: left=%d right=%d", l, r)
	}
	if e := img.RGBAAt(2, 4).R; e != 255 {
		t.Errorf("far left edge changed: %d", e)
	}
	if e := img.RGBAAt(61, 4).R; e != 0 {
		t.Errorf("far right edge changed: %d", e)
	}
	if a := img.RGBAAt(32, 4).A; a != 255 {
		t.Errorf("alpha drifted: %d", a)
	}
}

func TestTreatBackdropNoopWhenZero(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	fill(img, img.Bounds(), color.RGBA{10, 20, 30, 255})
	treatBackdrop(img, 0, 0)
	if got := img.RGBAAt(3, 3); got != (color.RGBA{10, 20, 30, 255}) {
		t.Errorf("zero treatment altered pixels: %v", got)
	}
}
