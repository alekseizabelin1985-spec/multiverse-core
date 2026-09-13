package outbox

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
)

// Notifier is the bell of a platform: a waiting long-poll listens to it, and
// the store rings it after it enqueued or released deliveries (component §8.3).
// A ring reaches every long-poll that took the bell before it; a long-poll that
// takes the bell after the ring leases first and finds the rows anyway.
type Notifier struct {
	mu    sync.Mutex
	bells map[string]chan struct{}
}

// NewNotifier returns a notifier without bells.
func NewNotifier() *Notifier { return &Notifier{bells: make(map[string]chan struct{})} }

// Bell is the channel that closes at the next ring of platform.
func (n *Notifier) Bell(platform string) <-chan struct{} {
	n.mu.Lock()
	defer n.mu.Unlock()
	ch, ok := n.bells[platform]
	if !ok {
		ch = make(chan struct{})
		n.bells[platform] = ch
	}
	return ch
}

// Notify rings the bell of platform.
func (n *Notifier) Notify(platform string) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if ch, ok := n.bells[platform]; ok {
		close(ch)
		delete(n.bells, platform)
	}
}

// NotifyAll rings every bell.
func (n *Notifier) NotifyAll() {
	n.mu.Lock()
	defer n.mu.Unlock()
	for platform, ch := range n.bells {
		close(ch)
		delete(n.bells, platform)
	}
}

// Routes is what the long-poll reads of links.db: the external address of a
// player at the moment of the delivery (links.Store.RouteFor).
type Routes interface {
	RouteFor(ctx context.Context, playerID string) (platform, externalID string, ok bool, err error)
}

// ServiceConfig builds a service. Every field but Log is required.
type ServiceConfig struct {
	Store  *Store
	Routes Routes
	Clock  clock.Clock
	// Timers bound the wait of a long-poll and its periodic wake-up. The wait
	// is a property of the connection, like the deadlines of runtime.SetDeadlines,
	// so the gateway passes wall-clock timers in every mode (C-01 v1.8): the
	// null timers of a replay would never end a long-poll.
	Timers clock.Timers
	Log    *slog.Logger
}

// Service serves the long-poll and the acknowledgements of deliveries.
type Service struct {
	cfg ServiceConfig
}

// NewService returns the service of cfg.
func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.Store == nil || cfg.Routes == nil || cfg.Clock == nil || cfg.Timers == nil {
		return nil, errors.New("outbox: Store, Routes, Clock and Timers are required")
	}
	if cfg.Log == nil {
		cfg.Log = slog.New(slog.DiscardHandler)
	}
	return &Service{cfg: cfg}, nil
}

// Poll is one long-poll of a client.
type Poll struct {
	ClientID string
	// Platform is the platform of the client; empty for a client that has
	// none, which is given nothing.
	Platform string
	// After is the cursor of the previous answer; the lease already keeps what
	// the client holds from being given again, so it only sets the cursor of an
	// empty answer.
	After int64
	Limit int
	Wait  time.Duration
	// Stop closes when the process server begins to stop
	// (runtime.ShuttingDown); nil never closes.
	Stop <-chan struct{}
}

