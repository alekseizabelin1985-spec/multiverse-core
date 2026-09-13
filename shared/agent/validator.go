package agent

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"math"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Set is a set of names: event types, blueprints, tools, schemas, invariants,
// models.
type Set map[string]struct{}

// NewSet returns a set holding items. It is never nil, so NewSet() is an empty
// set, not a missing one.
func NewSet(items ...string) Set {
	s := make(Set, len(items))
	for _, item := range items {
		s[item] = struct{}{}
	}
	return s
}

// Has reports whether item is in the set; a nil set holds nothing.
func (s Set) Has(item string) bool {
	_, ok := s[item]
	return ok
}

// ValidationEnv is what the validator needs to know about the project. The
// caller builds it (ADR-015 p. 3): the runtime of the swarm and mvctl
// blueprint validate fill it from the same sources, so that both reach the
// same verdict. shared/agent imports neither the contract registry nor a
// context to build it itself.
//
// Every missing part is read as empty, so a check against it fails closed:
// a nil FileExists finds no file, a nil OwnedEntityTypes owns nothing. The one
// exception is Models, see there.
type ValidationEnv struct {
	// EventTypes are the types of the event registry (contracts.All()).
	EventTypes Set
	// OwnedEntityTypes returns the entity types the ownership row of a level
	// covers. The caller builds it from contracts.OwnershipRules, the only
	// source of the ownership table (ADR-025, C-02 v1.4); shared/agent keeps
	// no copy of it.
	OwnedEntityTypes func(level, role string) []string
	// Blueprints are the names of the blueprints known to the project: the
	// targets of parent.name and encounter.child_blueprint.
	Blueprints Set
	// FileExists reports whether a path relative to the project root names a
	// file: laws_ref, rules_ref, absolute_limits_ref.
	FileExists func(path string) bool
	// Tools are the names of the tool registry; empty in MVP-1.
	Tools Set
	// Schemas are the schema files a phase may name in schema_ref, as paths
	// relative to the project root ("schemas/agent/narrative.json").
	Schemas Set
	// Invariants are the checks invariants[].check of a global blueprint may
	// name.
	Invariants Set
	// Models are the models of the selected provider (Provider.Models(), C-11
	// v1.1, SEC-21). nil means the provider is not reachable or the check is
	// off (mvctl --offline): the models are then not checked and the validator
	// says so with an info.
	Models Set
}

// Reasons shared by several rules.
const (
	reasonRequired      = "required"
	reasonModelsSkipped = "models not checked"
	reasonReserved      = "reserved level, spawn disabled"
)

// maxSafeInteger is the largest integer a JSON number carries exactly between
// the platform and its consumers: 2^53-1.
const maxSafeInteger = 1<<53 - 1

// forbiddenModels are too weak for Russian narrative (NFR-071).
var forbiddenModels = []string{"qwen:7b", "qwen:72b"}

var (
	versionRe = regexp.MustCompile(`^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)(\.(0|[1-9][0-9]*))?$`)
	lawsRefRe = regexp.MustCompile(`^laws/([^/@]+)@v([1-9][0-9]*)$`)
)

// Validate checks a parsed blueprint against the project (swarm-llm-laws.md
// §13.2, rules 1–14 and 7a) and returns every finding, the non-fatal ones of
// the parser included. The same code runs in the runtime of the swarm and in
// mvctl blueprint validate (C-11).
//
// The findings are sorted by field, severity and reason, so that one blueprint
// in one environment always gives one report.
func Validate(bp *AgentBlueprint, env ValidationEnv) []Issue {
	if bp == nil {
		return []Issue{{Reason: "no blueprint", Severity: SeverityError}}
	}
	v := &validator{bp: bp, env: env}
	v.issues = append(v.issues, bp.ParseIssues...)
	v.checkIdentity()
	v.checkScopeBinding()
	v.checkParent()
	v.checkTrigger()
	v.checkTTL()
	v.checkInstances()
	v.checkRound()
	v.checkLLM()
	v.checkWhiteLists()
	v.checkTools()
	v.checkRefs()
	v.checkDomain()
	v.checkGlobal()
	v.checkPrompts()
	v.checkReserved()
	v.checkValues()
	slices.SortStableFunc(v.issues, compareIssues)
	return v.issues
}

