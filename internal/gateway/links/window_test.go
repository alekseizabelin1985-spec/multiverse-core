package links_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
)

// The window of T-303 in a forced order: the checkpoint of the sweeper is
// done; a reader arrives, a /forget deletes its link and fails its own
// compaction; only then does the sweeper come to clear the mark. The mark
// stays, and the next /forget still answers that the wipe is pending. Once the
// reader leaves, the next sweep finishes the wipe.
func TestASweepKeepsTheMarkOfAForgetThatDeletedAfterItsCheckpoint(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")
	if _, err := f.db.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		t.Fatal(err)
	}
	reader, err := store.OpenLinks(ctx, f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()

	var (
		tx        *sql.Tx
		forgotten error
		fired     bool
	)
	links.SetCompacted(f.store, func() {
		if fired {
			return
		}
		fired = true
		if tx, err = reader.BeginTx(ctx, nil); err != nil {
			t.Fatal(err)
		}
		var n int
		if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&n); err != nil {
			t.Fatal(err)
		}
		_, forgotten = f.store.Forget(ctx, links.PlatformTelegram, externalID)
	})

	if err := f.store.Compact(ctx); err != nil {
		t.Fatalf("the compaction of the sweeper: %v", err)
	}
	if !fired || !errors.Is(forgotten, links.ErrCompactionPending) {
		t.Fatalf("control: the /forget inside the window = %v (fired %v), want ErrCompactionPending", forgotten, fired)
	}
	if !f.store.CompactionPending() {
		t.Error("the sweeper cleared the mark the /forget set after its checkpoint")
	}
	if _, err := f.store.Forget(ctx, links.PlatformTelegram, "5550001111"); !errors.Is(err, links.ErrCompactionPending) {
		t.Errorf("the next /forget = %v, want ErrCompactionPending", err)
	}

	_ = tx.Rollback()
	if err := f.store.Sweep(ctx, t0); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if f.store.CompactionPending() {
		t.Error("the sweep after the reader left did not finish the wipe")
	}
	for _, name := range []string{f.path, f.path + "-wal"} {
		if n := occurrences(t, name, externalID); n != 0 {
			t.Errorf("%s holds the forgotten ID %d times", name, n)
		}
	}
}

// DetachPlayer unbinds a character from the link that still points at it and
// leaves a link bound to another character alone.
func TestDetachPlayerUnbindsOnlyTheCharacterItNames(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")

	if changed, err := f.store.DetachPlayer(ctx, "player-B"); err != nil || changed {
		t.Fatalf("DetachPlayer of a character no link has = %v %v", changed, err)
	}
	if l, _, _ := f.store.ByExternal(ctx, links.PlatformTelegram, externalID); l.PlayerID == nil || *l.PlayerID != "player-A" {
		t.Fatalf("the link lost its character: %v", l.PlayerID)
	}
	if changed, err := f.store.DetachPlayer(ctx, "player-A"); err != nil || !changed {
		t.Fatalf("DetachPlayer = %v %v", changed, err)
	}
	l, found, err := f.store.ByExternal(ctx, links.PlatformTelegram, externalID)
	if err != nil || !found || l.PlayerID != nil || l.WorldID != nil || l.Status != links.StatusConsented {
		t.Errorf("link after DetachPlayer = %+v found %v %v, want consented without a character", l, found, err)
	}
	if _, err := f.store.DetachPlayer(ctx, ""); err == nil {
		t.Error("DetachPlayer of an empty player_id succeeded")
	}
}
