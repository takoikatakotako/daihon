package main

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/voice/estimate"
	"github.com/takoikatakotako/daihon/internal/voice/macsay"
	"github.com/takoikatakotako/daihon/internal/voice/voicevox"
)

var errCheckFailed = errors.New("check failed")

// cacheDir is where synthesised audio and the timeline live, next to movie.yml.
const cacheDir = ".daihon-cache"

func load(path string) (*parser.Project, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return parser.Load(path)
}

// newSynth wires up every engine the tool knows about. Which one a line uses is
// decided per character by `voice.engine`, or globally by --engine.
func newSynth(p *parser.Project) *voice.Synthesizer {
	base := os.Getenv("DAIHON_VOICEVOX_URL")
	return voice.NewSynthesizer(
		filepath.Join(p.Root, cacheDir, "audio"),
		voicevox.New(base),
		macsay.New(),
		estimate.New(),
	)
}
