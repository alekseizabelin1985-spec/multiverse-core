package mechanics

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"multiverse-core.io/shared/entity"
)

// The register of the laws of a world in executable form (NFR-020, §5.7).
//
// There is one list, and it is this one. The text of every law lives in
// laws/dark-forest-world.v1.yaml and nowhere else; the logic lives here and
// nowhere else; the identifiers are the same in both, and a contract test
// (mvctl laws check) compares the two sets so that a law cannot be written
// without being enforced or enforced without being written (ADR-012 p. 5).
//
// A rules file switches identifiers on through its invariants list — it does
// not describe them. An invariant is a property of the world, not a number a
// balance patch may change.
//
// What a check can see. A check is handed the world as it would be after a
// change — State builds that view from its working set and the copies it is
// about to commit, the guardian of the swarm builds it from its own view and
// the operations of a proposal — together with touched, the identifiers of the
// entities the change moves (state-and-mechanics.md §4.5 p. 8). It is not
// handed the world as it was. Everything here is therefore a property of one
// state of the world, looked at from the touched entities outwards, and never a
// property of a transition: "dead -> alive" and "a corpse is not changed" are
// decided by State against the entity before the change (§4.5 p. 5,
// dead_entity), where the old status is still at hand.
//
// Why from the touched entities outwards and not the whole world: a proposal is
// answered for what it does. A break some earlier change left behind is a
// matter for the audit, not a reason to refuse the next move of an unrelated
// character. A caller that wants the whole world checked names every entity.

// Where an invariant is enforced.
const (
	WhereState     = "state"
	WhereMechanics = "mechanics"
	WhereGateway   = "gateway"
	WhereGuardian  = "guardian"
	WhereAudit     = "audit"
)

// The identifiers of the ten invariants of MVP-1. They are the law ids of
// laws@v1 and the values of the invariants list of a rules file.
const (
	InvDeadDoesNotAct     = "inv-01" // the dead neither act nor are targets
	InvHPInRange          = "inv-02" // 0 <= hp <= hp_max
	InvOneTrophyPerNPC    = "inv-03" // one NPC leaves at most one trophy
	InvGroupPosition      = "inv-04" // a member stands where the group stands
	InvGroupSize          = "inv-05" // a group holds between 1 and 6 members
	InvOneScope           = "inv-06" // a character is in exactly one scope
	InvHPMatchesDecision  = "inv-07" // hp after the fact equals hp_after of combat.decided
	InvNoDecisionsUnasked = "inv-08" // no combat.decided against a character that did not act
	InvRespawnTTL         = "inv-09" // an NPC does not come back before its cooldown; dead never becomes alive
	InvOnePosition        = "inv-10" // one entity, one position
)

// allInvariants is the register in identifier order.
//
// The five laws of a solo fight carry a check (T-054). The three laws of a
// group — inv-04, inv-05, inv-06 — still have none: they read the group, its
// members and their scopes together, and they come with the group round of
// increment I2 (T-066). inv-07 and inv-08 keep a nil Check for good: they are
// decided against the journal rather than against the world, so no Applier can
// answer them (§5.7).
var allInvariants = []Invariant{
	{ID: InvDeadDoesNotAct, Where: []string{WhereState, WhereMechanics, WhereGateway}, Check: checkDeadDoesNotAct},
	{ID: InvHPInRange, Where: []string{WhereState, WhereMechanics}, Check: checkHPInRange},
	{ID: InvOneTrophyPerNPC, Where: []string{WhereState, WhereAudit}, Check: checkOneTrophyPerNPC},
	{ID: InvGroupPosition, Where: []string{WhereState}},
	{ID: InvGroupSize, Where: []string{WhereState, WhereGateway}},
	{ID: InvOneScope, Where: []string{WhereState}},
	{ID: InvHPMatchesDecision, Where: []string{WhereAudit}},
	{ID: InvNoDecisionsUnasked, Where: []string{WhereGuardian, WhereAudit}},
	{ID: InvRespawnTTL, Where: []string{WhereState}, Check: checkRespawnTTL},
	{ID: InvOnePosition, Where: []string{WhereState}, Check: checkOnePosition},
}

