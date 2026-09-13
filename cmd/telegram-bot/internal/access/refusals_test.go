package access

import (
	"testing"
	"time"

	"multiverse-core.io/cmd/telegram-bot/internal/sender"
	"multiverse-core.io/cmd/telegram-bot/internal/updates"
	"multiverse-core.io/shared/clock"
)

func (r *refusals) remembered() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.since)
}

// TestARefusedIDIsLetGoByTheNextUpdateAfterItsWindow (review #2 of T-310,
// N-7): the id of a refused chat is not kept until the table fills up. The
// first update of any kind the gate sees once the series is a Window old —
// here the command of an invited player — lets it go.
func TestARefusedIDIsLetGoByTheNextUpdateAfterItsWindow(t *testing.T) {
	const (
		player   = int64(111222333)
		stranger = int64(987654321)
		group    = int64(-100500600700)
	)
	epoch := time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC)
	c := clock.NewManual(epoch)
	g, err := New(Options{AllowedUserIDs: []int64{player}, Clock: c, Sender: &sender.Fake{}})
	if err != nil {
		t.Fatal(err)
	}
	private := func(user int64) updates.Update {
		return updates.Update{Message: &updates.Message{Chat: updates.Chat{ID: user, Type: updates.ChatPrivate}, From: &updates.User{ID: user}}}
	}
	g.Check(private(stranger))
	g.Check(updates.Update{Message: &updates.Message{Chat: updates.Chat{ID: group, Type: "group"}, From: &updates.User{ID: player}}})
	if n := g.refused.remembered(); n != 2 {
		t.Fatalf("%d chats remembered after two refusals, want 2", n)
	}

	c.Set(epoch.Add(Window - time.Nanosecond))
	g.Check(private(player))
	if n := g.refused.remembered(); n != 2 {
		t.Fatalf("%d chats remembered within the Window, want 2: the series still holds", n)
	}

	c.Set(epoch.Add(Window))
	g.Check(private(player))
	if n := g.refused.remembered(); n != 0 {
		t.Fatalf("%d chats remembered a Window after their series, want 0", n)
	}

	// An update without a message lets go as well.
	g.Check(private(stranger))
	c.Advance(Window)
	g.Check(updates.Update{})
	if n := g.refused.remembered(); n != 0 {
		t.Fatalf("%d chats remembered after an empty update, want 0", n)
	}
}
