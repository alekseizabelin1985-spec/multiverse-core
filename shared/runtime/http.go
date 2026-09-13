package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/env"
)

// ShutdownTimeout is the grace period the process HTTP server gets on Stop.
const ShutdownTimeout = 5 * time.Second

// Timeouts of the process HTTP server, shared by every context it serves
// (C-01 v1.8). There is deliberately no ReadTimeout and no WriteTimeout on the
// server: either would become a hidden ceiling for every route of every context
// — the long-poll of the gateway, the admin proxy, a future stream. A route
// that needs a limit sets it on its own request with SetDeadlines.
const (
	ReadHeaderTimeout = 5 * time.Second
	IdleTimeout       = 120 * time.Second
)

// Headers of the admin envelope (C-01, C-06, ADR-009 p. 9).
const (
	ActorKindHeader = "X-Actor-Kind"
	ClientIDHeader  = "X-Client-Id"
)

// EnvAdminClients names the variable holding the comma separated client ids
// allowed on /v1/admin/*; see AdminOnly. The value is read through the
// manifest of shared/env, which is where its default and its documentation
// live (NFR-074).
var EnvAdminClients = env.CoreAdminClients.Name()

// actorKinds is the actor_kind enum of the envelope (C-01, ADR-007 p. 1).
var actorKinds = map[string]struct{}{
	"human":  {},
	"ci":     {},
	"sim":    {},
	"system": {},
}

// HTTP is the HTTP server owned by the process, not by a context. It serves
// GET /health and whatever routes contexts mounted on Mux.
type HTTP struct {
	Addr string
	Mux  *http.ServeMux

	srv     *http.Server
	lis     net.Listener
	errored chan error

	// stopping is closed at the beginning of Stop and reaches every request
	// through the base context of the server; see ShuttingDown.
	stopping chan struct{}
	stopOnce sync.Once
}

// NewHTTP returns the process server bound to addr with GET /health wired to
// the aggregate health function.
func NewHTTP(addr string, health func() Status) *HTTP {
	mux := http.NewServeMux()
	h := &HTTP{Addr: addr, Mux: mux, errored: make(chan error, 1), stopping: make(chan struct{})}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeStatus(w, health())
	})
	return h
}

func writeStatus(w http.ResponseWriter, s Status) {
	code := http.StatusOK
	if s.Status == StatusFail {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(s)
}

// Start binds the listener and serves in the background. The bound address is
// available in Addr afterwards, which matters when the port was 0.
func (h *HTTP) Start() error {
	lis, err := net.Listen("tcp", h.Addr)
	if err != nil {
		return err
	}
	h.lis = lis
	h.Addr = lis.Addr().String()
	h.srv = &http.Server{
		Handler:           h.Mux,
		ReadHeaderTimeout: ReadHeaderTimeout,
		IdleTimeout:       IdleTimeout,
		BaseContext: func(net.Listener) context.Context {
			return context.WithValue(context.Background(), shuttingDownKey{}, (<-chan struct{})(h.stopping))
		},
	}
	go func() {
		if err := h.srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.errored <- err
		}
		close(h.errored)
	}()
	return nil
}

// Stop shuts the server down gracefully. It first closes the channel of
// ShuttingDown, so that a request waiting longer than ShutdownTimeout — a
// long-poll — answers now instead of holding Shutdown until it gives up:
// Shutdown does not cancel the contexts of the requests it waits for, and the
// process stops the server before its contexts (C-01 v1.8).
//
// Stop before Start does nothing, the channel included, so the server can still
// be started. A server that was started and stopped is not started again: its
// channel stays closed and every long-poll would answer at once.
func (h *HTTP) Stop(ctx context.Context) error {
	if h.srv == nil {
		return nil
	}
	h.stopOnce.Do(func() { close(h.stopping) })
	ctx, cancel := context.WithTimeout(ctx, ShutdownTimeout)
	defer cancel()
	return h.srv.Shutdown(ctx)
}

type shuttingDownKey struct{}

