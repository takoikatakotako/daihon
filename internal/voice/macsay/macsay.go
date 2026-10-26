// Package macsay synthesises speech with the macOS say command.
package macsay

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/wav"
)

// Engine uses the macOS speech service and ffmpeg to return PCM WAV data.
type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) ID() string { return "say" }

func (e *Engine) Version(ctx context.Context) (string, error) {
	if runtime.GOOS != "darwin" {
		return "", fmt.Errorf("say は macOS でのみ使えます")
	}
	for _, name := range []string{"say", "ffmpeg"} {
		if _, err := exec.LookPath(name); err != nil {
			return "", fmt.Errorf("%s が見つかりません: %w", name, err)
		}
	}
	b, err := exec.CommandContext(ctx, "sw_vers", "-productVersion").Output()
	if err != nil {
		return "", fmt.Errorf("macOS のバージョンを取得できません: %w", err)
	}
	return "macos-say-" + strings.TrimSpace(string(b)), nil
}

// Synth uses voice_name when provided, otherwise the system's default voice.
// say's rate is words per minute; a speed of 1.0 maps to 180 WPM.
func (e *Engine) Synth(ctx context.Context, text string, o voice.Options) ([]byte, error) {
	if _, err := e.Version(ctx); err != nil {
		return nil, err
	}
	if o.Pitch != 0 {
		return nil, fmt.Errorf("say は pitch に対応していません")
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("読み上げテキストが空です")
	}
	speed := o.Speed
	if speed == 0 {
		speed = 1
	}
	if speed <= 0 {
		return nil, fmt.Errorf("speed は正の数で指定してください")
	}

	dir, err := os.MkdirTemp("", "daihon-say-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	aiff := filepath.Join(dir, "voice.aiff")
	wave := filepath.Join(dir, "voice.wav")
	args := []string{"-r", strconv.Itoa(int(math.Round(180 * speed)))}
	if o.VoiceName != "" {
		args = append(args, "-v", o.VoiceName)
	}
	args = append(args, "-o", aiff, text)
	if out, err := exec.CommandContext(ctx, "say", args...).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("say の実行に失敗しました: %w: %s", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.CommandContext(ctx, "ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-i", aiff,
		"-ac", "1", "-ar", "24000", "-c:a", "pcm_s16le", wave).CombinedOutput(); err != nil {
		return nil, fmt.Errorf("say の音声を WAV に変換できません: %w: %s", err, strings.TrimSpace(string(out)))
	}
	b, err := os.ReadFile(wave)
	if err != nil {
		return nil, err
	}
	duration, err := wav.DurationMS(b)
	if err != nil || duration == 0 {
		return nil, fmt.Errorf("say が空または不正な音声を返しました（音声サービスを確認してください）")
	}
	return b, nil
}
