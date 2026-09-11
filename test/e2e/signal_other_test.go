//go:build e2e && !windows

package e2e_test

import (
	"os/exec"
	"syscall"
)

// inOwnGroup has nothing to do here: a signal is sent to one pid.
func inOwnGroup(*exec.Cmd) {}

// interrupt sends SIGTERM, what docker compose stop sends to the process.
func interrupt(cmd *exec.Cmd) error {
	return cmd.Process.Signal(syscall.SIGTERM)
}