func compareIssues(a, b Issue) int {
	return cmp.Or(
		cmp.Compare(a.File, b.File),
		compareFields(a.Field, b.Field),
		cmp.Compare(severityRank(a.Severity), severityRank(b.Severity)),
		cmp.Compare(a.Reason, b.Reason),
	)
}

var indexRe = regexp.MustCompile(`\[([0-9]+)\]`)

// compareFields orders fields as text, but their list indices as numbers:
// allowed_event_types[2] comes before allowed_event_types[10]. A field is split
// into text and indices alternately, so that the parts compared at one
// position are always of one kind; equal parts fall back to the plain text,
// which keeps the order total.
func compareFields(a, b string) int {
	pa, pb := fieldParts(a), fieldParts(b)
	for i := 0; i < len(pa) && i < len(pb); i++ {
		if c := compareFieldPart(i, pa[i], pb[i]); c != 0 {
			return c
		}
	}
	return cmp.Or(cmp.Compare(len(pa), len(pb)), cmp.Compare(a, b))
}

// fieldParts returns text, index, text, index, …, text.
func fieldParts(field string) []string {
	var parts []string
	last := 0
	for _, m := range indexRe.FindAllStringSubmatchIndex(field, -1) {
		parts = append(parts, field[last:m[0]], strings.TrimLeft(field[m[2]:m[3]], "0"))
		last = m[1]
	}
	return append(parts, field[last:])
}

func compareFieldPart(i int, a, b string) int {
	if i%2 == 0 {
		return cmp.Compare(a, b)
	}
	// An index without leading zeros: the longer number is the greater one.
	return cmp.Or(cmp.Compare(len(a), len(b)), cmp.Compare(a, b))
}

func severityRank(s Severity) int {
	switch s {
	case SeverityError:
		return 0
	case SeverityWarning:
		return 1
	default:
		return 2
	}
}

type validator struct {
	bp     *AgentBlueprint
	env    ValidationEnv
	issues []Issue
}

func (v *validator) add(severity Severity, field, reason string) {
	v.issues = append(v.issues, Issue{File: v.bp.SourceFile, Field: field, Reason: reason, Severity: severity})
}

func (v *validator) errorf(field, format string, args ...any) {
	v.add(SeverityError, field, fmt.Sprintf(format, args...))
}

func (v *validator) warnf(field, format string, args ...any) {
	v.add(SeverityWarning, field, fmt.Sprintf(format, args...))
}

func (v *validator) level() string { return v.bp.Level }
func (v *validator) role() string  { return v.bp.Role }

// levelKnown and pairKnown gate the rules that depend on the level or on the
// role: when rule 1 has already refused them, a rule built on them would only
// repeat the refusal in other words.
func (v *validator) levelKnown() bool { return IsKnownLevel(v.bp.Level) }

func (v *validator) pairKnown() bool {
	level, ok := RoleLevel(v.bp.Role)
	return ok && level == v.bp.Level
}

// roleIs reports whether the blueprint has one of the roles and the role fits
// its level: a rule that demands something of a role does not trust a role
// rule 1 has refused.
func (v *validator) roleIs(roles ...string) bool {
	return v.pairKnown() && slices.Contains(roles, v.bp.Role)
}

// Rule 1: name, version, level, role and their agreement, locale.
func (v *validator) checkIdentity() {
	bp := v.bp
	if bp.Name == "" {
		v.errorf("name", reasonRequired)
	}
	switch {
	case bp.Version == "":
		v.errorf("version", reasonRequired)
	case !versionRe.MatchString(bp.Version):
		v.errorf("version", "%q is not a version X.Y[.Z]", bp.Version)
	}
	switch {
	case bp.Level == "":
		v.errorf("level", reasonRequired)
	case !IsKnownLevel(bp.Level):
		v.errorf("level", "unknown level %q: want one of %s", bp.Level, strings.Join(LevelNames(), ", "))
	}
	roleLevel, roleKnown := RoleLevel(bp.Role)
	switch {
	case bp.Role == "":
		v.errorf("role", reasonRequired)
	case !roleKnown:
		v.errorf("role", "unknown role %q: want one of %s", bp.Role, strings.Join(RoleNames(), ", "))
	case v.levelKnown() && roleLevel != bp.Level:
		v.errorf("role", "role %q belongs to level %q, not %q", bp.Role, roleLevel, bp.Level)
	}
	if bp.Locale == "" {
		v.errorf("locale", reasonRequired)
	}
}

