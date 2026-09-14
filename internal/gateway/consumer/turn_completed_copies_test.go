package consumer_test

import (
	"fmt"
	"testing"

	"multiverse-core.io/shared/eventbus"
)

// oneTurnCompleted checks what the repeats of an ack left on the broker: every
// attempt under one id, and every copy on the broker under that id. Several
// copies are a duplicate of at-least-once, not a second turn (the guarantees
// of C-01: the consumer drops it by Event.ID; components/gateway-and-bot.md
// §5.5 and §7.7: a repeat keeps the id). On Linux kafka-go takes a refused
// connection for a transient error: a publication whose deadline ran out while
// the broker was away stays in its writer, is retried there and lands once the
// broker is back, next to the repeat that answered 200 (T-483). It returns ""
// when the check holds. The integration test of the outage uses it.
func oneTurnCompleted(published []eventbus.Event, ids []string) string {
	if len(ids) != 1 {
		return fmt.Sprintf("turn.completed attempted under ids %v, want one id for every attempt", ids)
	}
	if len(published) == 0 {
		return fmt.Sprintf("no turn.completed on the broker, want it under %s", ids[0])
	}
	for _, ev := range published {
		if ev.ID != ids[0] {
			return fmt.Sprintf("turn.completed on the broker under %s, want every copy under the id of the attempts %s", ev.ID, ids[0])
		}
	}
	return ""
}

func TestOneTurnCompletedTakesCopiesUnderOneIDAndRefusesAnyOtherID(t *testing.T) {
	copyOf := func(id string) eventbus.Event { return eventbus.Event{ID: id, Type: "analytics.turn.completed"} }
	cases := []struct {
		name      string
		published []eventbus.Event
		ids       []string
		holds     bool
	}{
		{"one copy under the id of the attempts", []eventbus.Event{copyOf("t-1")}, []string{"t-1"}, true},
		{"a late attempt landed next to the repeat", []eventbus.Event{copyOf("t-1"), copyOf("t-1")}, []string{"t-1"}, true},
		{"attempts under two ids", []eventbus.Event{copyOf("t-1")}, []string{"t-1", "t-2"}, false},
		{"no attempt", []eventbus.Event{copyOf("t-1")}, nil, false},
		{"nothing on the broker", nil, []string{"t-1"}, false},
		{"a copy under another id behind the right one", []eventbus.Event{copyOf("t-1"), copyOf("t-2")}, []string{"t-1"}, false},
		{"the only copy under another id", []eventbus.Event{copyOf("t-2")}, []string{"t-1"}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			msg := oneTurnCompleted(c.published, c.ids)
			if holds := msg == ""; holds != c.holds {
				t.Errorf("oneTurnCompleted(%d copies, ids %v) = %q, want holds=%v", len(c.published), c.ids, msg, c.holds)
			}
		})
	}
}
