// Package deliver takes the deliveries of the bot from the gateway and sends
// them to the players (component §10.4, ADR-006, C-08): a long-poll of
// deliveries, one sendMessage after another in the order of the answer, and
// one ack for the deliveries of the answer that are done with.
//
// The order of the messages of a player is kept by the gateway, which leases
// one delivery per player at a time; the loop sends in the order it received
// and reorders nothing. Whether a delivery is acknowledged is decided by the
// error of the Sender alone:
//
//   - sent, or refused for good (ErrBlocked, ErrChatNotFound, ErrRejected) —
//     acknowledged: repeating will not reach the player;
//   - ErrUnavailable, ErrUnauthorized, or any error the loop does not know —
//     not acknowledged: the lease of the gateway runs out and the delivery is
//     handed out again (MV_GATEWAY_DELIVERY_LEASE, 30 s).
//
// The cursor of the long-poll lives in the Loop only and is never written
// anywhere (component §16 p. 5): a restarted bot starts from an empty cursor,
// and whatever it had not acknowledged comes again.
//
// A line of the log names the kind of a delivery, the reason and a redacted
// error; never the chat, the external id or the text (SEC-01/02).
package deliver

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/flow"
	"multiverse-core.io/cmd/telegram-bot/internal/privacy"
	"multiverse-core.io/cmd/telegram-bot/internal/render"
	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/client"
	"multiverse-core.io/shared/clock"
)

// Limits of one long-poll: the largest the gateway accepts (C-08 v1.1).
const (
	Wait  = client.MaxPollWait
	Limit = client.MaxPollLimit
)

// RetryPause is the pause after the first failed long-poll; it doubles with
// every failure in a row up to MaxRetryPause, so that a gateway that is down
// costs a line of the log every half a minute, not every second.
const (
	RetryPause    = time.Second
	MaxRetryPause = 30 * time.Second
)

// EarlyEmpty is how soon an empty answer of the long-poll counts as early: half
// of wait_ms. The gateway answers an empty list before wait_ms only while its
// process stops (C-08, internal/gateway README): polled again at once, it would
// be asked again and again until it is gone. Only the long-poll itself is
// timed, not the ack before it.
//
// An empty answer that took less than EarlyEmpty counts as a failed long-poll:
// a pause of retryPause follows, and it adds one to the failed long-polls that
// make /health degraded (DegradedAfterFailedPolls) — a stopping gateway takes
// no delivery either. An answer with deliveries, or an empty one that took at
// least EarlyEmpty, resets both (acceptance of T-307, addition 2 of the
// orchestrator; Ma-1 of review #1 of T-315).
const EarlyEmpty = Wait / 2

// The client of the ack (acceptance of T-307, review #1). The client repeats
// an ack the gateway answers 503 bus_unavailable, and the loop flushes the ids
// that are left once more before the next long-poll. With the defaults of the
// client — four attempts of up to 35 s — a broker that answers in 5 s would
// hold the loop for about 43 s between two long-polls, longer than the lease
// of 30 s (MV_GATEWAY_DELIVERY_LEASE), and the same deliveries would be handed
// out and sent to the players again. AckHTTPTimeout is a little above the time
// the gateway gives a request that is not a long-poll (api.RequestTimeout,
// 5 s), and AckBackoff repeats once: one flush lasts at most MaxFlush, and the
// two between long-polls stay under the lease.
const (
	AckHTTPTimeout = 6 * time.Second
	MaxFlush       = 2*AckHTTPTimeout + 200*time.Millisecond
)

// AckBackoff is the repeat policy of the client of the ack.
var AckBackoff = client.Backoff{Retries: 1, Initial: 200 * time.Millisecond, Max: 200 * time.Millisecond}

// NewAckClient is the client of the ack of the bot: an HTTP client of its own
// with AckHTTPTimeout, AckBackoff, and pauses on timers (nil: the wall clock).
func NewAckClient(baseURL, clientID string, timers clock.Timers) *client.Client {
	c := client.New(baseURL, clientID)
	c.HTTP = &http.Client{Timeout: AckHTTPTimeout}
	c.Backoff = AckBackoff
	if timers != nil {
		c.Timers = timers
	}
	return c
}

