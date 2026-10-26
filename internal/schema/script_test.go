package schema

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func parseItems(t *testing.T, src string) []ScriptItem {
	t.Helper()
	var items []ScriptItem
	if err := yaml.Unmarshal([]byte(src), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return items
}

func TestFirstKeyDecidesKind(t *testing.T) {
	items := parseItems(t, `
- zunda: "なのだ"
  expr: happy
  wait: 800
- show: assets/x.png
  as: fig1
  at: center
- hide: fig1
- wait: 1200
- bg: assets/bg.png
- se: assets/pon.mp3
- note: "あとで直す"
`)
	if len(items) != 7 {
		t.Fatalf("got %d items", len(items))
	}

	line := items[0]
	if line.Kind != KindLine || line.Speaker != "zunda" || line.Text != "なのだ" {
		t.Errorf("line = %+v", line)
	}
	if line.Expr != "happy" || line.Wait == nil || *line.Wait != 800 {
		t.Errorf("modifiers not applied: %+v", line)
	}
	if items[1].Kind != KindShow || items[1].Src != "assets/x.png" || items[1].As != "fig1" {
		t.Errorf("show = %+v", items[1])
	}
	if items[2].Kind != KindHide || items[2].Target != "fig1" {
		t.Errorf("hide = %+v", items[2])
	}
	if items[3].Kind != KindWait || items[3].WaitMS != 1200 {
		t.Errorf("wait = %+v", items[3])
	}
	if items[4].Kind != KindBG || items[4].Path != "assets/bg.png" {
		t.Errorf("bg = %+v", items[4])
	}
	if items[5].Kind != KindSE || items[5].Path != "assets/pon.mp3" {
		t.Errorf("se = %+v", items[5])
	}
	if items[6].Kind != KindNote || items[6].Note != "あとで直す" {
		t.Errorf("note = %+v", items[6])
	}
}

func TestReadAndStyleRemainSeparate(t *testing.T) {
	items := parseItems(t, "- rikako: \"クロマトグラフィー。\"\n  read: \"ペーパークロマトグラフィー。\"\n  style: \"明るく\"\n")
	if len(items) != 1 || items[0].Text != "クロマトグラフィー。" || items[0].Read != "ペーパークロマトグラフィー。" || items[0].Style != "明るく" {
		t.Fatalf("line fields: %+v", items)
	}
}

// `wait` is both a directive and a modifier. Which one it is depends solely on
// whether it is the first key, so the two readings must not blur together.
func TestWaitIsDirectiveOnlyWhenFirst(t *testing.T) {
	items := parseItems(t, "- wait: 500\n- zunda: \"あ\"\n  wait: 500\n")
	if items[0].Kind != KindWait || items[0].WaitMS != 500 {
		t.Errorf("standalone wait = %+v", items[0])
	}
	if items[1].Kind != KindLine || items[1].Wait == nil || *items[1].Wait != 500 {
		t.Errorf("trailing wait = %+v", items[1])
	}
	if items[1].WaitMS != 0 {
		t.Errorf("modifier leaked into directive field: %+v", items[1])
	}
}

func TestTwoDirectivesInOneItemIsRejected(t *testing.T) {
	var items []ScriptItem
	err := yaml.Unmarshal([]byte("- show: a.png\n  hide: fig1\n"), &items)
	if err == nil {
		t.Fatal("expected an error for two directives in one item")
	}
	if !strings.Contains(err.Error(), "hide") {
		t.Errorf("error should name the offending key: %v", err)
	}
}

func TestReservedWords(t *testing.T) {
	for _, w := range []string{"show", "hide", "bg", "wait", "se", "note"} {
		if !IsReserved(w) {
			t.Errorf("%q should be reserved", w)
		}
	}
	if IsReserved("zunda") {
		t.Error("character IDs should not be reserved")
	}
}

func TestBackgroundAcceptsPathOrMapping(t *testing.T) {
	var v struct {
		Plain   Background `yaml:"plain"`
		Treated Background `yaml:"treated"`
		Missing Background `yaml:"missing"`
	}
	src := `
plain: assets/bg/a.png
treated:
  src: assets/bg/b.png
  dim: 0.35
  blur: 6
`
	if err := yaml.Unmarshal([]byte(src), &v); err != nil {
		t.Fatal(err)
	}
	if v.Plain != (Background{Src: "assets/bg/a.png"}) {
		t.Errorf("plain form: %+v", v.Plain)
	}
	if v.Treated != (Background{Src: "assets/bg/b.png", Dim: 0.35, Blur: 6}) {
		t.Errorf("mapping form: %+v", v.Treated)
	}
	if !v.Missing.IsZero() {
		t.Errorf("absent background should be zero: %+v", v.Missing)
	}
}

func TestEnterExitAreDirections(t *testing.T) {
	items := parseItems(t, `
- exit: metan
- zunda: "ひとりなのだ"
- enter: metan
- show: assets/x.png
  enter: fade
`)
	if items[0].Kind != KindExit || items[0].Target != "metan" {
		t.Errorf("exit = %+v", items[0])
	}
	if items[2].Kind != KindEnter || items[2].Target != "metan" {
		t.Errorf("enter = %+v", items[2])
	}
	// The same word as a modifier of show keeps its old meaning.
	if items[3].Kind != KindShow || items[3].Enter != "fade" {
		t.Errorf("show with enter modifier = %+v", items[3])
	}
	if !IsReserved("enter") || !IsReserved("exit") {
		t.Error("enter/exit must be reserved so they cannot be character IDs")
	}
}
