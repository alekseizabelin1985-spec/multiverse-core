package actions_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

func submit(t *testing.T, f fixture, c actions.Command) actions.Answer {
	t.Helper()
	if c.ActorKind == "" {
		c.ActorKind = api.ActorHuman
	}
	return f.svc.Submit(context.Background(), c)
}

func code(a actions.Answer) string {
	if a.Err == nil {
		return ""
	}
	return a.Err.Code
}

// checkPublished holds every published event to the contract: registered,
// valid against its schema, published by a source the registry lists.
func checkPublished(t *testing.T, events []eventbus.Event) {
	t.Helper()
	for _, ev := range events {
		spec, ok := contracts.Lookup(ev.Type)
		if !ok {
			t.Errorf("%s is not registered", ev.Type)
			continue
		}
		if !slices.Contains(spec.Publishers, ev.Source) {
			t.Errorf("%s from %s; the publishers of the registry are %v", ev.Type, ev.Source, spec.Publishers)
		}
		if err := contracts.Validate(ev); err != nil {
			t.Errorf("%s does not validate: %v", ev.Type, err)
		}
	}
}

func types(events []eventbus.Event) []string {
	out := make([]string, 0, len(events))
	for _, ev := range events {
		out = append(out, ev.Type)
	}
	return out
}

// Every solo action publishes its player.* event, and those that move the
// world the proposal with it; all of them pass the contract (the schemas of
// T-301 and the publishers of the registry).
func TestEveryActionPublishesEventsOfTheContract(t *testing.T) {
	cases := []struct {
		player, typ, target string
		text                *string
		want                []string
	}{
		{playerA, api.ActionEnter, forest, nil, []string{actions.TypeEnteredRegion, actions.TypeUpdateProposed}},
		{playerA, api.ActionLook, "", nil, []string{actions.TypeLooked}},
		{playerA, api.ActionRest, "", nil, []string{actions.TypeRested, actions.TypeUpdateProposed}},
		{playerA, api.ActionSay, "", text("привет"), []string{actions.TypeSaid}},
		{playerB, api.ActionLook, "", nil, []string{actions.TypeLooked}},
		{playerB, api.ActionAttack, "wolf-alpha", nil, []string{actions.TypeAttacked}},
		{playerB, api.ActionFlee, "", nil, []string{actions.TypeFleeAttempted}},
		{playerB, api.ActionDefend, "", nil, []string{actions.TypeDefended}},
	}
	for _, tc := range cases {
		t.Run(tc.player+" "+tc.typ, func(t *testing.T) {
			f := newFixture(t, options{})
			a := submit(t, f, actions.Command{PlayerID: tc.player, ActionKey: "key-1", Type: tc.typ, Target: tc.target, Text: tc.text})
			if a.Status != http.StatusAccepted || a.Accepted == nil {
				t.Fatalf("answer = %d %s", a.Status, code(a))
			}
			events := f.bus.published()
			if got := types(events); !slices.Equal(got, tc.want) {
				t.Fatalf("published %v, want %v", got, tc.want)
			}
			checkPublished(t, events)
			action := events[0]
			if action.Meta.CausationID != "" || action.Meta.CorrelationID != action.ID || action.Meta.Agent != nil {
				t.Errorf("the action is not a root without an agent: %+v", action.Meta)
			}
			if a.Accepted.CorrelationID != action.ID || a.Accepted.Status != api.ActionStatusAccepted || a.Accepted.Turn.Seq != 1 {
				t.Errorf("accepted = %+v, action %s", a.Accepted, action.ID)
			}
			if action.Scope == nil || action.Scope.ID != tc.player || action.World.Entity.ID != world {
				t.Errorf("scope %+v world %+v", action.Scope, action.World)
			}
			if got, _ := action.Path().GetString("action.key_hash"); got != actions.KeyHash("key-1") || strings.Contains(got, "key-1") {
				t.Errorf("key_hash = %q", got)
			}
			if got, _ := action.Path().GetString("session.id"); got != a.Accepted.Turn.SessionID {
				t.Errorf("session.id = %q, turn %+v", got, a.Accepted.Turn)
			}
			for _, ev := range events[1:] {
				if ev.Meta.CorrelationID != action.ID || ev.Meta.CausationID != action.ID {
					t.Errorf("%s does not follow the action: %+v", ev.Type, ev.Meta)
				}
				if got, _ := ev.Path().GetString("proposal_id"); got != ev.ID {
					t.Errorf("proposal_id = %q, id %s", got, ev.ID)
				}
				// The consumer finds the player of a refusal by this id (T-307).
				if got := actions.ProposalID(action.ID, tc.player); got != ev.ID {
					t.Errorf("ProposalID(%s, %s) = %s, the proposal is %s", action.ID, tc.player, got, ev.ID)
				}
			}
		})
	}
}