// DegradedAfterFailedPolls is how many failed long-polls in a row make /health
// of the bot degraded. A failed long-poll is one that returned an error or an
// empty answer sooner than EarlyEmpty, the answer of a stopping gateway: either
// way the bot delivers nothing. With the pauses of retryPause the seventh
// failure comes 61 s after the first: a gateway that restarts within a minute
// does not flip the health of the bot (acceptance of T-312; Ma-1 of review #1
// of T-315).
const DegradedAfterFailedPolls = 7

// AckOnStopTimeout bounds the last ack of a stopping loop. The context of the
// loop has ended by then, and an ack under it would not leave: the deliveries
// already sent would come again after the restart.
const AckOnStopTimeout = 5 * time.Second

// maxPendingAcks bounds the ids kept for the next ack while the gateway does
// not take them. Past it the oldest are let go: their leases have run out long
// ago, and the gateway hands such deliveries out again anyway.
const maxPendingAcks = 10 * Limit

// Gateway is what the loop calls of the gateway; *client.Client implements it.
type Gateway interface {
	Deliveries(ctx context.Context, after string, limit int, wait time.Duration) (api.DeliveriesResponse, error)
	Acker
}

// Acker acknowledges deliveries; *client.Client implements it.
type Acker interface {
	Ack(ctx context.Context, ids []string) (api.AckResponse, error)
}

var _ Gateway = (*client.Client)(nil)

// Options configures a Loop.
type Options struct {
	// Gateway is a client whose HTTP timeout is longer than Wait; the client
	// of the flow, with its short timeout, would cut every long-poll.
	Gateway Gateway
	// Acks acknowledges the deliveries; nil means Gateway. The bot passes
	// NewAckClient, whose repeats end within MaxFlush.
	Acks Acker
	// Sender sends the messages. The loop runs in a goroutine of its own, so
	// it repeats by sender.DefaultPolicy: a 429 is waited out, a network error
	// or a 5xx is tried three times.
	Sender sender.Sender
	// Timers drive the pauses between failed long-polls; nil means the wall
	// clock.
	Timers clock.Timers
	// Clock measures how long a long-poll took, to tell an early empty answer
	// (EarlyEmpty); nil means the wall clock.
	Clock clock.Clock
	// Log receives the lines of the loop; nil discards them. It must be built
	// on privacy.NewHandler, as every logger of the bot.
	Log *slog.Logger
}

// Loop is the delivery loop of one bot process.
type Loop struct {
	gw     Gateway
	acks   Acker
	send   sender.Sender
	timers clock.Timers
	clock  clock.Clock
	log    *slog.Logger

	// cursor and pending are touched by the goroutine of Run only.
	cursor  string
	pending []string

	// failedPolls and unauthorized are what Health reports; the goroutine of
	// Run writes them while /health reads them.
	failedPolls  atomic.Int64
	unauthorized atomic.Bool
}

// Health is what the delivery loop tells /health of the bot.
type Health struct {
	// FailedPolls is the number of long-polls in a row that failed; a
	// successful one resets it.
	FailedPolls int
	// Unauthorized: Telegram refused the token on the last send that got an
	// answer; a message sent resets it.
	Unauthorized bool
}

// Degraded says the bot does not deliver: DegradedAfterFailedPolls long-polls
// in a row failed or answered empty before EarlyEmpty, or Telegram refuses the
// token.
func (h Health) Degraded() bool {
	return h.FailedPolls >= DegradedAfterFailedPolls || h.Unauthorized
}

// Health reports the state of the loop. It is safe to call while Run runs.
func (l *Loop) Health() Health {
	return Health{FailedPolls: int(l.failedPolls.Load()), Unauthorized: l.unauthorized.Load()}
}

