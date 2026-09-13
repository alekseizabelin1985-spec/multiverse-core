package mechanics

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"multiverse-core.io/shared/entity"
)

// ErrInvalidRules is what every complaint about a rules file matches. The
// detail is in the ConfigError it wraps: which key, and what is wrong with it.
var ErrInvalidRules = errors.New("mechanics: invalid rules")

// ConfigError is one rejected key of a rules file. Path is the key in the
// dotted form a reader can find in the YAML — "entities.wolf.hp_max", not an
// offset into a byte slice.
type ConfigError struct {
	Path   string
	Reason string
}

func (e ConfigError) Error() string {
	if e.Path == "" {
		return "mechanics: invalid rules: " + e.Reason
	}
	return fmt.Sprintf("mechanics: invalid rules: %s: %s", e.Path, e.Reason)
}

// Is makes every ConfigError match ErrInvalidRules, so a caller that only wants
// to know whether the file is usable does not have to enumerate the reasons.
func (e ConfigError) Is(target error) bool { return target == ErrInvalidRules }

func badRules(path, format string, args ...any) error {
	return ConfigError{Path: path, Reason: fmt.Sprintf(format, args...)}
}

// SchemaVersion is the shape of a rules file this package understands. It is
// not rules_version: the schema says how the file is laid out, rules_version
// says which numbers are in it and travels in combat.decided.
const SchemaVersion = 1

// RulesDocument is rules/dark-forest.yaml as it is written
// (state-and-mechanics.md §5.2, ADR-012 p. 1). Every key of the file is a field
// here and every field is required unless its comment says otherwise: an
// unknown key is a typo, and a typo in a rules file is a fight that behaves
// differently from the one its author reviewed.
type RulesDocument struct {
	SchemaVersion int    `yaml:"schema_version"`
	RulesVersion  string `yaml:"rules_version"`
	World         string `yaml:"world"`

	Entities  map[string]StatsDoc `yaml:"entities"`
	Attack    AttackDoc           `yaml:"attack"`
	NPCAttack NPCAttackDoc        `yaml:"npc_attack"`
	Flee      FleeDoc             `yaml:"flee"`
	Rest      RestDoc             `yaml:"rest"`
	NPCTarget NPCTargetDoc        `yaml:"npc_target"`
	Loot      map[string][]Item   `yaml:"loot"`
	Round     RoundDoc            `yaml:"round"`

	// Invariants are the identifiers of the laws in force for this rule set.
	// The logic is in Go (invariants.go) and the text is in the laws file; this
	// list only switches identifiers on.
	Invariants []string `yaml:"invariants"`
}

// StatsDoc is the base line of a kind of actor: what a character or a wolf
// starts with and what an NPC is spawned with (Rules.Stats).
type StatsDoc struct {
	HPMax int    `yaml:"hp_max"`
	Atk   int    `yaml:"atk"`
	Def   int    `yaml:"def"`
	Dmg   string `yaml:"dmg"`

	// Flee is the bonus to a flight attempt; null for something that does not
	// run away (the wolf of v0.1).
	Flee *int `yaml:"flee"`
}

// AttackDoc is how a strike is decided.
type AttackDoc struct {
	Hit            string `yaml:"hit"`
	CritNatural    int    `yaml:"crit_natural"`
	CritMultiplier int    `yaml:"crit_multiplier"`
	FumbleNatural  int    `yaml:"fumble_natural"`

	// DamageFormula is either the name of an attribute of the attacker holding
	// a dice expression ("dmg") or a dice expression itself ("d6+1").
	DamageFormula string `yaml:"damage_formula"`

	// Rolls are the purposes of the rolls of one attack, in the order their
	// indices are handed out. The damage roll happens only on a hit, and its
	// index is spent either way.
	Rolls []string `yaml:"rolls"`
}

