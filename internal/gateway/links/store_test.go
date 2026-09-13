package links_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
)

func TestResolveCreatesAPendingLinkOnceAndTouchesItAfterwards(t *testing.T) {
	f := newFixture(t)
	first := f.resolve(t, externalID, t0)
	if !first.Created {
		t.Fatal("the first Resolve did not create the link")
	}
	l := first.Link
	if l.Status != links.StatusPendingConsent || l.LinkID == "" || l.PlayerID != nil || l.WorldID != nil {
		t.Fatalf("created link: status %q, link_id %q, player %v, world %v; want pending_consent with a link_id and no character",
			l.Status, l.LinkID, l.PlayerID, l.WorldID)
	}
	if !l.CreatedAt.Equal(t0) || !l.LastSeenAt.Equal(t0) || !first.PreviousSeenAt.Equal(t0) {
		t.Errorf("created %v, last seen %v, previous %v; want all %v", l.CreatedAt, l.LastSeenAt, first.PreviousSeenAt, t0)
	}

	later := t0.Add(time.Hour)
	second := f.resolve(t, externalID, later)
	switch {
	case second.Created:
		t.Error("the second Resolve created another link")
	case second.Link.LinkID != l.LinkID:
		t.Errorf("link_id changed from %q to %q", l.LinkID, second.Link.LinkID)
	case !second.PreviousSeenAt.Equal(t0):
		t.Errorf("PreviousSeenAt = %v, want the last_seen_at before the call %v", second.PreviousSeenAt, t0)
	case !second.Link.LastSeenAt.Equal(later):
		t.Errorf("LastSeenAt = %v, want %v", second.Link.LastSeenAt, later)
	}
	stored, _, err := f.store.ByExternal(context.Background(), links.PlatformTelegram, externalID)
	if err != nil || !stored.LastSeenAt.Equal(later) || !stored.CreatedAt.Equal(t0) {
		t.Errorf("stored link: last seen %v, created %v, err %v; want %v and %v", stored.LastSeenAt, stored.CreatedAt, err, later, t0)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links"); n != 1 {
		t.Errorf("links rows = %d, want 1", n)
	}
}

func TestIncompleteConsentLeavesTheLinkPending(t *testing.T) {
	for name, form := range map[string]links.ConsentForm{
		"no notice":  {Consent: true, AgeConfirmed: true, ShownAt: t0},
		"no consent": {NoticeShown: true, AgeConfirmed: true, ShownAt: t0},
		"no age":     {NoticeShown: true, Consent: true, ShownAt: t0},
		"nothing":    {},
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.resolve(t, externalID, t0)
			later := t0.Add(time.Minute)
			l, err := f.store.Consent(context.Background(), links.PlatformTelegram, externalID, form, later)
			if !errors.Is(err, links.ErrConsentIncomplete) {
				t.Fatalf("Consent = %v, want ErrConsentIncomplete", err)
			}
			stored, _, _ := f.store.ByExternal(context.Background(), links.PlatformTelegram, externalID)
			if stored.Status != links.StatusPendingConsent || stored.ConsentAt != nil || stored.AgeConfirmedAt != nil || stored.NoticeShownAt != nil {
				t.Errorf("stored link after an incomplete consent: %q %v %v %v, want pending without timestamps",
					stored.Status, stored.NoticeShownAt, stored.ConsentAt, stored.AgeConfirmedAt)
			}
			if !l.LastSeenAt.Equal(later) || !stored.LastSeenAt.Equal(later) {
				t.Errorf("last_seen_at = %v (stored %v), want %v", l.LastSeenAt, stored.LastSeenAt, later)
			}
		})
	}
}

// api-contracts.md §1.2: an incomplete consent of an account without a link
// creates nothing, so a refusal leaves no external ID behind (SEC-03).
func TestIncompleteConsentWithoutResolveCreatesNothing(t *testing.T) {
	f := newFixture(t)
	incomplete := links.ConsentForm{NoticeShown: true, Consent: true, ShownAt: t0}
	l, err := f.store.Consent(context.Background(), links.PlatformTelegram, externalID, incomplete, t0)
	if !errors.Is(err, links.ErrConsentIncomplete) {
		t.Fatalf("Consent = %v, want ErrConsentIncomplete", err)
	}
	if l.LinkID != "" || l.ExternalID != "" {
		t.Error("an incomplete consent returned a link")
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links"); n != 0 {
		t.Errorf("links rows after a refused consent = %d, want 0", n)
	}
}