// Rule 2: the scope types are known and fit the role; global and domain bind
// to one scope by id. A task binds by pattern or by its list of types, and the
// list is required of every level, so a task needs nothing more.
func (v *validator) checkScopeBinding() {
	sb := v.bp.ScopeBinding
	if len(sb.Type) == 0 {
		v.errorf("scope_binding.type", reasonRequired)
	}
	fit := roleScopeTypes(v.role())
	for _, t := range sb.Type {
		switch {
		case !slices.Contains(scopeTypes, t):
			v.errorf("scope_binding.type", "unknown scope type %q: want one of %s", t, strings.Join(ScopeTypeNames(), ", "))
		case v.pairKnown() && fit != nil && !slices.Contains(fit, t):
			v.errorf("scope_binding.type", "scope type %q does not fit role %q: want %s", t, v.role(), strings.Join(fit, " or "))
		}
	}
	if (v.level() == LevelNameGlobal || v.level() == LevelNameDomain) && sb.ID == "" {
		v.errorf("scope_binding.id", "required for level %s", v.level())
	}
}

// Rule 3: every level but global has a parent, which is a known blueprint or,
// for the narrators, the region GM resolved at spawn time.
func (v *validator) checkParent() {
	p := v.bp.Parent
	if v.level() == LevelNameGlobal {
		if p != nil {
			v.errorf("parent", "a global agent has no parent")
		}
		return
	}
	if p == nil {
		if v.levelKnown() {
			v.errorf("parent", "required for level %s", v.level())
		}
		return
	}
	switch p.Instance {
	case "":
		switch {
		case p.Name == "":
			v.errorf("parent.name", reasonRequired)
		case !v.env.Blueprints.Has(p.Name):
			v.errorf("parent.name", "unknown blueprint %q", p.Name)
		}
	case "dynamic":
		if !v.roleIs(RolePersonalGM, RoleGroupNarrator) {
			v.errorf("parent.instance", "instance dynamic is only for roles %s and %s", RolePersonalGM, RoleGroupNarrator)
		}
		if p.Name == "" {
			v.errorf("parent.name", reasonRequired)
		}
	default:
		v.errorf("parent.instance", "unknown instance %q: want dynamic or none", p.Instance)
	}
}

// Rule 4: a timer has its intervals, an event trigger names at least one type
// of the registry (a glob by segments is allowed).
func (v *validator) checkTrigger() {
	tr := v.bp.Trigger
	switch tr.Type {
	case "":
		v.errorf("trigger.type", reasonRequired)
	case "timer":
		var idle, active string
		if tr.Intervals != nil {
			idle, active = tr.Intervals.Idle, tr.Intervals.Active
		}
		if idle == "" {
			v.errorf("trigger.intervals.idle", "required for a timer trigger")
		} else {
			v.checkDuration("trigger.intervals.idle", idle)
		}
		switch {
		case active != "":
			v.checkDuration("trigger.intervals.active", active)
		case v.level() == LevelNameDomain:
			v.errorf("trigger.intervals.active", "required for a timer trigger of level domain")
		}
	case "event":
		if tr.EventName == "" {
			v.errorf("trigger.event_name", "required for an event trigger")
			return
		}
		matched, err := v.matchesRegistry(tr.EventName)
		switch {
		case err != nil:
			v.errorf("trigger.event_name", "%q is not a valid glob", tr.EventName)
		case !matched:
			v.errorf("trigger.event_name", "%q matches no event type of the registry", tr.EventName)
		}
	default:
		v.errorf("trigger.type", "unknown trigger type %q: want timer or event", tr.Type)
	}
}

func (v *validator) matchesRegistry(pattern string) (bool, error) {
	if _, err := MatchEventType(pattern, ""); err != nil {
		return false, err
	}
	for t := range v.env.EventTypes {
		if ok, _ := MatchEventType(pattern, t); ok {
			return true, nil
		}
	}
	return false, nil
}