// NPCAttackDoc is the answer of an NPC. It inherits the whole of attack and
// differs only in the purposes of its rolls and in how often it may happen.
type NPCAttackDoc struct {
	Inherit      string `yaml:"inherit"`
	OncePerRound bool   `yaml:"once_per_round"`
}

// FleeDoc is how walking away is decided.
type FleeDoc struct {
	Check string   `yaml:"check"`
	Rolls []string `yaml:"rolls"`

	// OnFail is what a failed attempt costs: free_attack — the enemy strikes
	// out of turn — or none.
	OnFail string `yaml:"on_fail"`

	// SuccessPosition is where a successful escape puts the character, as a
	// template over {world_id} and {region_id}.
	SuccessPosition string `yaml:"success_position"`
}

// RestDoc is how catching a breath works.
type RestDoc struct {
	// Restore is hp_max — a full recovery — and nothing else. A rest by dice
	// would be a roll no purpose of dice.rolled can carry, so Load refuses it.
	Restore            string `yaml:"restore"`
	AllowedInEncounter bool   `yaml:"allowed_in_encounter"`
}

// NPCTargetDoc is how an NPC picks whom to bite.
type NPCTargetDoc struct {
	Order   []string `yaml:"order"`
	Exclude []string `yaml:"exclude"`
}

// RoundDoc is the shape of a group round. The coordinator is the gateway
// (ADR-020); the numbers live here so that a blueprint can point at them
// instead of repeating them (C-05 v1.1).
type RoundDoc struct {
	Timeout         string `yaml:"timeout"`
	IdleAfterMissed int    `yaml:"idle_after_missed"`
}

// The order in which an NPC considers its candidates (§5.4).
const (
	OrderLastDamager = "last_damager"
	OrderMinHP       = "min_hp"
	OrderPlayerIDAsc = "player_id_asc"
)

// What a failed flight attempt costs.
const (
	OnFailFreeAttack = "free_attack"
	OnFailNone       = "none"
)

// RestoreToMax is the value of rest.restore that means a full recovery.
const RestoreToMax = "hp_max"

// The compiled checks of a rule set, by the key they are written under.
const (
	CheckAttackHit = "attack.hit"
	CheckFlee      = "flee.check"
)

