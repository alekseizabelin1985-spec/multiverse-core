// Package consumer reads the four topics of the gateway and hands every event
// to the projection and to the side effects registered for its type
// (component gateway-and-bot.md §2.1, §8.1, §11.2).
//
// Two cursors per topic. The projection lives in memory and moves with every
// event it applies; it is rebuilt at start from the snapshot of State, and an
// event it already holds changes nothing. The effects — outbox, turns, rounds
// in later tasks — live in gateway.db and happen once: an event is marked
// processed in the same transaction as its effects and the effects cursor, and
// only after the effects succeeded (ADR-027, the two-step Has/Add of C-01
// v1.4). An event at or behind the effects cursor goes to the projection only.
package consumer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"time"

	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// Group is the consumer group of the gateway (ADR-007 p. 6, component §13).
const Group = "gateway.consumer"

// Topics are the topics the gateway reads, in the order they are subscribed.
var Topics = []string{
	eventbus.TopicSystemEvents,
	eventbus.TopicGameEvents,
	eventbus.TopicWorldEvents,
	eventbus.TopicNarrativeOutput,
}

// timeLayout is the fixed-width form of the time columns of gateway.db, the
// one links.db uses: two stamps compare as text.
const timeLayout = "2006-01-02T15:04:05.000000000Z"

// Effect is a side effect of one event. It writes through tx and nothing else:
// gateway.db has one connection, which tx holds, and what it writes commits
// with the mark "processed" and the cursor or not at all. An error rolls the
// three back and goes to the bus, which retries the event.
//
// The transitions of an encounter in res are unique within the process only
// (readmodel.Result): after a restart the second event of a pair reports its
// transition again, so an effect of a transition is idempotent by the
// encounter in what it writes.
type Effect func(ctx context.Context, tx *sql.Tx, ev eventbus.Event, res readmodel.Result) error

// Config builds a dispatcher. Every field but Effects is required.
type Config struct {
	Bus     eventbus.Bus
	Journal eventbus.Journal
	DB      *sql.DB
	Model   *readmodel.Model
	Clock   clock.Clock
	Log     *slog.Logger
	// Effects are the side effects per event type, run in order.
	Effects map[string][]Effect
	// Repair, when set, is the repair of stale entities from the snapshot of
	// State (component §11.2). It returns nothing: a repair that fails leaves
	// the projection stale, and the event is not retried for it.
	//
	// In the subscription it is called with a snapshot.created the projection
	// has not seen yet, before the event moves either cursor. In the catch-up
	// only the last announcement Repairs accepts in the range read is kept and
	// repaired from once, after every topic reached its End and before the
	// subscriptions: an earlier snapshot is older than the last, its object may
	// be gone to the rotation already, and a read per announcement against a
	// store that hangs would spend the budget of the catch-up and fail the
	// start (Mi-1 of review #1 of T-309).
	Repair func(ctx context.Context, ev eventbus.Event)
	// Repairs says whether Repair reads from ev — the snapshot of State of the
	// world of the context. Nil accepts every snapshot.created.
	Repairs func(ev eventbus.Event) bool
}

// Dispatcher is the consumer of the gateway.
type Dispatcher struct {
	cfg Config

	mu   sync.Mutex
	done map[string]int64 // effects cursor: the last offset per topic whose effects committed
	ends map[string]int64 // the end of the journal per topic the catch-up read to
	live bool
	// catchingUp is set while Start reads the journal; announcement is the
	// last snapshot Repair is to read from when it ends.
	catchingUp   bool
	announcement *eventbus.Event
	cancel       context.CancelFunc
	running      sync.WaitGroup
	failure      error
}

// New checks the configuration.
func New(cfg Config) (*Dispatcher, error) {
	if cfg.Bus == nil || cfg.Journal == nil || cfg.DB == nil || cfg.Model == nil || cfg.Clock == nil || cfg.Log == nil {
		return nil, errors.New("consumer: Bus, Journal, DB, Model, Clock and Log are required")
	}
	return &Dispatcher{cfg: cfg, done: make(map[string]int64), ends: make(map[string]int64)}, nil
}

