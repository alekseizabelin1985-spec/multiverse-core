//go:build e2e

// The two ends of the life of the binary that only a real process shows: a
// start that fails, and a stop by signal (T-415).

package e2e_test

import (
	"context"
	"errors"
	"net"
	"os/exec"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/testkit"
)

// A process that exits on start fails the probe at once and with its exit
// code, not after startupBudget of polling a port nobody will open (review #1
// of T-255, Nit-3). "--bus=memory" with one context is refused by parseServe,
// which exits 2.
//
// The budget given here is shorter than startupBudget only so that a
// regression costs a quarter of a minute rather than a whole one: the process
// is gone within milliseconds either way.
func TestTheProbeGivesUpOnAProcessThatExited(t *testing.T) {
	binary := build(t)
	addr := emptyWorldEnv(t)
	proc := launch(context.Background(), t, binary, nil, "--contexts=state", "--bus=memory")

	_, err := waitHealthy(context.Background(), "http://"+addr+"/health", proc, 15*time.Second)
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("the probe answered %v, want the exit of the process", err)
	}
	if exit.ExitCode() != 2 {
		t.Errorf("exit code %d, want 2 — the code of a refused command line", exit.ExitCode())
	}
	if !strings.Contains(proc.output.String(), "requires --contexts=all") {
		t.Errorf("the process did not say why it refused: %q", proc.output.String())
	}
}

// stopBudget is how long the process gets to stop after the signal: the stop
// timeout of the contexts with room to spare.
const stopBudget = 30 * time.Second

// The process stops by signal the way it is designed to — the signal reaches
// withSignals, run stops the contexts and closes the bus, the exit code is 0
// and the port is free. A serve that called run without withSignals (mutant
// R5b of review #2 of T-410) passed every unit test, because a unit test
// cannot signal its own process on Windows; here it dies of the signal
// instead: 0xC000013A on Windows, killed by SIGTERM elsewhere.
func TestTheProcessStopsCleanlyOnASignal(t *testing.T) {
	binary := build(t)
	addr := emptyWorldEnv(t)
	proc := launch(context.Background(), t, binary, inOwnGroup, "--contexts=all", "--bus=memory")
	probe(context.Background(), t, "http://"+addr+"/health", proc)

	// A process that died after answering has no group left to signal, and its
	// death is the finding.
	select {
	case <-proc.exited:
		t.Fatalf("the process exited before the signal: %v", proc.err)
	default:
	}
	if err := interrupt(proc.cmd); err != nil {
		t.Skipf("cannot signal the process here (no console to share on Windows?): %v; "+
			"run go test -tags e2e -run TestTheProcessStopsCleanlyOnASignal ./test/e2e from a terminal", err)
	}
	select {
	case <-proc.exited:
	case <-testkit.After(stopBudget):
		t.Fatalf("the process did not stop within %s of the signal", stopBudget)
	}
	if code := proc.cmd.ProcessState.ExitCode(); code != 0 {
		t.Fatalf("exit code %d (%v) after the signal, want 0: the process died of it instead of stopping",
			code, proc.err)
	}
	if !strings.Contains(proc.output.String(), "multiverse started") {
		t.Errorf("the process never logged its start: %q", proc.output.String())
	}
	if conn, err := net.DialTimeout("tcp", addr, time.Second); err == nil {
		_ = conn.Close()
		t.Errorf("%s still accepts connections after the process exited", addr)
	}
}