// enter and leave propose the position and nothing else — no scope (C-04
// v1.2) — at the version the projection holds; rest proposes hp = hp_max.
func TestProposalsCarryTheirOnePath(t *testing.T) {
	f := newFixture(t, options{})
	if _, err := f.model.Apply(fact(t, world, "player-E", entity.TypePlayer, "Женя", player("player-E", forest))); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		player, typ, target, path string
		value                     any
		from                      string
	}{
		{playerA, api.ActionEnter, forest, entity.AttrPosition, forest, actions.OutsidePosition(world)},
		{"player-E", api.ActionLeave, "", entity.AttrPosition, actions.OutsidePosition(world), forest},
	}
	for i, tc := range cases {
		before := len(f.bus.published())
		a := submit(t, f, actions.Command{PlayerID: tc.player, ActionKey: "move-" + tc.typ, Type: tc.typ, Target: tc.target})
		if a.Status != http.StatusAccepted {
			t.Fatalf("case %d: %d %s", i, a.Status, code(a))
		}
		events := f.bus.published()[before:]
		action, proposal := events[0], events[1]
		if from, _ := action.Path().GetString("position.from"); from != tc.from {
			t.Errorf("%s position.from = %q, want %q", tc.typ, from, tc.from)
		}
		if to, _ := action.Path().GetString("position.to"); to != tc.value {
			t.Errorf("%s position.to = %q, want %v", tc.typ, to, tc.value)
		}
		assertOneOp(t, proposal, tc.player, actions.CauseMove, tc.path, tc.value)
	}
	before := len(f.bus.published())
	if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "rest", Type: api.ActionRest}); a.Status != http.StatusAccepted {
		t.Fatalf("rest: %d %s", a.Status, code(a))
	}
	assertOneOp(t, f.bus.published()[before+1], playerA, actions.CauseRest, entity.AttrHP, float64(10))
}

func assertOneOp(t *testing.T, ev eventbus.Event, playerID, cause, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(ev.Payload)
	if err != nil {
		t.Fatal(err)
	}
	var p struct {
		Changes []struct {
			Entity          eventbus.Entity `json:"entity"`
			ExpectedVersion *int64          `json:"expected_version"`
			Ops             []entity.Op     `json:"ops"`
		} `json:"changes"`
		Atomic bool   `json:"atomic"`
		Cause  string `json:"cause"`
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Changes) != 1 || p.Changes[0].Entity.Entity.ID != playerID || p.Cause != cause || !p.Atomic {
		t.Fatalf("proposal %s = %+v", ev.ID, p)
	}
	ch := p.Changes[0]
	if ch.ExpectedVersion == nil || *ch.ExpectedVersion != 1 {
		t.Errorf("expected_version = %v, want the version of the projection, 1", ch.ExpectedVersion)
	}
	if len(ch.Ops) != 1 || ch.Ops[0].Op != entity.OpSet || ch.Ops[0].Path != path || ch.Ops[0].Value != value {
		t.Errorf("ops = %+v, want one set of %s to %v", ch.Ops, path, value)
	}
	for _, op := range ch.Ops {
		if op.Path == entity.AttrScope {
			t.Errorf("the proposal carries the scope (C-04 v1.2)")
		}
	}
}

