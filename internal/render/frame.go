package render

import (
	"image"
	"image/color"
	"image/draw"
	"sort"
	"strconv"
	"strings"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/render/text"
	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/timeline"
)

// composer builds one frame at a time.
//
// Most of a frame does not change between consecutive frames: the background,
// diagrams and character bodies hold still for seconds at a stretch. Those are
// composed into a cached static layer that is only rebuilt when the set of
// things on screen actually changes; per frame we copy it and draw the two
// things that do move — the mouth and the subtitle.
type composer struct {
	project *parser.Project
	tl      *timeline.Timeline
	assets  *assets
	face    *text.Face
	small   *text.Face
	size    int // subtitle font size in output pixels
	w, h    int

	static    *image.RGBA
	staticKey string

	subtitle    *image.RGBA
	subtitleKey string

	// mouths caches each scaled mouth overlay by asset path.
	mouths map[string]*image.RGBA
}

func (c *composer) frame(t int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.w, c.h))

	if t >= c.tl.DurationMS && c.tl.Credit != "" {
		c.drawCredit(img)
		return img
	}

	key := c.stateKey(t)
	if key != c.staticKey || c.static == nil {
		c.static = c.buildStatic(t)
		c.staticKey = key
	}
	copy(img.Pix, c.static.Pix)

	c.drawMouths(img, t)
	c.drawSubtitle(img, t)
	return img
}

// stateKey identifies everything the static layer depends on.
func (c *composer) stateKey(t int) string {
	var b strings.Builder
	for _, tr := range c.tl.Tracks {
		switch tr.Type {
		case timeline.TrackBackground, timeline.TrackImage:
			for _, ev := range tr.Events {
				if active(ev, t) {
					b.WriteString(tr.Type)
					b.WriteString(ev.Src)
					// An image that is still fading in changes every frame,
					// so the layer must not be reused while that lasts.
					// Without this the layer built at alpha 0 was cached
					// and the image never appeared at all.
					if c.enterAlpha(ev, t) < 1 {
						b.WriteString(strconv.Itoa(t))
					}
					b.WriteByte('|')
				}
			}
		case timeline.TrackCharacter:
			if ev := find(tr.Events, t); ev != nil {
				b.WriteString(tr.ID)
				b.WriteString(ev.Expression)
				b.WriteByte('|')
			}
		}
	}
	return b.String()
}

func (c *composer) buildStatic(t int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
	full := img.Bounds()

	// A frame always has an opaque base, so a script with no background still
	// yields a watchable video rather than transparent (black) video.
	drawn := false
	for _, tr := range c.tl.Tracks {
		if tr.Type != timeline.TrackBackground {
			continue
		}
		if ev := find(tr.Events, t); ev != nil {
			if src := c.assets.load(ev.Src); src != nil {
				drawCover(img, src, full)
				// Blur is authored in pixels at the declared height, so it
				// looks the same in a draft as in the final render.
				treatBackdrop(img, ev.Dim, ev.Blur*float64(c.h)/float64(c.tl.Video.Height))
				drawn = true
			} else {
				draw.Draw(img, full, placeholderBackground(c.w, c.h, ev.Src, c.small), image.Point{}, draw.Src)
				drawn = true
			}
		}
	}
	if !drawn {
		draw.Draw(img, full, placeholderBackground(c.w, c.h, c.tl.Title, c.small), image.Point{}, draw.Src)
	}

	for _, tr := range c.tl.Tracks {
		if tr.Type != timeline.TrackImage {
			continue
		}
		for _, ev := range tr.Events {
			if !active(ev, t) {
				continue
			}
			r := c.scaleRect(ev.Rect)
			layer := image.NewRGBA(image.Rect(0, 0, c.w, c.h))
			if src := c.assets.load(ev.Src); src != nil {
				drawContain(layer, src, r)
			} else {
				draw.Draw(layer, r, placeholderImage(r.Dx(), r.Dy(), ev.Src, c.small), image.Point{}, draw.Over)
			}
			blend(img, layer, c.enterAlpha(ev, t))
		}
	}

	for _, tr := range c.tl.Tracks {
		if tr.Type != timeline.TrackCharacter {
			continue
		}
		ev := find(tr.Events, t)
		if ev == nil {
			continue
		}
		ch := c.project.Movie.Characters[tr.ID]
		if ch == nil {
			continue
		}
		c.drawCharacter(img, ch, ev.Expression, "")
	}
	return img
}

