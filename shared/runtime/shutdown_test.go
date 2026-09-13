package runtime_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/runtime"
)

// longPollWait is the wait of the long-poll of the gateway (C-08: wait_ms up
// to 25 s), five times ShutdownTimeout.
const longPollWait = 25 * time.Second

// mountLongPoll mounts a long-poll the way the gateway writes one: it answers
// with an empty list when the process stops, and only otherwise after the
// whole wait. entered reports that the request reached the handler.
func mountLongPoll(mux *http.ServeMux, entered chan<- struct{}) {
	mux.HandleFunc("GET /v1/test/poll", func(w http.ResponseWriter, r *http.Request) {
		entered <- struct{}{}
		wait := clock.RealTimers{}.After(longPollWait)
		defer wait.Stop()
		select {
		case <-runtime.ShuttingDown(r.Context()):
			_, _ = io.WriteString(w, `{"deliveries":[]}`)
		case <-wait.C():
			_, _ = io.WriteString(w, `{"waited":"25s"}`)
		}
	})
}

type pollResult struct {
	body string
	err  error
}

func poll(url string) <-chan pollResult {
	out := make(chan pollResult, 1)
	go func() {
		resp, err := http.Get(url)
		if err != nil {
			out <- pollResult{err: err}
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, err := io.ReadAll(resp.Body)
		out <- pollResult{body: string(body), err: err}
	}()
	return out
}

// C-01 v1.8: Shutdown waits for the requests in flight and does not cancel
// their contexts, so a long-poll of 25 s used to hold Stop for the whole
// ShutdownTimeout of 5 s and then make it fail. The channel of ShuttingDown
// closes at the beginning of Stop: the long-poll answers with an empty list at
// once and Stop returns without an error.
func TestStopEndsALongPollAtOnce(t *testing.T) {
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	entered := make(chan struct{}, 1)
	mountLongPoll(srv.Mux, entered)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	result := poll("http://" + srv.Addr + "/v1/test/poll")
	select {
	case <-entered:
	case r := <-result:
		t.Fatalf("the long-poll ended before it reached the handler: %+v", r)
	}

	began := clock.Real{}.Now()
	stopErr := srv.Stop(context.Background())
	stopped := clock.Real{}.Now().Sub(began)

	if stopErr != nil {
		t.Errorf("Stop = %v, want nil: the long-poll held the shutdown past ShutdownTimeout", stopErr)
	}
	if stopped >= time.Second {
		t.Errorf("Stop took %s with a long-poll in flight, want under 1s", stopped)
	}
	r := <-result
	if r.err != nil {
		t.Fatalf("the long-poll failed instead of answering: %v", r.err)
	}
	if r.body != `{"deliveries":[]}` {
		t.Errorf("the long-poll answered %q, want the empty list it gives on shutdown", r.body)
	}
}

// The channel is the process's, not a request's: every request in flight sees
// it close, and a second Stop does not close it twice.
func TestShuttingDownReachesEveryRequestAndStopIsRepeatable(t *testing.T) {
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	entered := make(chan struct{}, 2)
	mountLongPoll(srv.Mux, entered)
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	first := poll("http://" + srv.Addr + "/v1/test/poll")
	second := poll("http://" + srv.Addr + "/v1/test/poll")
	<-entered
	<-entered

	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("Stop: %v", err)
	}
	for i, result := range []<-chan pollResult{first, second} {
		if r := <-result; r.err != nil || r.body != `{"deliveries":[]}` {
			t.Errorf("long-poll %d = %+v, want the empty list", i+1, r)
		}
	}
	if err := srv.Stop(context.Background()); err != nil {
		t.Errorf("second Stop = %v, want nil", err)
	}
}

// Stop before Start leaves the channel open: a server stopped before it ever
// served still serves requests nothing is stopping (review #1 of T-446, N-2).
func TestStopBeforeStartLeavesTheServerStartable(t *testing.T) {
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	if err := srv.Stop(context.Background()); err != nil {
		t.Fatalf("Stop before Start = %v, want nil", err)
	}
	stopping := make(chan bool, 1)
	srv.Mux.HandleFunc("GET /v1/test/stopping", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-runtime.ShuttingDown(r.Context()):
			stopping <- true
		default:
			stopping <- false
		}
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start after Stop: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	if r := <-poll("http://" + srv.Addr + "/v1/test/stopping"); r.err != nil {
		t.Fatalf("GET: %v", r.err)
	}
	if <-stopping {
		t.Error("ShuttingDown is closed for a server that was started after an early Stop: every long-poll would answer at once")
	}
}

