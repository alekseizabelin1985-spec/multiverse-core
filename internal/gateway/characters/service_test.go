package characters_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/readmodel"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// US-001 in the order of its criteria: without a consented link, consent is
// required; a world the gateway does not know is refused and nothing is
// proposed; a name is required and has the form of FR-060; a valid request
// proposes the character and answers 201 with it.
func TestCreateCharacterByTheCriteriaOfUS001(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	if a := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); code(a) != api.CodeConsentRequired || a.Status != http.StatusForbidden {
		t.Fatalf("without a link = %d %s", a.Status, code(a))
	}
	if _, err := f.links.Resolve(ctx, links.PlatformTelegram, externalID, t0); err != nil {
		t.Fatal(err)
	}
	if a := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); code(a) != api.CodeConsentRequired {
		t.Fatalf("pending_consent = %s", code(a))
	}
	f.consent(externalID)

	unknown := request(externalID, "k-1", "Вася")
	unknown.WorldID = "no-such-world"
	if a := f.svc.Create(ctx, unknown); code(a) != api.CodeWorldNotFound || a.Status != http.StatusNotFound {
		t.Fatalf("unknown world = %d %s", a.Status, code(a))
	}
	for name, want := range map[string]string{
		"":                         api.CodeNameRequired,
		"   ":                      api.CodeNameRequired,
		"В":                        api.CodeNameInvalid,
		strings.Repeat("я", 33):    api.CodeNameInvalid,
		"Вася!":                    api.CodeNameInvalid,
		"Вася  Пупкин":             api.CodeNameInvalid,
		"<script>":                 api.CodeNameInvalid,
		"\u0438\u0306\u0432\u0430": api.CodeNameInvalid,
	} {
		if a := f.svc.Create(ctx, request(externalID, "k-1", name)); code(a) != want || a.Status != http.StatusBadRequest {
			t.Errorf("name %q = %d %s, want %s", name, a.Status, code(a), want)
		}
	}
	if n := len(f.proposals()); n != 0 {
		t.Fatalf("refused requests proposed %d characters", n)
	}
	if l := f.link(externalID); l.PlayerID != nil || f.pendingRows() != 0 {
		t.Fatalf("a refused request attached a character or left a row: %v, %d rows", l.PlayerID, f.pendingRows())
	}
	for _, bad := range []characters.Request{
		{Platform: "discord", ExternalID: externalID, WorldID: world, Name: "Вася", ActionKey: "k"},
		{Platform: links.PlatformTelegram, WorldID: world, Name: "Вася", ActionKey: "k"},
		{Platform: links.PlatformTelegram, ExternalID: externalID, WorldID: world, Name: "Вася"},
		{Platform: links.PlatformTelegram, ExternalID: externalID, WorldID: world, Name: "Вася", ActionKey: strings.Repeat("k", 65)},
	} {
		if a := f.svc.Create(ctx, bad); code(a) != api.CodeInvalidRequest {
			t.Errorf("request %+v = %s, want invalid_request", bad, code(a))
		}
	}

	a := f.svc.Create(ctx, request(externalID, "k-1", "  Вася Пупкин "))
	if a.Status != http.StatusCreated || a.Body == nil || a.Body.Created == nil || !*a.Body.Created || a.Body.Character == nil {
		t.Fatalf("create = %+v %s", a, code(a))
	}
	ch := a.Body.Character
	if ch.PlayerID != a.Body.PlayerID || !strings.HasPrefix(ch.PlayerID, links.PlayerIDPrefix) || ch.Name != "Вася Пупкин" ||
		ch.Status != entity.StatusAlive || *ch.HP != characters.StartHPMax || ch.Position == nil ||
		ch.Position.Kind != characters.PositionOutside || ch.Position.ID != world || ch.Scope == nil || ch.Scope.ID != ch.PlayerID {
		t.Errorf("character = %+v", ch)
	}
	proposals := f.proposals()
	if len(proposals) != 1 {
		t.Fatalf("proposals = %d", len(proposals))
	}
	p := proposals[0]
	if err := contracts.Validate(p); err != nil {
		t.Errorf("the proposal is not valid by C-02: %v", err)
	}
	pa := p.Path()
	proposalID, _ := pa.GetString("proposal_id")
	cause, _ := pa.GetString("cause")
	if proposalID != p.ID || cause != characters.CauseCreate || p.Source != contracts.SourceGateway || p.Meta.ActorKind != eventbus.ActorHuman ||
		p.World == nil || p.World.Entity.ID != world {
		t.Errorf("proposal = %+v", p)
	}
	if l := f.link(externalID); l.PlayerID == nil || *l.PlayerID != ch.PlayerID || l.WorldID == nil || *l.WorldID != world {
		t.Errorf("link = %v %v", l.PlayerID, l.WorldID)
	}
	if n := f.pendingRows(); n != 0 {
		t.Errorf("pending rows after the fact = %d", n)
	}
	if got := f.svc.Status(ch.PlayerID); got != characters.StatusAlive {
		t.Errorf("Status = %s", got)
	}
}