// ShuttingDown returns the channel that closes when the process HTTP server
// begins to stop. ctx is the context of a request the server serves
// (r.Context()). A handler that may wait longer than an ordinary request
// listens to it and answers normally when it closes — the long-poll of the
// gateway with an empty list of deliveries.
//
// A context that did not come from the process server — a request built by
// httptest, context.Background() — yields a nil channel, which never closes:
// nothing is stopping that request.
func ShuttingDown(ctx context.Context) <-chan struct{} {
	ch, _ := ctx.Value(shuttingDownKey{}).(<-chan struct{})
	return ch
}

// SetDeadlines sets the read and the write deadline of one request through
// http.ResponseController, counted from the wall clock in every mode. A
// duration of zero or less leaves that deadline as it is, so a route can limit
// one direction only. The write deadline covers the whole response, and a
// write past it fails.
//
// The read deadline covers more than what is left of the body. Once the body
// is read — at once for a request without one, a GET — the server keeps
// reading the connection in the background to notice a client that went away,
// and the read deadline applies to that read as well: when it passes, the
// server takes it for a lost client and cancels r.Context(), although the
// handler is still running and its response still goes through. A handler that
// runs longer than an ordinary request and uses its context — listens to
// r.Context().Done(), hands it to storage — therefore gets a read deadline no
// shorter than the whole handler, the long-poll wait plus its margin, or 0,
// which leaves the connection without a read deadline: the process server has
// no ReadTimeout.
//
// The error is the one of the ResponseController — http.ErrNotSupported when
// the writer cannot set deadlines — with both directions joined.
func SetDeadlines(w http.ResponseWriter, read, write time.Duration) error {
	rc := http.NewResponseController(w)
	// Deadlines of the transport are wall-clock deadlines in every mode, like
	// the redelivery timers of the bus (C-01 v1.4). A context has only
	// Deps.Clock, which in replay is a manual clock or the clock of the
	// journal, and a deadline taken from it would lie in the past or never come.
	now := clock.Real{}.Now()
	var errs []error
	if read > 0 {
		if err := rc.SetReadDeadline(now.Add(read)); err != nil {
			errs = append(errs, fmt.Errorf("read deadline: %w", err))
		}
	}
	if write > 0 {
		if err := rc.SetWriteDeadline(now.Add(write)); err != nil {
			errs = append(errs, fmt.Errorf("write deadline: %w", err))
		}
	}
	return errors.Join(errs...)
}

// Err reports a serve error if the server stopped on its own.
func (h *HTTP) Err() <-chan error { return h.errored }

// AdminOnly rejects requests that do not come from a client allowed on
// /v1/admin/*. Admission is decided by the client, not by the kind of actor
// (ADR-009 p. 9, C-06): X-Client-Id must be listed in MV_CORE_ADMIN_CLIENTS
// (comma separated; unset, it is the default declared by
// env.CoreAdminClients), and a missing or unlisted one is rejected.
// X-Actor-Kind is optional and only validated against the envelope enum
// human|ci|sim|system: it grants nothing on its own, the operator proxied by
// the gateway arrives with human. Contexts wrap their own /v1/admin/* handlers
// with it; /health stays open because the port is published on loopback only.
//
// The allow-list is read once, when the middleware is built.
func AdminOnly(next http.Handler) http.Handler {
	allowed := adminClients(env.CoreAdminClients.List())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := allowed[r.Header.Get(ClientIDHeader)]; !ok {
			forbid(w, "admin routes require header "+ClientIDHeader+" of a client listed in "+EnvAdminClients)
			return
		}
		if kind := r.Header.Get(ActorKindHeader); kind != "" {
			if _, ok := actorKinds[kind]; !ok {
				forbid(w, "header "+ActorKindHeader+" must be one of human|ci|sim|system")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// adminClients indexes the allow-list. Empty entries are already dropped by
// env.Var.List, so a request without X-Client-Id never matches.
func adminClients(list []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(list))
	for _, id := range list {
		allowed[id] = struct{}{}
	}
	return allowed
}

func forbid(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
}
