package links_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
)

type hookCall struct{ hook, player string }

func recordingHooks(calls *[]hookCall, failAt int) []links.ForgetHooks {
	hooks := make([]links.ForgetHooks, 0, 2)
	for i, name := range []string{"outbox", "session"} {
		hooks = append(hooks, links.ForgetFunc(func(_ context.Context, playerID string) error {
			*calls = append(*calls, hookCall{name, playerID})
			if i == failAt {
				return errors.New(name + " is down")
			}
			return nil
		}))
	}
	return hooks
}

// consentedWithCharacter makes the account of id a consented link with a
// character and one stored character request.
func consentedWithCharacter(t *testing.T, f fixture, id, playerID string) links.Link {
	t.Helper()
	ctx := context.Background()
	l := f.resolve(t, id, t0).Link
	if _, err := f.store.Consent(ctx, links.PlatformTelegram, id, complete(t0), t0); err != nil {
		t.Fatal(err)
	}
	if err := f.store.AttachPlayer(ctx, l.LinkID, playerID, "dark-forest-world"); err != nil {
		t.Fatal(err)
	}
	if err := f.store.SaveCharacterRequest(ctx, l.LinkID, "key-1",
		links.StoredResponse{PlayerID: playerID, StatusCode: 201, Body: []byte(`{}`)}, t0); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestForgetRunsTheHooksDeletesTheLinkAndItsRequests(t *testing.T) {
	var calls []hookCall
	f := newFixture(t, recordingHooks(&calls, -1)...)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")
	keep := consentedWithCharacter(t, f, "5550001111", "player-B")

	res, err := f.store.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if !res.Deleted || res.PlayerIDDetached == nil || *res.PlayerIDDetached != "player-A" {
		t.Fatalf("Forget = %+v, want deleted with player-A detached", res)
	}
	if want := []hookCall{{"outbox", "player-A"}, {"session", "player-A"}}; fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Errorf("hooks ran %v, want %v", calls, want)
	}
	if _, found, _ := f.store.ByExternal(ctx, links.PlatformTelegram, externalID); found {
		t.Error("the link is still there")
	}
	if _, found, _ := f.store.ByPlayer(ctx, "player-A"); found {
		t.Error("player-A still resolves to an external account")
	}
	if n := f.count(t, "SELECT COUNT(*) FROM character_requests WHERE player_id = 'player-A'"); n != 0 {
		t.Errorf("character_requests of the forgotten link = %d, want 0", n)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM character_requests WHERE link_id = ?", keep.LinkID); n != 1 {
		t.Errorf("character_requests of the other link = %d, want 1", n)
	}

	calls = nil
	again, err := f.store.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil || again.Deleted || again.PlayerIDDetached != nil {
		t.Errorf("repeated Forget = %+v %v, want {deleted:false} without an error", again, err)
	}
	if len(calls) != 0 {
		t.Errorf("a repeat ran the hooks %v", calls)
	}
	// After /forget the account starts over: a new link_id, no character.
	if res := f.resolve(t, externalID, t0); !res.Created || res.Link.PlayerID != nil {
		t.Errorf("Resolve after Forget: created %v, player %v; want a new link without a character", res.Created, res.Link.PlayerID)
	}
}

// US-009: /forget from an account without a character has nothing of a
// character to detach. The link itself holds the external ID and goes.
func TestForgetOfAnAccountWithoutACharacter(t *testing.T) {
	var calls []hookCall
	f := newFixture(t, recordingHooks(&calls, -1)...)
	f.resolve(t, externalID, t0)

	res, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID)
	if err != nil || !res.Deleted || res.PlayerIDDetached != nil {
		t.Fatalf("Forget = %+v %v, want the link deleted with no player detached", res, err)
	}
	if len(calls) != 0 {
		t.Errorf("hooks ran for an account without a character: %v", calls)
	}
	never, err := f.store.Forget(context.Background(), links.PlatformTelegram, "5550001111")
	if err != nil || never.Deleted || never.PlayerIDDetached != nil {
		t.Errorf("Forget of an unknown account = %+v %v, want nothing deleted", never, err)
	}
}

