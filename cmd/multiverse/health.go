package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"time"

	"multiverse-core.io/shared/runtime"
)

// healthTimeout keeps the probe well inside the compose healthcheck timeout.
const healthTimeout = 3 * time.Second

// runHealth implements "multiverse health --url <addr>": the healthcheck of
// the distroless image, which has neither shell nor curl (infrastructure.md
// §2.3). It exits 0 when the endpoint answers 200 with status ok.
func runHealth(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("multiverse health", flag.ContinueOnError)
	fs.SetOutput(stderr)
	url := fs.String("url", "http://"+defaultCoreAddr+"/health", "health endpoint to probe")
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
	if status.Status != runtime.StatusOK {
		return 1
	}
	return 0
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