// MatchEventType reports whether an event type matches a glob by segments
// (C-11 v1.1): the pattern and the type are split at dots, and each segment of
// the pattern matches one segment of the type as path.Match does, so "*" never
// crosses a dot. "player.*" matches player.attacked, not player or
// player.a.b. The error is path.ErrBadPattern for a malformed pattern,
// whatever the type.
func MatchEventType(pattern, eventType string) (bool, error) {
	patternSegments := strings.Split(pattern, ".")
	for _, seg := range patternSegments {
		if _, err := path.Match(seg, ""); err != nil {
			return false, err
		}
	}
	typeSegments := strings.Split(eventType, ".")
	if len(patternSegments) != len(typeSegments) {
		return false, nil
	}
	for i, seg := range patternSegments {
		if ok, _ := path.Match(seg, typeSegments[i]); !ok {
			return false, nil
		}
	}
	return true, nil
}

func (v *validator) checkDuration(field, value string) {
	d, err := time.ParseDuration(value)
	switch {
	case err != nil:
		v.errorf(field, "%q is not a duration", value)
	case d <= 0:
		v.errorf(field, "%q is not a positive duration", value)
	}
}

// Rule 5: global and domain agents never expire, so a ttl there is ignored.
// Elsewhere a ttl has to be a duration, and a task has to have one: it is the
// only stop of a personal GM (swarm-llm-laws.md §4.1; api-contracts.md §3.1).
// A monitor is reserved (rule 14) and its stop is not defined yet, so its ttl
// is not demanded.
func (v *validator) checkTTL() {
	ttl := v.bp.TTL
	if ttl == "" {
		if v.level() == LevelNameTask {
			v.errorf("ttl", "required for level task")
		}
		return
	}
	if v.level() == LevelNameGlobal || v.level() == LevelNameDomain {
		v.warnf("ttl", "ignored for level %s: the agent never expires", v.level())
		return
	}
	v.checkDuration("ttl", ttl)
}

// Rule 6: one instance per scope where the scope has one agent; at least one
// everywhere, since max_instances is required of every level (api-contracts.md
// §3.1) and a missing one reads as 0.
func (v *validator) checkInstances() {
	if v.bp.Constraints.MaxInstances == 1 {
		return
	}
	switch {
	case v.level() == LevelNameGlobal || v.level() == LevelNameDomain:
		v.errorf("constraints.max_instances", "must be 1 for level %s", v.level())
	case v.roleIs(RolePersonalGM, RoleGroupNarrator):
		v.errorf("constraints.max_instances", "must be 1 for role %s", v.role())
	case v.bp.Constraints.MaxInstances < 1:
		v.errorf("constraints.max_instances", "must be at least 1")
	}
}

// The round of a group encounter is read from the blueprint of the encounter
// (api-contracts.md §3.1: round is required of role encounter; it is the source
// of encounter.started.round, C-05).
func (v *validator) checkRound() {
	if !v.roleIs(RoleEncounter) {
		return
	}
	switch {
	case v.bp.Round == nil:
		v.errorf("round", "required for role encounter")
	case v.bp.Round.Timeout != "":
		v.checkDuration("round.timeout", v.bp.Round.Timeout)
	}
}

// phaseView is one phase of llm, whatever its Go type.
type phaseView struct {
	field       string
	rules       bool
	model       string
	temperature *float64
	maxTokens   int
	schemaRef   string
}

// llmPhases returns the phases the blueprint declares; a phase1 in mode rules
// is declared but calls no model.
func (v *validator) llmPhases() []phaseView {
	llm := v.bp.LLM
	var phases []phaseView
	if p := llm.Phase1; p != nil {
		phases = append(phases, phaseView{"llm.phase1", p.Mode == "rules", p.Model, p.Temperature, p.MaxTokens, p.SchemaRef})
	}
	if p := llm.Phase2; p != nil {
		phases = append(phases, phaseView{"llm.phase2", false, p.Model, p.Temperature, p.MaxTokens, p.SchemaRef})
	}
	if p := llm.Tick; p != nil {
		phases = append(phases, phaseView{"llm.tick", false, p.Model, p.Temperature, p.MaxTokens, p.SchemaRef})
	}
	return phases
}

// callsLLM reports whether some phase of the blueprint calls a model.
func (v *validator) callsLLM() bool {
	return slices.ContainsFunc(v.llmPhases(), func(p phaseView) bool { return !p.rules })
}

