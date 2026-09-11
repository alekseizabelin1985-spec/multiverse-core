// Package membus is the in-process Bus and Journal of the platform: the second
// implementation of C-01 next to the kafka adapter, run by the same contract
// test, and the transport of --bus=memory (contracts.md C-01 v1.4,
// foundation.md §2). Every epic writes its unit tests against it until the
// broker is worth starting.
//
// It implements the interfaces, not a second copy of the rules. Publication goes
// through eventbus.Route and reading through eventbus.Delivery, exactly as the
// kafka adapter does, so validation, the topic policies, the retries and the
// dead-letter fallback cannot drift between the two. What lives here is only
// what a transport owns: where the bytes are kept, how a reader waits for the
// next one, and where the offsets come from.
//
// A topic is an append-only slice, an offset is an index into it, and a
// consumer group is a cursor over one topic. That is the whole model: one
// broker, one partition, strict order (ADR-007 p. 4).
package membus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/eventbus"
)

// ErrNoTopic is returned for a topic the bus was not told about. It mirrors
// the broker of the platform, which runs with auto_create_topics_enabled=false
// and is written to with AllowAutoTopicCreation=false, so that a topic is
// never born without the retention redpanda-init gives it (infrastructure.md
// §5.1).
var ErrNoTopic = errors.New("membus: unknown topic")

// Chaos injects the failure modes C-01 allows a bus to have, so that a
// consumer can be tested against them (the --chaos=duplicate switch).
type Chaos struct {
	// Duplicate appends every published record twice. Delivery is
	// at-least-once, and a consumer that does not deduplicate by event id is
	// wrong even though it passes on a quiet day.
	Duplicate bool
}

// Config builds a bus. Only Registry is mandatory.
type Config struct {
	Registry eventbus.Registry
	// Topics are the topics that exist. An empty list means "create a topic
	// on first use", which is convenient for a unit test but is not what the
	// broker does; the contract test always passes the real list.
	Topics []string
	// SkipValidateOnRead turns off validation on read. The zero value
	// validates (SEC-16), like the kafka adapter.
	SkipValidateOnRead bool
	// Backoff overrides eventbus.DefaultBackoff; its length is the number of
	// retries. A test usually passes zero pauses.
	Backoff []time.Duration
	Timers  clock.Timers
	Log     *slog.Logger
	Chaos   Chaos
}

// Bus is the in-process implementation of eventbus.Bus and eventbus.Journal.
//
// What it reads to serve a call — the registry, the flags, the backoff — is
// its own; what it owns lives in state, which a view returned by Lenient
// shares with it.
type Bus struct {
	registry           eventbus.Registry
	skipValidateOnRead bool
	backoff            []time.Duration
	timers             clock.Timers
	log                *slog.Logger
	// open reports whether unknown topics are created on first use.
	open bool

	st *state
}

// state is the transport itself: the log, the cursors of the consumer groups,
// the chaos switch and the lifetime. It sits behind a pointer because a bus
// and the views of it are one transport — closing one closes them all, and
// what is published through one is read through the other.
type state struct {
	// done is closed by Close. A reader selects on it and returns nil, the
	// way the kafka adapter turns the closed reader of its own Close into an
	// orderly stop (eventbus.stopped).
	done chan struct{}

	mu     sync.Mutex
	closed bool
	chaos  Chaos
	topics map[string]*topic
	groups map[string]*group
}

var (
	_ eventbus.Bus     = (*Bus)(nil)
	_ eventbus.Journal = (*Bus)(nil)
)

// New builds the bus. Like the kafka adapter it fails fast without a registry:
// without one it cannot route a type, and a stub that accepts anything would
// let a defect through to the epic that trusts it.
func New(cfg Config) (*Bus, error) {
	reg := cfg.Registry
	if reg == nil {
		reg = eventbus.PackageRegistry()
	}
	if reg == nil {
		return nil, eventbus.ErrNoRegistry
	}
	b := &Bus{
		registry:           reg,
		skipValidateOnRead: cfg.SkipValidateOnRead,
		backoff:            cfg.Backoff,
		timers:             cfg.Timers,
		log:                cfg.Log,
		open:               len(cfg.Topics) == 0,
		st: &state{
			done:   make(chan struct{}),
			chaos:  cfg.Chaos,
			topics: make(map[string]*topic, len(cfg.Topics)+1),
			groups: make(map[string]*group),
		},
	}
	for _, name := range cfg.Topics {
		b.st.topics[name] = newTopic()
	}
	if !b.open {
		// Whatever the caller listed, the bus must be able to park a failed
		// event: a dead letter that cannot be written stalls the very topic it
		// exists to unblock.
		if _, ok := b.st.topics[eventbus.TopicDeadLetters]; !ok {
			b.st.topics[eventbus.TopicDeadLetters] = newTopic()
		}
	}
	return b, nil
}

