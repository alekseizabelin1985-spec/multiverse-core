package mechanics

import (
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/entity"
)

func candidate(id string, hp int, status, participation string) *Actor {
	return &Actor{
		ID: id, Type: entity.TypePlayer,
		HP: hp, HPMax: 10, Atk: 2, Def: 12, Dmg: "d6", Flee: "2",
		Status: status, Participation: participation,
	}
}

// permutations is every order of the candidates, so that a test of the choice
// proves the choice does not depend on how the list was assembled.
func permutations(in []*Actor) [][]*Actor {
	if len(in) <= 1 {
		return [][]*Actor{slices.Clone(in)}
	}
	var out [][]*Actor
	for i := range in {
		rest := slices.Concat(in[:i:i], in[i+1:])
		for _, p := range permutations(rest) {
			out = append(out, append([]*Actor{in[i]}, p...))
		}
	}
	return out
}

// TestNPCTargetOrder is npc_target.order of the rules file, key by key: the
// last damager while it is a candidate, then the fewest hit points, then the
// lowest id — the same answer for every order of the list.
func TestNPCTargetOrder(t *testing.T) {
	r := load(t)
	alive := entity.StatusAlive

	cases := []struct {
		name        string
		lastDamager string
		candidates  []*Actor
		want        string
	}{
		{
			name:        "the last damager, however healthy",
			lastDamager: "player-C",
			candidates: []*Actor{
				candidate("player-A", 2, alive, ""), candidate("player-B", 5, alive, ""), candidate("player-C", 10, alive, ""),
			},
			want: "player-C",
		},
		{
			name:        "an idle last damager is no candidate: the fewest hit points",
			lastDamager: "player-C",
			candidates: []*Actor{
				candidate("player-A", 6, alive, ""), candidate("player-B", 3, alive, entity.ParticipationActive),
				candidate("player-C", 1, alive, entity.ParticipationIdle),
			},
			want: "player-B",
		},
		{
			name:        "a dead last damager is no candidate",
			lastDamager: "player-A",
			candidates: []*Actor{
				candidate("player-A", 0, entity.StatusDead, ""), candidate("player-B", 7, alive, ""), candidate("player-C", 4, alive, ""),
			},
			want: "player-C",
		},
		{
			name:        "a last damager not among the candidates",
			lastDamager: "player-Z",
			candidates: []*Actor{
				candidate("player-A", 4, alive, ""), candidate("player-B", 9, alive, ""),
			},
			want: "player-A",
		},
		{
			name: "a tie on hit points goes to the lowest id",
			candidates: []*Actor{
				candidate("player-C", 4, alive, ""), candidate("player-B", 4, alive, ""), candidate("player-D", 9, alive, ""),
			},
			want: "player-B",
		},
		{
			name: "everyone the rules exclude is skipped even when weakest and first by id",
			candidates: []*Actor{
				candidate("player-0", 1, entity.StatusAbandoned, ""),
				candidate("player-1", 1, entity.StatusAscendedFinal, ""),
				candidate("player-2", 1, alive, entity.ParticipationOutOfCombat),
				candidate("player-3", 1, alive, entity.ParticipationIdle),
				candidate("player-4", 0, entity.StatusDead, ""),
				candidate("player-9", 8, alive, ""),
			},
			want: "player-9",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			wolf, _ := r.Stats("wolf")
			wolf.ID, wolf.LastDamager = "wolf-alpha", c.lastDamager
			for _, order := range permutations(c.candidates) {
				got, err := r.NPCTarget(&wolf, order)
				if err != nil {
					t.Fatalf("npc target: %v", err)
				}
				if got == nil || got.ID != c.want {
					t.Fatalf("picked %v out of %v, want %s", got, ids(order), c.want)
				}
			}
		})
	}
}

