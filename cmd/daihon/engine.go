package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/takoikatakotako/daihon/internal/parser"
	"github.com/takoikatakotako/daihon/internal/voice/voicevox"
)

// errNoEngine explains the ways out of a missing speech engine.
var errNoEngine = errors.New(
	"VOICEVOX ENGINE に接続できません。次のいずれかで解決できます:\n" +
		"  ・Docker で起動する:  docker compose up -d\n" +
		"  ・ネイティブ版を置く: DAIHON_VOICEVOX_BIN=/path/to/voicevox_engine/run （daihon が自動起動します）\n" +
		"  ・音声なしで進める:   --engine estimate")

// needsVoicevox reports whether any line in the project would be synthesised by
// VOICEVOX, taking --engine into account.
func needsVoicevox(p *parser.Project, override string) bool {
	if override != "" {
		return override == "voicevox"
	}
	for _, c := range p.Movie.Characters {
		if c.Voice.Engine == "voicevox" {
			return true
		}
	}
	return false
}

// ensureEngine makes a VOICEVOX ENGINE available for the duration of a command.
//
// An engine that is already listening — started by docker compose, by the
// desktop app, or by hand — is always used as-is. Only when nothing answers do
// we start a locally installed native engine ourselves and stop it afterwards,
// which gives the convenience of an embedded engine without linking one in.
func ensureEngine(ctx context.Context, p *parser.Project, override string, autostart bool) (func(), error) {
	stop := func() {}
	if !needsVoicevox(p, override) {
		return stop, nil
	}

	base := os.Getenv("DAIHON_VOICEVOX_URL")
	if _, err := voicevox.New(base).Version(ctx); err == nil {
		return stop, nil
	}
	if !autostart {
		return stop, errNoEngine
	}

	bin := voicevox.Locate()
	if bin == "" {
		return stop, errNoEngine
	}

	fmt.Fprintf(os.Stderr, "VOICEVOX ENGINE を起動しています (%s)…\n", bin)
	l := &voicevox.Launcher{Bin: bin, BaseURL: base}
	if err := l.Start(ctx); err != nil {
		return stop, err
	}
	return l.Stop, nil
}
