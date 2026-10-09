package voicevox

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// BinEnv names the variable that points at a local engine's launcher.
const BinEnv = "DAIHON_VOICEVOX_BIN"

// startTimeout bounds how long a freshly started engine may take to answer.
// The first start loads the voice models and can take tens of seconds.
const startTimeout = 90 * time.Second

// Locate returns the path of a local engine launcher, or "" if there is none.
//
// DAIHON_VOICEVOX_BIN wins when set. Otherwise the engine bundled with the
// VOICEVOX desktop app on macOS is used if present.
func Locate() string {
	candidates := []string{os.Getenv(BinEnv)}
	if home, err := os.UserHomeDir(); err == nil {
		candidates = append(candidates, filepath.Join(home, "Applications/VOICEVOX.app/Contents/Resources/vv-engine/run"))
	}
	candidates = append(candidates, "/Applications/VOICEVOX.app/Contents/Resources/vv-engine/run")

	for _, p := range candidates {
		if p == "" {
			continue
		}
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

// Launcher runs a local engine for the duration of one command.
type Launcher struct {
	Bin     string // engine launcher (the "run" executable)
	BaseURL string // where it should listen; DefaultBaseURL when empty

	cmd    *exec.Cmd
	exited chan struct{}
}

// Start launches the engine and waits until it answers /version.
func (l *Launcher) Start(ctx context.Context) error {
	base := l.BaseURL
	if base == "" {
		base = DefaultBaseURL
	}
	u, err := url.Parse(base)
	if err != nil {
		return fmt.Errorf("VOICEVOX ENGINE の URL が不正です: %w", err)
	}
	port := u.Port()
	if port == "" {
		port = "50021"
	}

	l.cmd = exec.Command(l.Bin, "--host", "127.0.0.1", "--port", port)
	l.cmd.Dir = filepath.Dir(l.Bin)
	setProcessGroup(l.cmd)
	if err := l.cmd.Start(); err != nil {
		return fmt.Errorf("VOICEVOX ENGINE を起動できません (%s): %w", l.Bin, err)
	}
	l.exited = make(chan struct{})
	go func() {
		_ = l.cmd.Wait()
		close(l.exited)
	}()

	eng := New(base)
	deadline := time.Now().Add(startTimeout)
	for {
		if _, err := eng.Version(ctx); err == nil {
			return nil
		}
		select {
		case <-l.exited:
			return errors.New("VOICEVOX ENGINE が起動直後に終了しました")
		case <-ctx.Done():
			l.Stop()
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
		if time.Now().After(deadline) {
			l.Stop()
			return fmt.Errorf("VOICEVOX ENGINE が %s 以内に応答しませんでした", startTimeout)
		}
	}
}

// Stop shuts the engine down. It is safe to call more than once.
func (l *Launcher) Stop() {
	if l.cmd == nil || l.cmd.Process == nil {
		return
	}
	select {
	case <-l.exited:
		return
	default:
	}
	terminate(l.cmd)
	select {
	case <-l.exited:
	case <-time.After(5 * time.Second):
		kill(l.cmd)
		<-l.exited
	}
}
