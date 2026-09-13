package mechanics

import (
	"fmt"

	"multiverse-core.io/shared/entity"
)

// ActorFromEntity reads a fighter out of an entity, and out of the encounter it
// is standing in when there is one (C-03 v1.1).
//
// The encounter is a second argument rather than a lookup because the two facts
// it carries — how a character is taking part in the round and who hit an NPC
// last — belong to the encounter and not to the fighter: two encounters would
// answer differently about the same wolf. Passing nil is legal and means the
// actor is not in a fight; what comes back is then the actor as it stands in
// the world.
//
// Everything a formula can read is required: an actor missing hp or def is a
// bug of whoever created it, and defaulting the number would turn that bug into
// a fight that quietly plays by different rules.
func ActorFromEntity(e *entity.Entity, enc *entity.Entity) (*Actor, error) {
	if e == nil {
		return nil, fmt.Errorf("mechanics: no entity")
	}
	if e.Type != entity.TypePlayer && e.Type != entity.TypeNPC {
		return nil, fmt.Errorf("mechanics: entity %s is a %s: only a player or an npc fights", e.ID, e.Type)
	}

	a := &Actor{ID: e.ID, Type: e.Type, Version: e.Version}
	var (
		ok  bool
		err error
	)
	if a.HP, ok = e.HP(); !ok {
		return nil, missingAttr(e, entity.AttrHP)
	}
	if a.HPMax, ok = e.HPMax(); !ok {
		return nil, missingAttr(e, entity.AttrHPMax)
	}
	if a.Atk, ok = e.Atk(); !ok {
		return nil, missingAttr(e, entity.AttrAtk)
	}
	if a.Def, ok = e.Def(); !ok {
		return nil, missingAttr(e, entity.AttrDef)
	}
	if a.Dmg, err = dmgOf(e); err != nil {
		return nil, err
	}
	if a.Status, ok = e.Status(); !ok {
		return nil, missingAttr(e, entity.AttrStatus)
	}
	// A flee bonus is optional: an actor without one does not run away.
	a.Flee, _ = e.Flee()
	if e.Type == entity.TypeNPC {
		// Required like the numbers are (data-model.md §3.4): an NPC without a
		// kind would fall without the trophy of its kind, and nothing would
		// notice. A kind the rules give no loot table is legal — it simply
		// leaves nothing behind.
		if a.Kind, ok = e.Kind(); !ok || a.Kind == "" {
			return nil, missingAttr(e, entity.AttrKind)
		}
	}

	if enc == nil {
		return a, nil
	}
	if enc.Type != entity.TypeEncounter {
		return nil, fmt.Errorf("mechanics: %s is a %s, not an encounter", enc.ID, enc.Type)
	}

	switch e.Type {
	case entity.TypePlayer:
		participants, err := enc.Participants()
		if err != nil {
			return nil, fmt.Errorf("mechanics: encounter %s: participants: %w", enc.ID, err)
		}
		for _, p := range participants {
			if p.PlayerID == e.ID {
				a.Participation = p.State
				break
			}
		}
	case entity.TypeNPC:
		npcs, err := enc.NPCs()
		if err != nil {
			return nil, fmt.Errorf("mechanics: encounter %s: npcs: %w", enc.ID, err)
		}
		for _, n := range npcs {
			if n.NPCID == e.ID {
				a.LastDamager = n.LastDamager
				break
			}
		}
	}
	return a, nil
}

// dmgOf reads the damage of a fighter, which is dice and only dice.
//
// data-model.md §3.3 types the combat stats together as "int / formula", and a
// flat number would read naturally for dmg. It is refused on purpose (T-053,
// the backlog item of T-050): the grammar of the rules has no flat damage — a
// dice expression needs at least one die of two sides (§5.3) — and widening the
// grammar is a task with a review of its own (ADR-012 p. 2). A number is
// therefore named as what it is rather than reported as a missing attribute,
// and an expression that does not parse is refused here, where the defect of
// whoever created the entity is visible, rather than in the middle of a fight.
func dmgOf(e *entity.Entity) (string, error) {
	raw, present := e.Attr(entity.AttrDmg)
	if !present {
		return "", missingAttr(e, entity.AttrDmg)
	}
	text, isText := raw.(string)
	if !isText {
		return "", fmt.Errorf("mechanics: %s %s: %s is %v, not a dice expression: damage is rolled, never a flat number",
			e.Type, e.ID, entity.AttrDmg, raw)
	}
	if _, err := ParseDice(text); err != nil {
		return "", fmt.Errorf("mechanics: %s %s: %s: %w", e.Type, e.ID, entity.AttrDmg, err)
	}
	return text, nil
}

func missingAttr(e *entity.Entity, attr string) error {
	return fmt.Errorf("mechanics: %s %s has no %s", e.Type, e.ID, attr)
}
