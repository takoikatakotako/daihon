package render

import (
	"image"
	"image/color"
	"image/draw"
	"path/filepath"

	"github.com/takoikatakotako/daihon/internal/render/text"
)

// Placeholders keep the promise from DESIGN.md §1.2: a project with no artwork
// still builds. Character art cannot ship with an MIT tool, so a script that
// references art nobody has must still produce a watchable video rather than an
// error. What is missing is drawn, labelled, so it is obvious on screen.

// placeholderStand draws a labelled silhouette for a character with no artwork.
func placeholderStand(w, h int, name, expr string, tint color.Color, face *text.Face) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))

	body := image.Rect(w/4, h/3, w*3/4, h)
	draw.Draw(img, body, image.NewUniform(withAlpha(tint, 0xcc)), image.Point{}, draw.Src)

	head := w / 4
	cx, cy := w/2, h/3-head/2
	drawCircle(img, cx, cy, head, withAlpha(tint, 0xcc))

	if face != nil {
		label := name
		if expr != "" && expr != "normal" {
			label = name + " / " + expr
		}
		tw := face.Width(label)
		face.Draw(img, (w-tw)/2, h/3+face.Height(), label, color.White, color.Black, 2)
	}
	return img
}

// placeholderBackground draws a grid carrying the scene ID, so it is clear
// which part of the script is on screen.
func placeholderBackground(w, h int, label string, face *text.Face) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0x20, 0x24, 0x2c, 0xff}), image.Point{}, draw.Src)

	line := color.RGBA{0x2c, 0x31, 0x3a, 0xff}
	step := max(w/24, 1)
	for x := 0; x < w; x += step {
		draw.Draw(img, image.Rect(x, 0, x+1, h), image.NewUniform(line), image.Point{}, draw.Src)
	}
	for y := 0; y < h; y += step {
		draw.Draw(img, image.Rect(0, y, w, y+1), image.NewUniform(line), image.Point{}, draw.Src)
	}
	if face != nil && label != "" {
		face.Draw(img, step/2, step, label, color.RGBA{0x6b, 0x74, 0x85, 0xff}, nil, 0)
	}
	return img
}

// placeholderImage draws an outlined box naming the file that is missing.
func placeholderImage(w, h int, path string, face *text.Face) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.RGBA{0x11, 0x14, 0x18, 0xaa}), image.Point{}, draw.Src)

	border := color.RGBA{0xd9, 0x77, 0x06, 0xff}
	const t = 4
	for _, r := range []image.Rectangle{
		image.Rect(0, 0, w, t), image.Rect(0, h-t, w, h),
		image.Rect(0, 0, t, h), image.Rect(w-t, 0, w, h),
	} {
		draw.Draw(img, r, image.NewUniform(border), image.Point{}, draw.Src)
	}
	if face != nil {
		label := filepath.Base(path)
		for i, l := range face.Wrap("missing: "+label, w-4*t) {
			face.Draw(img, 2*t, h/2+i*face.Height(), l, border, color.Black, 2)
		}
	}
	return img
}

func drawCircle(img *image.RGBA, cx, cy, r int, c color.Color) {
	src := image.NewUniform(c)
	for y := -r; y <= r; y++ {
		dx := isqrt(r*r - y*y)
		row := image.Rect(cx-dx, cy+y, cx+dx, cy+y+1)
		draw.Draw(img, row, src, image.Point{}, draw.Over)
	}
}

func isqrt(v int) int {
	if v <= 0 {
		return 0
	}
	x := v
	for i := 0; i < 20 && x*x > v; i++ {
		x = (x + v/x) / 2
	}
	return x
}

func withAlpha(c color.Color, a uint8) color.RGBA {
	r, g, b, _ := c.RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), a}
}