// A repeat of the key returns the first answer and proposes nothing; another
// key while the character lives answers 200 with the same player_id (A-7); a
// dead character is replaced by a new player_id on the same link (UC-002 A2).
func TestARepeatReturnsTheFirstAnswerAndADeadCharacterIsReplaced(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.consent(externalID)
	first := f.svc.Create(ctx, request(externalID, "k-1", "Вася"))
	if first.Status != http.StatusCreated {
		t.Fatalf("create = %d %s", first.Status, code(first))
	}
	playerID := first.Body.PlayerID

	repeat := f.svc.Create(ctx, request(externalID, "k-1", "Другое имя"))
	if repeat.Status != http.StatusCreated || repeat.Body.PlayerID != playerID || repeat.Body.Character.Name != "Вася" {
		t.Errorf("repeat of the key = %d %+v", repeat.Status, repeat.Body)
	}
	alive := f.svc.Create(ctx, request(externalID, "k-2", "Петя"))
	if alive.Status != http.StatusOK || alive.Body.PlayerID != playerID || alive.Body.Created == nil || *alive.Body.Created {
		t.Errorf("another key while alive = %d %+v", alive.Status, alive.Body)
	}
	if n := len(f.proposals()); n != 1 {
		t.Fatalf("proposals = %d, want the first only", n)
	}

	f.die(playerID)
	if got := f.svc.Status(playerID); got != characters.StatusDead {
		t.Errorf("Status of the dead = %s", got)
	}
	again := f.svc.Create(ctx, request(externalID, "k-3", "Петя"))
	if again.Status != http.StatusCreated || again.Body.PlayerID == playerID {
		t.Fatalf("create after death = %d %+v", again.Status, again.Body)
	}
	if l := f.link(externalID); l.PlayerID == nil || *l.PlayerID != again.Body.PlayerID {
		t.Errorf("link after death = %v, want %s", l.PlayerID, again.Body.PlayerID)
	}
	if n := len(f.proposals()); n != 2 {
		t.Errorf("proposals = %d, want 2", n)
	}
}

