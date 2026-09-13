package openaicompat_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.uber.org/goleak"

	"multiverse-core.io/internal/llm"
	openaicompat "multiverse-core.io/internal/llm/providers/openai_compat"
	"multiverse-core.io/shared/clock"
)

// waitFor fails the test when ch is not closed within a generous bound. The
// bound is on real timers: the server and the client run on real sockets.
func waitFor(t *testing.T, ch <-chan struct{}, what string) {
	t.Helper()
	timer := clock.RealTimers{}.After(10 * time.Second)
	defer timer.Stop()
	select {
	case <-ch:
	case <-timer.C():
		t.Fatalf("timed out waiting for %s", what)
	}
}

// TestCancelClosesTheConnection cancels a call while the server is answering:
// before the head of the answer, and in the middle of its body. The server sees
// the connection closed, the call returns the error of the context, and no
// goroutine of the provider, the transport or the server is left (ADR-014 p. 2).
func TestCancelClosesTheConnection(t *testing.T) {
	for _, tc := range []struct {
		name string
		head bool
	}{
		{name: "before the head of the answer"},
		{name: "in the middle of the body", head: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

			answering := make(chan struct{})
			closed := make(chan struct{})
			stop := make(chan struct{})
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// The server notices a closed connection only once the body of the
				// request is read to its end.
				_, _ = io.Copy(io.Discard, r.Body)
				if tc.head {
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"Вол`))
					w.(http.Flusher).Flush()
				}
				close(answering)
				select {
				case <-r.Context().Done():
					close(closed)
				case <-stop:
				}
			}))
			defer srv.Close()
			defer close(stop)
			p, err := openaicompat.New(llm.Config{URL: srv.URL})
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			defer p.CloseIdleConnections()

			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan struct{})
			var genErr error
			go func() {
				defer close(done)
				_, genErr = p.Generate(ctx, narrativeRequest())
			}()
			waitFor(t, answering, "the server to start answering")
			cancel()
			waitFor(t, done, "Generate to return")
			waitFor(t, closed, "the server to see the connection closed")

			if !errors.Is(genErr, context.Canceled) {
				t.Errorf("Generate error = %v, want context.Canceled", genErr)
			}
			if errors.Is(genErr, llm.ErrUnavailable) {
				t.Errorf("a cancelled call is reported as unavailable: %v", genErr)
			}
		})
	}
}

func TestTheTimeoutOfTheRequestBoundsTheCall(t *testing.T) {
	defer goleak.VerifyNone(t, goleak.IgnoreCurrent())

	stop := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		select {
		case <-r.Context().Done():
		case <-stop:
		}
	}))
	defer srv.Close()
	defer close(stop)
	p, err := openaicompat.New(llm.Config{URL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer p.CloseIdleConnections()
	req := narrativeRequest()
	req.Timeout = 50 * time.Millisecond
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, err = p.Generate(context.Background(), req)
	}()
	waitFor(t, done, "Generate to give up after the timeout of the request")
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Generate error = %v, want context.DeadlineExceeded", err)
	}
}