// Rule 7 and 7a: the phases of the role are there and complete; models are
// allowed and offered by the provider.
func (v *validator) checkLLM() {
	llm := v.bp.LLM
	switch {
	case v.roleIs(RolePersonalGM, RoleGroupNarrator) && llm.Phase2 == nil:
		v.errorf("llm.phase2", "required for role %s", v.role())
	case v.roleIs(RoleGlobalGM, RoleRegionGM, RoleCityGM) && llm.Tick == nil:
		v.errorf("llm.tick", "required for role %s", v.role())
	}
	if p := llm.Phase1; p != nil && p.Mode != "" && p.Mode != "rules" && p.Mode != "llm" {
		v.errorf("llm.phase1.mode", "unknown mode %q: want rules or llm", p.Mode)
	}
	if p := llm.Phase2; p != nil && p.Mode != "" && p.Mode != "llm" {
		v.errorf("llm.phase2.mode", "unknown mode %q: phase 2 always calls a model, only phase 1 has mode rules", p.Mode)
	}
	if p := llm.Tick; p != nil && p.LODDefault != "" && p.LODDefault != LODRuleOnly.String() && p.LODDefault != LODBasic.String() {
		v.errorf("llm.tick.lod_default", "unknown LOD %q: want %s or %s", p.LODDefault, LODRuleOnly, LODBasic)
	}

	phases := v.llmPhases()
	var named []phaseView
	for _, p := range phases {
		if p.rules {
			continue
		}
		v.checkPhase(p)
		// A forbidden model is refused already; asking the provider about it
		// would only report it twice.
		if p.model != "" && !slices.Contains(forbiddenModels, p.model) {
			named = append(named, p)
		}
	}
	if len(phases) > 0 {
		switch llm.Fallback {
		case "":
			v.errorf("llm.fallback", reasonRequired)
		case "template", "rules":
		default:
			v.errorf("llm.fallback", "unknown fallback %q: want template or rules", llm.Fallback)
		}
	}
	if llm.Retries != nil && *llm.Retries < 0 {
		v.errorf("llm.retries", "must not be negative")
	}

	if len(named) == 0 {
		return
	}
	if v.env.Models == nil {
		v.add(SeverityInfo, "llm", reasonModelsSkipped)
		return
	}
	for _, p := range named {
		if !v.env.Models.Has(p.model) {
			v.errorf(p.field+".model", "model %q is not offered by the provider", p.model)
		}
	}
}

func (v *validator) checkPhase(p phaseView) {
	switch {
	case p.model == "":
		v.errorf(p.field+".model", reasonRequired)
	case slices.Contains(forbiddenModels, p.model):
		v.errorf(p.field+".model", "model %q is not allowed (NFR-071)", p.model)
	}
	if t := p.temperature; t != nil && (math.IsNaN(*t) || *t < 0 || *t > 2) {
		v.errorf(p.field+".temperature", "%v is outside [0, 2]", *t)
	}
	if p.maxTokens <= 0 {
		v.errorf(p.field+".max_tokens", "must be greater than 0")
	}
	switch {
	case p.schemaRef == "":
		v.errorf(p.field+".schema_ref", reasonRequired)
	case !v.env.Schemas.Has(p.schemaRef):
		v.errorf(p.field+".schema_ref", "unknown schema %q", p.schemaRef)
	}
}

// Rule 8: what the blueprint publishes is in the registry and in the white
// list of its role; what it owns is in the ownership row of its level, as the
// caller built it from contracts.OwnershipRules.
//
// A role whose white list proposes no entity change (the narrators, the
// reserved roles) owns nothing, whatever the row of its level covers: the row
// of task is written for the encounter. The white list of the role is taken,
// not the one of the blueprint, because an encounter changes entities through
// the mechanics without listing entity.update.proposed itself.
func (v *validator) checkWhiteLists() {
	allowed := AllowedEventTypes(v.level(), v.role())
	for i, t := range v.bp.AllowedEventTypes {
		field := fmt.Sprintf("allowed_event_types[%d]", i)
		switch {
		case !v.env.EventTypes.Has(t):
			v.errorf(field, "%q is not a type of the registry", t)
		case v.pairKnown() && !slices.Contains(allowed, t):
			v.errorf(field, "%q is not allowed for role %s of level %s", t, v.role(), v.level())
		}
	}
	if !v.pairKnown() {
		return
	}
	proposes := slices.ContainsFunc(allowed, func(t string) bool {
		ok, _ := MatchEventType("entity.*.proposed", t)
		return ok
	})
	var owned []string
	if v.env.OwnedEntityTypes != nil {
		owned = v.env.OwnedEntityTypes(v.level(), v.role())
	}
	for i, t := range v.bp.OwnedEntityTypes {
		field := fmt.Sprintf("owned_entity_types[%d]", i)
		switch {
		case !proposes:
			v.errorf(field, "entity type %q: role %s proposes no entity changes", t, v.role())
		case !slices.Contains(owned, t):
			v.errorf(field, "entity type %q is outside the ownership row of level %s", t, v.level())
		}
	}
}