func TestAFailingHookKeepsTheLinkForARepeat(t *testing.T) {
	var calls []hookCall
	f := newFixture(t, recordingHooks(&calls, 0)...)
	consentedWithCharacter(t, f, externalID, "player-A")

	_, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID)
	if err == nil || !strings.Contains(err.Error(), "outbox is down") {
		t.Fatalf("Forget with a failing hook = %v, want its error", err)
	}
	if len(calls) != 1 {
		t.Errorf("hooks after the failing one ran: %v", calls)
	}
	if _, found, _ := f.store.ByPlayer(context.Background(), "player-A"); !found {
		t.Error("the link was deleted although the cascade stopped")
	}
}

func TestForgetByPlayer(t *testing.T) {
	f := newFixture(t)
	consentedWithCharacter(t, f, externalID, "player-A")
	res, err := f.store.ForgetByPlayer(context.Background(), "player-A")
	if err != nil || !res.Deleted || *res.PlayerIDDetached != "player-A" {
		t.Fatalf("ForgetByPlayer = %+v %v", res, err)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links"); n != 0 {
		t.Errorf("links left: %d", n)
	}
}

// A character bound while the hooks of /forget run (AttachPlayer of T-306)
// does not escape the cascade: the DELETE of the old character matches no
// row, the link is read again and the new character goes through the hooks
// before the link is deleted.
func TestForgetRepeatsTheCascadeForACharacterBoundMeanwhile(t *testing.T) {
	var calls []hookCall
	var s *links.SQLite
	var linkID string
	attached := false
	hook := links.ForgetFunc(func(ctx context.Context, playerID string) error {
		calls = append(calls, hookCall{"outbox", playerID})
		if !attached {
			attached = true
			return s.AttachPlayer(ctx, linkID, "player-B", "dark-forest-world")
		}
		return nil
	})
	f := newFixture(t, hook)
	s = f.store
	linkID = consentedWithCharacter(t, f, externalID, "player-A").LinkID

	res, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID)
	if err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if want := []hookCall{{"outbox", "player-A"}, {"outbox", "player-B"}}; fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Errorf("hooks ran %v, want %v", calls, want)
	}
	if !res.Deleted || res.PlayerIDDetached == nil || *res.PlayerIDDetached != "player-B" {
		t.Errorf("Forget = %+v, want deleted with player-B detached", res)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links"); n != 0 {
		t.Errorf("links rows = %d, want 0", n)
	}
}

// A character that keeps changing does not keep the cascade going forever.
func TestForgetGivesUpOnACharacterThatKeepsChanging(t *testing.T) {
	var s *links.SQLite
	var linkID string
	n := 0
	hook := links.ForgetFunc(func(ctx context.Context, _ string) error {
		n++
		return s.AttachPlayer(ctx, linkID, fmt.Sprintf("player-%d", n), "dark-forest-world")
	})
	f := newFixture(t, hook)
	s = f.store
	linkID = consentedWithCharacter(t, f, externalID, "player-A").LinkID

	if _, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID); err == nil {
		t.Fatal("Forget succeeded while the character changed on every attempt")
	}
	if n != 3 {
		t.Errorf("the cascade ran %d times, want 3", n)
	}
	if _, found, _ := f.store.ByExternal(context.Background(), links.PlatformTelegram, externalID); !found {
		t.Error("the link was deleted without the cascade of its character")
	}
}

// Two /forget of one account at once: the one whose DELETE finds the link
// gone answers {deleted: false}; only one reports the deletion.
func TestAParallelForgetAnswersNotDeleted(t *testing.T) {
	var s *links.SQLite
	var inner links.ForgetResult
	var innerErr error
	nested := false
	hook := links.ForgetFunc(func(ctx context.Context, _ string) error {
		if !nested {
			nested = true
			inner, innerErr = s.Forget(ctx, links.PlatformTelegram, externalID)
		}
		return nil
	})
	f := newFixture(t, hook)
	s = f.store
	consentedWithCharacter(t, f, externalID, "player-A")

	outer, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID)
	if innerErr != nil || !inner.Deleted {
		t.Fatalf("the parallel Forget = %+v %v, want the deletion", inner, innerErr)
	}
	if err != nil || outer.Deleted || outer.PlayerIDDetached != nil {
		t.Errorf("the Forget that came second = %+v %v, want {deleted:false}", outer, err)
	}
}