// New builds a loop. Gateway and Sender are required.
func New(opts Options) (*Loop, error) {
	if opts.Gateway == nil || opts.Sender == nil {
		return nil, errors.New("deliver: Gateway and Sender are required")
	}
	timers := opts.Timers
	if timers == nil {
		timers = clock.RealTimers{}
	}
	var now clock.Clock = clock.Real{}
	if opts.Clock != nil {
		now = opts.Clock
	}
	acks := opts.Acks
	if acks == nil {
		acks = opts.Gateway
	}
	log := opts.Log
	if log == nil {
		log = slog.New(slog.DiscardHandler)
	}
	return &Loop{gw: opts.Gateway, acks: acks, send: opts.Sender, timers: timers, clock: now, log: log}, nil
}

// Log keys of the loop.
const (
	keyKind   = "kind"
	keyReason = "reason"
	keyCount  = "count"
	keyError  = "error"
	keyPause  = "pause"
)

// Reasons a delivery is acknowledged without a message, or kept.
const (
	reasonBlocked      = "blocked"
	reasonChatNotFound = "chat_not_found"
	reasonRejected     = "rejected"
	reasonNoRoute      = "no_route"
	reasonUnavailable  = "unavailable"
	reasonUnauthorized = "unauthorized"
)

// Run polls, sends and acknowledges until ctx ends, then acknowledges what it
// has sent and returns. A failed long-poll, and an empty answer that came
// sooner than EarlyEmpty, are followed by a pause that grows with every one in
// a row and count as failed long-polls of Health; a long-poll that gave
// deliveries or took at least EarlyEmpty resets both.
func (l *Loop) Run(ctx context.Context) {
	defer l.ackOnStop(ctx)
	failures := 0
	for ctx.Err() == nil {
		n, took, err := l.once(ctx)
		if ctx.Err() != nil {
			return
		}
		early := err == nil && n == 0 && took < EarlyEmpty
		if err == nil && !early {
			if failures > 0 {
				l.log.Info("deliveries polled again", slog.Int(keyCount, failures))
			}
			failures = 0
			l.failedPolls.Store(0)
			continue
		}
		failures++
		l.failedPolls.Store(int64(failures))
		pause := retryPause(failures)
		if early {
			l.log.Info("deliveries answered empty before the wait, the gateway is stopping", slog.Duration(keyPause, pause))
		} else {
			l.log.Warn("deliveries not polled", slog.String(keyError, privacy.Redact(err.Error())), slog.Duration(keyPause, pause))
		}
		if l.pause(ctx, pause) != nil {
			return
		}
	}
}

// Once acknowledges what is left from before, takes one answer of the
// long-poll, sends its deliveries and acknowledges them. It returns the error
// of the long-poll; an ack that failed is kept for the next call.
func (l *Loop) Once(ctx context.Context) error {
	_, _, err := l.once(ctx)
	return err
}

// once is Once that also tells how many deliveries the answer held and how
// long the long-poll itself took, the ack before it not counted.
func (l *Loop) once(ctx context.Context) (int, time.Duration, error) {
	l.flush(ctx)
	began := l.clock.Now()
	res, err := l.gw.Deliveries(ctx, l.cursor, Limit, Wait)
	took := l.clock.Now().Sub(began)
	if err != nil {
		return 0, took, err
	}
	if res.Cursor != "" {
		l.cursor = res.Cursor
	}
	for _, d := range res.Deliveries {
		if ctx.Err() != nil {
			break
		}
		ack, stop := l.deliver(ctx, d)
		if ack {
			l.keep(d.ID)
		}
		if stop {
			break
		}
	}
	l.flush(ctx)
	return len(res.Deliveries), took, nil
}

// deliver sends one delivery. ack says the delivery is done with; stop says
// no other delivery of the answer can be sent either.
func (l *Loop) deliver(ctx context.Context, d api.Delivery) (ack, stop bool) {
	chat, ok := chatOf(d.Route)
	if !ok {
		// The gateway gives a route only to a client of the platform of the
		// link (SEC-12), so this delivery can never be sent by this bot. Kept,
		// it would come again every 30 s for 24 h and hold the queue of the
		// player behind it.
		l.log.Warn("delivery has no telegram route, acknowledged without a message", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonNoRoute))
		return true, false
	}
	for _, part := range render.Delivery(d) {
		err := l.send.Send(ctx, chat, part.Text, part.Keyboard)
		if err == nil {
			l.unauthorized.Store(false)
			continue
		}
		if ctx.Err() != nil {
			return false, true
		}
		return l.decide(d, err)
	}
	return true, false
}

