package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/takoikatakotako/daihon/internal/wav"
)

// Synthesizer synthesises lines through a set of engines, caching each line on
// disk. Synthesis dominates build time, so the cache is what makes editing a
// script feel like editing code: change one line and only that line is re-synthesised.
type Synthesizer struct {
	engines  map[string]Engine
	dir      string
	versions map[string]string // engine ID -> version, resolved once

	Hits   int
	Misses int
}

// NewSynthesizer caches into dir (conventionally .daihon-cache/audio).
func NewSynthesizer(dir string, engines ...Engine) *Synthesizer {
	m := make(map[string]Engine, len(engines))
	for _, e := range engines {
		m[e.ID()] = e
	}
	return &Synthesizer{engines: m, dir: dir, versions: map[string]string{}}
}

// Result is one synthesised line.
type Result struct {
	Path       string
	DurationMS int
	Wav        []byte
	Cached     bool
}

// Synth returns the audio for one line, from cache when possible.
func (s *Synthesizer) Synth(ctx context.Context, engineID, text string, o Options) (*Result, error) {
	eng, ok := s.engines[engineID]
	if !ok {
		return nil, fmt.Errorf("未知の音声エンジン %q です", engineID)
	}
	version, err := s.version(ctx, eng)
	if err != nil {
		return nil, err
	}

	key := cacheKey(engineID, version, text, o)
	path := filepath.Join(s.dir, key+".wav")

	if b, err := os.ReadFile(path); err == nil {
		if d, err := wav.DurationMS(b); err == nil {
			s.Hits++
			return &Result{Path: path, DurationMS: d, Wav: b, Cached: true}, nil
		}
		// Corrupt cache entry: fall through and re-synthesise.
	}

	b, err := eng.Synth(ctx, text, o)
	if err != nil {
		return nil, fmt.Errorf("音声合成に失敗しました (%q): %w", truncate(text), err)
	}
	d, err := wav.DurationMS(b)
	if err != nil {
		return nil, fmt.Errorf("%s が返した wav を解釈できません: %w", engineID, err)
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return nil, err
	}
	s.Misses++
	return &Result{Path: path, DurationMS: d, Wav: b}, nil
}

func (s *Synthesizer) version(ctx context.Context, e Engine) (string, error) {
	if v, ok := s.versions[e.ID()]; ok {
		return v, nil
	}
	v, err := e.Version(ctx)
	if err != nil {
		return "", fmt.Errorf("音声エンジン %s に接続できません: %w", e.ID(), err)
	}
	s.versions[e.ID()] = v
	return v, nil
}

// cacheKey covers everything that can change the waveform. Engine version is
// included so that upgrading the engine does not silently mix old and new audio.
func cacheKey(engineID, version, text string, o Options) string {
	h := sha256.New()
	for _, part := range []string{
		engineID,
		version,
		strconv.Itoa(o.SpeakerID),
		strconv.FormatFloat(o.Speed, 'f', 4, 64),
		strconv.FormatFloat(o.Pitch, 'f', 4, 64),
		text,
	} {
		h.Write([]byte(part))
		h.Write([]byte{0}) // separator, so fields cannot run together
	}
	return hex.EncodeToString(h.Sum(nil))[:32]
}

func truncate(s string) string {
	r := []rune(s)
	if len(r) <= 20 {
		return s
	}
	return string(r[:20]) + "…"
}
