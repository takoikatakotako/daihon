package timeline

import (
	"context"
	"testing"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/schema"
	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/voice/estimate"
)

// The estimate engine is deterministic, which makes cursor arithmetic testable
// without a speech server. Ask the engine itself rather than restating its
// formula, so recalibrating it does not break these tests.
func estimatedMS(text string) int { return estimate.DurationMS(text, 1.0) }

func newBuilder(t *testing.T, m *schema.Movie) *Builder {
	t.Helper()
	m.Version = 1
	m.ApplyDefaults()
	p := &parser.Project{Root: t.TempDir(), File: "movie.yml", Movie: m}
	return &Builder{
		Project:        p,
		Synth:          voice.NewSynthesizer(t.TempDir(), estimate.New()),
		EngineOverride: "estimate",
	}
}

func twoCast() map[string]*schema.Character {
	return map[string]*schema.Character{
		"a": {Name: "A", DefaultExpression: "normal", Stand: schema.Stand{Position: "right"}},
		"b": {Name: "B", DefaultExpression: "normal", Stand: schema.Stand{Position: "left"}},
	}
}

func track(tl *Timeline, kind, id string) *Track {
	for i := range tl.Tracks {
		if tl.Tracks[i].Type == kind && tl.Tracks[i].ID == id {
			return &tl.Tracks[i]
		}
	}
	return nil
}

// The cursor advances by the audio length plus the trailing pause; `wait`
// standing alone advances it on its own. Nobody writes a timestamp.
func TestCursorAdvancesByAudioThenPause(t *testing.T) {
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Audio:      schema.Audio{PauseBetweenLinesMS: 400},
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindLine, Speaker: "a", Text: "あいう"},
			{Kind: schema.KindWait, WaitMS: 1000},
			{Kind: schema.KindLine, Speaker: "b", Text: "かきくけこ"},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	first, second := estimatedMS("あいう"), estimatedMS("かきくけこ")
	want := first + 400 + 1000 + second + 400
	if tl.DurationMS != want {
		t.Errorf("duration = %d, want %d", tl.DurationMS, want)
	}

	voices := track(tl, TrackVoice, "")
	if voices == nil || len(voices.Events) != 2 {
		t.Fatalf("voice track = %+v", voices)
	}
	if got := voices.Events[0].EndMS; got != first {
		t.Errorf("first line ends at %d, want %d", got, first)
	}
	if got := voices.Events[1].StartMS; got != first+400+1000 {
		t.Errorf("second line starts at %d, want %d", got, first+400+1000)
	}
}

// A per-line `wait` overrides the global pause.
func TestLineWaitOverridesDefaultPause(t *testing.T) {
	w := 50
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Audio:      schema.Audio{PauseBetweenLinesMS: 400},
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindLine, Speaker: "a", Text: "あ", Wait: &w},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := estimatedMS("あ") + w; tl.DurationMS != want {
		t.Errorf("duration = %d, want %d", tl.DurationMS, want)
	}
}

// Every character must be covered by exactly one event at any instant, so the
// renderer never has to ask what someone is doing right now.
func TestCharacterTrackIsContiguous(t *testing.T) {
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindLine, Speaker: "a", Text: "あ", Expr: "happy"},
			{Kind: schema.KindLine, Speaker: "b", Text: "い"},
			{Kind: schema.KindLine, Speaker: "a", Text: "う"},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	for _, id := range []string{"a", "b"} {
		tr := track(tl, TrackCharacter, id)
		if tr == nil {
			t.Fatalf("no track for %q", id)
		}
		prev := 0
		for i, ev := range tr.Events {
			if ev.StartMS != prev {
				t.Errorf("%s event %d starts at %d, want %d (gap or overlap)", id, i, ev.StartMS, prev)
			}
			if ev.EndMS <= ev.StartMS {
				t.Errorf("%s event %d is empty: %+v", id, i, ev)
			}
			prev = ev.EndMS
		}
		if prev != tl.DurationMS {
			t.Errorf("%s track ends at %d, want %d", id, prev, tl.DurationMS)
		}
	}
}

// An expression is a state: it holds until the next one is named.
func TestExpressionPersistsUntilChanged(t *testing.T) {
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindLine, Speaker: "a", Text: "あ", Expr: "happy"},
			{Kind: schema.KindLine, Speaker: "a", Text: "い"},
			{Kind: schema.KindLine, Speaker: "a", Text: "う", Expr: "sad"},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tr := track(tl, TrackCharacter, "a")

	var speaking []string
	for _, ev := range tr.Events {
		if ev.Speaking {
			speaking = append(speaking, ev.Expression)
		}
	}
	want := []string{"happy", "happy", "sad"}
	if len(speaking) != len(want) {
		t.Fatalf("got %v, want %v", speaking, want)
	}
	for i := range want {
		if speaking[i] != want[i] {
			t.Errorf("line %d expression = %q, want %q", i, speaking[i], want[i])
		}
	}
}