func TestConsentSetsTheThreeTimestampsOnce(t *testing.T) {
	f := newFixture(t)
	f.resolve(t, externalID, t0)
	shown, at := t0.Add(time.Second), t0.Add(2*time.Second)
	l, err := f.store.Consent(context.Background(), links.PlatformTelegram, externalID, complete(shown), at)
	if err != nil {
		t.Fatalf("Consent: %v", err)
	}
	stored, _, _ := f.store.ByExternal(context.Background(), links.PlatformTelegram, externalID)
	for _, got := range []links.Link{l, stored} {
		if got.Status != links.StatusConsented || got.NoticeShownAt == nil || got.ConsentAt == nil || got.AgeConfirmedAt == nil {
			t.Fatalf("consented link %q has timestamps %v %v %v, want all three", got.Status, got.NoticeShownAt, got.ConsentAt, got.AgeConfirmedAt)
		}
		if !got.NoticeShownAt.Equal(shown) || !got.ConsentAt.Equal(at) || !got.AgeConfirmedAt.Equal(at) {
			t.Errorf("timestamps %v %v %v, want %v %v %v", *got.NoticeShownAt, *got.ConsentAt, *got.AgeConfirmedAt, shown, at, at)
		}
	}
	if n := f.count(t, `SELECT COUNT(*) FROM links WHERE status = 'consented'
		AND (notice_shown_at IS NULL OR consent_at IS NULL OR age_confirmed_at IS NULL)`); n != 0 {
		t.Errorf("%d consented rows lack a timestamp", n)
	}

	again := t0.Add(time.Hour)
	repeat, err := f.store.Consent(context.Background(), links.PlatformTelegram, externalID, complete(again), again)
	if err != nil {
		t.Fatalf("repeated Consent: %v", err)
	}
	if !repeat.ConsentAt.Equal(at) || !repeat.NoticeShownAt.Equal(shown) || !repeat.LastSeenAt.Equal(again) {
		t.Errorf("a repeat changed the evidence: consent %v, shown %v, last seen %v", *repeat.ConsentAt, *repeat.NoticeShownAt, repeat.LastSeenAt)
	}
}

func TestConsentWithoutResolveCreatesTheLink(t *testing.T) {
	f := newFixture(t)
	l, err := f.store.Consent(context.Background(), links.PlatformTelegram, externalID, complete(t0), t0)
	if err != nil || l.Status != links.StatusConsented || l.LinkID == "" {
		t.Fatalf("Consent on an unknown account = %q %q %v, want a consented link", l.Status, l.LinkID, err)
	}
	if res := f.resolve(t, externalID, t0); res.Created || res.Link.LinkID != l.LinkID {
		t.Errorf("Resolve after Consent: created %v, link_id %q, want the same link %q", res.Created, res.Link.LinkID, l.LinkID)
	}
}

func TestAttachPlayerReplacesTheCharacterAndKeepsTheLinkID(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l := f.resolve(t, externalID, t0).Link

	dead, err := links.NewPlayerID(sequence())
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.AttachPlayer(ctx, l.LinkID, dead, "dark-forest-world"); err != nil {
		t.Fatalf("AttachPlayer: %v", err)
	}
	// The character died: the account creates a new one on the same link.
	next := "player-new"
	if err := f.store.AttachPlayer(ctx, l.LinkID, next, "dark-forest-world"); err != nil {
		t.Fatalf("AttachPlayer of the new character: %v", err)
	}
	got, found, err := f.store.ByPlayer(ctx, next)
	if err != nil || !found {
		t.Fatalf("ByPlayer(new) = %v %v", found, err)
	}
	if got.LinkID != l.LinkID || *got.PlayerID != next || *got.WorldID != "dark-forest-world" {
		t.Errorf("after the replacement: link_id %q (want %q), player %q, world %q", got.LinkID, l.LinkID, *got.PlayerID, *got.WorldID)
	}
	if _, found, _ := f.store.ByPlayer(ctx, dead); found {
		t.Error("the dead player_id still resolves to the link")
	}
	platform, id, ok, err := f.store.RouteFor(ctx, next)
	if err != nil || !ok || platform != links.PlatformTelegram || id != externalID {
		t.Errorf("RouteFor(new) = %q %q %v %v", platform, id, ok, err)
	}
	if _, _, ok, err := f.store.RouteFor(ctx, dead); ok || err != nil {
		t.Errorf("RouteFor(dead) = %v %v, want no route", ok, err)
	}

	other := f.resolve(t, "5550001111", t0).Link
	if err := f.store.AttachPlayer(ctx, other.LinkID, next, "dark-forest-world"); !errors.Is(err, links.ErrPlayerIDTaken) {
		t.Errorf("AttachPlayer of a taken player_id = %v, want ErrPlayerIDTaken", err)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM links WHERE player_id = ?", next); n != 1 {
		t.Errorf("player_id %q is on %d links, want 1", next, n)
	}
	if err := f.store.AttachPlayer(ctx, "no-such-link", "player-x", "dark-forest-world"); !errors.Is(err, links.ErrNotFound) {
		t.Errorf("AttachPlayer on an unknown link = %v, want ErrNotFound", err)
	}
}

