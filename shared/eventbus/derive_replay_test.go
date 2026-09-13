package eventbus_test

import (
	"testing"

	"multiverse-core.io/shared/eventbus"
)

// C-01 v1.9: Derive copies Meta.Replay from the cause. The middleware of replay
// marks the event a handler reads, and everything the handler derives from it
// has to carry the mark down the chain, or a fact derived during a replay
// would pass for a live one (C-07). A separate file: the other tests of the
// package are being changed by T-460 at the same time (T-458).
func TestDeriveInheritsMetaReplay(t *testing.T) {
	for _, replayed := range []bool{true, false} {
		cause := eventbus.NewRoot("player.looked", "gateway", "dark-forest-world", nil, eventbus.ActorCI,
			map[string]any{"entity": map[string]any{"entity": map[string]any{"id": "player-A", "type": "player"}}})
		cause.Meta.Replay = replayed

		child := eventbus.Derive(cause, "dice.rolled", "core/swarm", map[string]any{"value": 17})
		if child.Meta.Replay != replayed {
			t.Errorf("cause with replay=%v: the derived event has replay=%v", replayed, child.Meta.Replay)
		}
		grandchild := eventbus.Derive(child, "combat.decided", "core/mechanics", map[string]any{"winner": "A"})
		if grandchild.Meta.Replay != replayed {
			t.Errorf("cause with replay=%v: the event derived from the derived one has replay=%v", replayed, grandchild.Meta.Replay)
		}
	}
}
