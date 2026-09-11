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

	a := &Actor{ID: e.ID, Type: e.Type}
	var ok bool
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
	if a.Dmg, ok = e.Dmg(); !ok {
		return nil, missingAttr(e, entity.AttrDmg)
	}
	if a.Status, ok = e.Status(); !ok {
		return nil, missingAttr(e, entity.AttrStatus)
	}
	// A flee bonus is optional: an actor without one does not run away.
	a.Flee, _ = e.Flee()

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

func missingAttr(e *entity.Entity, attr string) error {
	return fmt.Errorf("mechanics: %s %s has no %s", e.Type, e.ID, attr)
}
