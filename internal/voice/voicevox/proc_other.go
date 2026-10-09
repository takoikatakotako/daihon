//go:build !unix

package voicevox

import "os/exec"

func setProcessGroup(*exec.Cmd) {}

func terminate(cmd *exec.Cmd) { _ = cmd.Process.Kill() }

func kill(cmd *exec.Cmd) { _ = cmd.Process.Kill() }
