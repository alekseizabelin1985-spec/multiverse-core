package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/access"
	"multiverse-core.io/cmd/telegram-bot/internal/deliver"
	"multiverse-core.io/shared/env"
)

// statusOK is the status of a bot that runs and delivers; statusDegraded of
// one that runs and does not deliver. The shape of the answer is that of
// runtime.Status of the platform, {status, details}, so that one probe reads
// both processes.
const (
	statusOK       = "ok"
	statusDegraded = "degraded"
)

// healthTimeout keeps the probe well inside the timeout of the compose
// healthcheck (5 s, docker-compose.bot.yml).
const healthTimeout = 3 * time.Second

// health is the answer of GET /health.
type health struct {
	Status  string         `json:"status"`
	Details map[string]any `json:"details,omitempty"`
}

// healthHandler serves GET /health: the bot is up, how many updates the access
// gate refused since start (review #1 of T-310, M-1), and whether it delivers
// — degraded after DegradedAfterFailedPolls failed long-polls in a row or once
// Telegram refused the token on a send (acceptance of T-312). The counters are
// numbers only, never who was refused. A degraded bot still answers 200: the
// status tells the probe, which exits 1 on anything but ok.
func healthHandler(counters func() access.Counters, deliveries func() deliver.Health) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		c, d := counters(), deliveries()
		status := statusOK
		if d.Degraded() {
			status = statusDegraded
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(health{Status: status, Details: map[string]any{
			"bot_denied_total":            c.Denied,
			"bot_not_private_total":       c.NotPrivate,
			"bot_too_often_total":         c.TooOften,
			"deliveries_failed_polls":     d.FailedPolls,
			"telegram_token_unauthorized": d.Unauthorized,
		}})
	})
	return mux
}

// runHealth implements "telegram-bot health [--url <url>]", the healthcheck of
// the image (docker-compose.bot.yml). It exits 0 when the endpoint answers 200
// with status ok, 1 otherwise, 2 on a wrong command line.
func runHealth(args []string, e environment) int {
	fs := flag.NewFlagSet("telegram-bot health", flag.ContinueOnError)
	fs.SetOutput(e.stderr)
	url := fs.String("url", defaultHealthURL(env.TelegramHealthAddr.StringFrom(e.source)), "health endpoint to probe")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		_, _ = fmt.Fprintf(e.stderr, "telegram-bot health: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	status, err := probe(*url)
	if err != nil {
		_, _ = fmt.Fprintln(e.stderr, "telegram-bot health:", err)
		return 1
	}
	_, _ = fmt.Fprintln(e.stdout, status)
	if status != statusOK {
		return 1
	}
	return 0
}

// defaultHealthURL turns the listen address of /health into one a client can
// dial, as cmd/multiverse does for MV_CORE_ADDR: the shipped value is ":8089",
// and a wildcard bind is dialled on loopback, since the probe runs beside the
// process it probes.
func defaultHealthURL(addr string) string {
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

// probeClient never goes through a proxy: the probe asks the process next to
// it, and HTTP_PROXY of the container must not carry that request away.
var probeClient = &http.Client{Transport: &http.Transport{Proxy: nil}}

func probe(url string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), healthTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := probeClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: unexpected status %d", url, resp.StatusCode)
	}
	var h health
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return "", fmt.Errorf("%s: %w", url, err)
	}
	return h.Status, nil
}
