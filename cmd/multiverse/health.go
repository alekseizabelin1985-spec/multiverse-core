package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// healthTimeout keeps the probe well inside the compose healthcheck timeout.
const healthTimeout = 3 * time.Second

// runHealth implements "multiverse health --url <addr>": the healthcheck of
// the distroless image, which has neither shell nor curl (infrastructure.md
// §2.3). It exits 0 when the endpoint answers 200 with status ok or degraded,
// and 1 otherwise: fail (503), another code, an address that does not answer,
// a body that does not read, a status outside the three.
//
// degraded is a process that works (infrastructure.md §7.2): a world waiting
// for mvctl world init, a model that is away. Its container is healthy, or
// make up --wait and depends_on: service_healthy would wait for what no restart
// mends. The strict view of the operator is make health (state-and-mechanics.md
// §19, T-059).
func runHealth(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("multiverse health", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", defaultHealthURL(), "health endpoint to probe")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	status, err := probe(*url)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "multiverse health:", err)
		return 1
	}
	_, _ = fmt.Fprintln(stdout, status.Status)
	switch status.Status {
	case runtime.StatusOK, runtime.StatusDegraded:
		return 0
	default:
		return 1
	}
}

// defaultHealthURL turns the listen address of the process into one a client
// can dial. MV_CORE_ADDR is a bind address and its shipped value is ":8090"
// (infrastructure.md §4.2), so a plain concatenation yields "http://:8090/health"
// — a URL with no host, which no NO_PROXY entry matches and which therefore
// leaves through HTTP_PROXY instead of reaching the local server. A wildcard
// bind is dialled on loopback for the same reason: the probe runs beside the
// process it probes.
func defaultHealthURL() string {
	addr := env.CoreAddr.String()
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr + "/health"
	}
	switch host {
	case "", "0.0.0.0", "::":
		host = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(host, port) + "/health"
}

func probe(url string) (runtime.Status, error) {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return runtime.Status{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return runtime.Status{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return runtime.Status{}, fmt.Errorf("%s: unexpected status %d", url, resp.StatusCode)
	}
	var status runtime.Status
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		return runtime.Status{}, fmt.Errorf("%s: %w", url, err)
	}
	return status, nil
}