var (
	targetOrders   = []string{OrderLastDamager, OrderMinHP, OrderPlayerIDAsc}
	targetExcludes = []string{
		entity.ParticipationIdle, entity.ParticipationOutOfCombat,
		entity.StatusDead, entity.StatusAbandoned, entity.StatusAscendedFinal,
	}
	rulesVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+(\.[0-9]+)?$`)
	positionVars   = []string{"world_id", "region_id"}
)

// Rules is a loaded, validated and compiled rule set: the numbers of one world
// and the formulas that read them, with every string already parsed. Nothing
// mutates it after Load, so one instance serves every worker of a process.
type Rules struct {
	// Version is rules_version — the number that travels in combat.decided and
	// in a snapshot, and the one a change of balance bumps.
	Version string

	// World is the world these rules apply to.
	World string

	doc        RulesDocument
	checks     map[string]CheckExpr
	damage     damageFormula
	stats      map[string]Actor
	round      RoundRules
	invariants []Invariant
}

type damageFormula struct {
	attr string   // an attribute of the attacker holding a dice expression
	dice DiceExpr // used when attr is empty
}

// RoundRules are the round parameters of a group fight, parsed.
type RoundRules struct {
	Timeout         time.Duration
	IdleAfterMissed int
}

// Load reads a rules file from disk. It is the one function of this package
// that touches the file system, and it does so once: everything after it is
// arithmetic over what it parsed.
func Load(path string) (*Rules, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("mechanics: read rules: %w", err)
	}
	r, err := LoadBytes(b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

// LoadBytes parses, validates and compiles a rules document. It is what a test
// with an inline fixture uses and what Load is built from (C-03 v1.1).
//
// Every complaint is raised here rather than during a fight: an unparsable
// formula, an unknown key, a damage expression that could heal the target. A
// rules file that loads is a rules file whose every string has already been
// read successfully at least once.
func LoadBytes(b []byte) (*Rules, error) {
	var doc RulesDocument
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&doc); err != nil {
		return nil, ConfigError{Reason: "not a rules document: " + err.Error()}
	}
	// A yaml.Decoder stops at the first document, so a file where an editor
	// left a "---" would lose half its rules without a word — in a package
	// where an unknown key is already an error because it is probably a typo.
	if err := dec.Decode(new(yaml.Node)); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, ConfigError{Reason: "after the rules document: " + err.Error()}
		}
		return nil, ConfigError{Reason: "a second YAML document after ---: a rules file holds exactly one"}
	}

	r := &Rules{
		Version: doc.RulesVersion,
		World:   doc.World,
		doc:     doc,
		checks:  map[string]CheckExpr{},
		stats:   map[string]Actor{},
	}
	if err := r.compile(); err != nil {
		return nil, err
	}
	return r, nil
}

func (r *Rules) compile() error {
	doc := r.doc

	if doc.SchemaVersion != SchemaVersion {
		return badRules("schema_version", "want %d, have %d", SchemaVersion, doc.SchemaVersion)
	}
	if !rulesVersionRe.MatchString(doc.RulesVersion) {
		return badRules("rules_version", "want a version like 0.1 or 1.2.3, have %q", doc.RulesVersion)
	}
	if doc.World == "" {
		return badRules("world", "no world")
	}

	if err := r.compileEntities(); err != nil {
		return err
	}
	if err := r.compileAttack(); err != nil {
		return err
	}
	if err := r.compileFlee(); err != nil {
		return err
	}
	if err := r.compileRest(); err != nil {
		return err
	}
	if err := r.compileTarget(); err != nil {
		return err
	}
	if err := r.compileLoot(); err != nil {
		return err
	}
	if err := r.compileRound(); err != nil {
		return err
	}
	return r.compileInvariants()
}

func (r *Rules) compileEntities() error {
	if len(r.doc.Entities) == 0 {
		return badRules("entities", "no kinds: the rules describe nobody")
	}
	// Sorted, not in map order: with two broken kinds in one file the caller
	// must be told about the same one on every run, or a failure in CI is not
	// the failure the author of the rules reproduces at home.
	for _, kind := range slices.Sorted(maps.Keys(r.doc.Entities)) {
		s := r.doc.Entities[kind]
		at := "entities." + kind
		switch {
		case s.HPMax < 1:
			return badRules(at+".hp_max", "want at least 1, have %d", s.HPMax)
		case s.Def < 1:
			return badRules(at+".def", "want at least 1, have %d", s.Def)
		case s.Atk < 0:
			return badRules(at+".atk", "want a bonus of 0 or more, have %d", s.Atk)
		}

		dice, err := ParseDice(s.Dmg)
		if err != nil {
			return badRules(at+".dmg", "%s", err)
		}
		if dice.Min() < 0 {
			// A hit that heals is not a rule of a dark forest, it is a typo.
			return badRules(at+".dmg", "%s can roll %d: damage is never negative", dice, dice.Min())
		}

		actor := Actor{
			Type:   kindType(kind),
			HP:     s.HPMax,
			HPMax:  s.HPMax,
			Atk:    s.Atk,
			Def:    s.Def,
			Dmg:    dice.String(),
			Status: entity.StatusAlive,
		}
		if s.Flee != nil {
			actor.Flee = fmt.Sprint(*s.Flee)
		}
		// Only an NPC has a kind in the world (data-model.md §3.4); a
		// character created from these stats reads back without one.
		if actor.Type == entity.TypeNPC {
			actor.Kind = kind
		}
		r.stats[kind] = actor
	}
	return nil
}

func (r *Rules) compileAttack() error {
	a := r.doc.Attack

	check, err := r.compileCheck("attack.hit", a.Hit)
	if err != nil {
		return err
	}
	r.checks[CheckAttackHit] = check

	sides := check.Dice.Sides
	switch {
	case a.CritNatural < 1 || a.CritNatural > sides:
		return badRules("attack.crit_natural", "want a face of %s (1..%d), have %d", check.Dice, sides, a.CritNatural)
	case a.FumbleNatural < 1 || a.FumbleNatural > sides:
		return badRules("attack.fumble_natural", "want a face of %s (1..%d), have %d", check.Dice, sides, a.FumbleNatural)
	case a.CritNatural == a.FumbleNatural:
		return badRules("attack.fumble_natural", "%d is also crit_natural: one face cannot be both", a.FumbleNatural)
	case a.CritMultiplier < 1:
		return badRules("attack.crit_multiplier", "want at least 1, have %d", a.CritMultiplier)
	}

	if err := r.compileDamage(a.DamageFormula); err != nil {
		return err
	}
	if err := checkPurposes("attack.rolls", a.Rolls); err != nil {
		return err
	}

	if r.doc.NPCAttack.Inherit != "attack" {
		return badRules("npc_attack.inherit", "want attack, have %q: an NPC has no rules of its own in v0.1", r.doc.NPCAttack.Inherit)
	}
	return nil
}

func (r *Rules) compileDamage(formula string) error {
	if formula == "" {
		return badRules("attack.damage_formula", "no formula")
	}
	if isIdent(formula) {
		// The one attribute that holds a dice expression is dmg; anything else
		// would be a number, and a number is not damage a die decides.
		if formula != "dmg" {
			return badRules("attack.damage_formula", "%q is not an attribute holding dice: want dmg or a dice expression", formula)
		}
		r.damage = damageFormula{attr: formula}
		return nil
	}
	dice, err := ParseDice(formula)
	if err != nil {
		return badRules("attack.damage_formula", "%s", err)
	}
	if dice.Min() < 0 {
		return badRules("attack.damage_formula", "%s can roll %d: damage is never negative", dice, dice.Min())
	}
	r.damage = damageFormula{dice: dice}
	return nil
}

func (r *Rules) compileFlee() error {
	f := r.doc.Flee

	check, err := r.compileCheck("flee.check", f.Check)
	if err != nil {
		return err
	}
	r.checks[CheckFlee] = check

	if err := checkPurposes("flee.rolls", f.Rolls); err != nil {
		return err
	}
	if f.OnFail != OnFailFreeAttack && f.OnFail != OnFailNone {
		return badRules("flee.on_fail", "want one of %v, have %q", []string{OnFailFreeAttack, OnFailNone}, f.OnFail)
	}
	if f.SuccessPosition == "" {
		return badRules("flee.success_position", "no template: an escape has to lead somewhere")
	}
	for _, name := range templateVars(f.SuccessPosition) {
		if !slices.Contains(positionVars, name) {
			return badRules("flee.success_position", "unknown placeholder {%s}: want one of %v", name, positionVars)
		}
	}
	return nil
}

// compileRest accepts a full recovery and nothing else. Every roll of the
// platform is published as dice.rolled before the decision that rests on it,
// and the purposes of its schema have no rest to name: a rest by dice would be
// the one chance nobody could audit. It is refused here, where every other
// complaint about a rules file is raised, rather than in the middle of a game.
func (r *Rules) compileRest() error {
	if restore := r.doc.Rest.Restore; restore != RestoreToMax {
		return badRules("rest.restore", "want %s, have %q: a rest by dice would be a roll dice.rolled has no purpose for", RestoreToMax, restore)
	}
	return nil
}

func (r *Rules) compileTarget() error {
	t := r.doc.NPCTarget
	if len(t.Order) == 0 {
		return badRules("npc_target.order", "no order: an NPC would have no way to choose")
	}
	seen := map[string]bool{}
	for i, name := range t.Order {
		if !slices.Contains(targetOrders, name) {
			return badRules(fmt.Sprintf("npc_target.order[%d]", i), "want one of %v, have %q", targetOrders, name)
		}
		if seen[name] {
			return badRules(fmt.Sprintf("npc_target.order[%d]", i), "%q is already in the order", name)
		}
		seen[name] = true
	}
	for i, name := range t.Exclude {
		if !slices.Contains(targetExcludes, name) {
			return badRules(fmt.Sprintf("npc_target.exclude[%d]", i), "want one of %v, have %q", targetExcludes, name)
		}
	}
	return nil
}

func (r *Rules) compileLoot() error {
	// Sorted for the same reason as the kinds: one broken file, one message.
	for _, kind := range slices.Sorted(maps.Keys(r.doc.Loot)) {
		items := r.doc.Loot[kind]
		if _, ok := r.doc.Entities[kind]; !ok {
			return badRules("loot."+kind, "no such kind in entities")
		}
		if len(items) == 0 {
			return badRules("loot."+kind, "an empty table: drop the key instead")
		}
		for i, item := range items {
			at := fmt.Sprintf("loot.%s[%d]", kind, i)
			if item.Kind == "" {
				return badRules(at+".kind", "no kind")
			}
			if item.Name == "" {
				return badRules(at+".name", "no name: a trophy is shown to a player")
			}
		}
	}
	return nil
}

func (r *Rules) compileRound() error {
	timeout, err := time.ParseDuration(r.doc.Round.Timeout)
	if err != nil {
		return badRules("round.timeout", "%s", err)
	}
	if timeout <= 0 {
		return badRules("round.timeout", "want a positive duration, have %s", timeout)
	}
	if r.doc.Round.IdleAfterMissed < 1 {
		return badRules("round.idle_after_missed", "want at least 1, have %d", r.doc.Round.IdleAfterMissed)
	}
	r.round = RoundRules{Timeout: timeout, IdleAfterMissed: r.doc.Round.IdleAfterMissed}
	return nil
}

func (r *Rules) compileInvariants() error {
	if len(r.doc.Invariants) == 0 {
		return badRules("invariants", "no laws: a world without invariants is not guarded")
	}
	seen := map[string]bool{}
	for i, id := range r.doc.Invariants {
		at := fmt.Sprintf("invariants[%d]", i)
		if !slices.ContainsFunc(allInvariants, func(inv Invariant) bool { return inv.ID == id }) {
			return badRules(at, "unknown invariant %q", id)
		}
		if seen[id] {
			return badRules(at, "%q is already in the list", id)
		}
		seen[id] = true
	}
	for _, inv := range allInvariants {
		if seen[inv.ID] {
			r.invariants = append(r.invariants, inv)
		}
	}
	return nil
}

// compileCheck parses one check and makes sure every identifier in it is one
// somebody can answer for: the left side reads the actor doing the checking,
// the right side reads the target and the context of the turn.
func (r *Rules) compileCheck(at, text string) (CheckExpr, error) {
	if text == "" {
		return CheckExpr{}, badRules(at, "no formula")
	}
	check, err := ParseCheck(text)
	if err != nil {
		return CheckExpr{}, badRules(at, "%s", err)
	}
	left, right := check.idents()
	for _, name := range left {
		if !slices.Contains(actorIdents, name) {
			return CheckExpr{}, badRules(at, "left side reads %q, which is not an attribute of an actor %v", name, actorIdents)
		}
	}
	for _, name := range right {
		if !slices.Contains(actorIdents, name) && !slices.Contains(contextIdents, name) {
			return CheckExpr{}, badRules(at, "right side reads %q, which is neither an attribute %v nor a variable of the turn %v", name, actorIdents, contextIdents)
		}
	}
	return check, nil
}

func checkPurposes(at string, rolls []string) error {
	if len(rolls) == 0 {
		return badRules(at, "no rolls")
	}
	for i, purpose := range rolls {
		if !slices.Contains(Purposes, purpose) {
			return badRules(fmt.Sprintf("%s[%d]", at, i), "unknown purpose %q: want one of %v", purpose, Purposes)
		}
	}
	return nil
}

// kindType maps a kind of the entities table onto an entity type: player is
// the one kind a person drives, everything else in the table is an NPC.
func kindType(kind string) string {
	if kind == entity.TypePlayer {
		return entity.TypePlayer
	}
	return entity.TypeNPC
}

func templateVars(s string) []string {
	var out []string
	for rest := s; ; {
		_, after, found := strings.Cut(rest, "{")
		if !found {
			return out
		}
		name, tail, closed := strings.Cut(after, "}")
		if !closed {
			return out
		}
		out = append(out, name)
		rest = tail
	}
}

// --- the rule set, as its callers read it ---

// Document is the rules as they were written, for a caller that has to show
// them or compare them with a file. It is a copy: the maps of a loaded rule set
// stay the rule set's own.
//
// The copy is deep enough to keep that promise. StatsDoc.Flee is a pointer —
// null means "does not run away" — and a cloned map hands out the same pointer,
// so a caller editing what it was given would edit the rules themselves.
func (r *Rules) Document() RulesDocument {
	doc := r.doc
	doc.Entities = maps.Clone(r.doc.Entities)
	for kind, s := range doc.Entities {
		if s.Flee != nil {
			flee := *s.Flee
			s.Flee = &flee
			doc.Entities[kind] = s
		}
	}
	doc.Loot = maps.Clone(r.doc.Loot)
	for kind, items := range doc.Loot {
		doc.Loot[kind] = slices.Clone(items)
	}
	doc.Invariants = slices.Clone(r.doc.Invariants)
	doc.Attack.Rolls = slices.Clone(r.doc.Attack.Rolls)
	doc.Flee.Rolls = slices.Clone(r.doc.Flee.Rolls)
	doc.NPCTarget.Order = slices.Clone(r.doc.NPCTarget.Order)
	doc.NPCTarget.Exclude = slices.Clone(r.doc.NPCTarget.Exclude)
	return doc
}

// Stats are the numbers a kind of actor starts with: what an NPC is spawned
// with and what a new character is created with (C-03 v1.1). The actor comes
// back at full health and without an identity — the caller gives it one.
func (r *Rules) Stats(kind string) (Actor, bool) {
	a, ok := r.stats[kind]
	return a, ok
}

// Kinds are the kinds the rules describe, sorted, so that a caller enumerating
// them twice sees them in the same order.
func (r *Rules) Kinds() []string {
	out := make([]string, 0, len(r.stats))
	for kind := range r.stats {
		out = append(out, kind)
	}
	slices.Sort(out)
	return out
}

// Check is a compiled formula by name (CheckAttackHit, CheckFlee).
func (r *Rules) Check(name string) (CheckExpr, bool) {
	c, ok := r.checks[name]
	return c, ok
}

// Attack is how a strike is decided, as written.
func (r *Rules) Attack() AttackDoc { return r.doc.Attack }

// NPCAttack is how the answer of an NPC differs from a strike of a character.
func (r *Rules) NPCAttack() NPCAttackDoc { return r.doc.NPCAttack }

// Flee is how walking away is decided, as written.
func (r *Rules) Flee() FleeDoc { return r.doc.Flee }

// Rest is how catching a breath works, as written.
func (r *Rules) Rest() RestDoc { return r.doc.Rest }

// TargetRules is how an NPC picks whom to bite, as written.
func (r *Rules) TargetRules() NPCTargetDoc {
	t := r.doc.NPCTarget
	t.Order = slices.Clone(t.Order)
	t.Exclude = slices.Clone(t.Exclude)
	return t
}

// Round is the shape of a group round, parsed.
func (r *Rules) Round() RoundRules { return r.round }

// Loot is what a dead NPC of this kind leaves behind. The slice is a copy: a
// caller that hands a trophy out does not get to edit the table.
func (r *Rules) Loot(kind string) []Item { return slices.Clone(r.doc.Loot[kind]) }

// --- the formulas themselves ---

// Critical and Fumble read the natural die of an attack: a natural crit hits
// however low the sum is, a natural fumble misses however high it is
// (domain-review §3.3, §5.4).
func (r *Rules) Critical(natural int) bool { return natural == r.doc.Attack.CritNatural }
func (r *Rules) Fumble(natural int) bool   { return natural == r.doc.Attack.FumbleNatural }

// DamageDice is the dice this attacker deals damage with: either its own dmg
// or the expression the rules fix for everyone.
func (r *Rules) DamageDice(attacker Actor) (DiceExpr, error) {
	if r.damage.attr == "" {
		return r.damage.dice, nil
	}
	if attacker.Dmg == "" {
		return DiceExpr{}, fmt.Errorf("mechanics: actor %s has no %s", attacker.ID, r.damage.attr)
	}
	dice, err := ParseDice(attacker.Dmg)
	if err != nil {
		return DiceExpr{}, fmt.Errorf("mechanics: actor %s: %s %q: %w", attacker.ID, r.damage.attr, attacker.Dmg, err)
	}
	return dice, nil
}

// Damage is what a rolled damage expression costs the target: the roll, doubled
// on a critical, and never below nothing.
func (r *Rules) Damage(rolled int, critical bool) int {
	if critical {
		rolled *= r.doc.Attack.CritMultiplier
	}
	return max(rolled, 0)
}

// FleeThreshold is the number a flight attempt has to beat with this many
// enemies still standing. It is reported in combat.decided so that a player can
// see what the escape was worth (combat.decided.outcome.threshold).
func (r *Rules) FleeThreshold(target Actor, livingEnemies int) (int, error) {
	return r.checks[CheckFlee].Threshold(target, map[string]int{IdentLivingEnemies: livingEnemies})
}

// FleePosition is where a successful escape puts a character.
func (r *Rules) FleePosition(worldID, regionID string) string {
	return strings.NewReplacer(
		"{world_id}", worldID,
		"{region_id}", regionID,
	).Replace(r.doc.Flee.SuccessPosition)
}

// Restore is the hit points a rest leaves an actor with: a full recovery, the
// only rest Load admits (rest.restore).
func (r *Rules) Restore(a Actor) int { return a.HPMax }

// ClampHP holds hit points inside [0, hp_max] — the one invariant the mechanics
// enforce themselves rather than report (inv-02).
func (r *Rules) ClampHP(hp, hpMax int) int { return clampHP(hp, hpMax) }

// clampHP is the arithmetic of inv-02 in one place, for Rules.ClampHP and for
// ChangesFor, which is a function of no rule set.
func clampHP(hp, hpMax int) int { return min(max(hp, 0), hpMax) }

// Excluded says whether the rules keep this actor out of a round: a terminal
// status, or a participation the target rules exclude. A character who walked
// away is as unreachable as a dead one (§5.4, C-02 v1.2).
func (r *Rules) Excluded(a Actor) bool {
	if !a.Alive() {
		return true
	}
	participation := a.Participation
	if participation == "" {
		participation = entity.ParticipationActive
	}
	return slices.Contains(r.doc.NPCTarget.Exclude, participation)
}

// Invariants are the laws in force for this rule set, by identifier, in the
// order of the register (C-03, §5.7).
//
// The laws of a solo fight — inv-01, inv-02, inv-03, inv-09, inv-10 — carry a
// Check (T-054); the laws of a group — inv-04, inv-05, inv-06 — get theirs with
// increment I2 (T-066), and inv-07 and inv-08 never do, because they are decided
// against the journal. A caller must therefore treat a nil Check as "not checked
// here".
func (r *Rules) Invariants() []Invariant { return slices.Clone(r.invariants) }
