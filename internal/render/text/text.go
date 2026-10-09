// Package text draws Japanese text onto an image.
//
// ffmpeg cannot help here: the drawtext and subtitles filters need libfreetype
// and libass, which common builds (Homebrew's included) leave out. Drawing in
// Go also means line breaking, stroking and bubble layout are ordinary code we
// can test, rather than a filter string we can only run.
package text

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"strings"

	budoux "github.com/soundkitchen/budoux/golang"
	xfont "golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Face is a loaded font at one size, plus the phrase segmenter used to break
// lines. Japanese has no spaces, so breaking at an arbitrary character looks
// wrong; BudouX splits at phrase boundaries the way a person would.
type Face struct {
	face   xfont.Face
	parser *budoux.Parser
	size   float64
}

// Load opens a .ttf, .otf or .ttc and returns a face at the given pixel size.
func Load(path string, size float64) (*Face, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parse(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	face, err := opentype.NewFace(f, &opentype.FaceOptions{
		Size: size, DPI: 72,
		// Hinting on. It was off while the outline was drawn at eight points on
		// a circle, because each pass snapped to a different pixel grid and
		// frayed the edge. Filling the whole disc removed that problem, and
		// unhinted stems are visibly soft on dense kanji, so it belongs back on.
		Hinting: xfont.HintingFull,
	})
	if err != nil {
		return nil, err
	}
	return &Face{face: face, parser: budoux.NewDefaultJapaneseParser(), size: size}, nil
}

// parse accepts both single fonts and collections (.ttc), taking the first font
// of a collection.
func parse(b []byte) (*sfnt.Font, error) {
	if f, err := opentype.Parse(b); err == nil {
		return f, nil
	}
	c, err := opentype.ParseCollection(b)
	if err != nil {
		return nil, err
	}
	return c.Font(0)
}

// Height is the line height in pixels.
//
// This is the font's recommended line advance, not Ascent+Descent: the latter
// is the extent of one line's glyphs and leaves no gap, so consecutive lines
// of Japanese text collide.
func (f *Face) Height() int {
	return f.face.Metrics().Height.Ceil()
}

// Ascent is the distance from the baseline to the top of the line.
func (f *Face) Ascent() int { return f.face.Metrics().Ascent.Ceil() }

// Descent is the distance from the baseline to the bottom of the line.
func (f *Face) Descent() int { return f.face.Metrics().Descent.Ceil() }

// BlockHeight is the vertical extent of n lines: the glyph extent of one line
// plus the line advance for each additional line.
//
// This is deliberately not n*Height(). Height() is the advance between
// baselines and carries the font's leading, which belongs between lines, not
// after the last one. Hiragino at 48px advances 72px for glyphs that reach
// 43px above and 6px below the baseline, so sizing a box by the advance left
// 23px of dead space under the text and none above it.
func (f *Face) BlockHeight(n int) int {
	if n <= 0 {
		return 0
	}
	return (n-1)*f.Height() + f.Ascent() + f.Descent()
}

// Width measures one line of text in pixels.
func (f *Face) Width(s string) int {
	return xfont.MeasureString(f.face, s).Ceil()
}

// Wrap breaks s into lines no wider than maxWidth, preferring phrase
// boundaries. A single phrase longer than maxWidth is split by character
// rather than being allowed to overflow.
func (f *Face) Wrap(s string, maxWidth int) []string {
	if s == "" {
		return nil
	}
	if maxWidth <= 0 {
		return []string{s}
	}

	var lines []string
	cur := ""
	flush := func() {
		if cur != "" {
			lines = append(lines, cur)
			cur = ""
		}
	}
	for _, phrase := range f.parser.Parse(s) {
		if cur != "" && f.Width(cur+phrase) > maxWidth {
			flush()
		}
		if f.Width(phrase) > maxWidth {
			flush()
			lines = append(lines, f.splitRunes(phrase, maxWidth)...)
			continue
		}
		cur += phrase
	}
	flush()

	// A phrase boundary next to a space ("Wallet の" / " App") leaves that
	// space at the edge of a line, where it reads as a misaligned indent.
	// Spaces only mean anything between words on the same line.
	out := lines[:0]
	for _, l := range lines {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// splitRunes is the fallback for a phrase that cannot fit on its own line.
func (f *Face) splitRunes(s string, maxWidth int) []string {
	var out []string
	cur := ""
	for _, r := range s {
		next := cur + string(r)
		if cur != "" && f.Width(next) > maxWidth {
			out = append(out, cur)
			cur = string(r)
			continue
		}
		cur = next
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

// strokeOffsets returns every integer offset within radius r.
//
// Drawing the glyphs at eight points on the circle leaves gaps between those
// points once r exceeds a pixel or two, which frays the outline. Filling the
// disc gives a solid edge; the subtitle layer is cached per line, so the extra
// passes cost nothing per frame.
func strokeOffsets(r int) [][2]int {
	var out [][2]int
	for dy := -r; dy <= r; dy++ {
		for dx := -r; dx <= r; dx++ {
			if dx == 0 && dy == 0 {
				continue
			}
			if dx*dx+dy*dy <= r*r {
				out = append(out, [2]int{dx, dy})
			}
		}
	}
	return out
}

// Draw renders one line with its baseline at (x, y). When strokeWidth is
// positive the text is outlined in strokeColor first.
func (f *Face) Draw(dst *image.RGBA, x, y int, s string, fill, strokeColor color.Color, strokeWidth int) {
	if s == "" {
		return
	}
	if strokeWidth > 0 && strokeColor != nil {
		for _, off := range strokeOffsets(strokeWidth) {
			f.drawAt(dst, x+off[0], y+off[1], s, strokeColor)
		}
	}
	f.drawAt(dst, x, y, s, fill)
}

func (f *Face) drawAt(dst *image.RGBA, x, y int, s string, c color.Color) {
	d := xfont.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: f.face,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

// Close releases the face.
func (f *Face) Close() error { return f.face.Close() }
