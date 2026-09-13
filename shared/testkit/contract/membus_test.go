// The membus half of the gate of wave 1. It runs in the unit job, with no tag
// and no Docker, against exactly the assertions redpanda_integration_test.go
// runs against the broker: that is the whole point of the package, and the
// reason the two live in one directory rather than next to their
// implementations (tasks.md T-014).
package contract_test

import (
	"context"
	"testing"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/eventbus/membus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/contract"
)

// noPause is a backoff of three retries that does not wait. The number of
// attempts is the contract; the pauses would only make the job slower.
var noPause = []time.Duration{0, 0, 0}

// platformTopics is the topic list of the platform, the one redpanda-init
// creates in compose. Passing it to membus is what makes the stub refuse an
// unknown topic, as the broker does.
func platformTopics() []string {
	specs := contracts.Topics()
	names := make([]string, 0, len(specs))
	for _, spec := range specs {
		names = append(names, spec.Name)
	}
	return names
}

func TestBusContractOnMembus(t *testing.T) {
	bus, err := membus.New(membus.Config{
		Registry: contracts.Default(),
		Topics:   platformTopics(),
		Backoff:  noPause,
	})
	if err != nil {
		t.Fatalf("new membus: %v", err)
	}
	t.Cleanup(func() { _ = bus.Close() })

	contract.Run(t, contract.Target{
		Name:    "membus",
		Bus:     bus,
		Journal: bus,
		Append:  bus.Append,
		DeadLetters: func(context.Context) ([]eventbus.DeadLetter, error) {
			return bus.DeadLetters()
		},
		Duplicate: func(ctx context.Context, ev eventbus.Event) error {
			// The --chaos=duplicate switch of ADR-010 p. 4, turned on for one
			// publish: the stub is the only implementation that has one.
			bus.SetChaos(membus.Chaos{Duplicate: true})
			defer bus.SetChaos(membus.Chaos{})
			return bus.Publish(ctx, ev)
		},
		Lenient: func() (eventbus.Bus, func(), error) {
			// A view of the same log, not a second bus: a second membus would
			// have a log of its own, and nothing published into it.
			return bus.Lenient(), func() {}, nil
		},
		Spare: func() (eventbus.Bus, bool, error) {
			// A second membus has a log of its own, and its Close takes the log
			// with it: an in-memory transport does not outlive its bus, so
			// nothing it left behind can be read afterwards.
			spare, err := membus.New(membus.Config{
				Registry: contracts.Default(),
				Topics:   platformTopics(),
				Backoff:  noPause,
			})
			if err != nil {
				return nil, false, err
			}
			return spare, false, nil
		},
		Stalled: func() (contract.StalledBus, error) {
			// A bus of its own, like the spare: a view of the target could not
			// carry other timers. Its dead letters are read from its own log.
			stalled, err := membus.New(membus.Config{
				Registry: contracts.Default(),
				Topics:   platformTopics(),
				Backoff:  contract.StalledBackoff(),
				Timers:   clock.NewManual(testkit.Epoch).Timers(),
			})
			if err != nil {
				return contract.StalledBus{}, err
			}
			return contract.StalledBus{
				Bus:     stalled,
				Journal: stalled,
				DeadLetters: func(context.Context) ([]eventbus.DeadLetter, error) {
					return stalled.DeadLetters()
				},
				Release: func() { _ = stalled.Close() },
			}, nil
		},
		Close: bus.Close,
	})
}