// --id-source=sequence makes the identifiers of a run reproducible: the same
// calls on two fresh stores give the same link_id and player_id.
func TestIdentifiersAreReproducibleWithASequence(t *testing.T) {
	run := func() (string, string) {
		f := newFixture(t)
		l := f.resolve(t, externalID, t0).Link
		playerID, err := links.NewPlayerID(sequence())
		if err != nil {
			t.Fatal(err)
		}
		return l.LinkID, playerID
	}
	link1, player1 := run()
	link2, player2 := run()
	if link1 != link2 || player1 != player2 {
		t.Errorf("two runs gave %q/%q and %q/%q", link1, player1, link2, player2)
	}
	if player1 != links.PlayerIDPrefix+"seq-1" {
		t.Errorf("player_id = %q, want %sseq-1", player1, links.PlayerIDPrefix)
	}
	if _, err := links.NewLinkID(func() string { return "" }); err == nil {
		t.Error("an empty id was accepted as a link_id")
	}
}

func TestCharacterRequestsKeepTheFirstAnswerForTheirTTL(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	l := f.resolve(t, externalID, t0).Link

	first := links.StoredResponse{PlayerID: "player-1", StatusCode: 202, Body: []byte(`{"status":"creating"}`)}
	if err := f.store.SaveCharacterRequest(ctx, l.LinkID, "key-1", first, t0); err != nil {
		t.Fatalf("SaveCharacterRequest: %v", err)
	}
	second := links.StoredResponse{PlayerID: "player-2", StatusCode: 201, Body: []byte(`{}`)}
	if err := f.store.SaveCharacterRequest(ctx, l.LinkID, "key-1", second, t0.Add(time.Minute)); err != nil {
		t.Fatalf("SaveCharacterRequest again: %v", err)
	}
	got, ok, err := f.store.CharacterRequest(ctx, l.LinkID, "key-1", t0.Add(time.Hour))
	if err != nil || !ok || got.PlayerID != first.PlayerID || got.StatusCode != 202 || !bytes.Equal(got.Body, first.Body) {
		t.Fatalf("CharacterRequest = %+v %v %v, want the first answer", got, ok, err)
	}
	if _, ok, _ := f.store.CharacterRequest(ctx, l.LinkID, "key-2", t0); ok {
		t.Error("an unknown key has an answer")
	}

	expired := t0.Add(store.KeyTTL)
	if _, ok, _ := f.store.CharacterRequest(ctx, l.LinkID, "key-1", expired); ok {
		t.Error("the answer is still returned at its expiry")
	}
	if err := f.store.SaveCharacterRequest(ctx, l.LinkID, "key-1", second, expired); err != nil {
		t.Fatalf("SaveCharacterRequest after the expiry: %v", err)
	}
	if got, _, _ := f.store.CharacterRequest(ctx, l.LinkID, "key-1", expired); got.PlayerID != second.PlayerID {
		t.Errorf("after the expiry the key answers %q, want the new answer %q", got.PlayerID, second.PlayerID)
	}

	if err := f.store.Sweep(ctx, expired.Add(store.KeyTTL)); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if n := f.count(t, "SELECT COUNT(*) FROM character_requests"); n != 0 {
		t.Errorf("character_requests after the sweep = %d, want 0", n)
	}
	if err := f.store.SaveCharacterRequest(ctx, "no-such-link", "k", first, t0); err == nil {
		t.Error("a request of an unknown link was stored")
	}
}

// Link.LogValue and the fmt methods keep the external ID and link_id out of a
// log line and out of a formatted error (SEC-01, SEC-02).
func TestALinkNeverPrintsItsIdentifiers(t *testing.T) {
	f := newFixture(t)
	l := f.resolve(t, externalID, t0).Link

	var buf bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&buf, nil))
	log.Info("resolved", slog.Any("link", l), "value", l)
	log.Info("pointer", slog.Any("link", &l))
	// LogValue covers the value of an attribute only: nested in a struct or a
	// slice, the link reaches the JSON handler through encoding/json.
	log.Info("nested", slog.Any("res", links.Resolution{Link: l, Created: true}), slog.Any("links", []links.Link{l}))
	if data, err := json.Marshal(map[string]any{"link": l, "links": []*links.Link{&l}}); err != nil {
		t.Fatal(err)
	} else {
		buf.Write(data)
	}
	fmt.Fprintf(&buf, "%v %+v %#v %s %v", l, l, l, l, fmt.Errorf("wrapped: %v", l))

	for _, secret := range []string{externalID, l.LinkID} {
		if bytes.Contains(buf.Bytes(), []byte(secret)) {
			t.Errorf("the output contains %q: %s", secret, buf.String())
		}
	}
	if !bytes.Contains(buf.Bytes(), []byte("redacted")) {
		t.Errorf("the output does not say redacted: %s", buf.String())
	}
}

func TestNewSQLiteRefusesMissingDependencies(t *testing.T) {
	f := newFixture(t)
	if _, err := links.NewSQLite(nil, sequence()); err == nil {
		t.Error("nil database accepted")
	}
	if _, err := links.NewSQLite(f.db, nil); err == nil {
		t.Error("nil id source accepted")
	}
	if _, err := links.NewSQLite(f.db, sequence(), nil); err == nil {
		t.Error("nil hook accepted")
	}
}