// A repeat of action_key answers the first answer — the same correlation_id,
// or the same 4xx even after the world changed — and publishes nothing (C-08).
func TestARepeatOfTheKeyAnswersTheFirstAnswer(t *testing.T) {
	f := newFixture(t, options{})
	first := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-look", Type: api.ActionLook})
	again := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-look", Type: api.ActionLook})
	if first.Status != http.StatusAccepted || again.Status != http.StatusAccepted ||
		again.Accepted.CorrelationID != first.Accepted.CorrelationID || !again.Accepted.AckedAt.Equal(first.Accepted.AckedAt) {
		t.Fatalf("first %+v, repeat %+v", first.Accepted, again.Accepted)
	}
	// The body of a repeat is the stored one even when the request differs.
	changed := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-look", Type: api.ActionRest})
	if changed.Accepted == nil || changed.Accepted.CorrelationID != first.Accepted.CorrelationID {
		t.Errorf("a repeat with another body = %+v", changed)
	}
	if n := len(f.bus.published()); n != 1 {
		t.Fatalf("published %d events for one action and two repeats", n)
	}

	refused := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-flee", Type: api.ActionFlee})
	if code(refused) != api.CodeNotInEncounter {
		t.Fatalf("flee outside an encounter = %s", code(refused))
	}
	// player-A is put in an encounter: a new key flees, the old one still
	// answers what it answered.
	if _, err := f.model.Apply(fact(t, world, "encounter-A", entity.TypeEncounter, "",
		encounter(playerA, "encounter-wolf:solo:player-A", "wolf-alpha"))); err != nil {
		t.Fatal(err)
	}
	if again := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-flee", Type: api.ActionFlee}); code(again) != api.CodeNotInEncounter ||
		again.Status != http.StatusConflict || again.Err.Message != refused.Err.Message {
		t.Errorf("repeat of a refused key = %d %s", again.Status, code(again))
	}
	if fresh := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-flee-2", Type: api.ActionFlee}); fresh.Status != http.StatusAccepted {
		t.Errorf("a new key = %d %s", fresh.Status, code(fresh))
	}
	if got := f.turns.rejected; !slices.Equal(got, []string{api.CodeNotInEncounter}) {
		t.Errorf("rejected turns = %v, want one", got)
	}
}

// A repeat of a refusal answers the whole body of the first answer, details
// included: they tell text_invalid of a blocking filter from text_invalid, and
// name the field of invalid_request (review #1, Mi-3).
func TestARepeatOfARefusalIsTheWholeFirstBody(t *testing.T) {
	cases := []struct {
		name   string
		filter actions.InputFilter
		cmd    actions.Command
	}{
		{"text_invalid of the filter", failingFilter{status: actions.FilterBlocked},
			actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionSay, Text: text("привет")}},
		{"invalid_request of a field", nil,
			actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook, Target: forest}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t, options{filter: tc.filter})
			first := submit(t, f, tc.cmd)
			if first.Err == nil || len(first.Err.Details) == 0 {
				t.Fatalf("first answer = %d %+v, want a refusal with details", first.Status, first.Err)
			}
			again := submit(t, f, tc.cmd)
			if again.Err == nil || again.Status != first.Status {
				t.Fatalf("repeat = %d %+v", again.Status, again.Err)
			}
			if got, want := mustJSON(t, again.Err.Body()), mustJSON(t, first.Err.Body()); !bytes.Equal(got, want) {
				t.Errorf("repeat body %s, first body %s", got, want)
			}
		})
	}
}

// The actor kind of the request is the actor kind of every event of its
// action: the ci harness and the simulator are told from live players by it
// alone (C-10; review #1, Mi-1).
func TestTheActorKindOfTheRequestIsTheActorKindOfItsEvents(t *testing.T) {
	f := newFixture(t, options{gmPath: eventbus.GMPathLegacy})
	a := f.svc.Submit(context.Background(), actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionRest, ActorKind: api.ActorSim})
	if a.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s", a.Status, code(a))
	}
	events := f.bus.published()
	if got := types(events); !slices.Equal(got, []string{actions.TypeGMCreated, actions.TypeRested, actions.TypeUpdateProposed}) {
		t.Fatalf("published %v", got)
	}
	for _, ev := range events {
		if ev.Meta.ActorKind != api.ActorSim {
			t.Errorf("%s actor_kind = %q, want sim", ev.Type, ev.Meta.ActorKind)
		}
	}
}

// replaceWith is a filter that replaces every text by one text.
type replaceWith struct{ text string }

func (f replaceWith) Check(context.Context, string, string) (actions.FilterResult, error) {
	return actions.FilterResult{Status: actions.FilterReplaced, Text: f.text}, nil
}

// A filter that replaces the text by one that is no text of say — empty, over
// 500 characters, with a control character — refuses the action as
// text_invalid and publishes nothing (review #1, Mi-5).
func TestAReplacementThatIsNoTextIsRefused(t *testing.T) {
	for name, replacement := range map[string]string{
		"empty":             "",
		"501 runes":         strings.Repeat("ж", actions.MaxTextRunes+1),
		"control character": "a\x07b",
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, options{filter: replaceWith{text: replacement}})
			a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionSay, Text: text("привет")})
			if a.Status != http.StatusBadRequest || code(a) != api.CodeTextInvalid {
				t.Errorf("answer = %d %s, want 400 text_invalid", a.Status, code(a))
			}
			if n := len(f.bus.attempted()); n != 0 {
				t.Errorf("%d publications attempted", n)
			}
		})
	}
}