// Lenient returns a view of the bus that does not validate on read: the same
// log, the same cursors and the same lifetime, read with
// MV_BUS_VALIDATE_ON_READ in the other position.
//
// A test needs the view to check the half of the flag the safe default hides —
// that an event the schema rejects does reach the handler once the check is
// off — and it cannot use a second bus for that, because a second bus would
// have a log of its own that nothing ever published into.
func (b *Bus) Lenient() *Bus {
	view := *b
	view.skipValidateOnRead = true
	return &view
}

// SetChaos changes the failure modes of a running bus. Chaos is a property of
// the whole transport (ADR-010 p. 4), so a test that wants one publish
// duplicated turns the switch on around that publish rather than building a
// second bus, whose log would be empty.
func (b *Bus) SetChaos(c Chaos) {
	b.st.mu.Lock()
	defer b.st.mu.Unlock()
	b.st.chaos = c
}

func (b *Bus) chaos() Chaos {
	b.st.mu.Lock()
	defer b.st.mu.Unlock()
	return b.st.chaos
}

// stopping reports whether a delivery loop must give up: the caller cancelled
// the context, or Close closed the bus. Both are an orderly stop and return
// nil, which is what the kafka adapter reports as well — its reader fails with
// the cancellation or with io.ErrClosedPipe, and eventbus.stopped turns either
// into a nil return.
//
// Every loop asks before it takes the next record, not only when it is parked
// at the end of the log. A cancelled subscription that first drains whatever
// is already in the topic is the divergence review #1 of T-014 found: on the
// broker the fetch or the commit fails within one event, so a consumer test
// written against the stub — publish N, cancel, expect N handled — would be
// green here and red against Redpanda.
//
// A loop asks it after a failed delivery too, and for the same reason: a
// handler still failing when the bus is closed must not turn the shutdown into
// an error. Asking only ctx.Err() there made Close under a failing handler
// return "write dead letter: bus is closed" while the adapter returned nil
// (review #2 of T-014, Minor-3).
func (b *Bus) stopping(ctx context.Context) bool {
	if ctx.Err() != nil {
		return true
	}
	select {
	case <-b.st.done:
		return true
	default:
		return false
	}
}

// Publish routes the event by its type and appends it to that topic.
func (b *Bus) Publish(ctx context.Context, ev eventbus.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	name, err := eventbus.Route(b.registry, ev)
	if err != nil {
		return err
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("membus: marshal %s: %w", ev.Type, err)
	}
	if err := b.write(name, body); err != nil {
		return err
	}
	if b.chaos().Duplicate {
		return b.write(name, body)
	}
	return nil
}

// Append puts a raw body into a topic without routing or validating it. It is
// how a test produces the message a publisher never could — an envelope that
// fails validation, or bytes that are not an event at all — to check what the
// read side does with it.
func (b *Bus) Append(topicName string, body []byte) error {
	return b.write(topicName, append([]byte(nil), body...))
}

// Records returns the raw bodies of a topic, oldest first. A test reads
// dead_letters with it: a dead letter is not an event and must not be fed back
// through the journal, which would try to validate it and park it again.
func (b *Bus) Records(topicName string) ([][]byte, error) {
	t, err := b.topic(topicName)
	if err != nil {
		return nil, err
	}
	return t.all(), nil
}