// InvariantIDs are every identifier the register knows, in order. It is what a
// rules file may switch on and what the laws file is compared against.
func InvariantIDs() []string {
	out := make([]string, 0, len(allInvariants))
	for _, inv := range allInvariants {
		out = append(out, inv.ID)
	}
	return out
}

// --- inv-01: the dead neither act nor are targets ---

// checkDeadDoesNotAct reads every touched encounter that is still on and asks
// who it holds: a fighter whose status is terminal — dead, abandoned or
// ascended_final, all three alike (C-02 v1.2) — is neither a target on its NPC
// list nor a character still taking part.
//
// A character who fell or walked away stays in participants[] as history; what
// the law forbids is that the fight still counts it in, so a terminal character
// has to be idle, out of the fight or marked dead there. An NPC has no such
// mark, and a fight that goes on over a dead one would offer a corpse to strike.
//
// A terminal character still taking part breaks the law only when the same
// package touches the character (C-05 v1.8 p. 9 (b)). A /forget changes the
// character, not the encounter, and arrives on its own time: a package of the
// encounter built before its fact and touching only the encounter — "both
// missed" — is not refused for a participation the agent has not rewritten
// yet, and in a group that package is the whole round of the others. A death,
// by contrast, comes in the package that caused it, and that package is held.
// The participation nobody rewrote is the audit's (mvctl report --audit touches
// every entity).
//
// Why only the encounters that are touched. A death that comes from outside a
// fight — a /forget of a character in the middle of it (C-04 v1.1), an NPC
// killed by another hand (C-05 v1.4 p. 6) — changes the fighter and not the
// encounter, and the encounter agent closes the fight after that fact. Checking
// the encounters of every touched fighter would refuse the /forget itself. A
// package of the fight, by contrast, always writes the encounter (C-05 v1.3
// p. 2), and the one that kills closes it in the same package.
func checkDeadDoesNotAct(v StateView, touched []string) []Violation {
	var out []Violation
	for _, enc := range touchedOfType(v, touched, entity.TypeEncounter) {
		if state, _ := enc.State(); state == entity.EncounterStateResolved {
			continue
		}
		npcs, err := enc.NPCs()
		if err != nil {
			out = append(out, violation(InvDeadDoesNotAct, enc.ID, "npcs of the encounter do not decode: %v", err))
			continue
		}
		participants, err := enc.Participants()
		if err != nil {
			out = append(out, violation(InvDeadDoesNotAct, enc.ID, "participants of the encounter do not decode: %v", err))
			continue
		}
		// The message does not name the terminal status: the law answers dead,
		// abandoned and ascended_final alike, and so does its violation.
		for _, n := range npcs {
			if isTerminal(v, n.NPCID) {
				out = append(out, violation(InvDeadDoesNotAct, n.NPCID,
					"%s no longer lives and is still a target of encounter %s, which is not resolved", n.NPCID, enc.ID))
			}
		}
		for _, p := range participants {
			if !isTerminal(v, p.PlayerID) || !takesPart(p.State) || !slices.Contains(touched, p.PlayerID) {
				continue
			}
			out = append(out, violation(InvDeadDoesNotAct, p.PlayerID,
				"%s no longer lives and still takes part in encounter %s as %q", p.PlayerID, enc.ID, participationOf(p.State)))
		}
	}
	return sortViolations(out)
}

// takesPart says whether a participation still counts a character into the
// round. Empty reads as active, as it does for Actor.Participation.
func takesPart(state string) bool {
	switch state {
	case entity.ParticipationIdle, entity.ParticipationOutOfCombat, entity.StatusDead:
		return false
	default:
		return true
	}
}

func participationOf(state string) string {
	if state == "" {
		return entity.ParticipationActive
	}
	return state
}

// isTerminal says whether an entity of the view has a terminal status. An
// entity the view does not hold is not this law's business: an encounter naming
// a fighter that does not exist is a defect of whoever wrote it, and it is not a
// dead fighter acting.
func isTerminal(v StateView, id string) bool {
	e, ok := v.Get(id)
	return ok && e != nil && e.IsTerminal()
}