// drawCharacter draws one figure. When variant is set it is a lipsync overlay
// drawn with the same transform as the body, so mouth-only differential art and
// whole-body art both line up.
func (c *composer) drawCharacter(img *image.RGBA, ch *schema.Character, expr, variant string) {
	scale := ch.Stand.Scale
	if scale == 0 {
		scale = 1
	}
	ox := ch.Stand.Offset.X * c.w / c.tl.Video.Width
	oy := ch.Stand.Offset.Y * c.h / c.tl.Video.Height

	path := variant
	if path == "" {
		path = ch.Expressions[expr]
	}
	if src := c.assets.load(path); src != nil {
		drawBottomAnchored(img, src, ch.Stand.Position, scale, ox, oy)
		return
	}
	if variant != "" {
		return // no placeholder for a missing mouth overlay
	}
	ph := placeholderStand(c.w/3, c.h*2/3, ch.Name, expr, parseHex(ch.Bubble.Color), c.small)
	drawBottomAnchored(img, ph, ch.Stand.Position, scale, ox, oy)
}

// drawMouths overlays each character's mouth.
//
// The mouth is always drawn, not only while speaking: character art packs
// separate the mouth from the face precisely so it can be swapped, which means
// the expression image has no mouth of its own. Drawing it only during speech
// would leave the figure mouthless the rest of the time.
func (c *composer) drawMouths(img *image.RGBA, t int) {
	for _, tr := range c.tl.Tracks {
		if tr.Type != timeline.TrackCharacter {
			continue
		}
		ev := find(tr.Events, t)
		if ev == nil {
			continue
		}
		ch := c.project.Movie.Characters[tr.ID]
		if ch == nil || ch.Lipsync == nil {
			continue
		}
		path := ch.Lipsync.Close
		if ev.Speaking {
			rel := t - ev.StartMS
			for _, s := range ev.Mouth {
				if rel >= s.StartMS && rel < s.EndMS {
					if s.Open {
						path = ch.Lipsync.Open
					}
					break
				}
			}
		}
		c.drawMouth(img, ch, path)
	}
}

// drawMouth draws one mouth overlay, reusing the scaled result. Rescaling a
// full-canvas overlay every frame would dominate the render; there are only two
// mouth images per character, so caching them outright is enough.
func (c *composer) drawMouth(img *image.RGBA, ch *schema.Character, path string) {
	if path == "" {
		return
	}
	layer, ok := c.mouths[path]
	if !ok {
		src := c.assets.load(path)
		if src == nil {
			c.mouths[path] = nil
			return
		}
		layer = image.NewRGBA(image.Rect(0, 0, c.w, c.h))
		scale := ch.Stand.Scale
		if scale == 0 {
			scale = 1
		}
		drawBottomAnchored(layer, src, ch.Stand.Position, scale,
			ch.Stand.Offset.X*c.w/c.tl.Video.Width,
			ch.Stand.Offset.Y*c.h/c.tl.Video.Height)
		c.mouths[path] = layer
	}
	if layer != nil {
		draw.Draw(img, img.Bounds(), layer, image.Point{}, draw.Over)
	}
}

func (c *composer) drawSubtitle(img *image.RGBA, t int) {
	if c.project.Movie.Subtitle.Mode == "none" {
		return
	}
	var ev *timeline.Event
	for _, tr := range c.tl.Tracks {
		if tr.Type == timeline.TrackSubtitle {
			ev = find(tr.Events, t)
		}
	}
	if ev == nil {
		return
	}
	key := ev.Speaker + "\x00" + ev.Text
	if key != c.subtitleKey || c.subtitle == nil {
		c.subtitle = c.buildSubtitle(ev)
		c.subtitleKey = key
	}
	draw.Draw(img, img.Bounds(), c.subtitle, image.Point{}, draw.Over)
}

// buildSubtitle lays out the bubble around the wrapped text, so the box grows
// with the line rather than the author having to guess where to break.
func (c *composer) buildSubtitle(ev *timeline.Event) *image.RGBA {
	layer := image.NewRGBA(image.Rect(0, 0, c.w, c.h))

	pad := c.h / 40
	maxTextW := c.w*55/100 - 2*pad
	lines := c.face.Wrap(ev.Text, maxTextW)
	if len(lines) == 0 {
		return layer
	}

	textW := 0
	for _, l := range lines {
		textW = max(textW, c.face.Width(l))
	}
	boxW := textW + 2*pad
	// Size the box by the glyphs, not by the line advance, so the padding
	// above and below the text is the same (see text.Face.BlockHeight).
	boxH := c.face.BlockHeight(len(lines)) + 2*pad

	x := (c.w - boxW) / 2
	switch ev.Anchor {
	case "left":
		x = c.w / 12
	case "right":
		x = c.w - boxW - c.w/12
	}
	y := c.h - boxH - c.h/12

	if c.project.Movie.Subtitle.Mode == "telop" {
		c.drawLines(layer, lines, (c.w-textW)/2, y+pad)
		return layer
	}

	// No tail. The box already sits on the speaker's side and carries their
	// name above it, so a pointer is a third cue for something already said
	// twice — and it collided with the figure standing behind it.
	box := image.Rect(x, y, x+boxW, y+boxH)
	fill(layer, box, withAlpha(parseHex(ev.Bubble), 0xe6))

	if ev.Speaker != "" {
		c.small.Draw(layer, x+pad, y-pad/2, ev.Speaker, color.White, color.Black, 2)
	}
	c.drawLines(layer, lines, x+pad, y+pad)
	return layer
}