// DeadLetters returns what the bus has parked so far.
func (b *Bus) DeadLetters() ([]eventbus.DeadLetter, error) {
	records, err := b.Records(eventbus.TopicDeadLetters)
	if err != nil {
		return nil, err
	}
	letters := make([]eventbus.DeadLetter, 0, len(records))
	for i, body := range records {
		var dl eventbus.DeadLetter
		if err := json.Unmarshal(body, &dl); err != nil {
			return nil, fmt.Errorf("membus: decode dead letter %d: %w", i, err)
		}
		letters = append(letters, dl)
	}
	return letters, nil
}

// Subscribe consumes a topic under a consumer group until ctx is done, and
// returns nil when it stops that way — the semantics of the kafka adapter. A
// new group starts at the first offset, and the cursor moves only once the
// event is accounted for: handled, or parked in dead_letters.
//
// A cancelled subscription stops at the next record rather than finishing what
// is already in the topic, and the event whose delivery the cancellation
// interrupted stays uncommitted, so the next subscription of the group gets it
// again (at-least-once, C-01).
func (b *Bus) Subscribe(ctx context.Context, topicName, groupName string, h eventbus.Handler) error {
	if groupName == "" {
		return errors.New("membus: subscribe: empty consumer group")
	}
	if h == nil {
		return errors.New("membus: subscribe: nil handler")
	}
	t, err := b.topic(topicName)
	if err != nil {
		return err
	}
	g := b.group(topicName, groupName)
	delivery := b.delivery(groupName)

	for {
		if b.stopping(ctx) {
			return nil
		}
		// The cursor is held across the delivery so that one group handles one
		// event at a time, as a single-partition topic does.
		g.mu.Lock()
		offset := g.next
		changed := t.changed()
		body, ok := t.at(offset)
		if !ok {
			g.mu.Unlock()
			select {
			case <-ctx.Done():
				return nil
			case <-b.st.done:
				return nil
			case <-changed:
				continue
			}
		}
		pos := eventbus.Position{Topic: topicName, Offset: offset}
		err := b.deliver(ctx, delivery, pos, body, h)
		if err == nil {
			g.next = offset + 1
		}
		g.mu.Unlock()
		if err != nil {
			if b.stopping(ctx) {
				return nil
			}
			return err
		}
	}
}

// ReadRange delivers [from, min(to, End)) in increasing offset order and
// returns the offset to continue from. It stops at the end of the journal
// instead of waiting for the next event: a catch-up read must terminate, and
// Tail is what a caller uses to keep going.
func (b *Bus) ReadRange(ctx context.Context, topicName string, from, to int64, h eventbus.Handler) (int64, error) {
	if from < 0 {
		return from, fmt.Errorf("membus: read range %s: negative from %d", topicName, from)
	}
	t, err := b.topic(topicName)
	if err != nil {
		return from, err
	}
	if end := t.length(); to > end {
		to = end
	}
	if to <= from {
		return from, nil
	}
	delivery := b.delivery("journal." + topicName)
	next := from
	for next < to {
		if b.stopping(ctx) {
			return next, nil
		}
		body, ok := t.at(next)
		if !ok {
			return next, nil
		}
		pos := eventbus.Position{Topic: topicName, Offset: next}
		if err := b.deliver(ctx, delivery, pos, body, h); err != nil {
			if b.stopping(ctx) {
				return next, nil
			}
			return next, err
		}
		next++
	}
	return next, nil
}

// Tail delivers events from the given offset until ctx is done, and returns
// nil when it stops that way — like Subscribe.
func (b *Bus) Tail(ctx context.Context, topicName string, from int64, h eventbus.Handler) error {
	if from < 0 {
		return fmt.Errorf("membus: tail %s: negative from %d", topicName, from)
	}
	t, err := b.topic(topicName)
	if err != nil {
		return err
	}
	delivery := b.delivery("journal." + topicName)
	next := from
	for {
		if b.stopping(ctx) {
			return nil
		}
		changed := t.changed()
		body, ok := t.at(next)
		if !ok {
			select {
			case <-ctx.Done():
				return nil
			case <-b.st.done:
				return nil
			case <-changed:
				continue
			}
		}
		pos := eventbus.Position{Topic: topicName, Offset: next}
		if err := b.deliver(ctx, delivery, pos, body, h); err != nil {
			if b.stopping(ctx) {
				return nil
			}
			return err
		}
		next++
	}
}

