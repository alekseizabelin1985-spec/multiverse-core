package eventbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/segmentio/kafka-go"

	"multiverse-core.io/shared/clock"
)

// Reader tuning of ADR-007 p. 6: fetch as soon as there is a byte and wait no
// longer than 100 ms, otherwise the reaction time of the platform (NFR-001) is
// spent inside the client.
const (
	kafkaMinBytes = 1
	kafkaMaxBytes = 10 << 20
	kafkaMaxWait  = 100 * time.Millisecond
)

// Writer tuning. kafka-go closes a batch either when it holds BatchSize
// messages or when BatchTimeout has passed, and a synchronous WriteMessages
// waits for that batch; with the defaults (100 messages, 1 s) MVP-1 pays the
// full second on every publish, because it publishes one event at a time into
// single-partition topics and the batch never fills up. That alone breaks the
// 300 ms acknowledgement budget of NFR-001. BatchSize 1 sends immediately;
// BatchTimeout stays as a second guard, small enough to be invisible.
// Grouping becomes worth measuring only when the traffic makes it so — the
// publish latency itself is asserted by the contract test of T-014.
const (
	kafkaBatchSize    = 1
	kafkaBatchTimeout = 10 * time.Millisecond
)

// journalPartition is the only partition of every topic in MVP-1 (one
// partition per topic is what gives the strict order the replay relies on).
const journalPartition = 0

// KafkaConfig builds a Kafka bus. Only Brokers is mandatory; the registry
// falls back to the one installed with SetRegistry.
type KafkaConfig struct {
	Brokers  []string
	Registry Registry
	// SkipValidateOnRead turns off validation on read. The zero value
	// validates: SEC-16 wants the check on unless it is switched off on
	// purpose, so a caller that knows nothing about the flag gets the safe
	// behaviour. The process reads MV_BUS_VALIDATE_ON_READ and passes
	// SkipValidateOnRead = !MV_BUS_VALIDATE_ON_READ.
	SkipValidateOnRead bool
	// Backoff overrides DefaultBackoff.
	Backoff []time.Duration
	Timers  clock.Timers
	Log     *slog.Logger
}

// Kafka is the Bus and Journal over Redpanda, built on segmentio/kafka-go.
//
// One writer per topic is created lazily; subscriptions use a consumer group
// and commit only after the handler is done (at-least-once), while the journal
// reads by offset without a group.
type Kafka struct {
	brokers            []string
	registry           Registry
	skipValidateOnRead bool
	backoff            []time.Duration
	timers             clock.Timers
	log                *slog.Logger

	// closing is cancelled by Close. The loop of a subscription derives from it
	// the context of its fetch and of its commit, so that a call already in
	// flight when the bus closes ends instead of waiting for a reader that is
	// no longer there. The handler does not run on that context (C-01 v1.7
	// p. 3): see Subscribe.
	closing     context.Context
	stopReaders context.CancelFunc

	mu      sync.Mutex
	closed  bool
	writers map[string]*kafka.Writer
	readers map[*kafka.Reader]struct{}
}

var (
	_ Bus     = (*Kafka)(nil)
	_ Journal = (*Kafka)(nil)
)

// NewKafka builds the bus. It fails fast on a missing registry: without one
// the bus cannot route a type, and discovering that on the first publish would
// hide the misconfiguration until the world is already running.
func NewKafka(cfg KafkaConfig) (*Kafka, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("eventbus: kafka: no brokers configured")
	}
	reg := cfg.Registry
	if reg == nil {
		reg = PackageRegistry()
	}
	if reg == nil {
		return nil, ErrNoRegistry
	}
	closing, stopReaders := context.WithCancel(context.Background())
	return &Kafka{
		brokers:            append([]string(nil), cfg.Brokers...),
		registry:           reg,
		skipValidateOnRead: cfg.SkipValidateOnRead,
		backoff:            cfg.Backoff,
		timers:             cfg.Timers,
		log:                cfg.Log,
		closing:            closing,
		stopReaders:        stopReaders,
		writers:            make(map[string]*kafka.Writer),
		readers:            make(map[*kafka.Reader]struct{}),
	}, nil
}

// Publish routes the event by its type and sends it with the world as the
// message key, so that the events of one world keep their order.
func (k *Kafka) Publish(ctx context.Context, ev Event) error {
	topic, err := Route(k.registry, ev)
	if err != nil {
		return err
	}
	body, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("eventbus: marshal %s: %w", ev.Type, err)
	}
	return k.write(ctx, topic, ev.Key(), body)
}