// TestNPCTargetReadsTheOrderOfTheRules: the order is data. A rule set that
// ranks by id alone bites the lowest id however weak the others are, and one
// that puts the fewest hit points before the last damager prefers the weakest.
func TestNPCTargetReadsTheOrderOfTheRules(t *testing.T) {
	alive := entity.StatusAlive
	candidates := []*Actor{
		candidate("player-A", 9, alive, ""), candidate("player-B", 2, alive, ""), candidate("player-C", 5, alive, ""),
	}

	for _, c := range []struct {
		order string
		want  string
	}{
		{"order: [player_id_asc]", "player-A"},
		{"order: [min_hp, last_damager]", "player-B"},
		{"order: [last_damager]", "player-C"},
	} {
		t.Run(c.order, func(t *testing.T) {
			r := rulesWith(t, "order: [last_damager, min_hp, player_id_asc]", c.order)
			wolf, _ := r.Stats("wolf")
			wolf.ID, wolf.LastDamager = "wolf-alpha", "player-C"
			got, err := r.NPCTarget(&wolf, candidates)
			if err != nil || got == nil || got.ID != c.want {
				t.Errorf("picked (%v, %v), want %s", got, err, c.want)
			}
		})
	}
}

// TestNPCTargetBreaksWhatTheRulesLeaveTiedByID: an order without
// player_id_asc still answers the same for every order of the list — whatever
// the rules leave tied goes to the lowest id, so a replay that assembles the
// candidates differently bites the same character.
func TestNPCTargetBreaksWhatTheRulesLeaveTiedByID(t *testing.T) {
	r := rulesWith(t, "order: [last_damager, min_hp, player_id_asc]", "order: [min_hp]")
	wolf, _ := r.Stats("wolf")
	wolf.ID = "wolf-alpha"
	alive := entity.StatusAlive
	candidates := []*Actor{
		candidate("player-C", 3, alive, ""), candidate("player-A", 3, alive, ""), candidate("player-B", 3, alive, ""),
	}
	for _, order := range permutations(candidates) {
		got, err := r.NPCTarget(&wolf, order)
		if err != nil || got == nil || got.ID != "player-A" {
			t.Fatalf("picked (%v, %v) out of %v, want player-A", got, err, ids(order))
		}
	}
}

// TestNPCTargetNobodyIsAnAnswer: nobody left to bite is (nil, nil), the answer
// UC-008 A2 describes — not an error.
func TestNPCTargetNobodyIsAnAnswer(t *testing.T) {
	r := load(t)
	wolf, _ := r.Stats("wolf")
	wolf.ID = "wolf-alpha"

	for name, candidates := range map[string][]*Actor{
		"no candidates": nil,
		"empty list":    {},
		"all excluded": {
			candidate("player-A", 0, entity.StatusDead, ""),
			candidate("player-B", 5, entity.StatusAbandoned, ""),
			candidate("player-C", 5, entity.StatusAlive, entity.ParticipationIdle),
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := r.NPCTarget(&wolf, candidates)
			if got != nil || err != nil {
				t.Errorf("answered (%v, %v), want (nil, nil)", got, err)
			}
		})
	}
}

// TestNPCTargetRefusesDefects: an error is what a caller gets for handing in
// something no rule can choose from, and never for an empty choice.
func TestNPCTargetRefusesDefects(t *testing.T) {
	r := load(t)
	wolf, _ := r.Stats("wolf")
	wolf.ID = "wolf-alpha"
	deadWolf := wolf
	deadWolf.Status = entity.StatusDead
	player := candidate("player-A", 5, entity.StatusAlive, "")
	asNPC := *player
	asNPC.Type = entity.TypeNPC

	for _, c := range []struct {
		name       string
		npc        *Actor
		candidates []*Actor
		says       string
	}{
		{"no npc", nil, []*Actor{player}, "without an npc"},
		{"a character picks", player, []*Actor{player}, "not an npc"},
		{"a dead npc picks", &deadWolf, []*Actor{player}, "bites nobody"},
		{"a nil candidate", &wolf, []*Actor{player, nil}, "nil"},
		{"an npc as a candidate", &wolf, []*Actor{&asNPC}, "bites characters"},
		{"one character twice", &wolf, []*Actor{player, player}, "twice"},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, err := r.NPCTarget(c.npc, c.candidates)
			if err == nil {
				t.Fatalf("picked %v without an error", got)
			}
			if got != nil {
				t.Errorf("picked %v together with the error", got)
			}
			if !strings.Contains(err.Error(), c.says) {
				t.Errorf("error %q does not say %q", err, c.says)
			}
		})
	}
}

func ids(actors []*Actor) []string {
	out := make([]string, 0, len(actors))
	for _, a := range actors {
		out = append(out, a.ID)
	}
	return out
}