// Start loads the effects cursors, catches the projection and the effects up
// from the journal and subscribes to the four topics.
//
// from is the cursor of the snapshot the projection was loaded from; nil
// means no snapshot, and the journal of system_events is read from its start.
// A topic is read from the earlier of two offsets: the cursor of the snapshot,
// for the projection, and the offset after the effects cursor of gateway.db,
// for what the gateway did not finish before it stopped. A topic that has
// neither — no snapshot names it and gateway.db has taken nothing from it —
// is left to its subscription.
//
// Each topic is read up to its End as Start sees it (C-01 v1.1): once every
// topic is there the gateway has read the journal, and it goes live — the
// subscriptions start (component §11.2). What arrives after End comes through
// the subscriptions, and what they repeat of the catch-up takes no effect
// twice: the effects cursor and processed_events stop it (component §16 p. 7).
//
// The subscriptions run under a context of their own, cancelled by Stop and
// by nothing else: the bus closing does not cancel a handler (C-01 v1.7).
func (d *Dispatcher) Start(ctx context.Context, from map[string]int64) error {
	if err := d.loadCursors(ctx); err != nil {
		return err
	}
	if from == nil {
		from = map[string]int64{eventbus.TopicSystemEvents: 0}
	}
	d.mu.Lock()
	d.catchingUp, d.announcement = true, nil
	d.mu.Unlock()
	defer func() {
		d.mu.Lock()
		d.catchingUp, d.announcement = false, nil
		d.mu.Unlock()
	}()
	for _, topic := range Topics {
		offset, ok := d.catchUpFrom(topic, from)
		if !ok {
			continue
		}
		end, err := d.cfg.Journal.End(ctx, topic)
		if err != nil {
			return fmt.Errorf("consumer: end of %s: %w", topic, err)
		}
		d.mu.Lock()
		d.ends[topic] = end
		d.mu.Unlock()
		if offset >= end {
			continue
		}
		if _, err := d.cfg.Journal.ReadRange(ctx, topic, offset, end, d.Handle); err != nil {
			return fmt.Errorf("consumer: catch up %s [%d, %d): %w", topic, offset, end, err)
		}
	}
	d.mu.Lock()
	announcement := d.announcement
	d.catchingUp, d.announcement = false, nil
	d.mu.Unlock()
	if announcement != nil {
		d.cfg.Repair(ctx, *announcement)
	}

	subCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	d.mu.Lock()
	d.cancel = cancel
	d.live = true
	d.mu.Unlock()
	for _, topic := range Topics {
		d.running.Add(1)
		go d.subscribe(subCtx, topic)
	}
	return nil
}

// catchUpFrom is the offset the catch-up of topic starts at, and whether the
// topic is read at all.
func (d *Dispatcher) catchUpFrom(topic string, from map[string]int64) (int64, bool) {
	offset, ok := from[topic]
	d.mu.Lock()
	last, done := d.done[topic]
	d.mu.Unlock()
	if done && (!ok || last+1 < offset) {
		offset, ok = last+1, true
	}
	return offset, ok
}

// Live says whether the catch-up reached the end of the journal and the
// subscriptions started.
func (d *Dispatcher) Live() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.live
}

// Ends are the ends of the journal the catch-up read to, per topic it read.
func (d *Dispatcher) Ends() map[string]int64 {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make(map[string]int64, len(d.ends))
	maps.Copy(out, d.ends)
	return out
}

func (d *Dispatcher) subscribe(ctx context.Context, topic string) {
	defer d.running.Done()
	err := d.cfg.Bus.Subscribe(ctx, topic, Group, d.Handle)
	if err == nil || ctx.Err() != nil {
		return
	}
	d.cfg.Log.Error("consumer: subscription ended", slog.String("topic", topic), slog.String("error", err.Error()))
	d.mu.Lock()
	if d.failure == nil {
		d.failure = fmt.Errorf("consumer: subscription to %s: %w", topic, err)
	}
	d.mu.Unlock()
}

// Stop cancels the subscriptions and waits for their handlers to return, or
// for ctx to end. The databases stay open: the caller closes them after Stop.
func (d *Dispatcher) Stop(ctx context.Context) error {
	d.mu.Lock()
	cancel := d.cancel
	d.mu.Unlock()
	if cancel == nil {
		return nil
	}
	cancel()
	stopped := make(chan struct{})
	go func() {
		d.running.Wait()
		close(stopped)
	}()
	select {
	case <-stopped:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("consumer: subscriptions did not stop: %w", ctx.Err())
	}
}

// Err is the first subscription that ended with an error, if any: the gateway
// no longer hears that topic.
func (d *Dispatcher) Err() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.failure
}

// Handle is the handler of every subscription and of the catch-up. An error
// is returned to the bus, never swallowed: the bus retries and parks the event
// in dead_letters, and a panic is caught by eventbus.Delivery (C-01 v1.5).
func (d *Dispatcher) Handle(ctx context.Context, ev eventbus.Event) error {
	pos, ok := eventbus.PositionFromContext(ctx)
	if !ok {
		return fmt.Errorf("consumer: %s %s delivered without a position", ev.Type, ev.ID)
	}
	if ev.ID == "" {
		// The mark "processed" is the id: an event without one cannot be
		// deduplicated, and taking its effects would repeat them on every
		// delivery.
		return fmt.Errorf("consumer: %s at %s/%d has no id", ev.Type, pos.Topic, pos.Offset)
	}
	res, err := d.cfg.Model.Apply(ev)
	if err != nil {
		return err
	}
	if ev.Type == readmodel.TypeSnapshotCreated && d.cfg.Repair != nil {
		d.repair(ctx, pos, ev)
	}
	d.cfg.Model.Advance(pos)
	if d.behindCursor(pos) {
		return nil
	}
	return d.commit(ctx, pos, ev, res)
}

