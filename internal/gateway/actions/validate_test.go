package actions_test

import (
	"strings"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/actions"
	"multiverse-core.io/internal/gateway/api"
	"multiverse-core.io/shared/eventbus"
)

type validateCase struct {
	name   string
	player string
	typ    string
	target string
	text   *string
	after  time.Duration
	want   string // error code; empty when the action is valid
}

func runValidate(t *testing.T, cases []validateCase) {
	t.Helper()
	v := actions.Validator{Model: newWorld(t), Grace: 10 * time.Second}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := actions.Command{PlayerID: tc.player, ActionKey: "k", Type: tc.typ, Target: tc.target, Text: tc.text,
				ActorKind: api.ActorHuman}
			_, e := v.Validate(c, t0.Add(tc.after))
			got := ""
			if e != nil {
				got = e.Code
				if spec, _ := api.LookupError(e.Code); spec.Status != e.Status {
					t.Errorf("%s travels with %d, the table says %d", e.Code, e.Status, spec.Status)
				}
			}
			if got != tc.want {
				t.Errorf("Validate = %q, want %q", got, tc.want)
			}
		})
	}
}

// Every row of api-contracts.md §1.4 and every code of §1.6 that validation
// answers, one case each at least.
func TestValidateEveryRowAndCode(t *testing.T) {
	runValidate(t, []validateCase{
		{name: "enter a region of the world", player: playerA, typ: api.ActionEnter, target: forest},
		{name: "look outside", player: playerA, typ: api.ActionLook},
		{name: "look in an encounter", player: playerB, typ: api.ActionLook},
		{name: "attack in the encounter", player: playerB, typ: api.ActionAttack, target: "wolf-alpha"},
		{name: "flee", player: playerB, typ: api.ActionFlee},
		{name: "defend", player: playerB, typ: api.ActionDefend},
		{name: "rest outside an encounter", player: playerA, typ: api.ActionRest},
		{name: "say", player: playerA, typ: api.ActionSay, text: text("  привет  ")},
		{name: "say in an encounter", player: playerB, typ: api.ActionSay, text: text("держись")},

		{name: "player_not_found", player: "player-Z", typ: api.ActionLook, want: api.CodePlayerNotFound},
		{name: "character_dead", player: playerD, typ: api.ActionLook, want: api.CodeCharacterDead},
		{name: "character_dead abandoned", player: playerF, typ: api.ActionLook, want: api.CodeCharacterDead},
		{name: "unknown_action", player: playerA, typ: "dance", want: api.CodeUnknownAction},
		{name: "unknown_action empty", player: playerA, typ: "", want: api.CodeUnknownAction},
		{name: "invalid_request enter without target", player: playerA, typ: api.ActionEnter, want: api.CodeInvalidRequest},
		{name: "invalid_request attack without target", player: playerB, typ: api.ActionAttack, want: api.CodeInvalidRequest},
		{name: "invalid_request target of look", player: playerA, typ: api.ActionLook, target: forest, want: api.CodeInvalidRequest},
		{name: "invalid_request text of rest", player: playerA, typ: api.ActionRest, text: text("zzz"), want: api.CodeInvalidRequest},
		{name: "text_invalid missing", player: playerA, typ: api.ActionSay, want: api.CodeTextInvalid},
		{name: "unknown_target no such region", player: playerA, typ: api.ActionEnter, target: "nowhere", want: api.CodeUnknownTarget},
		{name: "unknown_target no such npc", player: playerB, typ: api.ActionAttack, target: "wolf-none", want: api.CodeUnknownTarget},
		{name: "in_encounter enter", player: playerB, typ: api.ActionEnter, target: forest, want: api.CodeInEncounter},
		{name: "in_encounter leave", player: playerB, typ: api.ActionLeave, want: api.CodeInEncounter},
		{name: "in_encounter rest", player: playerB, typ: api.ActionRest, want: api.CodeInEncounter},
		{name: "not_in_encounter attack", player: playerA, typ: api.ActionAttack, target: "wolf-alpha", want: api.CodeNotInEncounter},
		{name: "not_in_encounter flee", player: playerA, typ: api.ActionFlee, want: api.CodeNotInEncounter},
		{name: "not_in_encounter defend", player: playerA, typ: api.ActionDefend, want: api.CodeNotInEncounter},
		{name: "target_dead", player: playerB, typ: api.ActionAttack, target: "wolf-dead", want: api.CodeTargetDead},
		{name: "encounter_unavailable attack", player: playerC, typ: api.ActionAttack, target: "wolf-alpha", after: 11 * time.Second, want: api.CodeEncounterUnavailable},
		{name: "encounter_unavailable flee", player: playerC, typ: api.ActionFlee, after: 11 * time.Second, want: api.CodeEncounterUnavailable},
		{name: "encounter without agent within the grace", player: playerC, typ: api.ActionFlee, after: 10 * time.Second},
		{name: "not_in_region", player: playerA, typ: api.ActionLeave, want: api.CodeNotInRegion},
		{name: "unknown_target region of another world", player: playerA, typ: api.ActionEnter, target: farRegion, want: api.CodeUnknownTarget},
		{name: "unknown_target npc outside the encounter", player: playerB, typ: api.ActionAttack, target: "wolf-far", want: api.CodeUnknownTarget},
		{name: "unknown_target a target that is no npc", player: playerB, typ: api.ActionAttack, target: forest, want: api.CodeUnknownTarget},
		{name: "group.create waits for the groups", player: playerA, typ: api.ActionGroupCreate, want: api.CodeNotImplemented},
	})
}

