package validator

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/schema"
)

func TestCheckOverlays(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "logo.png"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := &schema.Movie{Version: 1, Video: schema.Video{Width: 1080, Height: 1920}}
	m.Video.Overlays = []schema.Overlay{
		{Src: "logo.png", X: 74, Y: 80, Width: 116, Height: 116},  // ok
		{Src: "missing.png", X: 0, Y: 0, Width: 10, Height: 10},   // warn: file
		{Src: "logo.png", X: 0, Y: 0, Width: 0, Height: 10},       // error: size
		{Src: "logo.png", X: 1000, Y: 0, Width: 116, Height: 116}, // warn: overflows
		{Src: "logo.png", X: 2000, Y: 0, Width: 116, Height: 116}, // error: off screen
		{X: 0, Y: 0, Width: 10, Height: 10},                       // error: no src
	}
	p := &parser.Project{Root: root, Movie: m}

	got := map[string]Severity{}
	for _, d := range checkOverlays(p) {
		got[d.Message] = d.Severity
	}
	want := []struct {
		sev    Severity
		substr string
	}{
		{Warn, "video.overlays[1].src: ファイルがありません"},
		{Error, "video.overlays[2] の width と height"},
		{Warn, "video.overlays[3] が画面 (1080x1920) からはみ出しています"},
		{Error, "video.overlays[4] が画面 (1080x1920) の外にあります"},
		{Error, "video.overlays[5].src がありません"},
	}
	if len(got) != len(want) {
		t.Errorf("got %d diagnostics, want %d: %v", len(got), len(want), got)
	}
	for _, w := range want {
		found := false
		for msg, sev := range got {
			if strings.Contains(msg, w.substr) && sev == w.sev {
				found = true
			}
		}
		if !found {
			t.Errorf("missing %s %q in %v", w.sev, w.substr, got)
		}
	}
	for msg := range got {
		if strings.Contains(msg, "overlays[0]") {
			t.Errorf("valid overlay reported: %s", msg)
		}
	}
}
