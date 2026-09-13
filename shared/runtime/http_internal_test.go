package runtime

import (
	"context"
	"testing"
	"time"
)

// C-01 v1.8: the process server limits the headers and the idle connections
// and nothing else. A server-wide ReadTimeout or WriteTimeout would cut the
// long-poll of the gateway and every other route that legitimately takes
// longer than an ordinary request; a route limits itself with SetDeadlines.
func TestProcessServerTimeouts(t *testing.T) {
	h := NewHTTP("127.0.0.1:0", OK)
	if err := h.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = h.Stop(context.Background()) })

	if h.srv.ReadHeaderTimeout != 5*time.Second {
		t.Errorf("ReadHeaderTimeout = %s, want 5s", h.srv.ReadHeaderTimeout)
	}
	if h.srv.IdleTimeout != 120*time.Second {
		t.Errorf("IdleTimeout = %s, want 120s", h.srv.IdleTimeout)
	}
	if h.srv.ReadTimeout != 0 || h.srv.WriteTimeout != 0 {
		t.Errorf("ReadTimeout/WriteTimeout = %s/%s, want none on the server", h.srv.ReadTimeout, h.srv.WriteTimeout)
	}
	if ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %s, want 5s unchanged", ShutdownTimeout)
	}
}