// A leave from a region is valid; the fixture has player-B in the forest, but
// in an encounter, so the valid leave is checked on a character moved there.
func TestValidateLeaveFromARegion(t *testing.T) {
	m := newWorld(t)
	ev := fact(t, world, "player-E", "player", "Женя", player("player-E", forest))
	if _, err := m.Apply(ev); err != nil {
		t.Fatal(err)
	}
	v := actions.Validator{Model: m, Grace: time.Second}
	got, e := v.Validate(actions.Command{PlayerID: "player-E", ActionKey: "k", Type: api.ActionLeave}, t0)
	if e != nil || got.Region == nil || got.Region.ID != forest {
		t.Fatalf("leave from the forest = %+v %v", got, e)
	}
}

// An active encounter whose entity has no time of creation (an old snapshot)
// gives the grace nothing to count from: its actions are not refused as
// encounter_unavailable however late they come (review #1 of T-305, N-2).
func TestAnEncounterWithoutItsCreationIsNotUnavailable(t *testing.T) {
	m := newWorld(t)
	undated := fact(t, world, "encounter-G", "encounter", "", encounter("player-G", "", "wolf-alpha"))
	undated.Timestamp = time.Time{}
	for _, ev := range []eventbus.Event{fact(t, world, "player-G", "player", "Гоша", player("player-G", forest)), undated} {
		if _, err := m.Apply(ev); err != nil {
			t.Fatal(err)
		}
	}
	v := actions.Validator{Model: m, Grace: 10 * time.Second}
	if _, e := v.Validate(actions.Command{PlayerID: "player-G", ActionKey: "k", Type: api.ActionFlee}, t0.Add(time.Hour)); e != nil {
		t.Errorf("flee from an undated encounter without an agent = %s, want accepted", e.Code)
	}
}

// The first check that fails is the answer: each case breaks two checks, and
// the earlier one of component §5.4 wins.
func TestValidateAnswersTheFirstFailingCheck(t *testing.T) {
	runValidate(t, []validateCase{
		{name: "not found before unknown action", player: "player-Z", typ: "dance", want: api.CodePlayerNotFound},
		{name: "dead before unknown action", player: playerD, typ: "dance", want: api.CodeCharacterDead},
		{name: "unknown action before the structure", player: playerA, typ: "dance", target: "nowhere", want: api.CodeUnknownAction},
		{name: "structure before the text", player: playerA, typ: api.ActionSay, target: forest, want: api.CodeInvalidRequest},
		{name: "text before the encounter", player: playerB, typ: api.ActionSay, text: text(""), want: api.CodeTextInvalid},
		{name: "absent target before the encounter", player: playerB, typ: api.ActionEnter, target: "nowhere", want: api.CodeUnknownTarget},
		{name: "encounter before the region of another world", player: playerB, typ: api.ActionEnter, target: farRegion, want: api.CodeInEncounter},
		{name: "not in encounter before the region", player: playerA, typ: api.ActionAttack, target: "wolf-far", want: api.CodeNotInEncounter},
		{name: "target dead before encounter unavailable", player: playerC, typ: api.ActionAttack, target: "wolf-dead", after: time.Minute, want: api.CodeTargetDead},
		{name: "encounter unavailable before the npc outside it", player: playerC, typ: api.ActionAttack, target: "wolf-far", after: time.Minute, want: api.CodeEncounterUnavailable},
		{name: "in encounter before not in region", player: playerB, typ: api.ActionLeave, want: api.CodeInEncounter},
		{name: "structure before group not implemented", player: playerA, typ: api.ActionGroupJoin, want: api.CodeInvalidRequest},
	})
}

// say: trimmed, 1 to 500 characters (runes, not bytes), no control characters
// (NFR-043).
func TestCheckTextOfSay(t *testing.T) {
	cases := []struct {
		name string
		in   *string
		want string
		ok   bool
	}{
		{"nil", nil, "", false},
		{"empty", text(""), "", false},
		{"spaces only", text(" \t "), "", false},
		{"trimmed", text("  волк рядом \n"), "волк рядом", true},
		{"one rune", text("а"), "а", true},
		{"500 runes of two bytes", text(strings.Repeat("ж", 500)), strings.Repeat("ж", 500), true},
		{"501 runes", text(strings.Repeat("ж", 501)), "", false},
		{"bell inside", text("a\x07b"), "", false},
		{"newline inside", text("a\nb"), "", false},
		{"escape inside", text("a\x1b[31mb"), "", false},
		{"invalid utf-8", text("a\xffb"), "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := actions.CheckText(tc.in)
			if got != tc.want || ok != tc.ok {
				t.Errorf("CheckText = %q %v, want %q %v", got, ok, tc.want, tc.ok)
			}
		})
	}
}

// Every action of the dictionary has a rule, and every solo rule names a
// registered player.* event (api-contracts.md §1.4).
func TestTheDictionaryIsTheActionTypesOfTheAPI(t *testing.T) {
	for _, typ := range api.ActionTypes() {
		rule, ok := actions.Rules[typ]
		if !ok {
			t.Errorf("action %s has no rule", typ)
			continue
		}
		if rule.Group != strings.HasPrefix(typ, "group.") {
			t.Errorf("action %s: Group = %v", typ, rule.Group)
		}
		if !rule.Group && !strings.HasPrefix(rule.Event, "player.") {
			t.Errorf("action %s is published as %q", typ, rule.Event)
		}
	}
	if len(actions.Rules) != len(api.ActionTypes()) {
		t.Errorf("%d rules for %d action types", len(actions.Rules), len(api.ActionTypes()))
	}
}
