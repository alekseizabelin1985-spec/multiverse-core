//go:build e2e

package e2e_test

import (
	"os/exec"
	"syscall"
)

// inOwnGroup starts the process in a process group of its own, so that a
// console event can be sent to it and to nothing else: the test, go test and
// the shell they run in share the console with it.
func inOwnGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP}
}

var generateConsoleCtrlEvent = syscall.NewLazyDLL("kernel32.dll").NewProc("GenerateConsoleCtrlEvent")

// interrupt sends CTRL_BREAK to the process group of cmd — the group id is the
// pid of the process that leads it. Go delivers it as os.Interrupt, which is
// syscall.SIGINT, the signal withSignals listens for. CTRL_C cannot be used:
// a process started with CREATE_NEW_PROCESS_GROUP ignores it.
//
// It fails when the test runs without a console to share, which is a limit
// of the environment rather than of the process.
func interrupt(cmd *exec.Cmd) error {
	ok, _, err := generateConsoleCtrlEvent.Call(syscall.CTRL_BREAK_EVENT, uintptr(cmd.Process.Pid))
	if ok == 0 {
		return err
	}
	return nil
}