func (c *composer) drawLines(dst *image.RGBA, lines []string, x, y int) {
	lh := c.face.Height()
	stroke := c.project.Movie.Subtitle.Stroke
	sw := stroke.Width * c.h / c.tl.Video.Height
	if stroke.Width == 0 {
		// Derive the outline from the type size, and from how much help it has
		// to give. Dense kanji have internal gaps only a few pixels wide at
		// subtitle sizes, and an outline wider than half that gap closes them
		// up, leaving the glyph reading as a blob.
		//
		// A bubble already puts a solid colour behind the text, so the outline
		// only has to separate glyph from box and can stay thin. A telop sits
		// straight on the picture and has to survive whatever is behind it.
		if c.project.Movie.Subtitle.Mode == "telop" {
			sw = c.size / 20
		} else {
			sw = c.size / 48
		}
	}
	sw = max(sw, 1)
	fillCol := parseHex(c.project.Movie.Subtitle.Color)
	strokeCol := parseHex(stroke.Color)
	for i, l := range lines {
		c.face.Draw(dst, x, y+i*lh+c.face.Ascent(), l, fillCol, strokeCol, sw)
	}
}

func (c *composer) drawCredit(img *image.RGBA) {
	fill(img, img.Bounds(), color.RGBA{0x0d, 0x0f, 0x12, 0xff})
	lines := strings.Split(strings.TrimSpace(c.tl.Credit), "\n")
	lh := c.face.Height()
	y := (c.h - len(lines)*lh) / 2
	for i, l := range lines {
		l = strings.TrimSpace(l)
		w := c.face.Width(l)
		c.face.Draw(img, (c.w-w)/2, y+i*lh+c.face.Ascent(), l, color.White, nil, 0)
	}
}

// enterAlpha ramps an image in over a short fade when `enter: fade` was used.
func (c *composer) enterAlpha(ev timeline.Event, t int) float64 {
	const fadeMS = 300
	if ev.Enter != "fade" {
		return 1
	}
	d := t - ev.StartMS
	if d >= fadeMS {
		return 1
	}
	return float64(d) / fadeMS
}

// scaleRect maps a rect expressed in timeline coordinates onto the output size,
// which differs in draft renders.
func (c *composer) scaleRect(r *timeline.Rect) image.Rectangle {
	if r == nil {
		return image.Rect(0, 0, c.w, c.h)
	}
	sx := func(v int) int { return v * c.w / c.tl.Video.Width }
	sy := func(v int) int { return v * c.h / c.tl.Video.Height }
	return image.Rect(sx(r.X), sy(r.Y), sx(r.X+r.W), sy(r.Y+r.H))
}

func active(ev timeline.Event, t int) bool { return t >= ev.StartMS && t < ev.EndMS }

func find(evs []timeline.Event, t int) *timeline.Event {
	i := sort.Search(len(evs), func(i int) bool { return evs[i].EndMS > t })
	if i < len(evs) && active(evs[i], t) {
		return &evs[i]
	}
	return nil
}

// parseHex reads "#rrggbb"; anything unparseable becomes opaque white, which is
// visible rather than silently invisible.
func parseHex(s string) color.RGBA {
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.RGBA{0xff, 0xff, 0xff, 0xff}
	}
	var v [3]uint8
	for i := range 3 {
		hi, ok1 := hexVal(s[2*i])
		lo, ok2 := hexVal(s[2*i+1])
		if !ok1 || !ok2 {
			return color.RGBA{0xff, 0xff, 0xff, 0xff}
		}
		v[i] = hi<<4 | lo
	}
	return color.RGBA{v[0], v[1], v[2], 0xff}
}

func hexVal(b byte) (uint8, bool) {
	switch {
	case b >= '0' && b <= '9':
		return b - '0', true
	case b >= 'a' && b <= 'f':
		return b - 'a' + 10, true
	case b >= 'A' && b <= 'F':
		return b - 'A' + 10, true
	}
	return 0, false
}
