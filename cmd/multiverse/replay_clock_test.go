package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"multiverse-core.io/internal/replay"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/recording"
	"multiverse-core.io/shared/runtime"
)

// C-01 v1.9, the DoD of T-458 A6-A7: in replay on a recording the process reads
// the file once and hands the contexts that very recording in Deps.Recording,
// the one its clock started from. Live mode and replay without --recording
// hand nil and read no file.
func TestTheProcessHandsTheRecordingItReadOnceToTheContexts(t *testing.T) {
	path := writeSession(t)
	for name, tc := range map[string]struct {
		opts  serveOptions
		reads int
	}{
		"replay on a recording":    {opts: replayOptions(path), reads: 1},
		"replay without recording": {opts: replayOptions(""), reads: 0},
		"live":                     {opts: memoryOptions(), reads: 0},
	} {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			var reads int
			var read *recording.Recording
			started := make(chan runtime.Deps, 1)
			p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
			p.opts = tc.opts
			p.openRecording = func(path string) (*recording.Recording, error) {
				reads++
				rec, err := recording.Open(path)
				read = rec
				return rec, err
			}

			runUntilStarted(t, p, started, func(deps runtime.Deps) {
				if tc.reads == 0 {
					if deps.Recording != nil {
						t.Errorf("Deps.Recording = %p, want nil without a recording", deps.Recording)
					}
					return
				}
				if deps.Recording == nil || deps.Recording != read {
					t.Fatalf("Deps.Recording = %p, want the recording the process read (%p)", deps.Recording, read)
				}
				if got := deps.Recording.Len(); got != 2 {
					t.Errorf("Deps.Recording.Len() = %d, want the 2 events of the file", got)
				}
				start, ok := deps.Recording.Start()
				if !ok || !start.Equal(recordedAt) || !deps.Clock.Now().Equal(start) {
					t.Errorf("Deps.Recording.Start() = %v, %v and the clock at %v; want both at %v",
						start, ok, deps.Clock.Now(), recordedAt)
				}
			})
			if reads != tc.reads {
				t.Errorf("the recording was read %d times in the run, want %d", reads, tc.reads)
			}
		})
	}
}

// C-01 v1.11, the decision on A5 of the review of T-458 by system-architect
// (the probe of iteration 1 turned over): recording.ReadJournal over
// Deps.Journal of a process in replay is a read of history. It goes through the
// middleware of T-060 like any read of the contexts, but the middleware lets
// its handler through: the EventClock of the process stays at the start of the
// recording, and the events come out as the journal holds them, without
// meta.replay. The symptom it closes: a read of llm_records whose events are
// later than the first input used to put the clock past it, and the harness got
// 409 clock_behind on its first call. The control at the end is a delivery over
// the same journal and the same events, which does move the clock and mark the
// events, as in T-060.
func TestReadJournalInReplayLeavesTheClockAndTheEventsAsTheJournalHoldsThem(t *testing.T) {
	onLoopback(t)
	t.Setenv(env.CoreAdminClients.Name(), "ci-harness")
	started := make(chan runtime.Deps, 1)
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
	p.opts = replayOptions(writeSession(t))

	runListening(t, p, started, func(addr string, deps runtime.Deps) {
		spec, _ := deps.Contracts.Lookup("player.looked")
		first, second := lookedEvent("first"), lookedEvent("second")
		first.Timestamp, second.Timestamp = recordedAt.Add(2*time.Hour), recordedAt.Add(time.Hour)
		for _, ev := range []eventbus.Event{first, second} {
			if ev.Meta.Replay {
				t.Fatal("the event is marked replayed before it was published: the probe proves nothing")
			}
			if err := deps.Bus.Publish(context.Background(), ev); err != nil {
				t.Fatalf("publish: %v", err)
			}
		}
		if now := deps.Clock.Now(); !now.Equal(recordedAt) {
			t.Fatalf("the clock is at %v before the read, want the start of the recording %v", now, recordedAt)
		}

		rec, err := recording.ReadJournal(context.Background(), deps.Journal, spec.Topic, 0)
		if err != nil {
			t.Fatalf("ReadJournal: %v", err)
		}
		if rec.Len() != 2 {
			t.Fatalf("Len = %d, want 2", rec.Len())
		}
		if now := deps.Clock.Now(); !now.Equal(recordedAt) {
			t.Errorf("after ReadJournal the clock is at %v, want it still at the start of the recording %v", now, recordedAt)
		}
		for ev := range rec.Events() {
			if ev.Meta.Replay {
				t.Errorf("%s came out of ReadJournal with meta.replay, the journal holds it without", ev.ID)
			}
		}

		// The first input of the scenario is earlier than every event read.
		at := recordedAt.Add(10 * time.Minute)
		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(at)); got.status != http.StatusNoContent {
			t.Errorf("the clock at the first input after ReadJournal: %d %+v, want 204", got.status, got.err)
		}
		if now := deps.Clock.Now(); !now.Equal(at) {
			t.Errorf("after 204 the clock is at %v, want %v", now, at)
		}

		var delivered []eventbus.Event
		if _, err := deps.Journal.ReadRange(context.Background(), spec.Topic, 0, 2, func(_ context.Context, ev eventbus.Event) error {
			delivered = append(delivered, ev)
			return nil
		}); err != nil {
			t.Fatalf("ReadRange: %v", err)
		}
		if now := deps.Clock.Now(); !now.Equal(first.Timestamp) {
			t.Errorf("control: after a delivery the clock is at %v, want the latest event %v", now, first.Timestamp)
		}
		if marks := replayMarks(delivered); len(marks) != 2 || !marks[0] || !marks[1] {
			t.Errorf("control: a delivery with marks %v, want 2 events marked replayed", marks)
		}
	})
}