// `show` handles are scene-scoped: an image left open closes when its scene ends.
func TestImageClosesAtSceneEnd(t *testing.T) {
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Scenes: []schema.Scene{
			{ID: "one", Script: []schema.ScriptItem{
				{Kind: schema.KindShow, Src: "x.png", As: "fig", At: "center"},
				{Kind: schema.KindLine, Speaker: "a", Text: "あ"},
			}},
			{ID: "two", Script: []schema.ScriptItem{
				{Kind: schema.KindLine, Speaker: "b", Text: "い"},
			}},
		},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	tr := track(tl, TrackImage, "")
	if tr == nil || len(tr.Events) != 1 {
		t.Fatalf("image track = %+v", tr)
	}
	sceneOneEnd := estimatedMS("あ") + 400
	if got := tr.Events[0].EndMS; got != sceneOneEnd {
		t.Errorf("image ends at %d, want scene end %d", got, sceneOneEnd)
	}
	if tr.Events[0].Rect == nil {
		t.Error("`at` should have been resolved to a rect")
	}
}

// --scene restricts the build for quick iteration on one part.
func TestSceneFilter(t *testing.T) {
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Scenes: []schema.Scene{
			{ID: "one", Script: []schema.ScriptItem{{Kind: schema.KindLine, Speaker: "a", Text: "あああ"}}},
			{ID: "two", Script: []schema.ScriptItem{{Kind: schema.KindLine, Speaker: "b", Text: "い"}}},
		},
	})
	b.SceneFilter = "two"
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := estimatedMS("い") + 400; tl.DurationMS != want {
		t.Errorf("duration = %d, want only scene two (%d)", tl.DurationMS, want)
	}
}

// Editing one line must not invalidate the others: this is what keeps a build fast.
func TestCacheReusesUnchangedLines(t *testing.T) {
	cache := t.TempDir()
	build := func(second string) (*Builder, *Timeline) {
		m := &schema.Movie{
			Characters: twoCast(),
			Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
				{Kind: schema.KindLine, Speaker: "a", Text: "かわらない"},
				{Kind: schema.KindLine, Speaker: "b", Text: second},
			}}},
		}
		m.Version = 1
		m.ApplyDefaults()
		b := &Builder{
			Project:        &parser.Project{Root: t.TempDir(), Movie: m},
			Synth:          voice.NewSynthesizer(cache, estimate.New()),
			EngineOverride: "estimate",
		}
		tl, err := b.Build(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return b, tl
	}

	b1, _ := build("さいしょ")
	if b1.Synth.Misses != 2 {
		t.Fatalf("cold build: %d misses, want 2", b1.Synth.Misses)
	}
	b2, _ := build("かきなおした")
	if b2.Synth.Hits != 1 || b2.Synth.Misses != 1 {
		t.Errorf("after editing one line: %d hits / %d misses, want 1/1", b2.Synth.Hits, b2.Synth.Misses)
	}
}