// End returns the offset the next message of the topic will get.
func (b *Bus) End(_ context.Context, topicName string) (int64, error) {
	t, err := b.topic(topicName)
	if err != nil {
		return 0, err
	}
	return t.length(), nil
}

// Close stops the bus. A reader parked on an empty topic returns nil rather
// than waiting for an event that will never come: closing the bus is an
// orderly stop, and the kafka adapter reports it the same way — its Close
// closes the readers, and eventbus.stopped turns that into a nil return
// (decision Mi-1 on the review of T-005).
func (b *Bus) Close() error {
	b.st.mu.Lock()
	defer b.st.mu.Unlock()
	if b.st.closed {
		return nil
	}
	b.st.closed = true
	close(b.st.done)
	return nil
}

func (b *Bus) delivery(consumer string) eventbus.Delivery {
	return eventbus.Delivery{
		Consumer:           consumer,
		Registry:           b.registry,
		DLQ:                deadLetters{bus: b},
		SkipValidateOnRead: b.skipValidateOnRead,
		Backoff:            b.backoff,
		Timers:             b.timers,
		Log:                b.log,
	}
}

func (b *Bus) deliver(ctx context.Context, d eventbus.Delivery, pos eventbus.Position, body []byte, h eventbus.Handler) error {
	var ev eventbus.Event
	if err := json.Unmarshal(body, &ev); err != nil {
		return d.DeliverRaw(ctx, pos, body, err)
	}
	return d.Deliver(ctx, pos, ev, h)
}

func (b *Bus) write(name string, body []byte) error {
	t, err := b.topic(name)
	if err != nil {
		return err
	}
	t.append(body)
	return nil
}

// topic resolves a topic, creating it on demand when the bus was built without
// a topic list.
func (b *Bus) topic(name string) (*topic, error) {
	b.st.mu.Lock()
	defer b.st.mu.Unlock()
	if b.st.closed {
		return nil, eventbus.ErrClosed
	}
	if t, ok := b.st.topics[name]; ok {
		return t, nil
	}
	if !b.open {
		return nil, fmt.Errorf("%w: %s", ErrNoTopic, name)
	}
	t := newTopic()
	b.st.topics[name] = t
	return t, nil
}

func (b *Bus) group(topicName, groupName string) *group {
	key := topicName + "\x00" + groupName
	b.st.mu.Lock()
	defer b.st.mu.Unlock()
	if g, ok := b.st.groups[key]; ok {
		return g
	}
	g := &group{}
	b.st.groups[key] = g
	return g
}

// group is the cursor of one consumer group over one topic.
type group struct {
	mu   sync.Mutex
	next int64
}

// topic is an append-only log. appended is a broadcast channel closed and
// replaced on every append, which is how a reader parked at the end of the log
// learns that there is more.
type topic struct {
	mu       sync.Mutex
	records  [][]byte
	appended chan struct{}
}

func newTopic() *topic { return &topic{appended: make(chan struct{})} }

func (t *topic) append(body []byte) int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.records = append(t.records, body)
	close(t.appended)
	t.appended = make(chan struct{})
	return int64(len(t.records)) - 1
}

func (t *topic) at(offset int64) ([]byte, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if offset < 0 || offset >= int64(len(t.records)) {
		return nil, false
	}
	return t.records[offset], true
}

func (t *topic) length() int64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return int64(len(t.records))
}

func (t *topic) all() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	return append([][]byte(nil), t.records...)
}

// changed returns the channel that closes on the next append. A reader takes
// it before looking at the log, so that an append landing between the two does
// not leave it asleep.
func (t *topic) changed() <-chan struct{} {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.appended
}

// deadLetters parks failed events in dead_letters. Like the kafka adapter it
// writes the wrapper directly instead of publishing it: DeadLetter is not a
// registered event type, and it carries an envelope whose type routes
// elsewhere.
type deadLetters struct{ bus *Bus }

func (s deadLetters) WriteDeadLetter(_ context.Context, dl eventbus.DeadLetter) error {
	body, err := json.Marshal(dl)
	if err != nil {
		return fmt.Errorf("membus: marshal dead letter: %w", err)
	}
	return s.bus.write(eventbus.TopicDeadLetters, body)
}