// A request that did not come through the process server has nothing stopping
// it: the channel is nil and a select on it never fires.
func TestShuttingDownOutsideTheProcessServerNeverCloses(t *testing.T) {
	if ch := runtime.ShuttingDown(context.Background()); ch != nil {
		t.Errorf("ShuttingDown(Background) = %v, want a nil channel", ch)
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/test/poll", nil)
	if ch := runtime.ShuttingDown(req.Context()); ch != nil {
		t.Errorf("ShuttingDown of an httptest request = %v, want a nil channel", ch)
	}
}

// slowWriter is a context whose route streams a chunk every 20 ms for up to
// five seconds under a write deadline of writeBudget. It is started through
// StartAll with a manual clock in Deps that nothing advances — the clock a
// context has in replay — so a deadline taken from the clock of the context
// would lie at the zero time, in the past.
type slowWriter struct {
	fake
	firstFailure chan time.Duration
	setErr       chan error
}

const writeBudget = 300 * time.Millisecond

func (s *slowWriter) Routes(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/test/slow", func(w http.ResponseWriter, _ *http.Request) {
		var wall clock.Real
		began := wall.Now()
		s.setErr <- runtime.SetDeadlines(w, 0, writeBudget)
		rc := http.NewResponseController(w)
		tick := clock.RealTimers{}.Every(20 * time.Millisecond)
		defer tick.Stop()
		for wall.Now().Sub(began) < 5*time.Second {
			_, err := io.WriteString(w, strings.Repeat("x", 512)+"\n")
			if err == nil {
				err = rc.Flush()
			}
			if err != nil {
				s.firstFailure <- wall.Now().Sub(began)
				return
			}
			<-tick.C()
		}
		close(s.firstFailure)
	})
}

// C-01 v1.8: the deadline of a route is counted on the wall clock whatever
// clock the context holds. The write goes through for a while — the deadline
// is not in the past — and is cut once the budget is spent — the deadline does
// come — although the clock of the context never moves.
func TestSetDeadlinesCutsASlowWriteOnTheWallClock(t *testing.T) {
	manual := clock.NewManual(time.Time{})
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	writer := &slowWriter{
		fake:         fake{name: "gateway"},
		firstFailure: make(chan time.Duration, 1),
		setErr:       make(chan error, 1),
	}
	deps := runtime.Deps{Mux: srv.Mux, Clock: manual, Timers: manual.Timers(), Mode: runtime.ModeReplay}
	if err := runtime.StartAll(context.Background(), []runtime.Context{writer}, deps); err != nil {
		t.Fatalf("StartAll: %v", err)
	}
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	result := poll("http://" + srv.Addr + "/v1/test/slow")
	if err := <-writer.setErr; err != nil {
		t.Fatalf("SetDeadlines = %v, want nil on the writer of the process server", err)
	}
	failedAt, cut := <-writer.firstFailure
	if !cut {
		t.Fatalf("the write went on for 5s under a write deadline of %s", writeBudget)
	}
	if failedAt < writeBudget-100*time.Millisecond {
		t.Errorf("the write failed after %s, before the budget of %s: the deadline was not counted from now",
			failedAt, writeBudget)
	}
	if failedAt > writeBudget+2*time.Second {
		t.Errorf("the write failed only after %s, long past the budget of %s", failedAt, writeBudget)
	}
	r := <-result
	if len(r.body) == 0 {
		t.Errorf("the client got nothing before the cut (%v): the write never went through", r.err)
	}
	if manual.Now() != (time.Time{}) {
		t.Errorf("the manual clock moved to %s: nothing but the test may move it", manual.Now())
	}
}