// 503 bus_unavailable when a publication fails: the key is not stored and the
// turn is not recorded, so the client repeats with the same key and the action
// is taken then (C-08, component §5.5).
func TestAFailedPublicationKeepsNothing(t *testing.T) {
	for _, failing := range []string{actions.TypeEnteredRegion, actions.TypeUpdateProposed} {
		t.Run(failing, func(t *testing.T) {
			f := newFixture(t, options{})
			f.bus.setFail(func(ev eventbus.Event) bool { return ev.Type == failing })
			cmd := actions.Command{PlayerID: playerA, ActionKey: "k-enter", Type: api.ActionEnter, Target: forest}
			a := submit(t, f, cmd)
			if a.Status != http.StatusServiceUnavailable || code(a) != api.CodeBusUnavailable {
				t.Fatalf("answer = %d %s", a.Status, code(a))
			}
			if _, found, err := f.keys.Lookup(context.Background(), playerA, "k-enter", f.clock.Now()); err != nil || found {
				t.Fatalf("key stored after a failed publication: %v %v", found, err)
			}
			if len(f.turns.accepted) != 0 || len(f.turns.rejected) != 0 {
				t.Fatalf("turns recorded: accepted %v, rejected %v", f.turns.accepted, f.turns.rejected)
			}
			f.bus.setFail(nil)
			a = submit(t, f, cmd)
			if a.Status != http.StatusAccepted {
				t.Fatalf("repeat after the bus came back = %d %s", a.Status, code(a))
			}
			if a.Accepted.Turn.Seq != 1 {
				t.Errorf("the failed action took a turn: seq %d", a.Accepted.Turn.Seq)
			}
			if _, found, _ := f.keys.Lookup(context.Background(), playerA, "k-enter", f.clock.Now()); !found {
				t.Error("key not stored after the accepted repeat")
			}
		})
	}
}

// A store of keys that fails to look up answers 500 and takes no action.
func TestAFailingKeyStoreTakesNoAction(t *testing.T) {
	f := newFixture(t, options{keys: brokenKeys{}})
	if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook}); code(a) != api.CodeInternal {
		t.Fatalf("answer = %d %s", a.Status, code(a))
	}
	if n := len(f.bus.published()); n != 0 {
		t.Errorf("published %d events", n)
	}
}

type brokenKeys struct{}

func (brokenKeys) Lookup(context.Context, string, string, time.Time) (actions.Stored, bool, error) {
	return actions.Stored{}, false, errors.New("disk I/O error")
}

func (brokenKeys) Save(context.Context, string, string, actions.Stored, time.Time) error {
	return errors.New("disk I/O error")
}

// A key is required and at most 64 bytes; a request without one is refused
// and nothing is kept, there is nothing to keep it under.
func TestTheActionKeyIsRequired(t *testing.T) {
	f := newFixture(t, options{})
	for _, key := range []string{"", strings.Repeat("k", actions.MaxActionKey+1)} {
		if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: key, Type: api.ActionLook}); code(a) != api.CodeInvalidRequest {
			t.Errorf("key of %d bytes = %s", len(key), code(a))
		}
	}
	if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: strings.Repeat("k", actions.MaxActionKey), Type: api.ActionLook}); a.Status != http.StatusAccepted {
		t.Errorf("key of 64 bytes = %d %s", a.Status, code(a))
	}
}

// An answer is kept 24 hours; past that the key is free and the sweeper
// deletes its row.
func TestAKeyExpires(t *testing.T) {
	f := newFixture(t, options{})
	ctx := context.Background()
	first := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook})
	f.clock.Advance(24*time.Hour - time.Second)
	if again := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook}); again.Accepted.CorrelationID != first.Accepted.CorrelationID {
		t.Fatal("the key expired before 24 hours")
	}
	f.clock.Advance(time.Second)
	if _, found, _ := f.keys.Lookup(ctx, playerA, "k", f.clock.Now()); found {
		t.Fatal("the key is found at its expiry")
	}
	if err := f.keys.Sweep(ctx, f.clock.Now()); err != nil {
		t.Fatal(err)
	}
	later := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionLook})
	if later.Accepted == nil || later.Accepted.CorrelationID == first.Accepted.CorrelationID {
		t.Errorf("after the expiry the key answers %+v", later)
	}
}

