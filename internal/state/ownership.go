package state

import (
	"slices"
	"strings"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// The kinds of proposer State tells apart (state-and-mechanics.md §4.1). An
// agent proposes under the level of its blueprint; the rows of the ownership
// table for gateway, author and system are proposers without an agent. The row
// system is reserved and empty (C-02 v1.8 p. 6): nobody proposes under it.
const (
	ProposerAgent   = "agent"
	ProposerGateway = contracts.ProposerGateway
	ProposerAuthor  = contracts.ProposerAuthor
)

// Proposer is who made a proposal, as the ownership check needs it (§4.1).
// Kind is empty when the envelope names nobody the table has a row for.
type Proposer struct {
	Kind    string
	Level   string
	AgentID string
	Source  string
}

// agentLevels are the levels an agent may propose under. An agent that names
// gateway, author or system as its level does not get their rights: those rows
// belong to proposers without an agent, and the row system, reserved and empty,
// gives no rights to anybody.
var agentLevels = []string{
	contracts.ProposerGlobal, contracts.ProposerDomain, contracts.ProposerTask,
	contracts.ProposerObject, contracts.ProposerMonitor,
}

// proposerOf derives the proposer from the envelope (§4.1): meta.agent names an
// agent and its level; without one the source decides — the gateway, or the
// author through mvctl (bootstrap included, C-02 v1.5).
//
// The double of the gateway (testkit/gateway) proposes as the gateway it
// stands in for: the registry lists it among the publishers of both proposal
// types for that reason. The double of the swarm proposes under meta.agent like
// the swarm. meta.actor_kind plays no part: it is about the session, not about
// the right to write.
func proposerOf(ev eventbus.Event) Proposer {
	p := Proposer{Source: ev.Source}
	if agent := ev.Meta.Agent; agent != nil {
		p.AgentID, p.Level = agent.ID, agent.Level
		if slices.Contains(agentLevels, agent.Level) {
			p.Kind = ProposerAgent
		}
		return p
	}
	switch ev.Source {
	case contracts.SourceGateway, contracts.SourceTestkitGateway:
		p.Kind = ProposerGateway
	case contracts.SourceMvctl:
		p.Kind = ProposerAuthor
	}
	return p
}

// row is the value of OwnershipRule.Proposer this proposer is checked against,
// or "" when the table has none for it.
func (p Proposer) row() string {
	if p.Kind == ProposerAgent {
		return p.Level
	}
	return p.Kind
}

// isGateway is the proposer the norms of C-02 v1.2 and v1.4 give status and hp
// of a character: the gateway, without an agent.
func (p Proposer) isGateway() bool { return p.Kind == ProposerGateway }

// ownership answers step 6 of §4.5 from contracts.OwnershipRules — the only
// source of the table (ADR-025) — and the norms C-02 puts over it.
//
// The table is the product of paths and causes, so it is wider than the norm:
// the row of the gateway lets set status through with cause=move. The pairs the
// norm refuses are checked here, on top of the table, and refused as
// level_violation like a break of the table itself (C-02 v1.4 "путь ↔ причина",
// v1.5): a pair that another proposer could have written legally says "not
// you", not "not like this". What depends on the state of the world — rest in
// an encounter, hp above hp_max — is decided after the operations
// (restRefusal), and the transition of the status on what changed
// (statusRefusal).
type ownership struct {
	rules []contracts.OwnershipRule
}

func newOwnership() ownership { return ownership{rules: contracts.OwnershipRules()} }

// mayCreate reports whether the proposer may create an entity of this type
// with this cause.
func (o ownership) mayCreate(p Proposer, typ, cause string) bool {
	row := p.row()
	if row == "" {
		return false
	}
	return slices.ContainsFunc(o.rules, func(r contracts.OwnershipRule) bool {
		return r.Create && r.Proposer == row && covers(r.EntityTypes, typ) && covers(r.Causes, cause)
	})
}

// mayChange reports whether every operation of a change set is the proposer's
// to make on an entity of this type with this cause. typ is the type of the
// entity in the world, never the one the change set claims (§4.6).
func (o ownership) mayChange(p Proposer, typ, cause string, ops []entity.Op) bool {
	row := p.row()
	if row == "" {
		return false
	}
	for _, op := range ops {
		allowed := slices.ContainsFunc(o.rules, func(r contracts.OwnershipRule) bool {
			return r.Proposer == row && covers(r.EntityTypes, typ) && covers(r.Causes, cause) &&
				slices.ContainsFunc(r.Paths, func(prefix string) bool { return underPrefix(op.Path, prefix) })
		})
		if !allowed || !normAllows(p, typ, cause, op.Path) {
			return false
		}
	}
	return true
}

// normAllows holds the pairs of path and cause C-02 and §4.6 narrow the table
// to:
//   - a rest of the gateway writes hp of a character and nothing else (C-02
//     v1.8 p. 3): it is the first check of the norm of rest, before the
//     encounter and the value restRefusal reads;
//   - the gateway writes status of a character only with cause=forget, and hp
//     only with cause=rest (C-02 v1.4, v1.5);
//   - the gateway writes everything of a group except encounter_id, which is
//     the encounter agent's;
//   - the region GM writes everything of a region except its description.
//
// The third exclusion of the table — an NPC's hp lowered by the region GM while
// a character is alive in its encounter — reads the encounter and is not held
// here (dev-log T-056).
func normAllows(p Proposer, typ, cause, path string) bool {
	root := rootOf(path)
	switch {
	case p.isGateway() && typ == entity.TypePlayer && cause == CauseRest:
		return root == entity.AttrHP
	case p.isGateway() && typ == entity.TypePlayer && root == entity.AttrStatus:
		return cause == CauseForget
	case p.isGateway() && typ == entity.TypePlayer && root == entity.AttrHP:
		return cause == CauseRest
	case p.isGateway() && typ == entity.TypeGroup && root == entity.AttrEncounterID:
		return false
	case p.Kind == ProposerAgent && p.Level == contracts.ProposerDomain && typ == entity.TypeRegion &&
		root == entity.AttrDescription:
		return false
	default:
		return true
	}
}

// The causes the norms name.
const (
	CauseForget = "forget"
	CauseRest   = "rest"
)

// covers reports whether a list of the table admits a value: the value itself
// or the wildcard, which is the same "*" for types, paths and causes.
func covers(list []string, value string) bool {
	return slices.Contains(list, value) || slices.Contains(list, contracts.AnyCause)
}

// underPrefix reports whether a path lies under a prefix of the table: the
// prefix itself, a field below it or an element of it. inventory covers
// inventory[0] and npcs covers npcs[1].last_damager; hp does not cover hp_max.
func underPrefix(path, prefix string) bool {
	if prefix == contracts.AnyPath {
		return true
	}
	rest, ok := strings.CutPrefix(path, prefix)
	return ok && (rest == "" || rest[0] == '.' || rest[0] == '[')
}

// rootOf is the first segment of a path: position of position, inventory of
// inventory[0].
func rootOf(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// statusRefusal decides the change of status a change set made, on the entity
// before and after the operations rather than on the paths of changed[]: a
// path under status (status.x, which entity.ApplyOps turns into an object) or a
// remove of status would otherwise write the status past the matrix (review #2
// of T-056, Ma-3). A status set to the value it holds is no change and no
// refusal (C-02 v1.4, "Статус в то же значение").
//
//   - Status after the operations that is not a string — gone, or an object —
//     is invalid_op, and so is a move the matrix of data-model.md §3.3 does not
//     know (entity.StatusTransitionAllowed).
//   - abandoned is reached only by the gateway, without an agent, with
//     cause=forget (C-02 v1.2): anybody else is refused level_violation — an
//     agent of the fight, or the gateway with another cause (C-02 v1.5).
//   - With the table in force, the gateway moves a status only from alive to
//     abandoned: its row names status for that one transition.
//
// A terminal entity never gets here: step 5 refuses it first with dead_entity.
func statusRefusal(p Proposer, cause string, set entity.ChangeSet, before, after *entity.Entity,
	checkOwnership bool) Reason {
	if !touchesRoot(set.Ops, entity.AttrStatus) {
		return ""
	}
	from, _ := before.Status()
	raw, present := after.Attr(entity.AttrStatus)
	to, isText := raw.(string)
	switch {
	case !present || !isText:
		return ReasonInvalidOp
	case from == to:
		return ""
	case !entity.StatusTransitionAllowed(from, to):
		return ReasonInvalidOp
	case to == entity.StatusAbandoned && (!p.isGateway() || cause != CauseForget):
		return ReasonLevelViolation
	case checkOwnership && p.isGateway() && (from != entity.StatusAlive || to != entity.StatusAbandoned):
		return ReasonLevelViolation
	}
	return ""
}

// restRefusal holds the part of the norm of rest that reads the world and the
// value (C-02 v1.8 p. 3, КД §4.6): a rest restores a character outside an
// encounter to exactly hp_max. The first part of the norm — a rest writes hp
// and nothing else — is step 6 (normAllows), so the checks run in the order
// C-02 gives: the path, the encounter, the kind of hp, the value.
//
//   - Inside an encounter: law_violation without invariant_id. It is a
//     condition of the world with no law of its own — a legitimate race, the
//     gateway checked its read-model before the encounter reached State — and
//     the report of NFR-020 does not count it.
//   - hp that is not a whole number: invalid_op.
//   - hp above hp_max, or no whole hp_max to bound it: law_violation {inv-02},
//     the ceiling breaks whoever proposes it.
//   - hp below hp_max: level_violation. The same value is legal from the
//     encounter agent, so the refusal says "not you": a rest that lowers or
//     only partly restores hp would be damage outside the mechanics.
//
// "Inside an encounter" is read on before, the character as the rest found it
// (КД §4.6: encounter_id of the entity before the operations). Since the rest
// writes nothing but hp this is also where the world stands after it; the
// reading stays on before so that the norm does not depend on step 6 having
// run (review #1 of T-056, Mi-1).
//
// The hit points are read on after, as an entity holds them, not by the path
// the operation named: hp after the rest has to be a whole number, or the
// operation wrote something else there (hp.x turns hp into an object) and is
// invalid_op; a ceiling that is not a whole number bounds nothing and breaks
// inv-02 (review #2 of T-056, Ma-3).
func restRefusal(p Proposer, set entity.ChangeSet, before, after *entity.Entity) (Reason, string) {
	if !p.isGateway() || before.Type != entity.TypePlayer || !touchesRoot(set.Ops, entity.AttrHP) {
		return "", ""
	}
	if encounter, _ := before.EncounterID(); encounter != "" {
		return ReasonLawViolation, ""
	}
	raw, _ := after.Attr(entity.AttrHP)
	hp, whole := after.HP()
	if _, isText := raw.(string); isText || !whole {
		return ReasonInvalidOp, ""
	}
	hpMax, bounded := after.HPMax()
	switch {
	case !bounded || hp > hpMax:
		return ReasonLawViolation, invHPInRange
	case hp < hpMax:
		return ReasonLevelViolation, ""
	}
	return "", ""
}

// invHPInRange is the identifier of the law 0 <= hp <= hp_max
// (mechanics.InvHPInRange), named here for the refusal of a rest above the
// maximum, which State decides whether or not the laws of the world are
// checked.
const invHPInRange = "inv-02"

func touchesRoot(ops []entity.Op, root string) bool {
	return slices.ContainsFunc(ops, func(op entity.Op) bool { return rootOf(op.Path) == root })
}

// corpsePaths are the attributes a terminal entity still takes: how it died,
// who killed it, who took the trophy and which encounter it fell in (§4.5
// p. 5). Without them a trophy could never be handed out, since the NPC it is
// taken from is dead by definition (inv-03).
var corpsePaths = []string{
	entity.AttrDiedAt, entity.AttrKilledBy, entity.AttrLootClaimedBy, entity.AttrEncounterID,
}

// onlyCorpsePaths reports whether every operation writes one of corpsePaths.
func onlyCorpsePaths(ops []entity.Op) bool {
	return len(ops) > 0 && !slices.ContainsFunc(ops, func(op entity.Op) bool {
		return !slices.Contains(corpsePaths, op.Path)
	})
}
