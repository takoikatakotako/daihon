package render

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/render/text"
	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/timeline"
)

// newTestComposer builds a composer over a temp project with one solid red
// 40x40 image asset. The frame is 200x100 at 1:1 with the timeline.
func newTestComposer(t *testing.T, tl *timeline.Timeline) *composer {
	t.Helper()
	fontPath := text.Fallback()
	if fontPath == "" {
		t.Skip("日本語フォントが見つからないためスキップ")
	}
	face, err := text.Load(fontPath, 12)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { face.Close() })

	root := t.TempDir()
	red := image.NewRGBA(image.Rect(0, 0, 40, 40))
	fill(red, red.Bounds(), color.RGBA{255, 0, 0, 255})
	f, err := os.Create(filepath.Join(root, "red.png"))
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(f, red); err != nil {
		t.Fatal(err)
	}
	f.Close()

	m := &schema.Movie{Version: 1, Video: schema.Video{Width: 200, Height: 100, FPS: 30}}
	m.ApplyDefaults()
	m.Video.Width, m.Video.Height = 200, 100
	tl.Video = timeline.Video{Width: 200, Height: 100, FPS: 30}
	p := &parser.Project{Root: root, File: "movie.yml", Movie: m}
	return &composer{
		project: p, tl: tl,
		assets: newAssets(p, face),
		face:   face, small: face, size: 12,
		mouths: map[string]*image.RGBA{},
		w:      200, h: 100,
	}
}

// A `show` with `enter: fade` must end up fully visible. The static layer is
// cached by what is on screen, and the first frame of a fade is at alpha 0;
// reusing that layer for the whole event made faded-in images vanish.
func TestFadedImageBecomesVisible(t *testing.T) {
	tl := &timeline.Timeline{
		DurationMS: 2000,
		Tracks: []timeline.Track{{
			Type: timeline.TrackImage,
			Events: []timeline.Event{{
				StartMS: 0, EndMS: 2000, Src: "red.png", Enter: "fade",
				Rect: &timeline.Rect{X: 80, Y: 30, W: 40, H: 40},
			}},
		}},
	}
	c := newTestComposer(t, tl)

	redness := func(t int) uint8 {
		img := c.frame(t)
		px := img.RGBAAt(100, 50)
		return px.R
	}
	// Frames are requested in order, as the renderer does, so the cache is
	// exercised the same way.
	first := redness(0)
	mid := redness(150)
	full := redness(1000)
	if first > 40 && first == full {
		t.Fatalf("no fade at all: first=%d full=%d", first, full)
	}
	if !(mid > first && mid < full) {
		t.Errorf("fade not progressing: t=0 %d, t=150 %d, t=1000 %d", first, mid, full)
	}
	if full < 240 {
		t.Errorf("image never became opaque: red at t=1000 is %d", full)
	}
}
