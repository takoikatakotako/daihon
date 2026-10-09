package render

import (
	"image"
	"image/color"
	"image/draw"

	xdraw "golang.org/x/image/draw"
)

// drawCover scales src to fill dst entirely, cropping the overflow. Used for
// backgrounds, where letterboxing would look like a mistake.
func drawCover(dst *image.RGBA, src image.Image, r image.Rectangle) {
	sb := src.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	scale := max(
		float64(r.Dx())/float64(sb.Dx()),
		float64(r.Dy())/float64(sb.Dy()),
	)
	w, h := int(float64(sb.Dx())*scale), int(float64(sb.Dy())*scale)
	box := image.Rect(0, 0, w, h).Add(image.Pt(
		r.Min.X+(r.Dx()-w)/2,
		r.Min.Y+(r.Dy()-h)/2,
	))
	xdraw.CatmullRom.Scale(dst, box, src, sb, draw.Over, nil)
}

// drawContain scales src to fit inside r, preserving aspect ratio and centring
// it. Used for diagrams and character art, where cropping would lose content.
func drawContain(dst *image.RGBA, src image.Image, r image.Rectangle) {
	sb := src.Bounds()
	if sb.Empty() || r.Empty() {
		return
	}
	scale := min(
		float64(r.Dx())/float64(sb.Dx()),
		float64(r.Dy())/float64(sb.Dy()),
	)
	w, h := int(float64(sb.Dx())*scale), int(float64(sb.Dy())*scale)
	box := image.Rect(0, 0, w, h).Add(image.Pt(
		r.Min.X+(r.Dx()-w)/2,
		r.Min.Y+(r.Dy()-h)/2,
	))
	xdraw.CatmullRom.Scale(dst, box, src, sb, draw.Over, nil)
}

// drawBottomAnchored places src so that its base sits on the bottom edge of the
// frame, horizontally at pos. Character art is drawn this way so that figures
// of different heights still stand on the same floor.
func drawBottomAnchored(dst *image.RGBA, src image.Image, pos string, scale float64, offX, offY int) {
	sb := src.Bounds()
	if sb.Empty() {
		return
	}
	fw, fh := dst.Bounds().Dx(), dst.Bounds().Dy()

	h := int(float64(fh) * 0.85 * scale)
	w := int(float64(sb.Dx()) * float64(h) / float64(sb.Dy()))

	var x int
	switch pos {
	case "left":
		x = fw/4 - w/2
	case "center":
		x = fw/2 - w/2
	default: // right
		x = fw*3/4 - w/2
	}
	box := image.Rect(0, 0, w, h).Add(image.Pt(x+offX, fh-h+offY))
	xdraw.CatmullRom.Scale(dst, box, src, sb, draw.Over, nil)
}

// fill paints a solid rectangle.
func fill(dst *image.RGBA, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Over)
}

// blend draws src over dst at the given alpha, used for fade-ins.
func blend(dst *image.RGBA, src *image.RGBA, alpha float64) {
	if alpha >= 1 {
		draw.Draw(dst, dst.Bounds(), src, src.Bounds().Min, draw.Over)
		return
	}
	if alpha <= 0 {
		return
	}
	mask := image.NewUniform(color.Alpha{uint8(alpha * 255)})
	draw.DrawMask(dst, dst.Bounds(), src, src.Bounds().Min, mask, image.Point{}, draw.Over)
}
