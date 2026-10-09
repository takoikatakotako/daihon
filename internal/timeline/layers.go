package timeline

import (
	"fmt"
	"sort"

	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/wav"
)

// layer emits one contiguous run of events for a value that changes over time
// (the background). Consecutive identical values are collapsed.
type layer struct {
	cur    schema.Background
	start  int
	active bool
	events []Event
}

func newLayer() *layer { return &layer{} }

func (l *layer) set(t int, bg schema.Background) {
	if l.active && l.cur == bg {
		return
	}
	l.close(t)
	if bg.IsZero() {
		return
	}
	l.cur, l.start, l.active = bg, t, true
}

func (l *layer) close(t int) {
	if l.active && t > l.start {
		l.events = append(l.events, Event{StartMS: l.start, EndMS: t,
			Src: l.cur.Src, Dim: l.cur.Dim, Blur: l.cur.Blur})
	}
	l.active = false
}

// imageLayer tracks `show`/`hide` pairs. Handles are scene-scoped.
type imageLayer struct {
	w, h   int
	open   map[string]Event
	order  []string
	anon   int
	events []Event
}

func newImageLayer(w, h int) *imageLayer {
	return &imageLayer{w: w, h: h, open: map[string]Event{}}
}

// show opens an image. occupied lists the sides ("left"/"right") where a
// character is currently on stage, so a centred image can keep clear of them.
func (il *imageLayer) show(t int, it schema.ScriptItem, occupied map[string]bool) {
	name := it.As
	if name == "" {
		// An unnamed image cannot be hidden explicitly; it lives until the
		// scene ends. Give it a private handle so it can still be tracked.
		il.anon++
		name = fmt.Sprintf("\x00anon%d", il.anon)
	}
	if _, dup := il.open[name]; !dup {
		il.order = append(il.order, name)
	}
	il.open[name] = Event{
		StartMS: t,
		Src:     it.Src,
		Rect:    il.resolve(it.At, occupied),
		Enter:   it.Enter,
	}
}

func (il *imageLayer) hide(t int, name string) {
	ev, ok := il.open[name]
	if !ok {
		return // validator has already reported this
	}
	ev.EndMS = t
	il.events = append(il.events, ev)
	delete(il.open, name)
}

func (il *imageLayer) closeAll(t int) {
	names := make([]string, 0, len(il.open))
	for n := range il.open {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		il.hide(t, n)
	}
	il.order = nil
}

// resolve turns `at` into a box. The renderer fits the image inside it
// preserving aspect ratio, so image dimensions are not needed here.
//
// `center` keeps clear of the figures. A standing character covers roughly
// the outer 35% of the frame on its side, so with someone on both sides the
// centre box shrinks to the 30% gap between them; with one side empty it
// extends to that edge. Images are drawn behind characters, so a box that
// overlapped a figure would just be hidden by it. `left` and `right` are
// deliberately left alone: they are for when that side is empty (a character
// has exited), and the author is asking for the space.
func (il *imageLayer) resolve(at string, occupied map[string]bool) *Rect {
	w, h := il.w*3/5, il.h*3/5
	// Sit above the vertical centre. The subtitle bubble owns the bottom of
	// the frame, and a tall image centred at 50% ran into it; at 12.5% from
	// the top a full-height image ends at 72.5%, clear of a two-line bubble.
	y := il.h / 8
	switch at {
	case "left":
		return &Rect{X: il.w / 20, Y: y, W: w, H: h}
	case "right":
		return &Rect{X: il.w - w - il.w/20, Y: y, W: w, H: h}
	default: // center
		x0, x1 := (il.w-w)/2, (il.w+w)/2
		if occupied["left"] {
			x0 = max(x0, il.w*35/100)
		}
		if occupied["right"] {
			x1 = min(x1, il.w*65/100)
		}
		return &Rect{X: x0, Y: y, W: x1 - x0, H: h}
	}
}

// charLayer keeps every character covered by exactly one event at any instant,
// so the renderer never has to ask "what is this character doing right now?".
type charLayer struct {
	states map[string]*charState
	order  []string
}

type charState struct {
	expr      string
	speaking  bool
	mouth     []wav.MouthSpan
	hidden    bool // off stage: no events are emitted, so the renderer draws nothing
	voiceOnly bool // stand.position: none — hidden for good; enter and speech do not bring it on
	start     int
	events    []Event
}

// flush records the state held since start as an event ending at t. A hidden
// span produces no event: absence is how "not on stage" reaches the renderer,
// which already treats a gap in the track as nothing to draw.
func (s *charState) flush(t int) {
	if t > s.start && !s.hidden {
		s.events = append(s.events, Event{
			StartMS: s.start, EndMS: t,
			Expression: s.expr, Speaking: s.speaking, Mouth: s.mouth,
		})
	}
	s.start = t
}

func newCharLayer(cs map[string]*schema.Character) *charLayer {
	cl := &charLayer{states: map[string]*charState{}}
	for id := range cs {
		cl.order = append(cl.order, id)
	}
	sort.Strings(cl.order) // deterministic output
	for _, id := range cl.order {
		none := cs[id].Stand.Position == schema.StandNone
		cl.states[id] = &charState{expr: cs[id].DefaultExpression, hidden: none, voiceOnly: none}
	}
	return cl
}

func (cl *charLayer) transition(id string, t int, expr string, speaking bool, mouth []wav.MouthSpan) {
	s := cl.states[id]
	if s == nil {
		return
	}
	s.flush(t)
	s.expr, s.speaking, s.mouth = expr, speaking, mouth
}

// setVisible puts a character on or off stage at t. Visibility is a state like
// the expression: it holds across lines and scenes until changed. A speaking
// character is always brought on (see Builder.speak), so `exit` is the only
// direction most scripts need.
func (cl *charLayer) setVisible(id string, t int, visible bool) {
	s := cl.states[id]
	if s == nil || s.voiceOnly || s.hidden == !visible {
		return
	}
	s.flush(t)
	s.hidden = !visible
}

func (cl *charLayer) visible(id string) bool {
	s := cl.states[id]
	return s != nil && !s.hidden
}

// occupiedSides reports which stand positions currently have a character on
// stage, for laying out images around them.
func (cl *charLayer) occupiedSides(cs map[string]*schema.Character) map[string]bool {
	out := map[string]bool{}
	for _, id := range cl.order {
		if cl.visible(id) {
			if c := cs[id]; c != nil {
				out[c.Stand.Position] = true
			}
		}
	}
	return out
}

func (cl *charLayer) close(t int) {
	for _, id := range cl.order {
		cl.transition(id, t, "", false, nil)
	}
}
