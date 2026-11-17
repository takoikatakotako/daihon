package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/timeline"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

// newOverlayComposer renders a 200x100 layout at w x h. The background is blue
// and dimmed; a green diagram sits under the overlay box; the overlay is a
// 20x10 logo whose left half is opaque red and right half fully transparent,
// pinned into a 40x40 box at (10, 10). Drawn with its aspect ratio kept, the
// logo is 40x20 at (10, 20).
func newOverlayComposer(t *testing.T, w, h int) *composer {
	tl := &timeline.Timeline{
		DurationMS: 2000,
		Tracks: []timeline.Track{
			{Type: timeline.TrackBackground, Events: []timeline.Event{
				{StartMS: 0, EndMS: 1000, Src: "blue.png", Dim: 0.5, Blur: 2},
				{StartMS: 1000, EndMS: 2000, Src: "white.png"},
			}},
			{Type: timeline.TrackImage, Events: []timeline.Event{
				{StartMS: 0, EndMS: 2000, Src: "green.png", Rect: &timeline.Rect{X: 0, Y: 0, W: 60, H: 60}},
			}},
		},
	}
	c := newTestComposer(t, tl)
	root := c.project.Root

	solid := func(name string, col color.RGBA) {
		img := image.NewRGBA(image.Rect(0, 0, 8, 8))
		fill(img, img.Bounds(), col)
		writePNG(t, filepath.Join(root, name), img)
	}
	solid("blue.png", color.RGBA{0, 0, 255, 255})
	solid("white.png", color.RGBA{255, 255, 255, 255})
	solid("green.png", color.RGBA{0, 255, 0, 255})

	logo := image.NewNRGBA(image.Rect(0, 0, 20, 10))
	for y := range 10 {
		for x := range 10 {
			logo.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
		}
	}
	writePNG(t, filepath.Join(root, "logo.png"), logo)

	c.project.Movie.Video.Overlays = []schema.Overlay{{Src: "logo.png", X: 10, Y: 10, Width: 40, Height: 40}}
	c.w, c.h = w, h
	return c
}

func TestOverlayFixedAcrossBackgrounds(t *testing.T) {
	for _, tc := range []struct {
		name string
		w, h int
	}{{"full", 200, 100}, {"draft", 100, 50}} {
		t.Run(tc.name, func(t *testing.T) {
			c := newOverlayComposer(t, tc.w, tc.h)
			at := func(img *image.RGBA, x, y int) color.RGBA {
				// Points are given in layout coordinates.
				return img.RGBAAt(x*tc.w/200, y*tc.h/100)
			}
			for _, ms := range []int{0, 1500} {
				img := c.frame(ms)
				// Opaque half: in front of the diagram, untouched by dim.
				if p := at(img, 18, 30); p.R != 255 || p.G != 0 || p.B != 0 {
					t.Errorf("t=%d: logo not drawn in front at full strength: %v", ms, p)
				}
				// Transparent half: the diagram shows through.
				if p := at(img, 42, 30); p.G < 200 || p.R > 40 {
					t.Errorf("t=%d: transparent part of the logo is not see-through: %v", ms, p)
				}
				// Above the logo inside the box: aspect ratio kept, so the
				// logo is not stretched up here.
				if p := at(img, 18, 14); p.R > 40 {
					t.Errorf("t=%d: logo stretched out of its aspect ratio: %v", ms, p)
				}
			}
		})
	}
}

// With no overlays the frame is exactly what it was before overlays existed.
func TestNoOverlayLeavesFrameUnchanged(t *testing.T) {
	c := newOverlayComposer(t, 200, 100)
	with := c.frame(0)
	c.project.Movie.Video.Overlays = nil
	c.overlays = nil
	without := c.frame(0)
	if p := without.RGBAAt(18, 30); p.R > 40 {
		t.Fatalf("logo drawn without overlays: %v", p)
	}
	if with.RGBAAt(100, 80) != without.RGBAAt(100, 80) {
		t.Fatalf("overlay changed pixels outside its box")
	}
}