// Subscribe consumes topic under the consumer group until ctx is done, and
// returns nil when it stops that way: a cancelled context is a shutdown, not a
// failure (the same semantics membus gives in T-014). The offset is committed
// only after the event is accounted for — handled, or parked in dead_letters —
// which is what makes delivery at-least-once.
//
// Closing the bus is an orderly stop as well: Subscribe returns nil. It ends
// the wait on the transport and not the work of the handler, which keeps ctx
// (C-01 v1.7 p. 3; the two contexts of the loop are explained below).
func (k *Kafka) Subscribe(ctx context.Context, topic, group string, h Handler) error {
	if group == "" {
		return errors.New("eventbus: subscribe: empty consumer group")
	}
	if h == nil {
		return errors.New("eventbus: subscribe: nil handler")
	}
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:     k.brokers,
		Topic:       topic,
		GroupID:     group,
		MinBytes:    kafkaMinBytes,
		MaxBytes:    kafkaMaxBytes,
		MaxWait:     kafkaMaxWait,
		StartOffset: kafka.FirstOffset,
		// Commit explicitly after the handler; a background commit would
		// acknowledge events that were never processed.
		CommitInterval: 0,
	})
	if err := k.track(reader); err != nil {
		return errors.Join(err, reader.Close())
	}
	// Two contexts, because Close may end the wait on the transport but not the
	// work of the handler (C-01 v1.7 p. 3, ADR-023 p. 4).
	//
	// loopCtx is the one Close cancels, and only the fetch and the commit run
	// on it. The loop follows the lifetime of the bus as well as that of its
	// caller: closing the bus closes the reader, which ends a fetch — but a
	// commit already on its way does not end with it, because kafka-go takes
	// the request into a buffered channel and answers it from a goroutine Close
	// has just stopped, so CommitMessages waits for a reply that will never
	// come (reader.go:894-912 of kafka-go v0.4.51). The process calls Close on
	// shutdown — once, after it has stopped its contexts — so a subscription
	// still caught between its handler and its commit at that moment would
	// hang it. Cancelling loopCtx
	// makes both the fetch and the commit end the way C-01 asks a stopped
	// subscription to end: with nil. Found by the Close case of the contract
	// test of T-014.
	//
	// The handler keeps ctx, the context of the caller. It may be halfway
	// through a PUT into object storage, and C-02 publishes the fact only once
	// that write is done, so the bus is not the one to cut it short: whoever
	// may stop the work is the one who cancels it, and the process does exactly
	// that — it cancels the context of the subscription in the Stop of the
	// context that owns it, before it closes the bus, with runtime.StopTimeout
	// over the whole path. An event whose handler finished after Close returned
	// is left uncommitted and delivered again: the goroutines of the reader are
	// gone by then, and nobody serves the commit. One that finished while Close
	// was still running may be committed or not — on cancellation kafka-go
	// flushes the commits already queued (reader.go:199-219) — and either is
	// allowed, because C-01 p. 2 does not promise the commit. That is the
	// at-least-once the platform already stands on. Until T-436 loopCtx went to the handler as well: the
	// cancellation was added for the commit and reached further than that.
	//
	// Whether the subscription stopped is therefore read from loopCtx and not
	// from ctx, and from the error as well: on a closed bus a dead letter fails
	// with ErrClosed, and that is the shutdown, not a failure of Subscribe —
	// also in the moment Close has marked the bus closed but loopCtx is not
	// cancelled yet (see stopped).
	loopCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	stopWatchingClose := context.AfterFunc(k.closing, cancel)
	defer stopWatchingClose()
	// The reader is closed when the subscription ends, not merely forgotten.
	// A kafka-go reader keeps its own goroutines and its membership of the
	// consumer group until Close: a subscription that stopped but left its
	// reader open still holds the single partition of the topic, so the next
	// subscription of the same group waits for a rebalance that may never give
	// it anything — while membus, whose cursor is a number, resumes at once.
	// The divergence was found by the contract test of T-014.
	defer k.closeReader(reader)

	delivery := k.delivery(group)
	for {
		msg, err := reader.FetchMessage(loopCtx)
		if err != nil {
			return k.readerError(loopCtx, topic, err)
		}
		pos := Position{Topic: topic, Offset: msg.Offset}
		if err := k.deliver(ctx, delivery, pos, msg.Value, h); err != nil {
			if stopped(loopCtx, err) {
				return nil
			}
			return err
		}
		if err := reader.CommitMessages(loopCtx, msg); err != nil {
			if stopped(loopCtx, err) {
				return nil
			}
			return fmt.Errorf("eventbus: commit %s@%d: %w", topic, msg.Offset, err)
		}
	}
}