// ForgetByPlayer (adminForgetLink of T-356) whose link gets another character
// while the hooks run: the repeat reads the link by link_id, not by the old
// player_id, which no longer finds it. Otherwise the call would answer
// {deleted: false} and leave the external ID in links.db.
func TestForgetByPlayerFollowsTheLinkToACharacterBoundMeanwhile(t *testing.T) {
	var calls []hookCall
	var s *links.SQLite
	var linkID string
	attached := false
	hook := links.ForgetFunc(func(ctx context.Context, playerID string) error {
		calls = append(calls, hookCall{"outbox", playerID})
		if !attached {
			attached = true
			return s.AttachPlayer(ctx, linkID, "player-B", "dark-forest-world")
		}
		return nil
	})
	f := newFixture(t, hook)
	s = f.store
	linkID = consentedWithCharacter(t, f, externalID, "player-A").LinkID

	res, err := f.store.ForgetByPlayer(context.Background(), "player-A")
	if err != nil {
		t.Fatalf("ForgetByPlayer: %v", err)
	}
	if want := []hookCall{{"outbox", "player-A"}, {"outbox", "player-B"}}; fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Errorf("hooks ran %v, want %v", calls, want)
	}
	if !res.Deleted || res.PlayerIDDetached == nil || *res.PlayerIDDetached != "player-B" {
		t.Errorf("ForgetByPlayer = %+v, want deleted with player-B detached", res)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links"); n != 0 {
		t.Errorf("links rows = %d, want 0", n)
	}
}

// A parallel /forget deletes the link and a new Resolve of the same account
// creates another one while the hooks of this /forget run. The repeat reads
// the old link_id, finds nothing and answers {deleted: false}: the new link
// belongs to a new player and is not this call's to delete.
func TestForgetDoesNotDeleteALinkCreatedAfterItBegan(t *testing.T) {
	var s *links.SQLite
	var inner links.ForgetResult
	var innerErr error
	var fresh links.Resolution
	nested := false
	hook := links.ForgetFunc(func(ctx context.Context, _ string) error {
		if !nested {
			nested = true
			inner, innerErr = s.Forget(ctx, links.PlatformTelegram, externalID)
			if innerErr != nil {
				return innerErr
			}
			var err error
			fresh, err = s.Resolve(ctx, links.PlatformTelegram, externalID, t0)
			return err
		}
		return nil
	})
	f := newFixture(t, hook)
	s = f.store
	old := consentedWithCharacter(t, f, externalID, "player-A")

	outer, err := f.store.Forget(context.Background(), links.PlatformTelegram, externalID)
	if innerErr != nil || !inner.Deleted {
		t.Fatalf("the parallel Forget = %+v %v, want the deletion", inner, innerErr)
	}
	if !fresh.Created || fresh.Link.LinkID == old.LinkID {
		t.Fatalf("control: Resolve after the parallel Forget = created %v, link_id %q; want a new link", fresh.Created, fresh.Link.LinkID)
	}
	if err != nil || outer.Deleted || outer.PlayerIDDetached != nil {
		t.Errorf("the Forget that began before the new link = %+v %v, want {deleted:false}", outer, err)
	}
	got, found, err := f.store.ByExternal(context.Background(), links.PlatformTelegram, externalID)
	if err != nil || !found || got.LinkID != fresh.Link.LinkID {
		t.Errorf("the link created after the Forget began: found %v, link_id %q, %v; want %q kept", found, got.LinkID, err, fresh.Link.LinkID)
	}
}