func replayMarks(events []eventbus.Event) []bool {
	marks := make([]bool, 0, len(events))
	for _, ev := range events {
		marks = append(marks, ev.Meta.Replay)
	}
	return marks
}

// runListening runs the process until it prints the address it listens on,
// hands the address and the Deps of the probe to fn, then stops the process.
func runListening(t *testing.T, p process, started <-chan runtime.Deps, fn func(addr string, deps runtime.Deps)) {
	t.Helper()
	line := &startLine{addr: make(chan string, 1)}
	p.stdout = line
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.run(ctx, cancel) }()

	budget := clock.RealTimers{}.After(30 * time.Second)
	defer budget.Stop()
	var deps runtime.Deps
	var addr string
	for addr == "" {
		select {
		case deps = <-started:
		case addr = <-line.addr:
		case err := <-done:
			t.Fatalf("the process ended before it listened: %v", err)
		case <-budget.C():
			t.Fatal("the process did not listen within 30s")
		}
	}
	if deps.Clock == nil {
		deps = <-started
	}
	fn(addr, deps)
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("run: %v", err)
	}
}

// routeAnswer is an answer of the process on the route of the clock. The route
// answers its errors in the shape Error, {"error": {"code", "message"}} (C-01
// v1.11), decoded strictly into err. runtime.AdminOnly answers 403 with
// {"error": "<text>"}, kept in forbidden.
type routeAnswer struct {
	status    int
	header    http.Header
	err       *apiError
	forbidden string
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (a routeAnswer) code() string {
	if a.err == nil {
		return ""
	}
	return a.err.Code
}

func (a routeAnswer) message() string {
	if a.err == nil {
		return ""
	}
	return a.err.Message
}

func callClock(t *testing.T, addr, method, client, body string) routeAnswer {
	t.Helper()
	req, err := http.NewRequest(method, "http://"+addr+replay.ClockPath, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if client != "" {
		req.Header.Set(runtime.ClientIDHeader, client)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, replay.ClockPath, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	answer := routeAnswer{status: resp.StatusCode, header: resp.Header}
	if len(raw) == 0 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "application/json") {
		return answer
	}
	if resp.StatusCode == http.StatusForbidden {
		var forbidden struct {
			Error string `json:"error"`
		}
		if err := json.Unmarshal(raw, &forbidden); err != nil {
			t.Fatalf("403 body %q: %v", raw, err)
		}
		answer.forbidden = forbidden.Error
		return answer
	}
	var shaped struct {
		Error *apiError `json:"error"`
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&shaped); err != nil || shaped.Error == nil {
		t.Fatalf("%d body %q is not {\"error\": {\"code\", \"message\"}}: %v", resp.StatusCode, raw, err)
	}
	answer.err = shaped.Error
	return answer
}

func atBody(at time.Time) string { return `{"at":"` + at.Format(time.RFC3339Nano) + `"}` }

// C-01 v1.9, the DoD of T-458 B2: the route of the replay clock on the HTTP
// server of the process, every answer, and the time of the root events a
// context builds after it — which never goes back. The route is there in every
// replay, on a recording and without one (review #1 of T-458, Mi-1): the
// scenario without a recording (E2E-12, the harness of T-061) is the one that
// needs it.
func TestTheReplayClockRouteSetsTheTimeOfRootEvents(t *testing.T) {
	for name, tc := range map[string]struct {
		opts  func(t *testing.T) serveOptions
		start time.Time
	}{
		"on a recording":    {opts: func(t *testing.T) serveOptions { return replayOptions(writeSession(t)) }, start: recordedAt},
		"without recording": {opts: func(*testing.T) serveOptions { return replayOptions("") }, start: time.Time{}},
	} {
		t.Run(name, func(t *testing.T) {
			onLoopback(t)
			t.Setenv(env.CoreAdminClients.Name(), "ci-harness")
			started := make(chan runtime.Deps, 1)
			p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)
			p.opts = tc.opts(t)
			checkTheClockRoute(t, p, started, tc.start)
		})
	}
}

func checkTheClockRoute(t *testing.T, p process, started <-chan runtime.Deps, start time.Time) {
	t.Helper()
	at1 := recordedAt.Add(10*time.Minute + 500*time.Millisecond)
	at0 := recordedAt.Add(5 * time.Minute)
	at2 := recordedAt.Add(20 * time.Minute)
	root := func() time.Time { return lookedEvent("root").Timestamp }

	runListening(t, p, started, func(addr string, deps runtime.Deps) {
		if got := callClock(t, addr, http.MethodPost, "", atBody(at1)); got.status != http.StatusForbidden || got.forbidden == "" {
			t.Errorf("without X-Client-Id: %d %q, want 403 with an error", got.status, got.forbidden)
		}
		if got := callClock(t, addr, http.MethodPost, "stranger", atBody(at1)); got.status != http.StatusForbidden {
			t.Errorf("with an unlisted client: %d, want 403", got.status)
		}
		// The pattern carries the method, so the mux refuses another one with
		// Allow: POST before admission and before the handler (C-01 v1.11). The
		// body is the mux's own, and the contract does not fix it.
		for _, client := range []string{"ci-harness", ""} {
			if got := callClock(t, addr, http.MethodGet, client, ""); got.status != http.StatusMethodNotAllowed ||
				got.header.Get("Allow") != http.MethodPost {
				t.Errorf("GET (client %q): %d, Allow %q; want 405 with Allow: POST", client, got.status, got.header.Get("Allow"))
			}
		}
		got := callClock(t, addr, http.MethodPost, "ci-harness", `{"at":"yesterday"}`)
		if got.status != http.StatusBadRequest || got.code() != replay.CodeInvalidBody || got.message() == "" {
			t.Errorf("a bad at: %d %+v, want 400 with error.code %s and a message", got.status, got.err, replay.CodeInvalidBody)
		}
		if ts := root(); !ts.Equal(start) {
			t.Fatalf("a root before any accepted call is stamped %v, want the start %v: a refused call moved the clock", ts, start)
		}

		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(at1)); got.status != http.StatusNoContent {
			t.Fatalf("at1: %d %+v, want 204", got.status, got.err)
		}
		if ts := root(); !ts.Equal(at1) {
			t.Errorf("a root after 204 on at1 is stamped %v, want %v", ts, at1)
		}

		got = callClock(t, addr, http.MethodPost, "ci-harness", atBody(at0))
		if got.status != http.StatusConflict || got.code() != replay.CodeClockBehind || !strings.Contains(got.message(), "clock behind") {
			t.Errorf("at0 < at1: %d %+v, want 409 with error.code %s and a message", got.status, got.err, replay.CodeClockBehind)
		}
		if ts := root(); !ts.Equal(at1) {
			t.Errorf("a root after 409 on at0 is stamped %v, want still %v: time went back", ts, at1)
		}

		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(at1)); got.status != http.StatusNoContent {
			t.Errorf("at1 again: %d, want 204 for a time equal to the clock", got.status)
		}
		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(at2)); got.status != http.StatusNoContent {
			t.Fatalf("at2: %d %+v, want 204", got.status, got.err)
		}
		if ts := root(); !ts.Equal(at2) {
			t.Errorf("a root after 204 on at2 is stamped %v, want %v", ts, at2)
		}
		if now := deps.Clock.Now(); !now.Equal(at2) {
			t.Errorf("Deps.Clock is at %v, want %v: the route moved another clock", now, at2)
		}
	})
}