// ReadRange delivers [from, min(to, End)) in increasing offset order and
// returns the offset to continue from.
//
// It clamps the range to the end of the journal on purpose. A catch-up read
// must terminate: without the clamp a caller asking for more than the topic
// holds — a replay whose recorded range is longer than what the broker kept,
// or a snapshot cursor read back with a generous window — would block inside
// FetchMessage until its context expired, and membus, which knows its own
// length, would return immediately. Tail is what a caller uses to keep
// following (divergence found by the contract test of T-014).
func (k *Kafka) ReadRange(ctx context.Context, topic string, from, to int64, h Handler) (int64, error) {
	if from < 0 {
		return from, fmt.Errorf("eventbus: read range %s: negative from %d", topic, from)
	}
	if to <= from {
		return from, nil
	}
	end, err := k.End(ctx, topic)
	if err != nil {
		return from, err
	}
	if to > end {
		to = end
	}
	if to <= from {
		return from, nil
	}
	reader, err := k.journalReader(topic, from)
	if err != nil {
		return from, err
	}
	defer k.closeReader(reader)

	delivery := k.delivery("journal." + topic)
	next := from
	for next < to {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return next, k.readerError(ctx, topic, err)
		}
		if msg.Offset >= to {
			return next, nil
		}
		pos := Position{Topic: topic, Offset: msg.Offset}
		if err := k.deliver(ctx, delivery, pos, msg.Value, h); err != nil {
			if stopped(ctx, err) {
				return next, nil
			}
			return next, err
		}
		next = msg.Offset + 1
	}
	return next, nil
}

// Tail delivers events from the given offset until ctx is done, and returns
// nil when it stops that way — like Subscribe.
func (k *Kafka) Tail(ctx context.Context, topic string, from int64, h Handler) error {
	if from < 0 {
		return fmt.Errorf("eventbus: tail %s: negative from %d", topic, from)
	}
	reader, err := k.journalReader(topic, from)
	if err != nil {
		return err
	}
	defer k.closeReader(reader)

	delivery := k.delivery("journal." + topic)
	for {
		msg, err := reader.FetchMessage(ctx)
		if err != nil {
			return k.readerError(ctx, topic, err)
		}
		pos := Position{Topic: topic, Offset: msg.Offset}
		if err := k.deliver(ctx, delivery, pos, msg.Value, h); err != nil {
			if stopped(ctx, err) {
				return nil
			}
			return err
		}
	}
}

// End returns the offset the next message of the topic will get.
func (k *Kafka) End(ctx context.Context, topic string) (int64, error) {
	conn, err := kafka.DialLeader(ctx, "tcp", k.brokers[0], topic, journalPartition)
	if err != nil {
		return 0, fmt.Errorf("eventbus: dial leader of %s: %w", topic, err)
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return 0, fmt.Errorf("eventbus: deadline on %s: %w", topic, err)
		}
	}
	end, err := conn.ReadLastOffset()
	if err != nil {
		return 0, fmt.Errorf("eventbus: read end of %s: %w", topic, err)
	}
	return end, nil
}

// Close stops every reader and writer the bus created.
func (k *Kafka) Close() error {
	k.mu.Lock()
	if k.closed {
		k.mu.Unlock()
		return nil
	}
	k.closed = true
	writers := make([]*kafka.Writer, 0, len(k.writers))
	for _, w := range k.writers {
		writers = append(writers, w)
	}
	k.writers = make(map[string]*kafka.Writer)
	readers := make([]*kafka.Reader, 0, len(k.readers))
	for r := range k.readers {
		readers = append(readers, r)
	}
	k.readers = make(map[*kafka.Reader]struct{})
	k.mu.Unlock()

	// Before the readers, not after: a subscription blocked on a commit is
	// released by the cancellation, and closing a reader out from under it
	// would not release it.
	k.stopReaders()

	var errs []error
	for _, r := range readers {
		errs = append(errs, r.Close())
	}
	for _, w := range writers {
		errs = append(errs, w.Close())
	}
	return errors.Join(errs...)
}

// delivery builds the read-side pipeline of one consumer.
func (k *Kafka) delivery(consumer string) Delivery {
	return Delivery{
		Consumer:           consumer,
		Registry:           k.registry,
		DLQ:                kafkaDeadLetters{bus: k},
		SkipValidateOnRead: k.skipValidateOnRead,
		Backoff:            k.backoff,
		Timers:             k.timers,
		Log:                k.log,
	}
}

