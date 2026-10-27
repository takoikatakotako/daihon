package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/takoikatakotako/daihon/internal/wav"
)

func TestCheckVoiceTimelineBuildWithEstimate(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg is required for build")
	}
	dir := t.TempDir()
	file := filepath.Join(dir, "movie.yml")
	content := `version: 1
meta: { title: test, output: out/test.mp4 }
video: { width: 64, height: 64, fps: 10 }
subtitle: { mode: none }
characters:
  narrator:
    name: narrator
    voice: { engine: estimate }
    stand: { position: none }
scenes:
  - id: intro
    script:
      - narrator: "こんにちは。"
        read: "コンニチハ。"
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"-f", file}); err != nil {
		t.Fatal(err)
	}
	if err := runVoice(context.Background(), []string{"-f", file}); err != nil {
		t.Fatal(err)
	}
	if err := runTimeline(context.Background(), []string{"-f", file}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, cacheDir, "timeline.json")); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Join(dir, cacheDir, "audio"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("audio cache entries: %v, %v", entries, err)
	}
	b, err := os.ReadFile(filepath.Join(dir, cacheDir, "audio", entries[0].Name()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := wav.DurationMS(b); err != nil {
		t.Fatal(err)
	}
	if err := runBuild(context.Background(), []string{"-f", file, "-o", filepath.Join(dir, "out.mp4")}); err != nil {
		t.Fatal(err)
	}
	if info, err := os.Stat(filepath.Join(dir, "out.mp4")); err != nil || info.Size() == 0 {
		t.Fatalf("build output: %v, %v", info, err)
	}
}

func TestGeminiOverrideValidatesBeforeSynthesis(t *testing.T) {
	file := filepath.Join(t.TempDir(), "movie.yml")
	content := `version: 1
subtitle: { mode: none }
characters:
  narrator:
    name: narrator
    voice: { engine: voicevox, speaker_id: 1, speed: 1.1 }
    stand: { position: none }
scenes:
  - id: intro
    script:
      - narrator: "こんにちは。"
`
	if err := os.WriteFile(file, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := runCheck([]string{"-f", file, "--engine", "gemini"}); err != errCheckFailed {
		t.Fatalf("check: got %v, want validation error", err)
	}
	if err := runVoice(context.Background(), []string{"-f", file, "--engine", "gemini"}); err != errCheckFailed {
		t.Fatalf("voice: got %v, want validation before API call", err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(file), cacheDir)); !os.IsNotExist(err) {
		t.Fatalf("audio cache created before validation: %v", err)
	}
}