// adminProxy is a context that mounts a wildcard under /v1/admin/ with a
// method, the way the gateway mounts its routes and an admin proxy would:
// "POST /v1/admin/{path...}".
type adminProxy struct {
	witness
	hits atomic.Int32
}

func (a *adminProxy) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/admin/{path...}", func(w http.ResponseWriter, _ *http.Request) {
		a.hits.Add(1)
		w.WriteHeader(http.StatusAccepted)
	})
}

// C-01 v1.11 (review of T-458 by system-architect, У-2): the mux of the process
// is shared by the contexts, and http.ServeMux panics on two overlapping
// patterns neither of which is narrower. Mounted without a method, the route of
// the clock and "POST /v1/admin/{path...}" of a context failed the start of a
// replay. With "POST /v1/admin/replay/clock" the process starts, the clock
// takes its own path, and the wildcard keeps every other one.
func TestTheReplayClockRouteLivesBesideAWildcardAdminRoute(t *testing.T) {
	onLoopback(t)
	t.Setenv(env.CoreAdminClients.Name(), "ci-harness")
	started := make(chan runtime.Deps, 1)
	proxy := &adminProxy{witness: witness{name: "probe", rec: &recorder{}, started: started}}
	p := newProcess([]runtime.Context{proxy}, openBus)
	p.opts = replayOptions("")

	at := recordedAt.Add(time.Minute)
	runListening(t, p, started, func(addr string, deps runtime.Deps) {
		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(at)); got.status != http.StatusNoContent {
			t.Errorf("POST %s beside the wildcard: %d %+v, want 204 from the route of the clock", replay.ClockPath, got.status, got.err)
		}
		if now := deps.Clock.Now(); !now.Equal(at) {
			t.Errorf("the clock is at %v, want %v", now, at)
		}
		if n := proxy.hits.Load(); n != 0 {
			t.Errorf("the wildcard took %d calls of the route of the clock", n)
		}

		resp, err := http.Post("http://"+addr+"/v1/admin/other", "application/json", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusAccepted || proxy.hits.Load() != 1 {
			t.Errorf("POST /v1/admin/other: %d, hits %d; want 202 from the wildcard of the context", resp.StatusCode, proxy.hits.Load())
		}
	})
}

// In live mode the route does not exist: the wall clock is not set by anybody.
func TestTheReplayClockRouteIsNotMountedInLiveMode(t *testing.T) {
	onLoopback(t)
	t.Setenv(env.CoreAdminClients.Name(), "ci-harness")
	started := make(chan runtime.Deps, 1)
	p := newProcess([]runtime.Context{&witness{name: "probe", rec: &recorder{}, started: started}}, openBus)

	runListening(t, p, started, func(addr string, _ runtime.Deps) {
		if got := callClock(t, addr, http.MethodPost, "ci-harness", atBody(recordedAt)); got.status != http.StatusNotFound {
			t.Errorf("POST %s in live mode: %d, want 404", replay.ClockPath, got.status)
		}
	})
	if ts := lookedEvent("after").Timestamp; ts.Equal(recordedAt) {
		t.Error("the call in live mode set the time of the constructors")
	}
}