// Rule 9: tools are registered; the registry of MVP-1 is empty.
func (v *validator) checkTools() {
	for i, tool := range v.bp.Tools {
		field := fmt.Sprintf("tools[%d].name", i)
		switch {
		case tool.Name == "":
			v.errorf(field, reasonRequired)
		case !v.env.Tools.Has(tool.Name):
			v.errorf(field, "tool %q is not registered", tool.Name)
		}
	}
}

// Rule 10: the laws, the rules and the absolute limits a level needs exist.
func (v *validator) checkRefs() {
	bp := v.bp
	if v.level() == LevelNameGlobal || v.level() == LevelNameDomain {
		v.checkLawsRef(bp.LawsRef)
	}
	switch {
	case v.level() == LevelNameDomain:
		v.checkRef("rules_ref", bp.RulesRef, "level domain")
	case v.roleIs(RoleEncounter):
		v.checkRef("rules_ref", bp.RulesRef, "role encounter")
	}
	if v.roleIs(RolePersonalGM, RoleGroupNarrator, RoleEncounter) {
		v.checkRef("absolute_limits_ref", bp.AbsoluteLimitsRef, "role "+v.role())
	}
}

// checkRef checks a reference that is a file path relative to the project.
func (v *validator) checkRef(field, ref, who string) {
	if ref == "" {
		v.errorf(field, "required for %s", who)
		return
	}
	if v.env.FileExists == nil || !v.env.FileExists(ref) {
		v.errorf(field, "file %s does not exist", ref)
	}
}

// checkLawsRef checks a versioned laws reference laws/<world>@vN
// (api-contracts.md §3.1, swarm-llm-laws.md §12.1). Its version is part of the
// file name: laws/dark-forest-world@v1 is laws/dark-forest-world.v1.yaml
// (FileSource, §12.2). A plain file path is not a laws reference.
func (v *validator) checkLawsRef(ref string) {
	if ref == "" {
		v.errorf("laws_ref", "required for level %s", v.level())
		return
	}
	m := lawsRefRe.FindStringSubmatch(ref)
	if m == nil {
		v.errorf("laws_ref", "%q is not a laws reference laws/<world>@vN", ref)
		return
	}
	file := "laws/" + m[1] + ".v" + m[2] + ".yaml"
	if v.env.FileExists == nil || !v.env.FileExists(file) {
		v.errorf("laws_ref", "%q: file %s does not exist", ref, file)
	}
}

// Rule 11: a region is described in full by its blueprint.
func (v *validator) checkDomain() {
	if v.level() != LevelNameDomain {
		return
	}
	bp := v.bp
	if len(bp.NPCTable) == 0 {
		v.errorf("npc_table", "required for level domain: at least one row")
	}
	if bp.RespawnTTL == "" {
		v.errorf("respawn_ttl", "required for level domain")
	} else {
		v.checkDuration("respawn_ttl", bp.RespawnTTL)
	}
	switch {
	case bp.Encounter == nil:
		v.errorf("encounter", "required for level domain")
	case bp.Encounter.ChildBlueprint == "":
		v.errorf("encounter.child_blueprint", reasonRequired)
	case !v.env.Blueprints.Has(bp.Encounter.ChildBlueprint):
		v.errorf("encounter.child_blueprint", "unknown blueprint %q", bp.Encounter.ChildBlueprint)
	}
	if len(bp.BackgroundEvents) == 0 {
		v.errorf("background_events", "required for level domain: at least one event")
	}
	if bp.Prompts.Description == "" {
		v.errorf(v.sectionField(SectionDescription), "required for level domain")
	}
}