// --- inv-02: 0 <= hp <= hp_max ---

// checkHPInRange holds the hit points of every touched entity that has them
// inside [0, hp_max]. An inc on hp already clamps (entity.ApplyOps), so what
// reaches this check is a set: a rest above the maximum, a blow written by hand
// below zero, or an hp_max lowered under the hit points it bounds.
//
// hp that is not a whole number, or hp without an hp_max to bound it, is a
// break of the same law rather than something to skip: a range that cannot be
// read is a range nobody holds.
//
// A character and an NPC always have hit points (data-model.md §3.3, §3.4), so
// a touched one without hp breaks the law the way a touched one without a
// position breaks inv-10: otherwise a remove on hp would pass every law, while
// hp written as null would not. Any other entity is checked when it has hp.
func checkHPInRange(v StateView, touched []string) []Violation {
	var out []Violation
	for _, e := range touchedEntities(v, touched) {
		if !e.HasAttr(entity.AttrHP) {
			if e.Type == entity.TypePlayer || e.Type == entity.TypeNPC {
				out = append(out, violation(InvHPInRange, e.ID, "%s %s has no hp to hold in range", e.Type, e.ID))
			}
			continue
		}
		hp, ok := e.HP()
		if !ok {
			out = append(out, violation(InvHPInRange, e.ID, "hp of %s %s is not a whole number", e.Type, e.ID))
			continue
		}
		hpMax, ok := e.HPMax()
		if !ok {
			out = append(out, violation(InvHPInRange, e.ID, "%s %s has hp %d and no whole hp_max to bound it", e.Type, e.ID, hp))
			continue
		}
		if hp < 0 || hp > hpMax {
			out = append(out, violation(InvHPInRange, e.ID, "hp of %s %s is %d, outside [0, %d]", e.Type, e.ID, hp, hpMax))
		}
	}
	return sortViolations(out)
}

// --- inv-03: one NPC leaves at most one trophy ---

// checkOneTrophyPerNPC keeps the trophies of every NPC in one pair of hands and
// from one decision.
//
// A trophy is an item whose source names the NPC it was taken from and the
// combat.decided of the last hit (data-model.md §3.5). A loot table may hold
// more than one item, so one character carrying two items of one wolf from the
// same decision is one handout, not two. What the law forbids is the second
// handout: items of one NPC in two inventories, items of one NPC from two
// different decisions, or items held by somebody other than the character the
// NPC records in loot_claimed_by.
//
// It is the lootIndex of §4.5 computed from the world rather than kept beside
// it: every character's inventory is read, so a second copy of an index cannot
// drift away from the inventories it indexes.
func checkOneTrophyPerNPC(v StateView, touched []string) []Violation {
	holders, broken := trophyHolders(v)

	var out []Violation
	for _, e := range touchedEntities(v, touched) {
		switch e.Type {
		case entity.TypeNPC:
			if reason := trophyBreak(v, e.ID, holders[e.ID], ""); reason != "" {
				out = append(out, violation(InvOneTrophyPerNPC, e.ID, "%s", reason))
			}
		case entity.TypePlayer:
			if err := broken[e.ID]; err != nil {
				out = append(out, violation(InvOneTrophyPerNPC, e.ID,
					"inventory of %s %s does not decode, its trophies cannot be counted: %v", e.Type, e.ID, err))
				continue
			}
			for _, source := range sourcesHeldBy(holders, e.ID) {
				if reason := trophyBreak(v, source, holders[source], e.ID); reason != "" {
					out = append(out, violation(InvOneTrophyPerNPC, e.ID, "%s", reason))
				}
			}
		}
	}
	return sortViolations(out)
}

// trophy is one item taken from an NPC: who holds it and which decision gave it.
type trophy struct {
	holder  string
	eventID string
}