func (k *Kafka) deliver(ctx context.Context, d Delivery, pos Position, body []byte, h Handler) error {
	var ev Event
	if err := json.Unmarshal(body, &ev); err != nil {
		return d.DeliverRaw(ctx, pos, body, err)
	}
	return d.Deliver(ctx, pos, ev, h)
}

func (k *Kafka) write(ctx context.Context, topic, key string, body []byte) error {
	w, err := k.writer(topic)
	if err != nil {
		return err
	}
	if err := w.WriteMessages(ctx, kafka.Message{Key: []byte(key), Value: body}); err != nil {
		return fmt.Errorf("eventbus: write to %s: %w", topic, err)
	}
	return nil
}

func (k *Kafka) writer(topic string) (*kafka.Writer, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return nil, ErrClosed
	}
	if w, ok := k.writers[topic]; ok {
		return w, nil
	}
	w := &kafka.Writer{
		Addr:     kafka.TCP(k.brokers...),
		Topic:    topic,
		Balancer: &kafka.Hash{},
		// The topics are created by redpanda-init with their retention: a
		// writer that creates them on the fly would silently produce a topic
		// that keeps data forever.
		AllowAutoTopicCreation: false,
		RequiredAcks:           kafka.RequireAll,
		BatchSize:              kafkaBatchSize,
		BatchTimeout:           kafkaBatchTimeout,
	}
	k.writers[topic] = w
	return w, nil
}

func (k *Kafka) journalReader(topic string, from int64) (*kafka.Reader, error) {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   k.brokers,
		Topic:     topic,
		Partition: journalPartition,
		MinBytes:  kafkaMinBytes,
		MaxBytes:  kafkaMaxBytes,
		MaxWait:   kafkaMaxWait,
	})
	if err := reader.SetOffset(from); err != nil {
		return nil, errors.Join(fmt.Errorf("eventbus: seek %s to %d: %w", topic, from, err), reader.Close())
	}
	if err := k.track(reader); err != nil {
		return nil, errors.Join(err, reader.Close())
	}
	return reader, nil
}

func (k *Kafka) closeReader(r *kafka.Reader) {
	k.untrack(r)
	_ = r.Close()
}

// track registers a reader so that Close can stop it, and refuses to start one
// on a closed bus.
func (k *Kafka) track(r *kafka.Reader) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return ErrClosed
	}
	k.readers[r] = struct{}{}
	return nil
}

func (k *Kafka) untrack(r *kafka.Reader) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.readers, r)
}

// readerError turns the end of a subscription into a normal return: a
// cancelled context or a reader closed by Close is a shutdown, not a failure.
func (k *Kafka) readerError(ctx context.Context, topic string, err error) error {
	if stopped(ctx, err) {
		return nil
	}
	return fmt.Errorf("eventbus: read %s: %w", topic, err)
}

// stopped reports whether err ends the loop because the caller or Close asked
// it to. Every exit of a read loop goes through it, so that a shutdown looks
// the same whether it interrupted the fetch, the handler or the commit.
//
// A closed bus is read from the error and not only from ctx. Close sets closed
// before it cancels the loop — the cancellation runs after the lock is
// released, and in the goroutine of context.AfterFunc — so a dead letter
// refused with ErrClosed inside that window finds loopCtx still alive, and
// Subscribe returned the refusal instead of nil (T-443). stopped includes
// busClosed, the test the log uses for bus_closed, so a Warn that calls the
// refusal an orderly stop always leads to a nil return; a nil return does not
// imply such a Warn. The cause of a dead letter is never in the chain (C-01
// v1.6), so a handler that itself failed with ErrClosed cannot pass for the
// end of the bus.
func stopped(ctx context.Context, err error) bool {
	return ctx.Err() != nil || errors.Is(err, io.EOF) || busClosed(err)
}

// kafkaDeadLetters parks failed events in dead_letters. The wrapper is written
// directly rather than published: it is not a registered event type, and it
// carries the original envelope whose type routes elsewhere.
type kafkaDeadLetters struct{ bus *Kafka }

func (s kafkaDeadLetters) WriteDeadLetter(ctx context.Context, dl DeadLetter) error {
	body, err := json.Marshal(dl)
	if err != nil {
		return fmt.Errorf("eventbus: marshal dead letter: %w", err)
	}
	return s.bus.write(ctx, TopicDeadLetters, dl.Original.Key(), body)
}