// SEC-03: the key of a character is the link and not the account, the same
// key of another account is another character, and gateway.db holds neither
// the external ID nor the link_id at any stage of a creation.
func TestTheKeyOfACharacterIsTheLink(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	link := f.consent(externalID)
	f.consent(otherID)
	f.bus.set(Silent)
	pending := f.createPastTheWait(request(externalID, "k-1", "Вася"))
	if pending.Status != http.StatusAccepted {
		t.Fatalf("create = %d %s", pending.Status, code(pending))
	}
	dump := dumpGateway(t, f.gatewayDB)
	for _, secret := range []string{externalID, link.LinkID} {
		if strings.Contains(dump, secret) {
			t.Errorf("gateway.db holds %q while the character is creating", secret)
		}
	}
	if !strings.Contains(dump, pending.Body.PlayerID) {
		t.Fatal("control: the dump does not see the pending character, the scan proves nothing")
	}
	f.bus.set(Creates)
	other := f.svc.Create(ctx, request(otherID, "k-1", "Лена"))
	if other.Status != http.StatusCreated || other.Body.PlayerID == pending.Body.PlayerID {
		t.Errorf("the same key of another account = %d %+v", other.Status, other.Body)
	}
	var stored int
	if err := f.linksDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM character_requests WHERE link_id = ? AND action_key = ?`,
		link.LinkID, "k-1").Scan(&stored); err != nil || stored != 1 {
		t.Errorf("character_requests under (link_id, key) = %d %v", stored, err)
	}
	dump = dumpGateway(t, f.gatewayDB)
	for _, secret := range []string{externalID, otherID, link.LinkID, f.link(otherID).LinkID} {
		if strings.Contains(dump, secret) {
			t.Errorf("gateway.db holds %q", secret)
		}
	}
}

// 202 creating: the fact is late, the character reads creating with its
// minimal state, then alive once its fact comes; the repeat of the key keeps
// the first answer.
func TestALateFactAnswersCreating(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.consent(externalID)
	f.bus.set(Silent)
	a := f.createPastTheWait(request(externalID, "k-1", "Вася"))
	if a.Status != http.StatusAccepted || a.Body.Status != characters.StatusCreating || a.Body.Character != nil || a.Body.PlayerID == "" {
		t.Fatalf("create with a late fact = %d %+v", a.Status, a.Body)
	}
	playerID := a.Body.PlayerID
	if got := f.svc.Status(playerID); got != characters.StatusCreating {
		t.Errorf("Status = %s", got)
	}
	state, e := f.svc.Player(ctx, playerID)
	if e != nil || state.Status != characters.StatusCreating || state.Name != "Вася" || state.WorldID != world || state.HP != nil || state.Position != nil {
		t.Fatalf("Player while creating = %+v %v", state, e)
	}

	f.create(f.proposals()[0])
	state, e = f.svc.Player(ctx, playerID)
	if e != nil || state.Status != entity.StatusAlive || state.HP == nil {
		t.Errorf("Player after the fact = %+v %v", state, e)
	}
	if got := f.svc.Status(playerID); got != characters.StatusAlive {
		t.Errorf("Status after the fact = %s", got)
	}
	if repeat := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); repeat.Status != http.StatusAccepted || repeat.Body.PlayerID != playerID {
		t.Errorf("repeat = %d %+v, want the kept 202", repeat.Status, repeat.Body)
	}
	if n := len(f.proposals()); n != 1 {
		t.Errorf("proposals = %d", n)
	}
}

// A character whose fact did not come by the deadline, or that State refused,
// leaves pending_characters and its link: it reads none, GET answers 404, and
// the next request creates a new player_id.
func TestACharacterPastItsDeadlineOrRefusedIsTakenOff(t *testing.T) {
	t.Run("deadline", func(t *testing.T) {
		f := newFixture(t)
		ctx := context.Background()
		f.consent(externalID)
		f.bus.set(Silent)
		a := f.createPastTheWait(request(externalID, "k-1", "Вася"))
		playerID := a.Body.PlayerID
		if n, err := f.svc.Sweep(ctx, t0.Add(characters.DefaultDeadline-time.Second)); err != nil || n != 0 {
			t.Fatalf("Sweep before the deadline = %d %v", n, err)
		}
		f.clock.Set(t0.Add(characters.DefaultDeadline))
		if got := f.svc.Status(playerID); got != characters.StatusNone {
			t.Errorf("Status at the deadline = %s", got)
		}
		if n, err := f.svc.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
			t.Fatalf("Sweep at the deadline = %d %v", n, err)
		}
		if f.pendingRows() != 0 || f.link(externalID).PlayerID != nil {
			t.Fatalf("after the deadline: %d rows, link %v", f.pendingRows(), f.link(externalID).PlayerID)
		}
		if _, e := f.svc.Player(ctx, playerID); e == nil || e.Code != api.CodePlayerNotFound {
			t.Errorf("Player after the deadline = %v", e)
		}
		f.bus.set(Creates)
		if next := f.svc.Create(ctx, request(externalID, "k-2", "Вася")); next.Status != http.StatusCreated || next.Body.PlayerID == playerID {
			t.Errorf("next create = %d %+v", next.Status, next.Body)
		}
	})
	t.Run("refused while waiting", func(t *testing.T) {
		f := newFixture(t)
		ctx := context.Background()
		f.consent(externalID)
		f.bus.set(Refuses)
		a := f.svc.Create(ctx, request(externalID, "k-1", "Вася"))
		if a.Status != http.StatusInternalServerError || code(a) != api.CodeInternal {
			t.Fatalf("create refused by State = %d %s", a.Status, code(a))
		}
		if f.pendingRows() != 0 || f.link(externalID).PlayerID != nil {
			t.Fatalf("after the refusal: %d rows, link %v", f.pendingRows(), f.link(externalID).PlayerID)
		}
		f.bus.set(Creates)
		if again := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); again.Status != http.StatusCreated {
			t.Errorf("the refusal was kept under the key: %d %s", again.Status, code(again))
		}
	})
	t.Run("refused after 202", func(t *testing.T) {
		f := newFixture(t)
		ctx := context.Background()
		f.consent(externalID)
		f.bus.set(Silent)
		a := f.createPastTheWait(request(externalID, "k-1", "Вася"))
		playerID := a.Body.PlayerID
		proposalID, _ := f.proposals()[0].Path().GetString("proposal_id")
		rejected := eventbus.Event{ID: "rejected-1", Type: readmodel.TypeUpdateRejected,
			Payload: map[string]any{"proposal_id": proposalID, "reason": "duplicate_entity"}}
		inTx(t, f.gatewayDB, func(tx *sql.Tx) error { return f.svc.OnRejected(ctx, tx, rejected, readmodel.Result{}) })
		if got := f.svc.Status(playerID); got != characters.StatusNone {
			t.Errorf("Status after the refusal = %s", got)
		}
		if n, err := f.svc.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
			t.Fatalf("Sweep after the refusal = %d %v", n, err)
		}
		if f.pendingRows() != 0 || f.link(externalID).PlayerID != nil {
			t.Errorf("after the refusal: %d rows, link %v", f.pendingRows(), f.link(externalID).PlayerID)
		}
	})
	t.Run("created after 202", func(t *testing.T) {
		f := newFixture(t)
		ctx := context.Background()
		f.consent(externalID)
		f.bus.set(Silent)
		f.createPastTheWait(request(externalID, "k-1", "Вася"))
		proposal := f.proposals()[0]
		proposalID, _ := proposal.Path().GetString("proposal_id")
		fact := eventbus.Event{ID: "created-x", Type: readmodel.TypeEntityCreated, Payload: map[string]any{"proposal_id": proposalID}}
		inTx(t, f.gatewayDB, func(tx *sql.Tx) error { return f.svc.OnCreated(ctx, tx, fact, readmodel.Result{}) })
		if f.pendingRows() != 0 {
			t.Errorf("the row stayed after the fact")
		}
	})
}

// A character whose fact came keeps its link when its row passes its deadline
// (Mi-3 of review #1 of T-306). The sweeper removes the row only, whether the
// fact came before the sweep or while the sweeper took the character off its
// link, and the next request of the link answers that character: no second
// character (A-7).
func TestACharacterWhoseFactCameAtItsDeadlineKeepsItsLink(t *testing.T) {
	for _, tc := range []struct {
		name     string
		during   bool
		detaches int
	}{
		{name: "fact before the sweep", during: false, detaches: 0},
		{name: "fact while the sweeper detaches", during: true, detaches: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			ctx := context.Background()
			f.consent(externalID)
			f.bus.set(Silent)
			a := f.createPastTheWait(request(externalID, "k-1", "Вася"))
			playerID := a.Body.PlayerID
			proposal := f.proposals()[0]
			if tc.during {
				f.hooks.setBeforeDetach(func(string) {
					f.hooks.setBeforeDetach(nil)
					f.create(proposal)
				})
			} else {
				f.create(proposal)
			}
			f.clock.Set(t0.Add(characters.DefaultDeadline))
			if n, err := f.svc.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
				t.Fatalf("Sweep at the deadline = %d %v, want the row", n, err)
			}
			if f.pendingRows() != 0 {
				t.Errorf("the row stayed: %d", f.pendingRows())
			}
			if l := f.link(externalID); l.PlayerID == nil || *l.PlayerID != playerID {
				t.Fatalf("link after the sweep = %v, want %s", l.PlayerID, playerID)
			}
			if n := f.hooks.detachCalls(); n != tc.detaches {
				t.Errorf("DetachPlayer called %d times, want %d", n, tc.detaches)
			}
			if got := f.svc.Status(playerID); got != characters.StatusAlive {
				t.Errorf("Status = %s", got)
			}
			next := f.svc.Create(ctx, request(externalID, "k-2", "Вася"))
			if next.Status != http.StatusOK || next.Body.PlayerID != playerID {
				t.Errorf("next create = %d %+v, want 200 for %s", next.Status, next.Body, playerID)
			}
			if n := len(f.proposals()); n != 1 {
				t.Errorf("proposals = %d, want 1", n)
			}
		})
	}
}

// A request of a link whose character reached its deadline without a fact, and
// before the sweeper took it off, creates a new character (N-6 of review #1 of
// T-306): at the deadline itself the proposal is not published again. The
// sweeper then removes the row of the first character and leaves the link of
// the second one.
func TestARequestAtTheDeadlineCreatesANewCharacter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.consent(externalID)
	f.bus.set(Silent)
	first := f.createPastTheWait(request(externalID, "k-1", "Вася")).Body.PlayerID
	f.clock.Set(t0.Add(characters.DefaultDeadline))
	f.bus.set(Creates)
	a := f.svc.Create(ctx, request(externalID, "k-2", "Вася"))
	if a.Status != http.StatusCreated || a.Body.PlayerID == first {
		t.Fatalf("create at the deadline = %d %+v, want 201 for a new player_id", a.Status, a.Body)
	}
	proposals := f.proposals()
	if len(proposals) != 2 {
		t.Fatalf("proposals = %d, want 2", len(proposals))
	}
	if id, _ := proposals[1].Path().GetString("entity.entity.id"); id != a.Body.PlayerID {
		t.Errorf("second proposal for %s, want %s", id, a.Body.PlayerID)
	}
	if n, err := f.svc.Sweep(ctx, f.clock.Now()); err != nil || n != 1 {
		t.Fatalf("Sweep = %d %v, want the row of the first character", n, err)
	}
	if l := f.link(externalID); l.PlayerID == nil || *l.PlayerID != a.Body.PlayerID {
		t.Errorf("link after the sweep = %v, want %s", l.PlayerID, a.Body.PlayerID)
	}
	if f.pendingRows() != 0 {
		t.Errorf("rows after the sweep = %d", f.pendingRows())
	}
}

// untrimmed is a name filter that returns its text with spaces around it.
type untrimmed struct{}

func (untrimmed) FilterText(_ context.Context, _, text string) (string, *api.Error) {
	return " " + text + " ", nil
}

// The result of the name filter is trimmed before it is checked again (Mi-4 of
// review #1 of T-306): a filter that adds spaces around the name does not make
// it invalid, and the proposal carries the name without them.
func TestTheNameOfTheFilterIsTrimmed(t *testing.T) {
	f := newFixture(t)
	f.svc = f.service(untrimmed{})
	f.consent(externalID)
	a := f.svc.Create(context.Background(), request(externalID, "k-1", "Петя"))
	if a.Status != http.StatusCreated || a.Body.Character.Name != "Петя" {
		t.Fatalf("create through a filter that pads the name = %d %s %+v", a.Status, code(a), a.Body)
	}
	if name, _ := f.proposals()[0].Path().GetString("entity.name"); name != "Петя" {
		t.Errorf("name of the proposal = %q, want %q", name, "Петя")
	}
}

// The name goes through the input filter of the gateway (N-1 of T-305): the bus
// gets the name the filter returns, a result that is no name is name_invalid,
// a block is name_invalid and a failure 422 filter_error; nothing is proposed
// for a refused name.
func TestTheNameGoesThroughTheInputFilter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.consent(externalID)
	for _, tc := range []struct {
		status, replace string
		fail            bool
		want            string
		httpStatus      int
	}{
		{replace: "В", want: api.CodeNameInvalid, httpStatus: http.StatusBadRequest},
		{replace: strings.Repeat("я", 33), want: api.CodeNameInvalid, httpStatus: http.StatusBadRequest},
		{replace: "Вася!", want: api.CodeNameInvalid, httpStatus: http.StatusBadRequest},
		{status: actions.FilterBlocked, want: api.CodeNameInvalid, httpStatus: http.StatusBadRequest},
		{fail: true, want: api.CodeFilterError, httpStatus: http.StatusUnprocessableEntity},
	} {
		f.filter.set(tc.status, tc.replace, tc.fail)
		if a := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); code(a) != tc.want || a.Status != tc.httpStatus {
			t.Errorf("filter %+v = %d %s, want %d %s", tc, a.Status, code(a), tc.httpStatus, tc.want)
		}
	}
	if n := len(f.proposals()); n != 0 {
		t.Fatalf("refused names proposed %d characters", n)
	}
	f.filter.set("", "Петя", false)
	a := f.svc.Create(ctx, request(externalID, "k-1", "Вася"))
	if a.Status != http.StatusCreated || a.Body.Character.Name != "Петя" {
		t.Fatalf("create through a replacing filter = %d %+v", a.Status, a.Body)
	}
	raw, _ := json.Marshal(f.proposals()[0])
	if strings.Contains(string(raw), "Вася") || !strings.Contains(string(raw), "Петя") {
		t.Errorf("the proposal carries the name of the player, not of the filter: %s", raw)
	}
}

// A proposal the bus refuses answers 503 and keeps nothing under the key; the
// repeat publishes the proposal again under the same proposal_id and the same
// player_id, which State applies once (C-02 v1.6).
func TestARepeatAfterABusFailureProposesTheSameCharacter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.consent(externalID)
	f.bus.set(Down)
	if a := f.svc.Create(ctx, request(externalID, "k-1", "Вася")); code(a) != api.CodeBusUnavailable || a.Status != http.StatusServiceUnavailable {
		t.Fatalf("create with the bus down = %d %s", a.Status, code(a))
	}
	l := f.link(externalID)
	if l.PlayerID == nil || f.pendingRows() != 1 {
		t.Fatalf("after the failure: link %v, %d rows", l.PlayerID, f.pendingRows())
	}
	var firstProposal string
	if err := f.gatewayDB.QueryRowContext(ctx, `SELECT proposal_id FROM pending_characters`).Scan(&firstProposal); err != nil {
		t.Fatal(err)
	}
	f.bus.set(Creates)
	a := f.svc.Create(ctx, request(externalID, "k-1", "Вася"))
	if a.Status != http.StatusCreated || a.Body.PlayerID != *l.PlayerID {
		t.Fatalf("repeat = %d %+v, want 201 for %s", a.Status, a.Body, *l.PlayerID)
	}
	proposals := f.proposals()
	if len(proposals) != 1 {
		t.Fatalf("proposals = %d", len(proposals))
	}
	if id, _ := proposals[0].Path().GetString("proposal_id"); id != firstProposal {
		t.Errorf("proposal_id of the repeat = %s, want %s", id, firstProposal)
	}
}

// GET /v1/players/{player_id}: the state of a character in a region, in an
// encounter, with its session; 404 for a player the gateway does not know.
func TestPlayerIsTheStateOfTheCharacter(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	f.apply(created("wolf-alpha", entity.TypeNPC, "Альфа-волк", "create-wolf",
		map[string]any{"kind": "wolf", "region_id": forest, "position": forest, "status": "alive", "hp": 4, "hp_max": 10}))
	f.apply(created("player-A", entity.TypePlayer, "Вася", "create-player-A", map[string]any{
		"hp": 7, "hp_max": 10, "status": "alive", "position": forest,
		"scope": map[string]any{"id": "player-A", "type": "solo"}, "actor_kind": "human",
		"inventory": []any{map[string]any{"item_id": "item-1", "kind": "wolf-pelt", "name": "волчья шкура",
			"source": map[string]any{"event_id": "e-1", "type": "loot"}, "acquired_at": "2026-09-13T12:00:00Z"}},
	}))
	f.apply(created("enc-1", entity.TypeEncounter, "", "create-enc", map[string]any{
		"region_id": forest, "state": "active", "round_seq": 2, "task_agent_id": "agent-1",
		"scope":        map[string]any{"id": "player-A", "type": "solo"},
		"participants": []any{map[string]any{"player_id": "player-A", "state": "active", "damage_dealt": 0}},
		"npcs":         []any{map[string]any{"npc_id": "wolf-alpha"}},
	}))
	s, _, err := f.sessions.Touch(ctx, eventbus.ScopeRef{ID: "player-A", Type: "solo"}, world, "player-A", "human", t0)
	if err != nil {
		t.Fatal(err)
	}

	state, e := f.svc.Player(ctx, "player-A")
	if e != nil {
		t.Fatal(e)
	}
	if state.Status != entity.StatusAlive || *state.HP != 7 || *state.HPMax != 10 || state.Position.Kind != characters.PositionRegion ||
		state.Position.Name != "Опушка" || state.Encounter == nil || state.Encounter.EncounterID != "enc-1" ||
		len(state.Encounter.NPCs) != 1 || state.Encounter.NPCs[0].HP != 4 || state.Encounter.RoundSeq == nil ||
		len(state.Inventory) != 1 || state.Inventory[0].Kind != "wolf-pelt" || state.Session == nil || state.Session.SessionID != s.ID ||
		*state.Version != 1 {
		raw, _ := json.Marshal(state)
		t.Errorf("state = %s", raw)
	}
	if _, e := f.svc.Player(ctx, "player-Z"); e == nil || e.Code != api.CodePlayerNotFound {
		t.Errorf("unknown player = %v", e)
	}
	if got := f.svc.Status("player-Z"); got != characters.StatusNone {
		t.Errorf("Status of an unknown player = %s", got)
	}
}

// The stats of a new character are the player row of the rules of the world:
// the entity is the truth about the character, the rules about the formulas,
// and data-model.md §3.3 copies the one into the other at creation.
func TestTheStatsOfANewCharacterAreThoseOfTheRules(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "rules", "dark-forest.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Entities struct {
			Player struct {
				HPMax int    `yaml:"hp_max"`
				Atk   int    `yaml:"atk"`
				Def   int    `yaml:"def"`
				Dmg   string `yaml:"dmg"`
				Flee  string `yaml:"flee"`
			} `yaml:"player"`
		} `yaml:"entities"`
	}
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	p := doc.Entities.Player
	if p.HPMax != characters.StartHPMax || p.Atk != characters.StartAtk || p.Def != characters.StartDef ||
		p.Dmg != characters.StartDmg || p.Flee != characters.StartFlee {
		t.Errorf("rules %+v differ from the stats of a new character", p)
	}
	attrs := characters.Attributes(characters.Pending{PlayerID: "player-A", WorldID: world, ActorKind: "ci"})
	if attrs[entity.AttrHP] != characters.StartHPMax || attrs[entity.AttrStatus] != entity.StatusAlive ||
		attrs[entity.AttrPosition] != "outside:"+world || attrs[entity.AttrActorKind] != "ci" {
		t.Errorf("attributes = %v", attrs)
	}
}

func TestNewChecksItsConfig(t *testing.T) {
	if _, err := characters.New(characters.Config{}); err == nil {
		t.Error("New without its dependencies succeeded")
	}
	f := newFixture(t)
	if _, err := characters.New(characters.Config{Links: f.hooks, Model: f.model, DB: f.gatewayDB, Bus: f.bus,
		Filter: untrimmed{}, IDs: f.ids, Clock: f.clock, Wait: time.Minute, Deadline: time.Minute}); err == nil {
		t.Error("New with a wait as long as the deadline succeeded")
	}
}

func inTx(t *testing.T, db *sql.DB, fn func(tx *sql.Tx) error) {
	t.Helper()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := fn(tx); err != nil {
		_ = tx.Rollback()
		t.Fatal(err)
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

// dumpGateway is every value of every table of gateway.db as text.
func dumpGateway(t *testing.T, db *sql.DB) string {
	t.Helper()
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type = 'table'`)
	if err != nil {
		t.Fatal(err)
	}
	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		tables = append(tables, name)
	}
	_ = rows.Close()
	var b strings.Builder
	for _, table := range tables {
		r, err := db.QueryContext(ctx, `SELECT * FROM "`+table+`"`)
		if err != nil {
			t.Fatal(err)
		}
		cols, _ := r.Columns()
		for r.Next() {
			values := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range values {
				ptrs[i] = &values[i]
			}
			if err := r.Scan(ptrs...); err != nil {
				t.Fatal(err)
			}
			for _, v := range values {
				switch x := v.(type) {
				case []byte:
					b.Write(x)
				case string:
					b.WriteString(x)
				}
				b.WriteByte('|')
			}
		}
		_ = r.Close()
	}
	return b.String()
}
