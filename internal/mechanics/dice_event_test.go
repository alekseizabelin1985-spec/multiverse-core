package mechanics

import (
	"encoding/json"
	"strconv"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// TestDiceRolledPayloadValidates is the join between the library and the wire:
// what DiceRolledPayload builds has to pass dice.rolled.v1.json, or the rolls
// of a fight never reach the journal (§5.6, C-03).
//
// The event is built the way the encounter agent builds it — derived from the
// action that caused the roll, so the seed of the payload and the cause of the
// envelope agree.
func TestDiceRolledPayloadValidates(t *testing.T) {
	r := load(t)
	cause := playerAttacked()

	roll, err := r.Roll(cause.ID, 0, "d20", PurposeHit)
	if err != nil {
		t.Fatalf("roll: %v", err)
	}
	payload := DiceRolledPayload(roll, entity.Ref{ID: "player-A", Type: entity.TypePlayer})
	ev := eventbus.Derive(cause, "dice.rolled", contracts.SourceSwarm, payload)

	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("dice.rolled does not validate: %v", err)
	}

	// The payload says the same thing the Roll does, field by field, and the
	// seed it carries is the one the address produces.
	var back struct {
		Roll struct {
			Index   int    `json:"index"`
			Formula string `json:"formula"`
			Seed    string `json:"seed"`
			Result  int    `json:"result"`
			Natural int    `json:"natural"`
		} `json:"roll"`
		Purpose string `json:"purpose"`
		Roller  struct {
			Entity entity.Ref `json:"entity"`
		} `json:"roller"`
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Roll.Index != 0 || back.Roll.Formula != "d20" || back.Roll.Result != roll.Result || back.Roll.Natural != roll.Natural {
		t.Errorf("payload roll %+v, want %+v", back.Roll, roll)
	}
	if want := strconv.FormatUint(Seed(cause.ID, 0), 10); back.Roll.Seed != want {
		t.Errorf("payload seed %s, want %s", back.Roll.Seed, want)
	}

	// The bus decodes a payload into map[string]any (eventbus.Event.Payload),
	// where a JSON number becomes a float64. A seed above 2^53 read back that
	// way would be a different number, and events_hash_match would never
	// agree on replay; as a string it survives the trip.
	var generic map[string]any
	if err := json.Unmarshal(raw, &generic); err != nil {
		t.Fatalf("generic unmarshal: %v", err)
	}
	got, ok := generic["roll"].(map[string]any)["seed"].(string)
	if !ok {
		t.Fatalf("seed decodes as %T, want string", generic["roll"].(map[string]any)["seed"])
	}
	if want := strconv.FormatUint(Seed(cause.ID, 0), 10); got != want {
		t.Errorf("seed through a generic decode = %s, want %s", got, want)
	}
	if back.Purpose != PurposeHit {
		t.Errorf("purpose %q, want %q", back.Purpose, PurposeHit)
	}
	if back.Roller.Entity != (entity.Ref{ID: "player-A", Type: entity.TypePlayer}) {
		t.Errorf("roller %+v", back.Roller.Entity)
	}
}

// TestDiceRolledPayloadEveryPurpose: every purpose the rules may name in
// attack.rolls or flee.rolls is one the schema accepts. A rules file that
// loads must not produce an event the bus refuses.
func TestDiceRolledPayloadEveryPurpose(t *testing.T) {
	r := load(t)
	cause := playerAttacked()

	purposes := append(append([]string{}, r.Attack().Rolls...), r.Flee().Rolls...)
	purposes = append(purposes, PurposeNPCHit, PurposeNPCDamage, PurposeEncounterChance, PurposeBackground)

	for i, purpose := range purposes {
		t.Run(purpose, func(t *testing.T) {
			roll, err := r.Roll(cause.ID, i, "2d6+1", purpose)
			if err != nil {
				t.Fatalf("roll: %v", err)
			}
			payload := DiceRolledPayload(roll, entity.Ref{ID: "wolf-alpha", Type: entity.TypeNPC})
			ev := eventbus.Derive(cause, "dice.rolled", contracts.SourceSwarm, payload)
			if err := contracts.Validate(ev); err != nil {
				t.Fatalf("dice.rolled with purpose %s does not validate: %v", purpose, err)
			}
		})
	}
}

// TestOutcomeFitsCombatDecided checks that what the mechanics decide fits the
// event that reports it: every field of combat.decided.outcome and of its hp
// block has something in Outcome to fill it with, and the numbers survive the
// trip through JSON (combat.decided.v1.json, C-05).
//
// The payload is built here rather than in the library on purpose: the type
// belongs to EPIC-003, which publishes it — the mechanics only have to fit
// into it (ADR-012 p. 6).
func TestOutcomeFitsCombatDecided(t *testing.T) {
	r := load(t)
	cause := playerAttacked()

	hitRoll, err := r.Roll(cause.ID, 0, "d20", PurposeHit)
	if err != nil {
		t.Fatalf("roll: %v", err)
	}
	damageRoll, err := r.Roll(cause.ID, 1, "d6", PurposeDamage)
	if err != nil {
		t.Fatalf("roll: %v", err)
	}
	wolf, _ := r.Stats("wolf")
	damage := r.Damage(damageRoll.Result, false)
	out := Outcome{
		Hit:        true,
		Natural:    hitRoll.Natural,
		Damage:     damage,
		HPBefore:   wolf.HP,
		HPAfter:    r.ClampHP(wolf.HP-damage, wolf.HPMax),
		TargetDead: r.ClampHP(wolf.HP-damage, wolf.HPMax) == 0,
	}

	payload := map[string]any{
		"encounter": ref("encounter-1", entity.TypeEncounter, "Схватка"),
		"round":     map[string]any{"seq": 1},
		"action":    ActionAttack,
		"attacker":  ref("player-A", entity.TypePlayer, "Вася"),
		"defender":  ref("wolf-alpha", entity.TypeNPC, "Вожак"),
		"outcome": map[string]any{
			"hit":         out.Hit,
			"natural":     out.Natural,
			"damage":      out.Damage,
			"critical":    out.Critical,
			"fumble":      out.Fumble,
			"target_dead": out.TargetDead,
		},
		"hp": map[string]any{
			"defender_before": out.HPBefore,
			"defender_after":  out.HPAfter,
			"defender_max":    wolf.HPMax,
		},
		"rolls": []any{
			map[string]any{"event": map[string]any{"id": "01JCROLL0", "type": "dice.rolled"}, "index": hitRoll.Index},
			map[string]any{"event": map[string]any{"id": "01JCROLL1", "type": "dice.rolled"}, "index": damageRoll.Index},
		},
		// The version of the rules that decided it: the number Load read out of
		// rules/dark-forest.yaml and the reason a balance change is auditable.
		"rules_version": r.Version,
		"phase1_mode":   "rules",
	}

	ev := eventbus.Derive(cause, "combat.decided", contracts.SourceSwarm, payload,
		eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-1", Level: "task", Blueprint: "encounter-dark-forest"}))
	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("combat.decided does not validate: %v", err)
	}
	if r.Version != "0.1" {
		t.Errorf("rules_version in the event is %q, want the one Load read", r.Version)
	}
}

// TestFleeOutcomeFitsCombatDecided: a flight attempt reports a threshold and a
// success where an attack reports a hit, and carries neither defender nor hp.
func TestFleeOutcomeFitsCombatDecided(t *testing.T) {
	r := load(t)
	cause := playerAttacked()
	wolf, _ := r.Stats("wolf")

	threshold, err := r.FleeThreshold(wolf, 1)
	if err != nil {
		t.Fatalf("threshold: %v", err)
	}
	success := false
	out := Outcome{Natural: 4, Threshold: threshold, Success: &success}

	payload := map[string]any{
		"encounter": ref("encounter-1", entity.TypeEncounter, "Схватка"),
		"round":     map[string]any{"seq": 1},
		"action":    ActionFlee,
		"attacker":  ref("player-A", entity.TypePlayer, "Вася"),
		"outcome": map[string]any{
			"hit":            false,
			"natural":        out.Natural,
			"success":        out.Success,
			"threshold":      out.Threshold,
			"living_enemies": 1,
		},
		"rolls": []any{
			map[string]any{"event": map[string]any{"id": "01JCROLL0", "type": "dice.rolled"}, "index": 0},
		},
		"rules_version": r.Version,
		"phase1_mode":   "rules",
	}

	ev := eventbus.Derive(cause, "combat.decided", contracts.SourceSwarm, payload,
		eventbus.WithAgent(eventbus.AgentRef{ID: "encounter-1", Level: "task", Blueprint: "encounter-dark-forest"}))
	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("combat.decided for a flight does not validate: %v", err)
	}
	if out.Threshold != 11 {
		t.Errorf("threshold %d, want 11 against one wolf", out.Threshold)
	}
}

func ref(id, typ, name string) map[string]any {
	return map[string]any{
		"entity": map[string]any{"id": id, "type": typ},
		"name":   name,
	}
}

// playerAttacked is the event a fight rolls from: the action of the player, as
// the gateway publishes it (C-04).
func playerAttacked() eventbus.Event {
	return eventbus.NewRoot("player.attacked", contracts.SourceGateway, "dark-forest-world",
		&eventbus.ScopeRef{ID: "solo:player-A", Type: "solo"}, eventbus.ActorCI,
		map[string]any{
			"entity": ref("player-A", entity.TypePlayer, "Вася"),
			"target": ref("wolf-alpha", entity.TypeNPC, "Вожак"),
		})
}
