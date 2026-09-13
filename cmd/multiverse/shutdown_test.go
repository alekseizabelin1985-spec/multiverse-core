package main

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/runtime"
)

// poller is a context with the long-poll of the gateway on the process mux: it
// waits 25 s for deliveries and answers with an empty list when the process
// stops (C-01 v1.8, gateway-and-bot.md §5.1 p. 8).
type poller struct {
	witness
	entered chan struct{}
}

func (p *poller) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/test/deliveries", func(w http.ResponseWriter, r *http.Request) {
		p.entered <- struct{}{}
		wait := clock.RealTimers{}.After(25 * time.Second)
		defer wait.Stop()
		select {
		case <-runtime.ShuttingDown(r.Context()):
			_, _ = io.WriteString(w, `{"deliveries":[]}`)
		case <-wait.C():
			_, _ = io.WriteString(w, `{"waited":"25s"}`)
		}
	})
}

// pollAnswer is what the client of the long-poll got, and when.
type pollAnswer struct {
	body string
	at   time.Time
	err  error
}

// startLine hands over the address the process prints once it listens.
type startLine struct {
	once sync.Once
	addr chan string
}

func (s *startLine) Write(b []byte) (int, error) {
	line := string(b)
	if _, rest, ok := strings.Cut(line, " listening on "); ok {
		addr, _, _ := strings.Cut(rest, ",")
		s.once.Do(func() { s.addr <- addr })
	}
	return len(b), nil
}

// The DoD of T-446: the process stops its HTTP server before its contexts, and
// Shutdown does not cancel the requests it waits for. A long-poll of 25 s used
// to outlive ShutdownTimeout, the process logged "http shutdown" with the
// deadline and the gateway went on serving its outbox while it was being
// stopped. Now the long-poll answers within a second of the stop and the
// shutdown is clean.
func TestServeEndsALongPollWhenItStops(t *testing.T) {
	onLoopback(t)
	rec := &recorder{}
	probe := &poller{witness: witness{name: "gateway", rec: rec}, entered: make(chan struct{}, 1)}
	p := newProcess([]runtime.Context{probe}, recordingOpen(rec))
	line := &startLine{addr: make(chan string, 1)}
	p.stdout = line
	// Written by run only; read after run has returned.
	var logged strings.Builder
	p.log = slog.New(slog.NewTextHandler(&logged, nil))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.run(ctx, cancel) }()

	budget := clock.RealTimers{}.After(30 * time.Second)
	defer budget.Stop()
	var addr string
	select {
	case addr = <-line.addr:
	case err := <-done:
		t.Fatalf("the process ended before it listened: %v", err)
	case <-budget.C():
		t.Fatal("the process did not listen within 30s")
	}

	answered := make(chan pollAnswer, 1)
	go func() {
		resp, err := http.Get("http://" + addr + "/v1/test/deliveries")
		if err != nil {
			answered <- pollAnswer{err: err, at: clock.Real{}.Now()}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		answered <- pollAnswer{body: string(body), at: clock.Real{}.Now(), err: err}
	}()
	select {
	case <-probe.entered:
	case <-budget.C():
		t.Fatal("the long-poll did not reach the handler within 30s")
	}

	stoppedAt := clock.Real{}.Now()
	cancel()
	answer := <-answered
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}

	if answer.err != nil {
		t.Fatalf("the long-poll failed instead of answering: %v", answer.err)
	}
	if answer.body != `{"deliveries":[]}` {
		t.Errorf("the long-poll answered %q, want the empty list", answer.body)
	}
	if took := answer.at.Sub(stoppedAt); took >= time.Second {
		t.Errorf("the long-poll answered %s after the stop, want under 1s", took)
	}
	if strings.Contains(logged.String(), "http shutdown") {
		t.Errorf("the shutdown of the HTTP server failed: %s", logged.String())
	}
	want := []string{"start gateway", "stop gateway", "bus closed"}
	if got := rec.list(); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("lifetime = %q, want %q", got, want)
	}
}