// The physical deletion of SEC-04 and NFR-042 through the public API: right
// after Forget returns, the external ID is in none of links.db, -wal, -shm.
func TestForgetWipesTheExternalIDFromTheFileAndTheWAL(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")
	for i := range 20 {
		consentedWithCharacter(t, f, fmt.Sprintf("555%07d", i), fmt.Sprintf("player-%02d", i))
	}
	// Move the rows into the file itself, so the wipe is proven on the file and
	// not only on a WAL that would be dropped anyway.
	if err := f.store.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	if occurrences(t, f.path, externalID) == 0 {
		t.Fatal("control: the ID is not in links.db before Forget, the scan would prove nothing")
	}

	if _, err := f.store.Forget(ctx, links.PlatformTelegram, externalID); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	for _, name := range []string{f.path, f.path + "-wal", f.path + "-shm"} {
		if n := occurrences(t, name, externalID); n != 0 {
			t.Errorf("%s holds the forgotten ID %d times after Forget", name, n)
		}
	}
	if occurrences(t, f.path, "5550000019") == 0 {
		t.Error("the scan cannot see an ID that stays: the file was not read")
	}
}

// A reader outside the store holding an old snapshot blocks the checkpoint.
// The link is deleted, but Forget must not report the data as gone; the next
// Forget or Sweep finishes the wipe.
func TestABlockedCompactionIsReportedAndFinishedLater(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")
	if err := f.store.Compact(ctx); err != nil {
		t.Fatal(err)
	}

	reader, err := store.OpenLinks(ctx, f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&n); err != nil {
		t.Fatal(err)
	}
	// Do not wait five seconds for the reader.
	if _, err := f.db.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		t.Fatal(err)
	}

	res, err := f.store.Forget(ctx, links.PlatformTelegram, externalID)
	if !errors.Is(err, links.ErrCompactionPending) {
		t.Fatalf("Forget under a reader = %+v %v, want ErrCompactionPending", res, err)
	}
	if !res.Deleted || res.PlayerIDDetached == nil {
		t.Errorf("Forget under a reader = %+v, want the deletion reported with the error", res)
	}
	if !f.store.CompactionPending() {
		t.Error("the compaction is not marked pending")
	}
	if occurrences(t, f.path, externalID)+occurrences(t, f.path+"-wal", externalID) == 0 {
		t.Fatal("control: the blocked checkpoint left no trace, the finish below proves nothing")
	}

	// The client repeats while the reader is still there: still pending.
	if _, err := f.store.Forget(ctx, links.PlatformTelegram, externalID); !errors.Is(err, links.ErrCompactionPending) {
		t.Errorf("repeat under the reader = %v, want ErrCompactionPending", err)
	}

	_ = tx.Rollback()
	again, err := f.store.Forget(ctx, links.PlatformTelegram, externalID)
	if err != nil || again.Deleted {
		t.Fatalf("repeat after the reader left = %+v %v, want {deleted:false}", again, err)
	}
	if f.store.CompactionPending() {
		t.Error("the compaction is still pending after it succeeded")
	}
	for _, name := range []string{f.path, f.path + "-wal"} {
		if n := occurrences(t, name, externalID); n != 0 {
			t.Errorf("%s holds the forgotten ID %d times after the finished compaction", name, n)
		}
	}
}

func TestSweepFinishesAPendingCompaction(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	consentedWithCharacter(t, f, externalID, "player-A")

	reader, err := store.OpenLinks(ctx, f.path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reader.Close() }()
	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.ExecContext(ctx, "PRAGMA busy_timeout = 0"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Forget(ctx, links.PlatformTelegram, externalID); !errors.Is(err, links.ErrCompactionPending) {
		t.Fatalf("Forget = %v, want ErrCompactionPending", err)
	}
	if occurrences(t, f.path+"-wal", externalID)+occurrences(t, f.path, externalID) == 0 {
		t.Fatal("control: the blocked checkpoint left no trace, the scan after Sweep proves nothing")
	}
	_ = tx.Rollback()

	if err := f.store.Sweep(ctx, t0); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if f.store.CompactionPending() {
		t.Error("Sweep did not finish the compaction")
	}
	if n := occurrences(t, f.path+"-wal", externalID) + occurrences(t, f.path, externalID); n != 0 {
		t.Errorf("the forgotten ID occurs %d times after Sweep", n)
	}
}