// Rule 12: the world has a background budget, background events to spend it
// on (api-contracts.md §3.1: background_events is required of global and
// domain) and invariants the project knows.
func (v *validator) checkGlobal() {
	if v.level() != LevelNameGlobal {
		return
	}
	bp := v.bp
	if bp.Budget == nil || bp.Budget.BackgroundCallsPerHourWorld <= 0 {
		v.errorf("budget.background_calls_per_hour_world", "must be greater than 0 for level global")
	}
	if len(bp.BackgroundEvents) == 0 {
		v.errorf("background_events", "required for level global: at least one event")
	}
	if len(bp.Invariants) == 0 {
		v.errorf("invariants", "required for level global: at least one invariant")
	}
	for i, inv := range bp.Invariants {
		field := fmt.Sprintf("invariants[%d].check", i)
		switch {
		case inv.Check == "":
			v.errorf(field, reasonRequired)
		case !v.env.Invariants.Has(inv.Check):
			v.errorf(field, "unknown invariant check %q", inv.Check)
		}
	}
}

// sectionField names a prompt the way the file wrote it: a section of a .md
// file, a key under prompts of a YAML file.
func (v *validator) sectionField(section string) string {
	switch strings.ToLower(filepath.Ext(v.bp.SourceFile)) {
	case ".yaml", ".yml":
		return "prompts." + section
	}
	return "## " + section
}

// Rule 13: placeholders come from the dictionary, and a role that calls a
// model has a system prompt. A name in the exact form outside the dictionary
// is an error; something that only looks like a placeholder ({Player.name},
// { x }, {игрок}) is a warning, because braces also occur in prose and
// formulas (\frac{A}{B}).
func (v *validator) checkPrompts() {
	p := v.bp.Prompts
	texts := []struct{ section, text string }{
		{SectionSystem, p.System},
		{SectionPhase1, p.Phase1},
		{SectionPhase2, p.Phase2},
		{SectionTick, p.Tick},
		{SectionDescription, p.Description},
	}
	for _, item := range p.Canon {
		texts = append(texts, struct{ section, text string }{SectionCanon, item})
	}
	for _, t := range texts {
		field := v.sectionField(t.section)
		for _, name := range UnknownPlaceholders(t.text) {
			v.errorf(field, "unknown placeholder {%s}", name)
		}
		for _, fragment := range SuspiciousPlaceholders(t.text) {
			v.warnf(field, "%s looks like a placeholder but is not one: the prompt goes to the model with it as is", fragment)
		}
	}
	if p.System == "" && v.callsLLM() {
		v.errorf(v.sectionField(SectionSystem), "required: the blueprint calls a model")
	}
}

// Rule 14: a reserved level is valid, and nothing of it is spawned.
func (v *validator) checkReserved() {
	if IsReservedLevel(v.level()) {
		v.add(SeverityInfo, "level", reasonReserved)
	}
}

// checkValues warns about untyped values State refuses only when it applies
// them (C-02 v1.6): a number JSON does not carry exactly, and an unquoted YAML
// date, which decodes to a timestamp and reaches the event as a string.
func (v *validator) checkValues() {
	for i, c := range v.bp.Trigger.Conditions {
		v.checkValue(fmt.Sprintf("trigger.conditions[%d].value", i), c.Value)
	}
	for i, ev := range v.bp.BackgroundEvents {
		for j, op := range ev.Ops {
			v.checkValue(fmt.Sprintf("background_events[%d].ops[%d].value", i, j), op.Value)
		}
	}
}

func (v *validator) checkValue(field string, value any) {
	switch x := value.(type) {
	case time.Time:
		v.warnf(field, "an unquoted YAML date is a timestamp and reaches JSON as a string: quote it")
	case int:
		v.checkInteger(field, x >= -maxSafeInteger && x <= maxSafeInteger, x)
	case int64:
		v.checkInteger(field, x >= -maxSafeInteger && x <= maxSafeInteger, x)
	case uint64:
		v.checkInteger(field, x <= maxSafeInteger, x)
	case float64:
		switch {
		case math.IsNaN(x) || math.IsInf(x, 0):
			v.warnf(field, "%v is not a JSON number", x)
		case math.Abs(x) > maxSafeInteger:
			v.checkInteger(field, false, x)
		}
	case []any:
		for i, item := range x {
			v.checkValue(fmt.Sprintf("%s[%d]", field, i), item)
		}
	case map[string]any:
		for _, key := range sortedKeys(x) {
			v.checkValue(field+"."+key, x[key])
		}
	case map[any]any:
		keys := make(map[string]any, len(x))
		for key, item := range x {
			keys[fmt.Sprint(key)] = item
		}
		for _, key := range sortedKeys(keys) {
			v.checkValue(field+"."+key, keys[key])
		}
	}
}

