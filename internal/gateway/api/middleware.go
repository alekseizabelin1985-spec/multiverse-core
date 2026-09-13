package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"mime"
	"net/http"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
)

// Middleware wraps the handler of one route. It sees the Route, so a policy
// that belongs to some operations only (nolog, the long-poll guard, the
// timeout) is decided by operationId, not by matching URLs at request time.
type Middleware func(Route, http.Handler) http.Handler

// Limits of component §5.1 and C-08 v1.1.
const (
	// BodyLimit is the largest request body (SEC-11).
	BodyLimit = 64 << 10
	// RequestTimeout bounds a request that is not a long-poll.
	RequestTimeout = 5 * time.Second
)

// noLogOperations carry external IDs in their bodies: their log line holds
// request_id and code, nothing else, whatever went wrong (SEC-01, SEC-02). The
// spec marks the same operations x-nolog (openapi_test.go).
var noLogOperations = []string{"resolveLink", "consentLink", "forgetLink", "createCharacter"}

// longPollOperations hold the request open by design: one per client, and no
// request timeout. The limit of their answer comes with T-307 and the
// deadlines of the process server with T-446 (runtime.SetDeadlines,
// runtime.ShuttingDown): until then the timeout middleware leaves them alone,
// and this list is the one place T-307 connects the long-poll bound to.
var longPollOperations = []string{"pollDeliveries"}

// NoLogOperations returns the operations whose bodies and causes are never
// logged.
func NoLogOperations() []string { return slices.Clone(noLogOperations) }

// LongPollOperations returns the operations served as long-polls.
func LongPollOperations() []string { return slices.Clone(longPollOperations) }

// RateLimiter decides whether a request may proceed. It is the insertion point
// of the action rate limit of T-305 (30 per minute per player_id, SEC-11);
// retryAfter is the whole number of seconds sent in Retry-After.
type RateLimiter interface {
	Allow(r *http.Request, rt Route) (ok bool, retryAfter int)
}

// Config is what the middleware of the gateway needs. The gateway context
// builds the chain in Routes, before Start, and fills the fields in Start;
// the process HTTP server starts serving only after every Start returned, so
// the fields are never read before they are set.
type Config struct {
	// ClientIDs are the admitted X-Client-Id (MV_GATEWAY_CLIENT_IDS).
	ClientIDs []string
	// ActorKindClients may send X-Actor-Kind ci or sim
	// (MV_GATEWAY_ACTOR_KIND_CLIENTS).
	ActorKindClients []string
	// RequestIDs generates X-Request-Id.
	RequestIDs func() string
	Clock      clock.Clock
	Log        *slog.Logger
	// Limiter is nil until T-305.
	Limiter RateLimiter
	// Timeout is RequestTimeout when zero.
	Timeout time.Duration

	polls sync.Map // client_id → struct{}: the long-polls in flight
}

// Chain returns the middleware of component §5.1 for cfg in the order they
// wrap a handler, outermost first:
//
//  1. request_id and the access log — X-Request-Id on every answer, one log
//     line per request; for a nolog operation the line is request_id and code;
//  2. recover — a panic becomes 500 internal and handled=false;
//  3. client — client_unknown, invalid X-Actor-Kind, actor_kind_forbidden,
//     client_mismatch;
//  4. body_limit — 64 KiB (413 payload_too_large) and the JSON content type;
//  5. nolog is not a wrapper of its own: the policy is the operation, applied
//     by the access log of step 1, which must see every answer, including the
//     403 of step 3;
//  6. ratelimit — the insertion point of T-305;
//  7. pollguard — one long-poll per client (409 poll_in_progress);
//  8. timeout — RequestTimeout on the context of everything but a long-poll.
//
// The request id comes before recover, unlike the numbering of §5.1, so that
// the 500 of a panic still carries X-Request-Id and still reaches the log.
func Chain(cfg *Config) []Middleware {
	return []Middleware{cfg.requestLog, cfg.recoverPanics, cfg.admitClient, cfg.limitBody,
		cfg.rateLimit, cfg.guardPolls, cfg.timeout}
}

type ctxKey struct{}

// requestInfo is what the middleware learns about a request and the handlers
// read back: RequestID, ClientID and ActorKind.
type requestInfo struct {
	id, clientID, actorKind string
}

func infoFrom(ctx context.Context) *requestInfo {
	info, _ := ctx.Value(ctxKey{}).(*requestInfo)
	if info == nil {
		return &requestInfo{}
	}
	return info
}

// RequestIDFrom returns the X-Request-Id of the request of ctx.
func RequestIDFrom(ctx context.Context) string { return infoFrom(ctx).id }

// ClientIDFrom returns the admitted X-Client-Id of the request of ctx.
func ClientIDFrom(ctx context.Context) string { return infoFrom(ctx).clientID }

// ActorKindFrom returns the actor kind of the request of ctx; human unless the
// client was allowed another.
func ActorKindFrom(ctx context.Context) string { return infoFrom(ctx).actorKind }

// recorder remembers the status and the error code of an answer for the
// access log. WriteError reports the code through it.
type recorder struct {
	http.ResponseWriter
	status int
	code   string
}