// Parallel requests with one key publish one action: the second waits for the
// first and answers what it answered.
func TestParallelRepeatsOfAKeyPublishOnce(t *testing.T) {
	f := newFixture(t, options{})
	var wg sync.WaitGroup
	answers := make([]actions.Answer, 8)
	for i := range answers {
		wg.Go(func() {
			answers[i] = f.svc.Submit(context.Background(), actions.Command{PlayerID: playerA, ActionKey: "same", Type: api.ActionLook, ActorKind: api.ActorHuman})
		})
	}
	wg.Wait()
	if n := len(f.bus.published()); n != 1 {
		t.Fatalf("published %d actions for one key", n)
	}
	for _, a := range answers {
		if a.Accepted == nil || a.Accepted.CorrelationID != answers[0].Accepted.CorrelationID {
			t.Fatalf("answers differ: %+v", answers)
		}
	}
}

// With MV_GM_PATH=legacy an accepted action announces the game master of its
// scope first; with agent it does not (US-019, FR-014, C-04 v1.4).
func TestTheLegacyPathAnnouncesTheGameMaster(t *testing.T) {
	legacy := newFixture(t, options{gmPath: eventbus.GMPathLegacy})
	if a := submit(t, legacy, actions.Command{PlayerID: playerB, ActionKey: "k", Type: api.ActionAttack, Target: "wolf-alpha"}); a.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s", a.Status, code(a))
	}
	events := legacy.bus.published()
	if got := types(events); !slices.Equal(got, []string{actions.TypeGMCreated, actions.TypeAttacked}) {
		t.Fatalf("legacy published %v", got)
	}
	checkPublished(t, events)
	gm, action := events[0], events[1]
	if gm.Meta.GMPath != eventbus.GMPathLegacy || action.Meta.GMPath != eventbus.GMPathLegacy {
		t.Errorf("gm_path %s and %s, want legacy", gm.Meta.GMPath, action.Meta.GMPath)
	}
	if id, _ := gm.Path().GetString("scope_id"); id != playerB || gm.Scope == nil || gm.Scope.ID != playerB {
		t.Errorf("gm.created scope_id %q, scope %+v", id, gm.Scope)
	}
	if focus, _ := gm.Path().GetSlice("config.focus_entities"); len(focus) != 1 || focus[0] != playerB {
		t.Errorf("focus_entities = %v", focus)
	}

	agent := newFixture(t, options{})
	submit(t, agent, actions.Command{PlayerID: playerB, ActionKey: "k", Type: api.ActionAttack, Target: "wolf-alpha"})
	for _, ev := range agent.bus.published() {
		if ev.Type == actions.TypeGMCreated || ev.Meta.GMPath != eventbus.GMPathAgent {
			t.Errorf("agent path published %s with gm_path %s", ev.Type, ev.Meta.GMPath)
		}
	}
}

// replaceWord is the test filter of an operator: it replaces a word.
type replaceWord struct {
	kinds []string
}

func (f *replaceWord) Check(_ context.Context, kind, text string) (actions.FilterResult, error) {
	f.kinds = append(f.kinds, kind)
	if !strings.Contains(text, "дурак") {
		return actions.FilterResult{Status: actions.FilterPass, Text: text}, nil
	}
	return actions.FilterResult{Status: actions.FilterReplaced, Text: strings.ReplaceAll(text, "дурак", "***")}, nil
}

type failingFilter struct{ status string }

func (f failingFilter) Check(_ context.Context, _, text string) (actions.FilterResult, error) {
	if f.status != "" {
		return actions.FilterResult{Status: f.status, Text: text}, nil
	}
	return actions.FilterResult{}, errors.New("filter of the operator down")
}

// The bus gets the text the filter returned, and the text of the player is in
// no record of the bus and in no line of the log (US-016, FR-056).
func TestTheBusGetsTheTextOfTheFilter(t *testing.T) {
	filter := &replaceWord{}
	f := newFixture(t, options{filter: filter})
	const said = "  ты дурак, волк  "
	a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-say", Type: api.ActionSay, Text: text(said)})
	if a.Status != http.StatusAccepted {
		t.Fatalf("answer = %d %s", a.Status, code(a))
	}
	events := f.bus.published()
	if got, _ := events[0].Path().GetString("text"); got != "ты ***, волк" {
		t.Errorf("player.said.text = %q", got)
	}
	checkPublished(t, events)
	for _, topic := range contracts.Topics() {
		records, err := f.raw.Records(topic.Name)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range records {
			if strings.Contains(string(r), "дурак") {
				t.Errorf("the text of the player is in %s: %s", topic.Name, r)
			}
		}
	}
	if log := f.log.String(); strings.Contains(log, "дурак") || strings.Contains(log, "***") || strings.Contains(log, "k-say") {
		t.Errorf("the log carries the text or the key: %s", log)
	}
	lines := f.log.lines(t, "input_filter")
	if len(lines) != 1 || lines[0]["applied"] != true || lines[0]["status"] != actions.FilterReplaced ||
		lines[0]["filter"] != "test" || lines[0]["text_len"] != float64(len([]rune(strings.TrimSpace(said)))) {
		t.Errorf("input_filter records = %v", lines)
	}
	if !slices.Equal(filter.kinds, []string{actions.KindSay}) {
		t.Errorf("the filter checked %v", filter.kinds)
	}
}