// repair hands a snapshot.created to Repair: kept for the end of the catch-up
// while Start reads the journal, at once in the subscription. An announcement
// the projection has already seen — the subscription repeating the catch-up —
// is not repaired from again.
func (d *Dispatcher) repair(ctx context.Context, pos eventbus.Position, ev eventbus.Event) {
	if d.cfg.Repairs != nil && !d.cfg.Repairs(ev) {
		return
	}
	d.mu.Lock()
	if d.catchingUp {
		kept := ev
		d.announcement = &kept
		d.mu.Unlock()
		return
	}
	d.mu.Unlock()
	if d.cfg.Model.Cursor()[pos.Topic] > pos.Offset {
		return
	}
	d.cfg.Repair(ctx, ev)
}

// behindCursor says whether the effects of the event at pos are already
// committed.
func (d *Dispatcher) behindCursor(pos eventbus.Position) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	last, ok := d.done[pos.Topic]
	return ok && pos.Offset <= last
}

// commit runs the effects of the event and records it: the check "processed",
// the effects, the mark and the cursor are one transaction. An event already
// processed — the same id delivered again at another offset — moves the
// cursor and does nothing else (NFR-013).
func (d *Dispatcher) commit(ctx context.Context, pos eventbus.Position, ev eventbus.Event, res readmodel.Result) (err error) {
	tx, err := d.cfg.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("consumer: begin %s %s: %w", ev.Type, ev.ID, err)
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, ignoreDone(tx.Rollback()))
		}
	}()

	var processed int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM processed_events WHERE event_id = ?`, ev.ID).Scan(&processed); err != nil {
		return fmt.Errorf("consumer: look up %s: %w", ev.ID, err)
	}
	now := d.cfg.Clock.Now().UTC().Format(timeLayout)
	if processed == 0 {
		for _, effect := range d.cfg.Effects[ev.Type] {
			if err = effect(ctx, tx, ev, res); err != nil {
				return fmt.Errorf("consumer: effect of %s %s: %w", ev.Type, ev.ID, err)
			}
		}
		if _, err = tx.ExecContext(ctx,
			`INSERT INTO processed_events (event_id, topic, processed_at) VALUES (?, ?, ?)`,
			ev.ID, pos.Topic, now); err != nil {
			return fmt.Errorf("consumer: mark %s processed: %w", ev.ID, err)
		}
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO cursors (topic, "offset", event_id, updated_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT (topic) DO UPDATE SET "offset" = excluded."offset", event_id = excluded.event_id, updated_at = excluded.updated_at
		 WHERE excluded."offset" > cursors."offset"`,
		pos.Topic, pos.Offset, ev.ID, now); err != nil {
		return fmt.Errorf("consumer: move the cursor of %s: %w", pos.Topic, err)
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("consumer: commit %s %s: %w", ev.Type, ev.ID, err)
	}
	d.mu.Lock()
	if last, ok := d.done[pos.Topic]; !ok || pos.Offset > last {
		d.done[pos.Topic] = pos.Offset
	}
	d.mu.Unlock()
	return nil
}

func (d *Dispatcher) loadCursors(ctx context.Context) error {
	cursors, err := Cursors(ctx, d.cfg.DB)
	if err != nil {
		return fmt.Errorf("consumer: read cursors: %w", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for topic, offset := range cursors {
		d.done[topic] = offset
	}
	return nil
}

// Sweep deletes the marks "processed" past their retention: older than
// store.ProcessedEventsKeep, or beyond the newest store.ProcessedEventsMax
// (component §4.2). The effects cursor stays: an event behind it is not taken
// again whatever the window holds.
func (d *Dispatcher) Sweep(ctx context.Context, now time.Time) error {
	cutoff := now.Add(-store.ProcessedEventsKeep).UTC().Format(timeLayout)
	if _, err := d.cfg.DB.ExecContext(ctx, `DELETE FROM processed_events WHERE processed_at < ?`, cutoff); err != nil {
		return fmt.Errorf("consumer: sweep processed events: %w", err)
	}
	if _, err := d.cfg.DB.ExecContext(ctx,
		`DELETE FROM processed_events WHERE event_id IN (
		   SELECT event_id FROM processed_events ORDER BY processed_at DESC, event_id DESC LIMIT -1 OFFSET ?)`,
		store.ProcessedEventsMax); err != nil {
		return fmt.Errorf("consumer: sweep processed events: %w", err)
	}
	return nil
}

// ignoreDone drops the error of a rollback after the transaction already
// ended: a failed commit has rolled back by itself.
func ignoreDone(err error) error {
	if errors.Is(err, sql.ErrTxDone) {
		return nil
	}
	return err
}

// Cursors returns the effects cursor per topic as gateway.db holds it.
func Cursors(ctx context.Context, db *sql.DB) (map[string]int64, error) {
	rows, err := db.QueryContext(ctx, `SELECT topic, "offset" FROM cursors`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := make(map[string]int64)
	for rows.Next() {
		var topic string
		var offset int64
		if err := rows.Scan(&topic, &offset); err != nil {
			return nil, err
		}
		out[topic] = offset
	}
	return out, rows.Err()
}