// trophyHolders reads the inventory of every character of the world and groups
// the trophies by the NPC they were taken from. An inventory that does not
// decode is reported back rather than dropped, so that a touched character
// whose inventory is broken is refused instead of passing uncounted.
func trophyHolders(v StateView) (map[string][]trophy, map[string]error) {
	holders := map[string][]trophy{}
	broken := map[string]error{}
	for _, p := range v.ByType(entity.TypePlayer) {
		if p == nil {
			continue
		}
		items, err := p.Inventory()
		if err != nil {
			broken[p.ID] = err
			continue
		}
		for _, item := range items {
			if item.Source.Entity.ID == "" {
				continue
			}
			holders[item.Source.Entity.ID] = append(holders[item.Source.Entity.ID],
				trophy{holder: p.ID, eventID: item.Source.EventID})
		}
	}
	return holders, broken
}

// sourcesHeldBy are the NPCs a character holds trophies of, in map order: the
// violations found for them are ordered by sortViolations.
func sourcesHeldBy(holders map[string][]trophy, holder string) []string {
	var out []string
	for source, trophies := range holders {
		if slices.ContainsFunc(trophies, func(t trophy) bool { return t.holder == holder }) {
			out = append(out, source)
		}
	}
	return out
}

// trophyBreak names what is wrong with the trophies of one NPC, or returns ""
// when they are one handout to the character the NPC names. who is the
// character the question is asked for, "" when it is asked for the NPC.
func trophyBreak(v StateView, npcID string, trophies []trophy, who string) string {
	var hands, decisions []string
	for _, t := range trophies {
		hands = appendUnique(hands, t.holder)
		decisions = appendUnique(decisions, t.eventID)
	}
	slices.Sort(hands)
	slices.Sort(decisions)

	switch {
	case len(hands) > 1:
		return fmt.Sprintf("trophies of %s are held by %s: one NPC leaves one trophy", npcID, strings.Join(hands, ", "))
	case len(decisions) > 1:
		return fmt.Sprintf("trophies of %s come from %d decisions (%s): one NPC leaves one trophy",
			npcID, len(decisions), strings.Join(decisions, ", "))
	case len(hands) == 0:
		return ""
	}

	npc, ok := v.Get(npcID)
	if !ok || npc == nil {
		return ""
	}
	claimed, ok := npc.AttrString(entity.AttrLootClaimedBy)
	if !ok || claimed == "" || claimed == hands[0] {
		return ""
	}
	if who == "" {
		who = hands[0]
	}
	return fmt.Sprintf("%s holds a trophy of %s, which records %s in loot_claimed_by", who, npcID, claimed)
}

func appendUnique(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}

// --- inv-09: dead never becomes alive ---

// checkRespawnTTL refuses a touched entity that carries a death record and is
// not dead.
//
// The world a check sees has no "before", so the transition dead -> alive is
// caught by State against the entity as it was (§4.5 p. 5). What remains
// visible after the change is the record the death left: died_at and killed_by
// are written together with status=dead (ChangesFor) and are among the four
// paths a corpse may still take (§4.5 p. 5). An entity carrying them with a
// living status is one that came back under the same identifier — and a
// respawn is a new NPC with a new identifier, never the old one revived
// (§5.7, inv-09).
//
// The cooldown itself is the other half of the law and is not decided here: the
// region GM spawns the new NPC after respawn_ttl (§5.7, "GM региона (TTL, новый
// id)"), and nothing in the world ties a newly spawned wolf to the one it
// replaces.
func checkRespawnTTL(v StateView, touched []string) []Violation {
	var out []Violation
	for _, e := range touchedEntities(v, touched) {
		record := deathRecord(e)
		if len(record) == 0 {
			continue
		}
		status, _ := e.Status()
		if entity.IsTerminalStatus(status) {
			continue
		}
		out = append(out, violation(InvRespawnTTL, e.ID,
			"%s %s carries %s and its status is %q: the dead never become alive, a respawn is a new entity",
			e.Type, e.ID, strings.Join(record, " and "), status))
	}
	return sortViolations(out)
}