// Serve answers a long-poll: the deliveries leased to the client as soon as
// there are any, or none once the wait is over or Stop closed (component §8.3).
// Between two leases it waits on the bell of the platform, at most WakeEvery,
// holding no connection of gateway.db. The error is an error of the store or
// the end of ctx.
//
// A delivery whose player has no route, or a route of another platform than
// the client's, is dropped and left out of the answer: nobody can deliver it,
// and the external ID of a link goes only to a client of its platform (SEC-12).
func (s *Service) Serve(ctx context.Context, p Poll) (api.DeliveriesResponse, error) {
	limit := min(max(p.Limit, 1), MaxLimit)
	wait := min(max(p.Wait, 0), MaxWait)
	out := api.DeliveriesResponse{Deliveries: []api.Delivery{}, Cursor: strconv.FormatInt(p.After, 10)}
	deadline := s.cfg.Timers.After(wait)
	defer deadline.Stop()
	for {
		// The bell is taken before the lease: a ring between the lease and the
		// wait is not lost.
		bell := s.cfg.Store.notifier.Bell(p.Platform)
		leased, err := s.lease(ctx, p, limit)
		if err != nil {
			return api.DeliveriesResponse{}, err
		}
		if len(leased) > 0 {
			last := p.After
			for _, d := range leased {
				out.Deliveries = append(out.Deliveries, d.Delivery)
				last = max(last, d.seq)
			}
			out.Cursor = strconv.FormatInt(last, 10)
			return out, nil
		}
		if wait == 0 {
			return out, nil
		}
		wake := s.cfg.Timers.After(WakeEvery)
		select {
		case <-bell:
		case <-wake.C():
		case <-deadline.C():
			wake.Stop()
			return out, nil
		case <-p.Stop:
			wake.Stop()
			return out, nil
		case <-ctx.Done():
			wake.Stop()
			return api.DeliveriesResponse{}, ctx.Err()
		}
		wake.Stop()
		// A ring or a wake-up that came together with the end of the wait does
		// not start another lease: select picks among ready cases at random,
		// and a lease at the very end may wait for the connection held by an
		// acknowledgement or the consumer past the write deadline of the
		// answer, which leaves the leased deliveries to come again after the
		// lease (review #1 of T-307, N-3). A lease that starts before the end
		// can still wait that long; bounding it by the write deadline is in the
		// backlog.
		select {
		case <-deadline.C():
			return out, nil
		default:
		}
	}
}

type given struct {
	api.Delivery
	seq int64
}

// lease leases the next deliveries and gives each its route; those without one
// are dropped.
func (s *Service) lease(ctx context.Context, p Poll, limit int) ([]given, error) {
	if p.Platform == "" {
		return nil, nil
	}
	leased, err := s.cfg.Store.Lease(ctx, p.ClientID, p.Platform, limit, s.cfg.Clock.Now())
	if err != nil {
		return nil, err
	}
	out := make([]given, 0, len(leased))
	for _, d := range leased {
		platform, externalID, ok, err := s.cfg.Routes.RouteFor(ctx, d.PlayerID)
		if err != nil {
			return nil, fmt.Errorf("outbox: route of %s: %w", d.ID, err)
		}
		if !ok || platform != p.Platform {
			if err := s.cfg.Store.Drop(ctx, d.ID); err != nil {
				return nil, err
			}
			s.cfg.Log.LogAttrs(ctx, slog.LevelWarn, "delivery without a route of its platform dropped",
				slog.String("delivery_id", d.ID), slog.String("player_id", d.PlayerID), slog.Bool("handled", true))
			continue
		}
		wire := wireOf(d)
		wire.Route = &api.DeliveryRoute{ExternalPlatform: platform, ExternalID: externalID}
		out = append(out, given{Delivery: wire, seq: d.Seq})
	}
	return out, nil
}

// wireOf is a delivery as a client receives it, without its route.
func wireOf(d Delivery) api.Delivery {
	w := api.Delivery{
		ID: d.ID, PlayerID: d.PlayerID, Kind: d.Kind, CorrelationID: d.CorrelationID, EventID: d.EventID,
		RoundSeq: d.RoundSeq, GeneratedBy: d.GeneratedBy, FallbackReason: d.FallbackReason, Text: d.Text,
		Data: d.Data, CreatedAt: d.CreatedAt,
	}
	if id, ok := d.Data[DataNarrativeEventID].(string); ok && id != "" {
		w.NarrativeEventID = &id
	}
	return w
}

// Ack confirms deliveries of a client at the time of the clock; see Store.Ack.
func (s *Service) Ack(ctx context.Context, clientID string, ids []string, onDelivered OnDelivered) (api.AckResponse, error) {
	acked, unknown, err := s.cfg.Store.Ack(ctx, clientID, ids, s.cfg.Clock.Now(), onDelivered)
	if err != nil {
		return api.AckResponse{}, err
	}
	return api.AckResponse{Acked: len(acked), Unknown: unknown}, nil
}
