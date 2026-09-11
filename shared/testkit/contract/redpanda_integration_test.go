//go:build integration

// The other half of the gate of wave 1: the same contract, run against the
// broker the platform actually ships with. Nothing in this file describes
// behaviour — every assertion lives in contract.go and runs against membus too
// (tasks.md T-014, ADR-010).
package contract_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/testcontainers/testcontainers-go"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/contract"
)

func TestBusContractOnRedpanda(t *testing.T) {
	ctx := context.Background()

	topics := platformTopics()
	rp, err := testkit.StartRedpanda(ctx, topics...)
	if rp != nil {
		testcontainers.CleanupContainer(t, rp.Container())
	}
	if err != nil {
		t.Fatalf("start redpanda: %v", err)
	}
	t.Logf("redpanda at %s with %d topics", rp.Broker, len(topics))

	bus, err := eventbus.NewKafka(eventbus.KafkaConfig{
		Brokers:  rp.Brokers(),
		Registry: contracts.Default(),
		Backoff:  noPause,
	})
	if err != nil {
		t.Fatalf("new kafka bus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	contract.Run(t, contract.Target{
		Name:    "redpanda",
		Bus:     bus,
		Journal: bus,
		Live:    true,
		Append: func(topic string, body []byte) error {
			return appendRaw(ctx, rp.Broker, topic, body)
		},
		DeadLetters: func(ctx context.Context) ([]eventbus.DeadLetter, error) {
			return readDeadLetters(ctx, rp.Broker)
		},
		Duplicate: func(ctx context.Context, ev eventbus.Event) error {
			// A broker has no chaos switch. Publishing the same event twice is
			// what a producer retry after a lost acknowledgement puts on the
			// wire, and it is indistinguishable from the switch where it
			// matters: the consumer sees one event id delivered twice.
			if err := bus.Publish(ctx, ev); err != nil {
				return err
			}
			return bus.Publish(ctx, ev)
		},
		Lenient: func() (eventbus.Bus, func(), error) {
			// A second bus over the same broker: the topics are the transport
			// here, so only the flag differs. NewKafka opens nothing until it
			// is used.
			lenient, err := eventbus.NewKafka(eventbus.KafkaConfig{
				Brokers:            rp.Brokers(),
				Registry:           contracts.Default(),
				Backoff:            noPause,
				SkipValidateOnRead: true,
			})
			if err != nil {
				return nil, nil, err
			}
			return lenient, func() { _ = lenient.Close() }, nil
		},
		Spare: func() (eventbus.Bus, bool, error) {
			// A second client of the same broker: its Close ends its own readers
			// and writers, and the topics — with whatever it left uncommitted or
			// parked — stay, as they do for a process that stops.
			spare, err := eventbus.NewKafka(eventbus.KafkaConfig{
				Brokers:  rp.Brokers(),
				Registry: contracts.Default(),
				Backoff:  noPause,
			})
			if err != nil {
				return nil, false, err
			}
			return spare, true, nil
		},
		Close: bus.Close,
	})
}

// appendRaw writes a body into a topic without routing or validating it, which
// is how the suite produces a message no publisher could.
func appendRaw(ctx context.Context, broker, topic string, body []byte) error {
	w := &kafka.Writer{
		Addr:                   kafka.TCP(broker),
		Topic:                  topic,
		AllowAutoTopicCreation: false,
		RequiredAcks:           kafka.RequireAll,
		BatchSize:              1,
		BatchTimeout:           10 * time.Millisecond,
	}
	defer func() { _ = w.Close() }()
	if err := w.WriteMessages(ctx, kafka.Message{Value: body}); err != nil {
		return fmt.Errorf("append to %s: %w", topic, err)
	}
	return nil
}

// readDeadLetters reads dead_letters with a plain reader. It cannot go through
// the journal: a dead letter is not an event, and reading it as one would fail
// validation and park it again, forever.
func readDeadLetters(ctx context.Context, broker string) ([]eventbus.DeadLetter, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	conn, err := kafka.DialLeader(ctx, "tcp", broker, eventbus.TopicDeadLetters, 0)
	if err != nil {
		return nil, fmt.Errorf("dial dead_letters: %w", err)
	}
	end, err := conn.ReadLastOffset()
	_ = conn.Close()
	if err != nil {
		return nil, fmt.Errorf("read the end of dead_letters: %w", err)
	}
	if end == 0 {
		return nil, nil
	}

	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{broker},
		Topic:     eventbus.TopicDeadLetters,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10 << 20,
		MaxWait:   100 * time.Millisecond,
	})
	defer func() { _ = reader.Close() }()
	if err := reader.SetOffset(0); err != nil {
		return nil, fmt.Errorf("seek dead_letters: %w", err)
	}

	letters := make([]eventbus.DeadLetter, 0, end)
	for range end {
		msg, err := reader.ReadMessage(ctx)
		if err != nil {
			return nil, fmt.Errorf("read dead_letters: %w", err)
		}
		var dl eventbus.DeadLetter
		if err := json.Unmarshal(msg.Value, &dl); err != nil {
			return nil, fmt.Errorf("decode dead letter at %d: %w", msg.Offset, err)
		}
		letters = append(letters, dl)
	}
	return letters, nil
}