// The read direction is set on its own: a body that trickles in slower than
// the read deadline is cut, while no write deadline is imposed on the answer.
func TestSetDeadlinesCutsASlowBody(t *testing.T) {
	const readBudget = 300 * time.Millisecond
	srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
	readFailed := make(chan time.Duration, 1)
	srv.Mux.HandleFunc("POST /v1/test/upload", func(w http.ResponseWriter, r *http.Request) {
		began := clock.Real{}.Now()
		if err := runtime.SetDeadlines(w, readBudget, 0); err != nil {
			t.Errorf("SetDeadlines: %v", err)
		}
		_, err := io.ReadAll(r.Body)
		if err == nil {
			close(readFailed)
			return
		}
		readFailed <- clock.Real{}.Now().Sub(began)
	})
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = srv.Stop(context.Background()) })

	body, feed := io.Pipe()
	defer func() { _ = feed.Close() }()
	go func() {
		tick := clock.RealTimers{}.Every(50 * time.Millisecond)
		defer tick.Stop()
		for i := 0; i < 100; i++ {
			if _, err := feed.Write([]byte("x")); err != nil {
				return
			}
			<-tick.C()
		}
		_ = feed.Close()
	}()
	go func() {
		resp, err := http.Post("http://"+srv.Addr+"/v1/test/upload", "text/plain", body)
		if err == nil {
			_ = resp.Body.Close()
		}
	}()

	failedAt, cut := <-readFailed
	if !cut {
		t.Fatalf("the body was read to its end over 5s under a read deadline of %s", readBudget)
	}
	if failedAt < readBudget-100*time.Millisecond || failedAt > readBudget+2*time.Second {
		t.Errorf("the read failed after %s, want about the budget of %s", failedAt, readBudget)
	}
}

// The read deadline of a request without a body reaches the background read
// the server keeps on the connection, so its end cancels r.Context() while the
// handler still runs, and the answer still goes through; with 0 the context
// lives past the same point (review #1 of T-446, Ma-2). A long-poll that got
// the read deadline of an ordinary route would lose its context halfway
// through the wait. The windows are wide on purpose: the race job runs on a
// slow runner.
func TestAReadDeadlineCancelsTheContextOfARequestWithoutABody(t *testing.T) {
	const readBudget = 300 * time.Millisecond
	cases := []struct {
		name     string
		read     time.Duration
		hold     time.Duration
		canceled bool
	}{
		{"read deadline", readBudget, 5 * time.Second, true},
		{"no read deadline", 0, readBudget + time.Second, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := runtime.NewHTTP("127.0.0.1:0", runtime.OK)
			canceledAt := make(chan time.Duration, 1)
			srv.Mux.HandleFunc("GET /v1/test/wait", func(w http.ResponseWriter, r *http.Request) {
				var wall clock.Real
				began := wall.Now()
				if err := runtime.SetDeadlines(w, tc.read, 0); err != nil {
					t.Errorf("SetDeadlines: %v", err)
				}
				hold := clock.RealTimers{}.After(tc.hold)
				defer hold.Stop()
				select {
				case <-r.Context().Done():
					canceledAt <- wall.Now().Sub(began)
				case <-hold.C():
					close(canceledAt)
				}
				_, _ = io.WriteString(w, "answered")
			})
			if err := srv.Start(); err != nil {
				t.Fatalf("Start: %v", err)
			}
			t.Cleanup(func() { _ = srv.Stop(context.Background()) })

			result := poll("http://" + srv.Addr + "/v1/test/wait")
			at, canceled := <-canceledAt
			switch {
			case canceled && !tc.canceled:
				t.Errorf("r.Context() was canceled after %s without a read deadline", at)
			case !canceled && tc.canceled:
				t.Errorf("r.Context() lived through %s under a read deadline of %s", tc.hold, tc.read)
			case canceled && (at < readBudget-100*time.Millisecond || at > readBudget+2*time.Second):
				t.Errorf("r.Context() was canceled after %s, want about the read deadline of %s", at, readBudget)
			}
			if r := <-result; r.err != nil || r.body != "answered" {
				t.Errorf("the client got %+v, want the answer the handler wrote", r)
			}
		})
	}
}

// A writer that cannot set deadlines says so instead of pretending it did.
func TestSetDeadlinesReportsAWriterWithoutDeadlines(t *testing.T) {
	err := runtime.SetDeadlines(httptest.NewRecorder(), time.Second, time.Second)
	if !errors.Is(err, http.ErrNotSupported) {
		t.Fatalf("SetDeadlines(recorder) = %v, want http.ErrNotSupported", err)
	}
	if !strings.Contains(err.Error(), "read deadline") || !strings.Contains(err.Error(), "write deadline") {
		t.Errorf("SetDeadlines(recorder) = %q, want both directions named", err)
	}
	if err := runtime.SetDeadlines(httptest.NewRecorder(), 0, 0); err != nil {
		t.Errorf("SetDeadlines(recorder, 0, 0) = %v, want nil: nothing was asked", err)
	}
}