func (r *recorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *recorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

func (r *recorder) setCode(code string) { r.code = code }

// Unwrap lets http.ResponseController reach the connection.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

type codeSetter interface{ setCode(string) }

func (c *Config) requestLog(rt Route, next http.Handler) http.Handler {
	noLog := slices.Contains(noLogOperations, rt.OperationID)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := c.Clock.Now()
		info := &requestInfo{id: c.RequestIDs(), actorKind: ActorHuman}
		w.Header().Set(HeaderRequestID, info.id)
		rec := &recorder{ResponseWriter: w}
		next.ServeHTTP(rec, r.WithContext(context.WithValue(r.Context(), ctxKey{}, info)))

		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		level := slog.LevelDebug
		switch {
		case status >= 500:
			level = slog.LevelError
		case status >= 400:
			level = slog.LevelWarn
		}
		attrs := []slog.Attr{slog.String("request_id", info.id)}
		if rec.code != "" {
			attrs = append(attrs, slog.String("code", rec.code))
		}
		if !noLog {
			attrs = append(attrs,
				slog.String("operation", rt.OperationID),
				slog.String("method", rt.Method),
				slog.String("route", rt.Path),
				slog.Int("status", status),
				slog.Int64("duration_ms", c.Clock.Now().Sub(start).Milliseconds()))
		}
		c.Log.LogAttrs(r.Context(), level, "http request", attrs...)
	})
}

func (c *Config) recoverPanics(rt Route, next http.Handler) http.Handler {
	noLog := slices.Contains(noLogOperations, rt.OperationID)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			v := recover()
			if v == nil {
				return
			}
			if v == http.ErrAbortHandler {
				panic(v)
			}
			attrs := []slog.Attr{slog.String("request_id", RequestIDFrom(r.Context())), slog.Bool("handled", false)}
			if !noLog {
				// The value of a panic is whatever the code panicked with; on a
				// nolog operation it may be a body, so it is not written there.
				attrs = append(attrs, slog.Any("panic", v), slog.String("stack", string(debug.Stack())))
			}
			c.Log.LogAttrs(r.Context(), slog.LevelError, "http handler panicked", attrs...)
			if rec, ok := w.(*recorder); ok && rec.status != 0 {
				return
			}
			_ = WriteError(w, NewError(CodeInternal, nil))
		}()
		next.ServeHTTP(w, r)
	})
}

func (c *Config) admitClient(rt Route, next http.Handler) http.Handler {
	mustMatch := strings.Contains(rt.Path, "{client_id}")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := r.Header.Get(HeaderClientID)
		if clientID == "" || !slices.Contains(c.ClientIDs, clientID) {
			_ = WriteError(w, NewError(CodeClientUnknown, nil))
			return
		}
		kind := r.Header.Get(HeaderActorKind)
		switch kind {
		case "":
			kind = ActorHuman
		case ActorHuman:
		case ActorCI, ActorSim:
			if !slices.Contains(c.ActorKindClients, clientID) {
				_ = WriteError(w, NewError(CodeActorKindForbidden, nil))
				return
			}
		default:
			_ = WriteError(w, NewError(CodeInvalidRequest, map[string]any{"header": HeaderActorKind}))
			return
		}
		if mustMatch && r.PathValue("client_id") != clientID {
			_ = WriteError(w, NewError(CodeClientMismatch, nil))
			return
		}
		info := infoFrom(r.Context())
		info.clientID, info.actorKind = clientID, kind
		next.ServeHTTP(w, r)
	})
}

func (c *Config) limitBody(_ Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ContentLength > BodyLimit {
			_ = WriteError(w, NewError(CodePayloadTooLarge, nil))
			return
		}
		if r.Body != nil && r.Body != http.NoBody {
			if ct := r.Header.Get("Content-Type"); ct != "" || r.ContentLength > 0 {
				if media, _, err := mime.ParseMediaType(ct); err != nil || media != "application/json" {
					_ = WriteError(w, NewError(CodeInvalidRequest, map[string]any{"header": "Content-Type"}))
					return
				}
			}
			r.Body = http.MaxBytesReader(w, r.Body, BodyLimit)
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Config) rateLimit(rt Route, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if c.Limiter != nil {
			if ok, retryAfter := c.Limiter.Allow(r, rt); !ok {
				w.Header().Set(HeaderRetryAfter, strconv.Itoa(max(retryAfter, 1)))
				_ = WriteError(w, NewError(CodeRateLimited, nil))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (c *Config) guardPolls(rt Route, next http.Handler) http.Handler {
	if !slices.Contains(longPollOperations, rt.OperationID) {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientID := ClientIDFrom(r.Context())
		if _, busy := c.polls.LoadOrStore(clientID, struct{}{}); busy {
			_ = WriteError(w, NewError(CodePollInProgress, nil))
			return
		}
		defer c.polls.Delete(clientID)
		next.ServeHTTP(w, r)
	})
}

func (c *Config) timeout(rt Route, next http.Handler) http.Handler {
	if slices.Contains(longPollOperations, rt.OperationID) {
		return next
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		d := c.Timeout
		if d <= 0 {
			d = RequestTimeout
		}
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// DecodeJSON reads the body of r into v. A body over BodyLimit is 413
// payload_too_large, anything else that is not one JSON value of v is 400
// invalid_request. Unknown fields are accepted: fields are added to the API
// compatibly (api-contracts.md §1.1).
func DecodeJSON(r *http.Request, v any) *Error {
	if r.Body == nil || r.Body == http.NoBody {
		return NewError(CodeInvalidRequest, map[string]any{"body": "required"})
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(v); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return NewError(CodePayloadTooLarge, nil)
		}
		return NewError(CodeInvalidRequest, nil)
	}
	if dec.More() {
		return NewError(CodeInvalidRequest, nil)
	}
	return nil
}