// decide maps an error of the Sender onto the fate of a delivery.
func (l *Loop) decide(d api.Delivery, err error) (ack, stop bool) {
	detail := slog.String(keyError, privacy.Redact(err.Error()))
	switch {
	case errors.Is(err, sender.ErrBlocked):
		l.log.Info("player unreachable, delivery acknowledged", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonBlocked))
		return true, false
	case errors.Is(err, sender.ErrChatNotFound):
		l.log.Info("player unreachable, delivery acknowledged", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonChatNotFound))
		return true, false
	case errors.Is(err, sender.ErrRejected):
		l.log.Warn("message rejected by telegram, delivery acknowledged", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonRejected), detail)
		return true, false
	case errors.Is(err, sender.ErrUnauthorized):
		// Every other message of the answer would be refused the same way.
		// The update source meets the same 401 and stops the process.
		l.unauthorized.Store(true)
		l.log.Error("bot token rejected, deliveries left to the gateway", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonUnauthorized), detail)
		return false, true
	default:
		l.log.Warn("telegram unavailable, delivery left to the gateway", slog.String(keyKind, d.Kind), slog.String(keyReason, reasonUnavailable), detail)
		return false, false
	}
}

// chatOf reads the chat of a delivery from its route: the external id of a
// Telegram link is the user id, which is the chat of the private chat with
// the bot (SEC-07).
func chatOf(r *api.DeliveryRoute) (int64, bool) {
	if r == nil || r.ExternalPlatform != flow.Platform {
		return 0, false
	}
	id, err := strconv.ParseInt(r.ExternalID, 10, 64)
	if err != nil || id <= 0 {
		return 0, false
	}
	return id, true
}

// keep adds an id to the next ack, once.
func (l *Loop) keep(id string) {
	for _, known := range l.pending {
		if known == id {
			return
		}
	}
	l.pending = append(l.pending, id)
	if over := len(l.pending) - maxPendingAcks; over > 0 {
		l.pending = append([]string(nil), l.pending[over:]...)
	}
}

// flush acknowledges the kept ids in one request and forgets them once the
// gateway took the request. Ids the gateway does not know are not an error:
// their lease ran out, or a lost answer of an earlier ack already confirmed
// them (C-08).
func (l *Loop) flush(ctx context.Context) {
	if len(l.pending) == 0 || ctx.Err() != nil {
		return
	}
	res, err := l.acks.Ack(ctx, l.pending)
	if err != nil {
		if ctx.Err() == nil {
			l.log.Warn("deliveries not acknowledged, kept for the next ack", slog.Int(keyCount, len(l.pending)), slog.String(keyError, privacy.Redact(err.Error())))
		}
		return
	}
	if len(res.Unknown) > 0 {
		l.log.Debug("gateway did not know some acknowledged deliveries", slog.Int(keyCount, len(res.Unknown)))
	}
	l.pending = nil
}

// ackOnStop sends the last ack under a context of its own: the one of the loop
// has ended.
func (l *Loop) ackOnStop(ctx context.Context) {
	if len(l.pending) == 0 {
		return
	}
	stopCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), AckOnStopTimeout)
	defer cancel()
	l.flush(stopCtx)
}

func (l *Loop) pause(ctx context.Context, d time.Duration) error {
	t := l.timers.After(d)
	select {
	case <-ctx.Done():
		t.Stop()
		return ctx.Err()
	case <-t.C():
		return nil
	}
}

// retryPause is the pause after the n-th failed long-poll in a row, n from 1.
func retryPause(n int) time.Duration {
	d := RetryPause
	for i := 1; i < n && d < MaxRetryPause; i++ {
		d *= 2
	}
	return min(d, MaxRetryPause)
}