func (v *validator) checkInteger(field string, safe bool, x any) {
	if !safe {
		v.warnf(field, "%v is outside +-(2^53-1): State refuses it when applying", x)
	}
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// EnvFromProject builds the environment of the validator from a project
// checkout: the blueprints under root/blueprints, the schemas under
// root/schemas/agent, the files under root. The rest is what only the caller
// can know without an import shared/agent must not have: the types of the
// event registry, the ownership view built from contracts.OwnershipRules, the
// invariant checks and the models of the provider (nil: not checked). The
// tool registry of MVP-1 is empty.
//
// A blueprint that does not parse gives no name here (a directory named like
// a blueprint does not parse either); validating the directory reports it on
// its own.
func EnvFromProject(root string, eventTypes []string, ownedEntityTypes func(level, role string) []string, invariants, models []string) (ValidationEnv, error) {
	env := ValidationEnv{
		EventTypes:       NewSet(eventTypes...),
		OwnedEntityTypes: ownedEntityTypes,
		Blueprints:       NewSet(),
		FileExists:       projectFileExists(root),
		Tools:            NewSet(),
		Schemas:          NewSet(),
		Invariants:       NewSet(invariants...),
	}
	if models != nil {
		env.Models = NewSet(models...)
	}

	blueprintDir := filepath.Join(root, "blueprints")
	present, err := optionalDir(blueprintDir)
	if err != nil {
		return ValidationEnv{}, fmt.Errorf("read blueprints: %w", err)
	}
	var entries []fs.DirEntry
	if present {
		if entries, err = os.ReadDir(blueprintDir); err != nil {
			return ValidationEnv{}, fmt.Errorf("read blueprints: %w", err)
		}
	}
	for _, entry := range entries {
		switch strings.ToLower(filepath.Ext(entry.Name())) {
		case ".md", ".yaml", ".yml":
		default:
			continue
		}
		if bp, err := ParseFile(filepath.Join(root, "blueprints", entry.Name())); err == nil && bp.Name != "" {
			env.Blueprints[bp.Name] = struct{}{}
		}
	}

	schemaDir := filepath.Join(root, "schemas", "agent")
	present, err = optionalDir(schemaDir)
	if err != nil {
		return ValidationEnv{}, fmt.Errorf("read schemas: %w", err)
	}
	if !present {
		return env, nil
	}
	err = filepath.WalkDir(schemaDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.Type().IsRegular() && strings.EqualFold(filepath.Ext(p), ".json") {
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			env.Schemas[filepath.ToSlash(rel)] = struct{}{}
		}
		return nil
	})
	if err != nil {
		return ValidationEnv{}, fmt.Errorf("read schemas: %w", err)
	}
	return env, nil
}

// optionalDir reports whether dir is there. A project without it yet is an
// empty one; something else in its place is an error, on every platform alike
// (reading a file as a directory fails differently on Windows and Linux).
func optionalDir(dir string) (bool, error) {
	info, err := os.Stat(dir)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	case err != nil:
		return false, err
	case !info.IsDir():
		return false, fmt.Errorf("%s is not a directory", dir)
	}
	return true, nil
}

// projectFileExists finds regular files whose path is lexically under root: a
// reference that is absolute or climbs out of the project with .. names no
// file of it. Symbolic links are followed by os.Stat, also out of root; the
// three readers of the project (blueprints, schemas, files) are to be aligned
// on links together with mvctl blueprint validate (T-204).
func projectFileExists(root string) func(string) bool {
	return func(rel string) bool {
		local := filepath.FromSlash(rel)
		if !filepath.IsLocal(local) {
			return false
		}
		info, err := os.Stat(filepath.Join(root, local))
		return err == nil && info.Mode().IsRegular()
	}
}