// deathRecord names the attributes of a death an entity carries. A null or an
// empty string is no record: it is what a create without one may write.
func deathRecord(e *entity.Entity) []string {
	var out []string
	for _, attr := range []string{entity.AttrDiedAt, entity.AttrKilledBy} {
		raw, ok := e.Attr(attr)
		if !ok || raw == nil || raw == "" {
			continue
		}
		out = append(out, attr)
	}
	return out
}

// --- inv-10: one entity, one position ---

// outsidePrefix is the position of a character who stands in no region:
// outside:{world_id} (data-model.md §3.3, rules flee.success_position).
const outsidePrefix = "outside:"

// checkOnePosition holds every touched entity that stands somewhere to exactly
// one position, written in one of the two forms the model knows: the id of a
// region of this world, or outside:{world_id} of this world (§5.7).
//
// A character and an NPC always stand somewhere, so a touched one without a
// position breaks the law as surely as one with two. Any other entity is
// checked when it has a position at all: a group carries one (data-model.md
// §3.6), and whether it matches its members is inv-04, not this.
func checkOnePosition(v StateView, touched []string) []Violation {
	var out []Violation
	for _, e := range touchedEntities(v, touched) {
		raw, present := e.Attr(entity.AttrPosition)
		if !present {
			if e.Type == entity.TypePlayer || e.Type == entity.TypeNPC {
				out = append(out, violation(InvOnePosition, e.ID, "%s %s stands nowhere: it has no position", e.Type, e.ID))
			}
			continue
		}
		position, ok := raw.(string)
		if !ok {
			out = append(out, violation(InvOnePosition, e.ID,
				"position of %s %s is %T, not one position", e.Type, e.ID, raw))
			continue
		}
		if reason := positionBreak(v, position); reason != "" {
			out = append(out, violation(InvOnePosition, e.ID, "position of %s %s is %q: %s", e.Type, e.ID, position, reason))
		}
	}
	return sortViolations(out)
}

// positionBreak says what is wrong with a position, or "" when it names a place
// of this world.
func positionBreak(v StateView, position string) string {
	if world, outside := strings.CutPrefix(position, outsidePrefix); outside {
		if world != v.WorldID() {
			return fmt.Sprintf("want outside:%s, the world the entity is in", v.WorldID())
		}
		return ""
	}
	region, ok := v.Get(position)
	switch {
	case !ok || region == nil:
		return "no such region in the world"
	case region.Type != entity.TypeRegion:
		return fmt.Sprintf("%s is a %s, not a region", position, region.Type)
	default:
		return ""
	}
}

// --- shared by the checks ---

// touchedEntities are the touched entities the view holds. An id the view does
// not hold is skipped — whether an entity exists is unknown_entity, decided by
// State before any law (§4.5 p. 3). The order and the repeats of touched do not
// reach the answer: sortViolations orders it and drops what a repeated id
// found twice.
func touchedEntities(v StateView, touched []string) []*entity.Entity {
	out := make([]*entity.Entity, 0, len(touched))
	for _, id := range touched {
		if e, ok := v.Get(id); ok && e != nil {
			out = append(out, e)
		}
	}
	return out
}

func touchedOfType(v StateView, touched []string, t string) []*entity.Entity {
	return slices.DeleteFunc(touchedEntities(v, touched), func(e *entity.Entity) bool { return e.Type != t })
}

func violation(id, entityID, format string, args ...any) Violation {
	return Violation{InvariantID: id, EntityID: entityID, Message: fmt.Sprintf(format, args...)}
}

// sortViolations puts the violations of one check in a fixed order and drops
// exact repeats, so that the answer to one world is the same answer on every
// run and every machine, whatever order the caller collected touched in and the
// view lists the world in (ADR-003 p. 5).
func sortViolations(vs []Violation) []Violation {
	slices.SortFunc(vs, func(a, b Violation) int {
		return cmp.Or(
			cmp.Compare(a.InvariantID, b.InvariantID),
			cmp.Compare(a.EntityID, b.EntityID),
			cmp.Compare(a.Message, b.Message),
		)
	})
	return slices.Compact(vs)
}
