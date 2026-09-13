package recorded_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"

	"multiverse-core.io/internal/llm"
	"multiverse-core.io/internal/llm/providers/recorded"
	"multiverse-core.io/shared/eventbus"
)

// overTheWire returns the events as a consumer receives them: encoded and
// decoded with encoding/json, so numbers of the payload come back as float64.
// It is a helper of the tests, not a reader of recordings: the provider gets
// its records from a Source built outside it.
func overTheWire(t *testing.T, events ...eventbus.Event) []eventbus.Event {
	t.Helper()
	out := make([]eventbus.Event, len(events))
	for i, ev := range events {
		raw, err := json.Marshal(ev)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(raw, &out[i]); err != nil {
			t.Fatal(err)
		}
	}
	return out
}

func ids(t *testing.T, src recorded.Source) []string {
	t.Helper()
	var got []string
	if err := src(context.Background(), func(_ context.Context, ev eventbus.Event) error {
		got = append(got, ev.ID)
		return nil
	}); err != nil {
		t.Fatalf("source: %v", err)
	}
	return got
}

// A record decoded from JSON keys the same way as one built in Go.
func TestADecodedRecordKeysTheSame(t *testing.T) {
	root := cause()
	a := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationInvalid, "not json")
	b := record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationValid, `{"text":"Волк рычит."}`)
	decoded := overTheWire(t, a, b)
	if _, ok := decoded[1].Payload["attempt"].(float64); !ok {
		t.Fatalf("attempt decoded as %T, want float64", decoded[1].Payload["attempt"])
	}
	p, err := recorded.New(context.Background(), recorded.Slice(decoded...))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := p.Generate(withCall(agentGM, 2), req(root, llm.PhaseNarrative))
	if err != nil || got.Content != `{"text":"Волк рычит."}` || got.Tokens.Cached != 400 || got.ReasoningLen != 17 {
		t.Fatalf("Generate = %+v, %v", got, err)
	}
}

func TestSourcesStopOnADoneContext(t *testing.T) {
	root := cause()
	ev := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"x"}`)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	nop := func(context.Context, eventbus.Event) error { return nil }
	if err := recorded.Slice(ev)(ctx, nop); !errors.Is(err, context.Canceled) {
		t.Errorf("Slice: err = %v, want context.Canceled", err)
	}
	if _, err := recorded.New(ctx, recorded.Slice(ev)); !errors.Is(err, context.Canceled) {
		t.Errorf("New: err = %v, want context.Canceled", err)
	}
}

// Slice stops at the first error of the handler and returns it.
func TestSliceStopsOnAHandlerError(t *testing.T) {
	root := cause()
	events := []eventbus.Event{
		record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"a"}`),
		record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationValid, `{"text":"b"}`),
	}
	if got := ids(t, recorded.Slice(events...)); !slices.Equal(got, []string{events[0].ID, events[1].ID}) {
		t.Fatalf("ids = %v", got)
	}
	stop := errors.New("stop")
	calls := 0
	err := recorded.Slice(events...)(context.Background(), func(context.Context, eventbus.Event) error {
		calls++
		return stop
	})
	if !errors.Is(err, stop) || calls != 1 {
		t.Fatalf("err = %v after %d calls, want stop after 1", err, calls)
	}
}

// Events adapts a sequence and stops pulling it when the handler fails.
func TestEventsAdaptsASequence(t *testing.T) {
	root := cause()
	a := record(root, agentGM, llm.PhaseNarrative, 1, llm.ValidationValid, `{"text":"a"}`)
	b := record(root, agentGM, llm.PhaseNarrative, 2, llm.ValidationValid, `{"text":"b"}`)
	pulled := 0
	seq := func(yield func(eventbus.Event) bool) {
		for _, ev := range []eventbus.Event{a, b} {
			pulled++
			if !yield(ev) {
				return
			}
		}
	}
	if got := ids(t, recorded.Events(seq)); !slices.Equal(got, []string{a.ID, b.ID}) {
		t.Fatalf("ids = %v", got)
	}
	pulled = 0
	stop := errors.New("stop")
	if err := recorded.Events(seq)(context.Background(), func(context.Context, eventbus.Event) error { return stop }); !errors.Is(err, stop) {
		t.Fatalf("err = %v, want stop", err)
	}
	if pulled != 1 {
		t.Fatalf("pulled %d events after the handler failed on the first, want 1", pulled)
	}
}