// `exit` takes a character off stage: its track has a gap there. Speaking
// brings the character back without an explicit `enter`, and visibility
// carries across scene boundaries like an expression does.
func TestExitLeavesGapUntilEnterOrSpeech(t *testing.T) {
	const pause = 100
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(),
		Audio:      schema.Audio{PauseBetweenLinesMS: pause},
		Scenes: []schema.Scene{
			{ID: "s1", Script: []schema.ScriptItem{
				{Kind: schema.KindExit, Target: "b"},
				{Kind: schema.KindLine, Speaker: "a", Text: "あいう"},
				{Kind: schema.KindLine, Speaker: "b", Text: "かきく"}, // b appears here
				{Kind: schema.KindExit, Target: "b"},
			}},
			{ID: "s2", Script: []schema.ScriptItem{
				{Kind: schema.KindLine, Speaker: "a", Text: "さしす"}, // b still gone
				{Kind: schema.KindEnter, Target: "b"},
				{Kind: schema.KindWait, WaitMS: 500},
			}},
		},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a1, b1, a2 := estimatedMS("あいう"), estimatedMS("かきく"), estimatedMS("さしす")
	bStart := a1 + pause         // b's line begins after a's line and its pause
	bExit := bStart + b1 + pause // exit is written after b's line, i.e. after its pause
	bEnter := bExit + a2 + pause // s2: a's line, then enter

	tr := track(tl, TrackCharacter, "b")
	if tr == nil {
		t.Fatal("no track for b")
	}
	type span struct {
		start, end int
		speaking   bool
	}
	var got []span
	for _, e := range tr.Events {
		got = append(got, span{e.StartMS, e.EndMS, e.Speaking})
	}
	want := []span{
		{bStart, bStart + b1, true},    // speaking
		{bStart + b1, bExit, false},    // lingering through the pause, then exit
		{bEnter, tl.DurationMS, false}, // back after enter, to the end
	}
	if len(got) != len(want) {
		t.Fatalf("b spans = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("span %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	// The gap [bExit, bEnter) is where b is off stage.
	if got[1].end >= got[2].start {
		t.Errorf("no gap between exit and enter: %+v", got)
	}

	// a never left, so its track is continuous.
	ta := track(tl, TrackCharacter, "a")
	if ta.Events[0].StartMS != 0 || ta.Events[len(ta.Events)-1].EndMS != tl.DurationMS {
		t.Errorf("a track should span the whole movie: %+v", ta.Events)
	}
}

// A centred image stays out from behind the figures: with characters on both
// sides it is confined to the gap between them, and it widens toward a side
// whose character has exited.
func TestCenterImageAvoidsStandingCharacters(t *testing.T) {
	const w = 1000
	b := newBuilder(t, &schema.Movie{
		Characters: twoCast(), // a right, b left
		Video:      schema.Video{Width: w, Height: 500},
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindShow, Src: "x.png", As: "both", At: "center"},
			{Kind: schema.KindLine, Speaker: "a", Text: "あ"},
			{Kind: schema.KindHide, Target: "both"},
			{Kind: schema.KindExit, Target: "b"},
			{Kind: schema.KindShow, Src: "x.png", As: "one", At: "center"},
			{Kind: schema.KindLine, Speaker: "a", Text: "い"},
			{Kind: schema.KindHide, Target: "one"},
			{Kind: schema.KindShow, Src: "x.png", As: "side", At: "left"},
			{Kind: schema.KindLine, Speaker: "a", Text: "う"},
			{Kind: schema.KindHide, Target: "side"},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	evs := track(tl, TrackImage, "").Events
	if len(evs) != 3 {
		t.Fatalf("got %d image events", len(evs))
	}
	both, one, side := evs[0].Rect, evs[1].Rect, evs[2].Rect
	if both.X != w*35/100 || both.X+both.W != w*65/100 {
		t.Errorf("with both figures the box should span 35%%-65%%: %+v", both)
	}
	if one.X != w*20/100 || one.X+one.W != w*65/100 {
		t.Errorf("with the left figure gone the box should span 20%%-65%%: %+v", one)
	}
	if side.X != w/20 || side.W != w*3/5 {
		t.Errorf("`at: left` should be untouched by figures: %+v", side)
	}
}

// A voice-only character (stand.position: none) is heard but never drawn:
// speaking and enter both leave it off stage, and it does not narrow the box
// for centred images.
func TestVoiceOnlyCharacterNeverAppears(t *testing.T) {
	const w = 1000
	b := newBuilder(t, &schema.Movie{
		Characters: map[string]*schema.Character{
			"n": {Name: "N", Stand: schema.Stand{Position: schema.StandNone}},
		},
		Video: schema.Video{Width: w, Height: 500},
		Scenes: []schema.Scene{{ID: "s", Script: []schema.ScriptItem{
			{Kind: schema.KindLine, Speaker: "n", Text: "あいう"},
			{Kind: schema.KindEnter, Target: "n"},
			{Kind: schema.KindShow, Src: "x.png", As: "x"},
			{Kind: schema.KindLine, Speaker: "n", Text: "かきく"},
		}}},
	})
	tl, err := b.Build(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if tr := track(tl, TrackCharacter, "n"); tr != nil && len(tr.Events) > 0 {
		t.Errorf("voice-only character drawn: %+v", tr.Events)
	}
	if got := len(track(tl, TrackVoice, "").Events); got != 2 {
		t.Errorf("voice events = %d, want 2", got)
	}
	if r := track(tl, TrackImage, "").Events[0].Rect; r.W != w*3/5 {
		t.Errorf("centre box width = %d, want %d (nothing on stage)", r.W, w*3/5)
	}
}
