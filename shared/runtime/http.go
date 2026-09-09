package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// ShutdownTimeout is the grace period the process HTTP server gets on Stop.
const ShutdownTimeout = 5 * time.Second

// Headers of the admin envelope (C-01, C-06, ADR-009 p. 9).
const (
	ActorKindHeader = "X-Actor-Kind"
	ClientIDHeader  = "X-Client-Id"

	// EnvAdminClients holds the comma separated client ids allowed on
	// /v1/admin/*; see AdminOnly and .env.example.
	EnvAdminClients = "MV_CORE_ADMIN_CLIENTS"
	// DefaultAdminClients is the list used when EnvAdminClients is unset.
	DefaultAdminClients = "operator"
)

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
}

// NewHTTP returns the process server bound to addr with GET /health wired to
// the aggregate health function.
func NewHTTP(addr string, health func() Status) *HTTP {
	mux := http.NewServeMux()
	h := &HTTP{Addr: addr, Mux: mux, errored: make(chan error, 1)}
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
	h.srv = &http.Server{Handler: h.Mux, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := h.srv.Serve(lis); err != nil && !errors.Is(err, http.ErrServerClosed) {
			h.errored <- err
		}
		close(h.errored)
	}()
	return nil
}

// Stop shuts the server down gracefully.
func (h *HTTP) Stop(ctx context.Context) error {
	if h.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(ctx, ShutdownTimeout)
	defer cancel()
	return h.srv.Shutdown(ctx)
}

// Err reports a serve error if the server stopped on its own.
func (h *HTTP) Err() <-chan error { return h.errored }

// AdminOnly rejects requests that do not come from a client allowed on
// /v1/admin/*. Admission is decided by the client, not by the kind of actor
// (ADR-009 p. 9, C-06): X-Client-Id must be listed in MV_CORE_ADMIN_CLIENTS
// (comma separated, DefaultAdminClients when unset), and a missing or unlisted
// one is rejected. X-Actor-Kind is optional and only validated against the
// envelope enum human|ci|sim|system: it grants nothing on its own, the operator
// proxied by the gateway arrives with human. Contexts wrap their own
// /v1/admin/* handlers with it; /health stays open because the port is
// published on loopback only.
//
// The allow-list is read once, when the middleware is built.
func AdminOnly(next http.Handler) http.Handler {
	allowed := adminClients(envOr(EnvAdminClients, DefaultAdminClients))
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

// adminClients parses the allow-list. Empty entries are dropped, so a request
// without X-Client-Id never matches.
func adminClients(list string) map[string]struct{} {
	allowed := make(map[string]struct{})
	for _, id := range strings.Split(list, ",") {
		if id = strings.TrimSpace(id); id != "" {
			allowed[id] = struct{}{}
		}
	}
	return allowed
}

func forbid(w http.ResponseWriter, reason string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusForbidden)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": reason})
}

// EnvAddr reads a listen address from the environment, falling back to def.
//
// Reading the environment lives here because the linter forbids os.Getenv
// outside shared/*; shared/env (F-5, T-007) replaces this helper with the
// declared manifest.
func EnvAddr(key, def string) string { return envOr(key, def) }

func envOr(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}
