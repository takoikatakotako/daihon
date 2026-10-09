//go:build unix

package voicevox

import (
	"os/exec"
	"syscall"
)

// The engine's launcher may spawn helper processes. Giving it its own process
// group lets us signal all of them at once instead of orphaning the helpers.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func terminate(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }

func kill(cmd *exec.Cmd) { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
