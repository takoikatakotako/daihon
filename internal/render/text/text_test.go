package text

import (
	"image"
	"image/color"
	"strings"
	"testing"
)

func testFace(t *testing.T, size float64) *Face {
	t.Helper()
	p := Fallback()
	if p == "" {
		t.Skip("日本語フォントが見つからないためスキップ")
	}
	f, err := Load(p, size)
	if err != nil {
		t.Fatalf("Load(%s): %v", p, err)
	}
	t.Cleanup(func() { f.Close() })
	return f
}

func TestWrapRespectsWidth(t *testing.T) {
	f := testFace(t, 48)
	const maxWidth = 400
	s := "VOICEVOX は AWS 上に作る、自分専用の仮想ネットワークについて解説するのだ。"

	lines := f.Wrap(s, maxWidth)
	if len(lines) < 2 {
		t.Fatalf("want the text to wrap, got %d line(s)", len(lines))
	}
	for i, l := range lines {
		if w := f.Width(l); w > maxWidth {
			t.Errorf("line %d is %dpx wide, over the %dpx limit: %q", i, w, maxWidth, l)
		}
	}

	// A space that lands on a line break is dropped on purpose, so compare
	// with spaces removed.
	joined := strings.ReplaceAll(strings.Join(lines, ""), " ", "")
	if want := strings.ReplaceAll(s, " ", ""); joined != want {
		t.Errorf("wrapping lost or reordered text:\n got %q\nwant %q", joined, want)
	}
}

// A single unbreakable run must be split rather than allowed to overflow.
func TestWrapSplitsOverlongPhrase(t *testing.T) {
	f := testFace(t, 48)
	const maxWidth = 120
	lines := f.Wrap("あああああああああああああああ", maxWidth)
	if len(lines) < 2 {
		t.Fatalf("want a split, got %v", lines)
	}
	for _, l := range lines {
		if w := f.Width(l); w > maxWidth {
			t.Errorf("line %q is %dpx, over %dpx", l, w, maxWidth)
		}
	}
}

func TestWrapEmpty(t *testing.T) {
	f := testFace(t, 48)
	if got := f.Wrap("", 400); got != nil {
		t.Errorf("Wrap(\"\") = %v, want nil", got)
	}
}

func TestDrawPutsInkOnTheImage(t *testing.T) {
	f := testFace(t, 48)
	dst := image.NewRGBA(image.Rect(0, 0, 400, 100))
	f.Draw(dst, 10, 60, "ずんだもん", color.White, color.Black, 3)

	var lit int
	for _, v := range dst.Pix {
		if v != 0 {
			lit++
		}
	}
	if lit == 0 {
		t.Error("nothing was drawn")
	}
}

func TestBlockHeightMatchesGlyphExtent(t *testing.T) {
	f := testFace(t, 48)
	one := f.BlockHeight(1)
	if one >= f.Height() {
		t.Errorf("one line should be shorter than the advance: BlockHeight(1)=%d Height=%d", one, f.Height())
	}
	if got, want := f.BlockHeight(3), one+2*f.Height(); got != want {
		t.Errorf("BlockHeight(3)=%d, want %d (one line + two advances)", got, want)
	}
	if f.BlockHeight(0) != 0 {
		t.Errorf("BlockHeight(0) should be 0")
	}

	// The glyphs of a dense line must fit inside the block with the baseline
	// at Ascent, which is how drawLines places them.
	img := image.NewRGBA(image.Rect(0, 0, 600, 200))
	const top = 50
	f.Draw(img, 10, top+f.Ascent(), "YAML って聞いたのだ。設定ファイルなのだ？", color.White, nil, 0)
	minY, maxY := -1, -1
	for y := 0; y < 200; y++ {
		for x := 0; x < 600; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a > 0 {
				if minY < 0 {
					minY = y
				}
				maxY = y
				break
			}
		}
	}
	if minY < top || maxY >= top+one {
		t.Errorf("ink rows %d-%d fall outside block %d-%d", minY, maxY, top, top+one-1)
	}
	// And the slack above and below the ink should be nearly equal.
	above, below := minY-top, top+one-1-maxY
	if d := above - below; d < -3 || d > 3 {
		t.Errorf("ink is off-centre: %dpx above, %dpx below", above, below)
	}
}

func TestWrapTrimsSpacesAtLineEdges(t *testing.T) {
	f := testFace(t, 48)
	s := "2つめ。Sample Wallet の連携ボタンから Demo App とつなぎます。"
	// Narrow enough to force a break somewhere in the Latin words.
	lines := f.Wrap(s, f.Width("2つめ。Sample Wallet の連携ボタンから")+2)
	if len(lines) < 2 {
		t.Fatalf("want the text to wrap, got %q", lines)
	}
	for _, l := range lines {
		if l != strings.TrimSpace(l) {
			t.Errorf("line has edge whitespace: %q", l)
		}
		if l == "" {
			t.Errorf("empty line in %q", lines)
		}
	}
	if got := strings.Join(lines, ""); strings.ReplaceAll(got, " ", "") != strings.ReplaceAll(s, " ", "") {
		t.Errorf("wrapping lost characters: %q", lines)
	}
}
