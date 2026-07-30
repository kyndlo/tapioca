//go:build windows

package server

import "os/exec"

func configureProcess(_ *exec.Cmd) {}

func stopProcess(cmd *exec.Cmd) error {
	return cmd.Process.Kill()
}
