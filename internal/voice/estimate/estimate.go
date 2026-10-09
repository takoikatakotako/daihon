// Package estimate is a voice.Engine that does not synthesise anything: it
// guesses how long a line would take to read and returns that much silence.
//
// It exists so the pipeline runs end to end without a speech engine — useful
// for checking pacing and layout in a draft build, in CI, and on a machine
// where no engine is available.
package estimate

import (
	"context"

	"github.com/takoikatakotako/daihon/internal/voice"
	"github.com/takoikatakotako/daihon/internal/wav"
)

const (
	// Calibrated against VOICEVOX ENGINE 0.25.1 at speed 1.0. A model linear
	// in character count lands within roughly ±10%: the rest is genuine
	// variance from punctuation and kanji readings, which no amount of fitting
	// on character count alone can recover. This is for pacing, not for cutting.
	msPerRune = 165
	// Fixed lead-in and trailing silence around every utterance.
	baseMS = 300

	sampleRate = 24000 // matches VOICEVOX's default output
)

// DurationMS is the estimate itself, exposed so callers (and tests) can predict
// what this engine will return without synthesising.
func DurationMS(text string, speed float64) int {
	ms := baseMS + len([]rune(text))*msPerRune
	if speed > 0 {
		ms = int(float64(ms) / speed)
	}
	return ms
}

type Engine struct{}

func New() *Engine { return &Engine{} }

func (e *Engine) ID() string { return "estimate" }

func (e *Engine) Version(context.Context) (string, error) { return "estimate-2", nil }

func (e *Engine) Synth(_ context.Context, text string, o voice.Options) ([]byte, error) {
	return wav.Silence(DurationMS(text, o.Speed), sampleRate), nil
}
