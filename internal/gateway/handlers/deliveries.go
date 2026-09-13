package handlers

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/outbox"
	"multiverse-core.io/internal/gateway/turns"
	"multiverse-core.io/shared/runtime"
)

// ClientPlatforms is the platform of each client that has one: the platform
// whose deliveries it takes and whose external IDs it may see (SEC-12). The two
// lists of the manifest, MV_GATEWAY_CLIENT_IDS and
// MV_GATEWAY_ACTOR_KIND_CLIENTS, do not say it, so MVP-1 fixes it here:
// telegram-bot takes telegram, and so does ci-harness, because the HTTP API
// creates links of telegram only and the harness of the tests (T-308, T-313)
// must see the deliveries of the players it registers. That second row is
// provisional until system-architect decides (decision 1 of the orchestrator on
// T-307). A client missing from MV_GATEWAY_CLIENT_IDS — ci-harness in
// production — is refused by the middleware before it reaches this table. No
// other client has a platform, is given a delivery or sees a route. The
// variable comes with a second bot (E-H).
var ClientPlatforms = map[string]string{
	"telegram-bot": links.PlatformTelegram,
	"ci-harness":   links.PlatformTelegram,
}

// WriteMargin is what the write deadline of a long-poll adds to its wait, and
// MaxPollWrite the ceiling of that deadline (component §5.1 p. 8, C-01 v1.8).
const (
	WriteMargin  = 5 * time.Second
	MaxPollWrite = 30 * time.Second
)

// DeliveryService is what the handlers of deliveries use of outbox.Service.
type DeliveryService interface {
	Serve(ctx context.Context, p outbox.Poll) (api.DeliveriesResponse, error)
	Ack(ctx context.Context, clientID string, ids []string, onDelivered outbox.OnDelivered) (api.AckResponse, error)
}

// TurnDeliveries is what an acknowledgement moves on: the turn whose narrative
// was delivered (turns.Tracker).
type TurnDeliveries interface {
	OnDelivered(ctx context.Context, q turns.DB, correlationID string, at time.Time) error
}

// Deliveries serves pollDeliveries, ackDeliveries and streamDeliveries. Its
// fields are set before the process server serves; see api.Config for why that
// is safe.
//
// Nothing here logs a text or a route: the answers carry the external IDs of
// the players, and the access log writes the route of the request and its code.
type Deliveries struct {
	Service   DeliveryService
	Turns     TurnDeliveries
	Platforms map[string]string
	Log       *slog.Logger
}

// Poll serves GET /v1/clients/{client_id}/deliveries. wait_ms and limit above
// their bounds are brought down to them (C-08 v1.1); a value that is not a
// number, below the bound or a negative after, is 400 invalid_request.
//
// The write deadline of the connection is the wait plus WriteMargin, at most
// MaxPollWrite, on the wall clock; there is no read deadline, which would
// cancel the context of the waiting request (component §5.1 p. 8). The
// long-poll answers an empty list as soon as the process server begins to stop.
func (h *Deliveries) Poll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	after, ok := intParam(q.Get("after"), 0, 0)
	if !ok {
		_ = api.WriteError(w, api.NewError(api.CodeInvalidRequest, map[string]any{"param": "after"}))
		return
	}
	limit, ok := intParam(q.Get("limit"), outbox.MaxLimit, 1)
	if !ok {
		_ = api.WriteError(w, api.NewError(api.CodeInvalidRequest, map[string]any{"param": "limit"}))
		return
	}
	waitMS, ok := intParam(q.Get("wait_ms"), outbox.MaxWait.Milliseconds(), 0)
	if !ok {
		_ = api.WriteError(w, api.NewError(api.CodeInvalidRequest, map[string]any{"param": "wait_ms"}))
		return
	}
	wait := time.Duration(min(waitMS, outbox.MaxWait.Milliseconds())) * time.Millisecond
	// A writer that cannot take deadlines (a test recorder) keeps none; the
	// process server always can.
	_ = runtime.SetDeadlines(w, 0, min(wait+WriteMargin, MaxPollWrite))

	clientID := api.ClientIDFrom(r.Context())
	resp, err := h.Service.Serve(r.Context(), outbox.Poll{
		ClientID: clientID, Platform: h.Platforms[clientID], After: after,
		Limit: int(min(limit, int64(outbox.MaxLimit))), Wait: wait, Stop: runtime.ShuttingDown(r.Context()),
	})
	if err != nil {
		if r.Context().Err() != nil {
			// The client went away: nobody reads an answer.
			return
		}
		h.fail(r.Context(), "long-poll of deliveries failed", err)
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
		return
	}
	_ = api.WriteJSON(w, http.StatusOK, resp)
}

// Ack serves POST /v1/clients/{client_id}/deliveries/ack. An acknowledgement
// that completes a turn publishes its analytics; when the bus does not take
// them, nothing of the call is acknowledged and the answer is 503
// bus_unavailable, which the client repeats.
func (h *Deliveries) Ack(w http.ResponseWriter, r *http.Request) {
	var req api.AckRequest
	if e := api.DecodeJSON(r, &req); e != nil {
		_ = api.WriteError(w, e)
		return
	}
	if len(req.IDs) == 0 {
		_ = api.WriteError(w, api.NewError(api.CodeInvalidRequest, map[string]any{"field": "ids"}))
		return
	}
	resp, err := h.Service.Ack(r.Context(), api.ClientIDFrom(r.Context()), req.IDs, h.onDelivered)
	switch {
	case errors.Is(err, turns.ErrPublish):
		h.fail(r.Context(), "acknowledgement of deliveries rolled back: analytics not published", err)
		_ = api.WriteError(w, api.NewError(api.CodeBusUnavailable, nil))
	case err != nil:
		h.fail(r.Context(), "acknowledgement of deliveries failed", err)
		_ = api.WriteError(w, api.NewError(api.CodeInternal, nil))
	default:
		_ = api.WriteJSON(w, http.StatusOK, resp)
	}
}

// Stream serves GET /v1/clients/{client_id}/stream, reserved for E-H.
func (h *Deliveries) Stream(w http.ResponseWriter, _ *http.Request) {
	_ = api.WriteError(w, api.NewError(api.CodeNotImplemented, nil))
}

// onDelivered moves on the turn of a delivered narrative, once per delivery:
// the store calls it only for a delivery the acknowledgement moved to
// delivered.
func (h *Deliveries) onDelivered(ctx context.Context, q outbox.DB, d outbox.Delivery, at time.Time) error {
	if d.Kind != outbox.KindNarrative || h.Turns == nil {
		return nil
	}
	return h.Turns.OnDelivered(ctx, q, d.CorrelationID, at)
}

func (h *Deliveries) fail(ctx context.Context, msg string, err error) {
	if h.Log == nil {
		return
	}
	h.Log.LogAttrs(ctx, slog.LevelError, msg, slog.String("request_id", api.RequestIDFrom(ctx)),
		slog.Bool("handled", true), slog.String("error", err.Error()))
}

// intParam reads a query parameter: empty is def, a number below lowest or not
// a number is not ok.
func intParam(raw string, def, lowest int64) (int64, bool) {
	if raw == "" {
		return def, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < lowest {
		return 0, false
	}
	return n, true
}
