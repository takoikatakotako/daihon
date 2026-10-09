// Package voice turns text into speech. The tool knows nothing about any
// particular character or engine: an engine is just something that returns a
// WAVE for a string, so VOICEVOX is one implementation among several.
package voice

import "context"

// Options are the per-line synthesis parameters.
type Options struct {
	SpeakerID int
	Speed     float64
	Pitch     float64
}

// Engine synthesises speech.
type Engine interface {
	// ID is the name used in movie.yml (`voice.engine`).
	ID() string
	// Version participates in the cache key, so upgrading an engine
	// invalidates previously synthesised lines.
	Version(ctx context.Context) (string, error)
	// Synth returns a 16-bit PCM WAVE.
	Synth(ctx context.Context, text string, o Options) ([]byte, error)
}
