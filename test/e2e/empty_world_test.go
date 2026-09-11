//go:build e2e

// The end-to-end scenario of the empty world: the single binary of the
// platform starts every bounded context on the in-process bus and answers the
// health probe compose asks it (design.md §10, ADR-001, tasks.md T-018).
//
// It runs the command, not a copy of its wiring: the point is that
// "multiverse --contexts=all --bus=memory" is a thing that works, and a test
// that assembled the same contexts by hand would say nothing about the flags,
// the listener or the exit path.

package e2e_test

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/testkit"
)

// startupBudget is how long the process gets to bind its port and answer. It
// is generous on purpose: a cold Windows machine with a virus scanner in front
// of a freshly linked binary is slow, and a flaky start is worse than a slow
// test.
const startupBudget = 60 * time.Second

func TestTheEmptyWorldAnswersTheHealthProbe(t *testing.T) {
	binary := build(t)
	addr := freeAddress(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// The address travels through the environment of the test process, which
	// the child inherits: replacing the whole environment of the child would
	// also drop the toolchain settings the build of the platform runs with,
	// and reading it back out is what shared/env exists to avoid (NFR-074).
	t.Setenv(env.CoreAddr.Name(), addr)
	// The empty world is the process without the fake of I1-α. A developer who
	// exported MV_SWARM_FAKE=true in their shell must not change what this test
	// proves — the child inherits the environment, runs from test/e2e without
	// rules/ and would refuse to start (review #1 of T-255, Mi-2).
	t.Setenv(env.SwarmFake.Name(), "false")
	cmd := exec.CommandContext(ctx, binary, "--contexts=all", "--bus=memory")
	var output strings.Builder
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Start(); err != nil {
		t.Fatalf("start %s: %v", binary, err)
	}
	t.Cleanup(func() {
		cancel()
		_ = cmd.Wait()
		if t.Failed() {
			t.Logf("output of the process:\n%s", output.String())
		}
	})

	status := probe(ctx, t, "http://"+addr+"/health")
	if status.Status != "ok" {
		t.Fatalf("/health says %q, want ok: %+v", status.Status, status.Details)
	}
	// Every context of the process is in the answer, or the probe is telling
	// the truth about a process that started less than it was asked to.
	if len(status.Details) == 0 {
		t.Error("/health names no contexts: --contexts=all started nothing")
	}
	for name, detail := range status.Details {
		if detail == nil {
			t.Errorf("context %q reports nothing", name)
		}
	}

	// The same probe through the subcommand compose actually calls: a healthy
	// process must make "multiverse health" exit zero.
	probeCmd := exec.CommandContext(ctx, binary, "health", "--url", "http://"+addr+"/health")
	if out, err := probeCmd.CombinedOutput(); err != nil {
		t.Errorf("multiverse health: %v\n%s", err, out)
	}
}

// build compiles the command into the temporary directory of the test. The
// binary is built rather than looked up so that the test cannot pass against
// yesterday's installation of the platform.
func build(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "multiverse")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	// -buildvcs=false is what the platform builds with everywhere (Makefile,
	// build/Dockerfile): stamping the revision needs a git binary the test
	// environment is not promised to have.
	cmd := exec.Command("go", "build", "-buildvcs=false", "-o", binary,
		"multiverse-core.io/cmd/multiverse")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build cmd/multiverse: %v\n%s", err, out)
	}
	return binary
}

// freeAddress reserves a port on the loopback interface and gives it up again,
// so that two runs of the suite do not fight over the default 8090.
//
// The gap between closing the listener and the process binding it is a race
// nothing can close from here — the process takes an address, not a listener —
// but the window is short and the port is one the operating system has just
// said is free.
func freeAddress(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve a port: %v", err)
	}
	addr := lis.Addr().String()
	if err := lis.Close(); err != nil {
		t.Fatalf("release the port: %v", err)
	}
	return addr
}

// health is the answer of GET /health (shared/runtime.Status).
type health struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details"`
}

// probe polls the endpoint until it answers or the budget runs out.
func probe(ctx context.Context, t *testing.T, url string) health {
	t.Helper()
	deadline := testkit.After(startupBudget)
	client := &http.Client{Timeout: 2 * time.Second}
	var last error
	for {
		status, err := get(ctx, client, url)
		if err == nil {
			return status
		}
		last = err
		select {
		case <-deadline:
			t.Fatalf("no answer from %s within %s: %v", url, startupBudget, last)
		case <-clock.RealTimers{}.After(50 * time.Millisecond).C():
		}
	}
}

func get(ctx context.Context, client *http.Client, url string) (health, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return health{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return health{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	var status health
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return health{}, err
	}
	return status, nil
}