// The name of a new character goes through the same filter (T-306 calls
// FilterText with KindName).
func TestTheNameOfACharacterGoesThroughTheFilter(t *testing.T) {
	filter := &replaceWord{}
	f := newFixture(t, options{filter: filter})
	name, e := f.svc.FilterText(context.Background(), actions.KindName, "дурак")
	if e != nil || name != "***" || !slices.Equal(filter.kinds, []string{actions.KindName}) {
		t.Errorf("FilterText(name) = %q %v, kinds %v", name, e, filter.kinds)
	}
}

// A filter that fails fails the action closed: 422 filter_error, nothing
// published; a text it blocks is text_invalid (FR-056).
func TestAFilterThatFailsFailsClosed(t *testing.T) {
	cases := []struct {
		filter actions.InputFilter
		want   string
	}{
		{failingFilter{}, api.CodeFilterError},
		{failingFilter{status: actions.FilterBlocked}, api.CodeTextInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.want, func(t *testing.T) {
			f := newFixture(t, options{filter: tc.filter})
			a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k", Type: api.ActionSay, Text: text("привет")})
			if code(a) != tc.want {
				t.Fatalf("answer = %d %s, want %s", a.Status, code(a), tc.want)
			}
			if n := len(f.bus.published()); n != 0 {
				t.Errorf("published %d events", n)
			}
			if log := f.log.String(); strings.Contains(log, "привет") || strings.Contains(log, "operator down") {
				t.Errorf("the log carries the text or the error of the filter: %s", log)
			}
		})
	}
}

func TestFilterForKnowsOnlyNoop(t *testing.T) {
	if f, err := actions.FilterFor(actions.FilterNoop); err != nil || f == nil {
		t.Fatalf("FilterFor(noop) = %v %v", f, err)
	}
	if _, err := actions.FilterFor("other"); err == nil {
		t.Error("FilterFor(other) is accepted")
	}
	res, err := actions.NoopFilter{}.Check(context.Background(), actions.KindSay, "как есть")
	if err != nil || res.Status != actions.FilterPass || res.Text != "как есть" {
		t.Errorf("NoopFilter = %+v %v", res, err)
	}
}

// 202 comes back without waiting for anything but the acknowledgement of the
// broker: no fact, no consumer runs here (NFR-003). The budget of 100 ms is
// the DoD of T-305; the fastest of five tries is measured, so that a pause of
// the race detector or of the machine under CI does not fail the test.
func TestAcceptanceDoesNotWaitForTheWorld(t *testing.T) {
	f := newFixture(t, options{})
	fastest := time.Hour
	for i := range 5 {
		began := clock.Real{}.Now()
		a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "k-" + string(rune('a'+i)), Type: api.ActionLook})
		took := clock.Real{}.Now().Sub(began)
		if a.Status != http.StatusAccepted {
			t.Fatalf("answer = %d %s", a.Status, code(a))
		}
		fastest = min(fastest, took)
	}
	if fastest > 100*time.Millisecond {
		t.Errorf("the fastest acceptance took %s, the budget is 100 ms", fastest)
	}
}

// Only a 4xx is kept under its key: the 501 of a group action is not an answer
// of the contract to that action, and a later build answers it.
func TestNotImplementedIsNotKept(t *testing.T) {
	f := newFixture(t, options{})
	if a := submit(t, f, actions.Command{PlayerID: playerA, ActionKey: "g", Type: api.ActionGroupCreate}); a.Status != http.StatusNotImplemented {
		t.Fatalf("group.create = %d %s", a.Status, code(a))
	}
	if _, found, _ := f.keys.Lookup(context.Background(), playerA, "g", f.clock.Now()); found {
		t.Error("a 501 is kept under its key")
	}
}
