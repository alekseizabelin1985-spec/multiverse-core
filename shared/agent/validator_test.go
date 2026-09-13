package agent_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/contracts"
)

const invalidDir = "testdata/blueprints/invalid"

const testModel = "Qwen3.8-27B-UD-Q3_K_XL"

// ownedFromContracts is the view of the ownership table a caller builds for
// the validator: the entity types of the rows of a level. It is built here,
// in the test, from the only source of the table (ADR-025).
func ownedFromContracts(level, _ string) []string {
	var owned []string
	for _, rule := range contracts.OwnershipRules() {
		if rule.Proposer == level {
			owned = append(owned, rule.EntityTypes...)
		}
	}
	return owned
}

func registryTypes() []string {
	var types []string
	for _, spec := range contracts.All() {
		types = append(types, spec.Type)
	}
	return types
}

// testEnv is the project the corpus is written for: the valid corpus passes
// it without an error, and each invalid file breaks exactly its rule.
func testEnv() agent.ValidationEnv {
	files := agent.NewSet("laws/dark-forest-world.v1.yaml", "rules/dark-forest.yaml", "config/absolute-limits.yaml")
	return agent.ValidationEnv{
		EventTypes:       agent.NewSet(registryTypes()...),
		OwnedEntityTypes: ownedFromContracts,
		Blueprints: agent.NewSet("global-test-world", "domain-test-forest", "domain-test-fair",
			"encounter-test-wolf", "player-test-gm", "group-test-narrator"),
		FileExists: files.Has,
		Tools:      agent.NewSet(),
		Schemas:    agent.NewSet("schemas/agent/narrative.json", "schemas/agent/tick-global.json", "schemas/agent/tick-region.json"),
		Invariants: agent.NewSet("dead_does_not_act", "hp_within_bounds"),
		Models:     agent.NewSet(testModel),
	}
}

// render writes an issue as "severity|field|reason" for comparison.
func render(issues []agent.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, string(issue.Severity)+"|"+issue.Field+"|"+issue.Reason)
	}
	return out
}

func assertIssues(t *testing.T, got []agent.Issue, want []string) {
	t.Helper()
	if want == nil {
		want = []string{}
	}
	if rendered := render(got); !slices.Equal(rendered, want) {
		t.Errorf("issues:\n  got  %q\n  want %q", rendered, want)
	}
}

// issuesOfFile parses and validates a file; a file the parser refuses gives
// its parse errors as issues, the way mvctl blueprint validate reports them.
func issuesOfFile(t *testing.T, path string, env agent.ValidationEnv) []agent.Issue {
	t.Helper()
	bp, err := agent.ParseFile(path)
	if err == nil {
		return agent.Validate(bp, env)
	}
	var issues []agent.Issue
	for _, e := range unjoin(err) {
		var pe *agent.ParseError
		if !errors.As(e, &pe) {
			t.Fatalf("%s: not a parse error: %v", path, e)
		}
		issues = append(issues, agent.Issue{File: pe.File, Field: pe.Field, Reason: pe.Reason, Severity: agent.SeverityError})
	}
	return issues
}

func unjoin(err error) []error {
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		return joined.Unwrap()
	}
	return []error{err}
}

// One invalid file per rule; the table names what each one breaks.
func TestInvalidCorpus(t *testing.T) {
	cases := []struct {
		file string
		want []string
	}{
		{"rule-01-identity.md", []string{
			`error|role|role "region-gm" belongs to level "domain", not "task"`,
			`error|version|"v1" is not a version X.Y[.Z]`,
		}},
		{"rule-02-scope-binding.md", []string{
			`error|scope_binding.type|scope type "group" does not fit role "personal-gm": want solo`,
			`error|scope_binding.type|unknown scope type "forest": want one of group, region, solo, world`,
		}},
		{"rule-03-parent.md", []string{`error|parent.name|unknown blueprint "domain-test-swamp"`}},
		{"rule-04-trigger.md", []string{`error|trigger.intervals.idle|required for a timer trigger`}},
		{"rule-05-ttl.md", []string{`warning|ttl|ignored for level global: the agent never expires`}},
		{"rule-05-ttl-required.md", []string{`error|ttl|required for level task`}},
		{"rule-06-instances.md", []string{`error|constraints.max_instances|must be 1 for role personal-gm`}},
		{"rule-06-instances-at-least-one.md", []string{`error|constraints.max_instances|must be at least 1`}},
		{"rule-07-llm.md", []string{
			`error|llm.fallback|required`,
			`error|llm.phase2.max_tokens|must be greater than 0`,
			`error|llm.phase2.model|model "qwen:7b" is not allowed (NFR-071)`,
			`error|llm.phase2.schema_ref|unknown schema "schemas/agent/narrative-v9.json"`,
			`error|llm.phase2.temperature|2.5 is outside [0, 2]`,
		}},
		{"rule-07a-models.md", []string{`error|llm.phase2.model|model "qwen3:30b-a3b" is not offered by the provider`}},
		{"rule-07a-provider.md", []string{`error|llm.phase2.provider|unknown field`}},
		{"rule-08-white-lists.md", []string{
			`error|allowed_event_types[1]|"world.weather_changed" is not allowed for role personal-gm of level task`,
			`error|allowed_event_types[2]|"player.teleported" is not a type of the registry`,
			`error|owned_entity_types[0]|entity type "world": role personal-gm proposes no entity changes`,
		}},
		{"rule-08-ownership-row.md", []string{
			`error|owned_entity_types[1]|entity type "world" is outside the ownership row of level task`,
		}},
		{"rule-09-tools.md", []string{`error|tools[0].name|tool "dice_roller" is not registered`}},
		{"rule-10-refs.md", []string{
			`error|absolute_limits_ref|required for role encounter`,
			`error|rules_ref|file rules/swamp.yaml does not exist`,
		}},
		{"rule-11-domain.md", []string{
			`error|## description|required for level domain`,
			`error|encounter.child_blueprint|unknown blueprint "encounter-test-bear"`,
			`error|npc_table|required for level domain: at least one row`,
		}},
		{"rule-12-global.md", []string{
			`error|background_events|required for level global: at least one event`,
			`error|budget.background_calls_per_hour_world|must be greater than 0 for level global`,
			`error|invariants[1].check|unknown invariant check "hp_never_negative"`,
		}},
		{"rule-13-placeholders.md", []string{
			`error|## phase2|unknown placeholder {player.mood}`,
			`warning|## phase2|{Player.name} looks like a placeholder but is not one: the prompt goes to the model with it as is`,
			`error|## system|required: the blueprint calls a model`,
		}},
		{"rule-14-reserved.md", []string{`info|level|reserved level, spawn disabled`}},
		// Required fields of api-contracts.md §3.1 outside the numbered rules.
		{"required-round.md", []string{`error|round|required for role encounter`}},
	}
	covered := map[string]bool{}
	for _, tc := range cases {
		t.Run(tc.file, func(t *testing.T) {
			path := filepath.Join(invalidDir, tc.file)
			issues := issuesOfFile(t, path, testEnv())
			assertIssues(t, issues, tc.want)
			for _, issue := range issues {
				if issue.File != path {
					t.Errorf("issue %q names file %q, want %q", issue.Field, issue.File, path)
				}
			}
		})
		covered[tc.file] = true
	}
	files, err := filepath.Glob(filepath.Join(invalidDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if !covered[filepath.Base(f)] {
			t.Errorf("%s is in the corpus but not in the table", f)
		}
	}
	for rule := 1; rule <= 14; rule++ {
		prefix := fmt.Sprintf("rule-%02d-", rule)
		if !slices.ContainsFunc(files, func(f string) bool { return strings.HasPrefix(filepath.Base(f), prefix) }) {
			t.Errorf("no invalid file for rule %d", rule)
		}
	}
}

// The valid corpus has no error; the dates of domain-fair.md are warnings.
func TestValidCorpus(t *testing.T) {
	want := map[string][]string{
		"domain-fair.md": {
			`warning|background_events[0].ops[0].value|an unquoted YAML date is a timestamp and reaches JSON as a string: quote it`,
			`warning|background_events[0].ops[1].value|an unquoted YAML date is a timestamp and reaches JSON as a string: quote it`,
			`warning|trigger.conditions[0].value|an unquoted YAML date is a timestamp and reaches JSON as a string: quote it`,
		},
	}
	files, err := filepath.Glob(filepath.Join(validDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("empty valid corpus")
	}
	for _, f := range files {
		t.Run(filepath.Base(f), func(t *testing.T) {
			assertIssues(t, issuesOfFile(t, f, testEnv()), want[filepath.Base(f)])
		})
	}
}

// T-416: the ownership view comes from contracts.OwnershipRules through the
// environment. A type outside the row of the level is an error; the same type
// inside the row of another level is not borrowed from it.
func TestOwnedEntityTypesFollowTheRowOfTheLevel(t *testing.T) {
	bp := parseValid(t, "encounter.md")
	assertIssues(t, agent.Validate(bp, testEnv()), nil)

	bp.OwnedEntityTypes = []string{"encounter", "region", "world"}
	assertIssues(t, agent.Validate(bp, testEnv()), []string{
		`error|owned_entity_types[1]|entity type "region" is outside the ownership row of level task`,
		`error|owned_entity_types[2]|entity type "world" is outside the ownership row of level task`,
	})

	domain := parseValid(t, "domain-region.md")
	domain.OwnedEntityTypes = append(domain.OwnedEntityTypes, "player")
	assertIssues(t, agent.Validate(domain, testEnv()), []string{
		`error|owned_entity_types[3]|entity type "player" is outside the ownership row of level domain`,
	})

	// Without a view nothing is owned: the check fails closed.
	env := testEnv()
	env.OwnedEntityTypes = nil
	assertIssues(t, agent.Validate(parseValid(t, "global-world.md"), env), []string{
		`error|owned_entity_types[0]|entity type "world" is outside the ownership row of level global`,
	})
}

// ADR-015 p. 3, ADR-025: shared/agent does not import the contract registry,
// and as a library layer no context either. Test imports do not count: this
// file imports shared/contracts to build the environment, as a caller would.
func TestPackageDoesNotImportContractsOrContexts(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("go is not on PATH: %v", err)
	}
	out, err := exec.Command(goBin, "list", "-deps", "./...").CombinedOutput()
	if err != nil {
		t.Fatalf("go list -deps: %v\n%s", err, out)
	}
	deps := strings.Fields(string(out))
	if !slices.Contains(deps, "multiverse-core.io/shared/agent") {
		t.Fatalf("go list -deps did not list the package itself: %q", deps)
	}
	for _, dep := range deps {
		if dep == "multiverse-core.io/shared/contracts" || strings.HasPrefix(dep, "multiverse-core.io/shared/contracts/") ||
			dep == "multiverse-core.io/internal" || strings.HasPrefix(dep, "multiverse-core.io/internal/") {
			t.Errorf("shared/agent depends on %s", dep)
		}
	}
}

// Rule 7a: without the models of the provider the check is skipped and said
// to be skipped; an empty list is a provider without models, not a skip.
func TestModelsNotChecked(t *testing.T) {
	env := testEnv()
	env.Models = nil
	assertIssues(t, agent.Validate(parseValid(t, "personal-gm.md"), env), []string{`info|llm|models not checked`})
	bp, err := agent.ParseFile(filepath.Join(invalidDir, "rule-07a-models.md"))
	if err != nil {
		t.Fatal(err)
	}
	assertIssues(t, agent.Validate(bp, env), []string{`info|llm|models not checked`})

	// A blueprint that names no model has nothing left unchecked.
	assertIssues(t, agent.Validate(parseValid(t, "encounter.md"), env), nil)

	env.Models = agent.NewSet()
	assertIssues(t, agent.Validate(parseValid(t, "personal-gm.md"), env), []string{
		`error|llm.phase2.model|model "Qwen3.8-27B-UD-Q3_K_XL" is not offered by the provider`,
	})
}

// Decision 1 of swarm-llm-laws.md §13.2: a caller tells findings apart by a
// stable code, not by the text of Reason. Only the findings someone acts on
// carry one; a reason edited in a later version keeps its code.
func TestIssueCodes(t *testing.T) {
	codes := func(issues []agent.Issue) []string {
		out := make([]string, 0, len(issues))
		for _, issue := range issues {
			out = append(out, issue.Field+"="+issue.Code)
		}
		return out
	}
	cases := []struct {
		name   string
		file   string
		change func(*agent.AgentBlueprint)
		env    func(*agent.ValidationEnv)
		want   []string
	}{
		{"model outside the provider", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Model = "qwen3:30b-a3b" }, nil,
			[]string{"llm.phase2.model=model_missing"}},
		{"model of a tick outside the provider", "global-world.md", nil, func(env *agent.ValidationEnv) { env.Models = agent.NewSet("other") },
			[]string{"llm.tick.model=model_missing"}},
		{"models not checked", "personal-gm.md", nil, func(env *agent.ValidationEnv) { env.Models = nil },
			[]string{"llm=models_not_checked"}},
		// A forbidden model is rule 7, not 7a: the runtime must not lower it.
		{"forbidden model has no code", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Model = "qwen:7b" }, nil,
			[]string{"llm.phase2.model="}},
		{"missing file", "encounter.md", nil, func(env *agent.ValidationEnv) { env.FileExists = nil },
			[]string{"absolute_limits_ref=file_missing", "rules_ref=file_missing"}},
		{"missing laws file", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "laws/dark-forest-world@v2" }, nil,
			[]string{"laws_ref=file_missing"}},
		// A reference that is absent or of the wrong form is not a missing file.
		{"required reference has no code", "encounter.md", func(bp *agent.AgentBlueprint) { bp.RulesRef = "" }, nil,
			[]string{"rules_ref="}},
		{"malformed laws reference has no code", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "laws/x.yaml" }, nil,
			[]string{"laws_ref="}},
		{"reserved role has no code", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.Role, bp.AllowedEventTypes, bp.OwnedEntityTypes = "city-gm", nil, nil
		}, nil, []string{"role="}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bp := parseValid(t, tc.file)
			if tc.change != nil {
				tc.change(bp)
			}
			env := testEnv()
			if tc.env != nil {
				tc.env(&env)
			}
			if got := codes(agent.Validate(bp, env)); !slices.Equal(got, tc.want) {
				t.Errorf("codes:\n  got  %q\n  want %q", got, tc.want)
			}
		})
	}
	if agent.CodeModelMissing != "model_missing" || agent.CodeModelsNotChecked != "models_not_checked" || agent.CodeFileMissing != "file_missing" {
		t.Error("a code changed its value: codes are stable between versions")
	}
}

func TestValidateNil(t *testing.T) {
	assertIssues(t, agent.Validate(nil, testEnv()), []string{"error||no blueprint"})
}

// The warnings of the parser (migrations of the as-is format) are part of the
// report of the validator.
func TestValidateReportsParseIssues(t *testing.T) {
	data, err := os.ReadFile(filepath.Join(validDir, "personal-gm.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.Replace(text, "role: personal-gm\n", "type: personal-gm\n", 1)
	bp, err := agent.ParseBytes("legacy.md", []byte(text))
	if err != nil {
		t.Fatal(err)
	}
	assertIssues(t, agent.Validate(bp, testEnv()), nil)
	bp.ParseIssues = []agent.Issue{{File: "legacy.md", Field: "llm.model", Reason: "moved", Severity: agent.SeverityWarning}}
	assertIssues(t, agent.Validate(bp, testEnv()), []string{"warning|llm.model|moved"})

	// Within one field an error comes first, then a warning, then an info,
	// whatever their reasons say.
	bp.Name = ""
	bp.ParseIssues = []agent.Issue{
		{File: "legacy.md", Field: "name", Reason: "0 info", Severity: agent.SeverityInfo},
		{File: "legacy.md", Field: "name", Reason: "1 warning", Severity: agent.SeverityWarning},
	}
	assertIssues(t, agent.Validate(bp, testEnv()), []string{"error|name|required", "warning|name|1 warning", "info|name|0 info"})
}

// One blueprint in one environment gives one report, in one order.
func TestValidateIsDeterministic(t *testing.T) {
	bp := parseValid(t, "domain-region.md")
	bp.Name, bp.Version, bp.Locale = "", "x", ""
	bp.AllowedEventTypes = []string{"z.z", "a.a", "world.weather_changed", "narrative.output"}
	bp.OwnedEntityTypes = []string{"world", "player"}
	bp.Tools = []agent.ToolReference{{Name: "b"}, {Name: "a"}}
	bp.Prompts.Tick = "{b.b} {a.a} {Z} {Y}"
	first := render(agent.Validate(bp, testEnv()))
	if len(first) < 10 {
		t.Fatalf("the case is too small to test the order: %q", first)
	}
	for i := 0; i < 50; i++ {
		if got := render(agent.Validate(bp, testEnv())); !slices.Equal(got, first) {
			t.Fatalf("run %d:\n  got  %q\n  want %q", i, got, first)
		}
	}
	issues := agent.Validate(bp, testEnv())
	for i := 1; i < len(issues); i++ {
		a, b := issues[i-1], issues[i]
		if a.Field > b.Field {
			t.Errorf("issues are not sorted by field: %q before %q", a.Field, b.Field)
		}
	}
}

// List indices are ordered as numbers: a report of twelve items reads [0]…[11],
// not [0], [1], [10], [11], [2], …
func TestIssuesSortIndicesAsNumbers(t *testing.T) {
	bp := parseValid(t, "personal-gm.md")
	bp.AllowedEventTypes = nil
	var want []string
	for i := 0; i < 12; i++ {
		bp.AllowedEventTypes = append(bp.AllowedEventTypes, fmt.Sprintf("x.t%02d", i))
		want = append(want, fmt.Sprintf(`error|allowed_event_types[%d]|"x.t%02d" is not a type of the registry`, i, i))
	}
	assertIssues(t, agent.Validate(bp, testEnv()), want)

	// Text and indices alternate; equal numbers fall back to the text.
	bp = parseValid(t, "personal-gm.md")
	for _, field := range []string{"a.b", "a[10]", "a[2]", "a[1].b[10]", "a", "a[02]", "a[1].b[9]"} {
		bp.ParseIssues = append(bp.ParseIssues, agent.Issue{Field: field, Reason: "r", Severity: agent.SeverityInfo})
	}
	var got []string
	for _, issue := range agent.Validate(bp, testEnv()) {
		got = append(got, issue.Field)
	}
	if want := []string{"a", "a[1].b[9]", "a[1].b[10]", "a[02]", "a[2]", "a[10]", "a.b"}; !slices.Equal(got, want) {
		t.Errorf("fields:\n  got  %q\n  want %q", got, want)
	}
}

func TestSet(t *testing.T) {
	var missing agent.Set
	if missing.Has("x") {
		t.Error("a nil set holds nothing")
	}
	empty := agent.NewSet()
	if empty == nil || empty.Has("") {
		t.Error("NewSet() is an empty, non-nil set")
	}
	if s := agent.NewSet("a", "b"); !s.Has("a") || !s.Has("b") || s.Has("c") {
		t.Errorf("NewSet(a, b) = %v", s)
	}
}

func TestMatchEventType(t *testing.T) {
	cases := []struct {
		pattern, eventType string
		want               bool
	}{
		{"player.*", "player.attacked", true},
		{"player.*", "player", false},
		{"player.*", "player.a.b", false},
		{"*.attacked", "player.attacked", true},
		{"*", "player.attacked", false},
		{"player.att*", "player.attacked", true},
		{"player.?ttacked", "player.attacked", true},
		{"world.laws.*", "world.laws.changed", true},
		{"entity.*.proposed", "entity.update.proposed", true},
		{"entity.*.proposed", "entity.updated", false},
		{"group.created", "group.created", true},
		{"group.created", "group.joined", false},
	}
	for _, tc := range cases {
		got, err := agent.MatchEventType(tc.pattern, tc.eventType)
		if err != nil || got != tc.want {
			t.Errorf("MatchEventType(%q, %q) = %v, %v; want %v", tc.pattern, tc.eventType, got, err, tc.want)
		}
	}
	// A malformed pattern is an error whatever the type, even one of another
	// number of segments.
	for _, eventType := range []string{"player.attacked", "player", ""} {
		if _, err := agent.MatchEventType("player.[", eventType); err == nil {
			t.Errorf("MatchEventType(player.[, %q): want an error", eventType)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// The rules case by case, beyond the one file per rule of the corpus.
func TestRules(t *testing.T) {
	cases := []struct {
		name   string
		file   string
		change func(*agent.AgentBlueprint)
		env    func(*agent.ValidationEnv)
		want   []string
	}{
		// Rule 1.
		{"identity required", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Name, bp.Version, bp.Locale = "", "", "" }, nil, []string{
			"error|locale|required", "error|name|required", "error|version|required",
		}},
		{"versions", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Version = "1.0.1" }, nil, nil},
		{"version with a leading zero", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Version = "01.0" }, nil, []string{
			`error|version|"01.0" is not a version X.Y[.Z]`,
		}},
		{"version of four parts", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Version = "1.0.0.1" }, nil, []string{
			`error|version|"1.0.0.1" is not a version X.Y[.Z]`,
		}},
		{"level and role required", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Level, bp.Role = "", "" }, nil, []string{
			"error|level|required", "error|role|required",
		}},
		{"level and role unknown", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Level, bp.Role = "quest", "bard" }, nil, []string{
			`error|level|unknown level "quest": want one of domain, global, monitor, object, task`,
			`error|role|unknown role "bard": want one of city-gm, encounter, entity-actor, global-gm, group-narrator, guardian-monitor, personal-gm, region-gm`,
		}},
		{"known role, unknown level", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Level = "quest" }, nil, []string{
			`error|level|unknown level "quest": want one of domain, global, monitor, object, task`,
		}},

		// Rule 2.
		{"scope type required", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = nil }, nil, []string{
			"error|scope_binding.type|required",
		}},
		{"scope id of a domain", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.ID = "" }, nil, []string{
			"error|scope_binding.id|required for level domain",
		}},
		{"scope id of the world", "global-world.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.ID = "" }, nil, []string{
			"error|scope_binding.id|required for level global",
		}},
		{"encounter binds to solo or group only", "encounter.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = []string{"solo", "region"} }, nil, []string{
			`error|scope_binding.type|scope type "region" does not fit role "encounter": want solo or group`,
		}},
		{"the world binds to the world only", "global-world.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = []string{"region"} }, nil, []string{
			`error|scope_binding.type|scope type "region" does not fit role "global-gm": want world`,
		}},
		{"a region binds to a region only", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = []string{"world"} }, nil, []string{
			`error|scope_binding.type|scope type "world" does not fit role "region-gm": want region`,
		}},
		{"a group narrator binds to a group only", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Type = []string{"solo"} }, nil, []string{
			`error|scope_binding.type|scope type "solo" does not fit role "group-narrator": want group`,
		}},
		{"a task needs no id", "encounter.md", func(bp *agent.AgentBlueprint) { bp.ScopeBinding.Pattern = "" }, nil, nil},
		{"an entity actor puts no constraint on the scope", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.Level, bp.Role, bp.ScopeBinding.Type = "object", "entity-actor", []string{"region"}
			bp.AllowedEventTypes, bp.OwnedEntityTypes, bp.RulesRef, bp.AbsoluteLimitsRef = nil, nil, "", ""
		}, nil, []string{"info|level|reserved level, spawn disabled"}},

		// Rule 3.
		{"a global agent has no parent", "global-world.md", func(bp *agent.AgentBlueprint) { bp.Parent = &agent.ParentReference{Name: "x"} }, nil, []string{
			"error|parent|a global agent has no parent",
		}},
		{"parent required", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.Parent = nil }, nil, []string{
			"error|parent|required for level domain",
		}},
		{"parent of an unknown level is not demanded", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Level, bp.Parent = "quest", nil }, nil, []string{
			`error|level|unknown level "quest": want one of domain, global, monitor, object, task`,
		}},
		{"parent name required", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Parent.Name = "" }, nil, []string{
			"error|parent.name|required",
		}},
		{"dynamic only for the narrators", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Parent.Instance = "dynamic" }, nil, []string{
			"error|parent.instance|instance dynamic is only for roles personal-gm and group-narrator",
		}},
		{"dynamic parent needs a name", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.Parent.Name = "" }, nil, []string{
			"error|parent.name|required",
		}},
		{"unknown instance", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Parent.Instance = "static" }, nil, []string{
			`error|parent.instance|unknown instance "static": want dynamic or none`,
		}},

		// Rule 4.
		{"trigger type required", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Trigger = agent.BlueprintTrigger{} }, nil, []string{
			"error|trigger.type|required",
		}},
		{"unknown trigger type", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Trigger.Type = "cron" }, nil, []string{
			`error|trigger.type|unknown trigger type "cron": want timer or event`,
		}},
		{"timer of a domain needs active", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.Trigger.Intervals.Active = "" }, nil, []string{
			"error|trigger.intervals.active|required for a timer trigger of level domain",
		}},
		{"timer of a domain without intervals", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.Trigger.Intervals = nil }, nil, []string{
			"error|trigger.intervals.active|required for a timer trigger of level domain",
			"error|trigger.intervals.idle|required for a timer trigger",
		}},
		{"intervals are durations", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.Trigger.Intervals = &agent.Intervals{Idle: "half an hour", Active: "0s"}
		}, nil, []string{
			`error|trigger.intervals.active|"0s" is not a positive duration`,
			`error|trigger.intervals.idle|"half an hour" is not a duration`,
		}},
		{"event name required", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Trigger.EventName = "" }, nil, []string{
			"error|trigger.event_name|required for an event trigger",
		}},
		{"event name matches nothing", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Trigger.EventName = "player.*.*" }, nil, []string{
			`error|trigger.event_name|"player.*.*" matches no event type of the registry`,
		}},
		{"star does not cross a dot", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Trigger.EventName = "*" }, nil, []string{
			`error|trigger.event_name|"*" matches no event type of the registry`,
		}},
		{"malformed glob", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Trigger.EventName = "player.[" }, nil, []string{
			`error|trigger.event_name|"player.[" is not a valid glob`,
		}},
		{"malformed glob with an empty registry", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Trigger.EventName = "player.[" },
			func(env *agent.ValidationEnv) { env.EventTypes = nil }, []string{
				`error|allowed_event_types[0]|"narrative.output" is not a type of the registry`,
				`error|trigger.event_name|"player.[" is not a valid glob`,
			}},

		// Rule 5.
		{"ttl of a domain", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.TTL = "1h" }, nil, []string{
			"warning|ttl|ignored for level domain: the agent never expires",
		}},
		{"ttl of a task is a duration", "encounter.md", func(bp *agent.AgentBlueprint) { bp.TTL = "forever" }, nil, []string{
			`error|ttl|"forever" is not a duration`,
		}},
		{"ttl of a task required", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.TTL = "" }, nil, []string{
			"error|ttl|required for level task",
		}},
		{"a reserved monitor needs no ttl", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.Level, bp.Role, bp.ScopeBinding.Type, bp.TTL = "monitor", "guardian-monitor", []string{"world"}, ""
			bp.AllowedEventTypes, bp.OwnedEntityTypes, bp.RulesRef, bp.AbsoluteLimitsRef = nil, nil, "", ""
		}, nil, []string{"info|level|reserved level, spawn disabled"}},

		// Rule 6.
		{"one domain agent", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.Constraints.MaxInstances = 0 }, nil, []string{
			"error|constraints.max_instances|must be 1 for level domain",
		}},
		{"one world agent", "global-world.md", func(bp *agent.AgentBlueprint) { bp.Constraints.MaxInstances = 2 }, nil, []string{
			"error|constraints.max_instances|must be 1 for level global",
		}},
		{"one group narrator", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.Constraints.MaxInstances = 3 }, nil, []string{
			"error|constraints.max_instances|must be 1 for role group-narrator",
		}},
		{"encounters are not limited to one", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Constraints.MaxInstances = 5 }, nil, nil},
		{"an encounter has at least one instance", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Constraints.MaxInstances = -3 }, nil, []string{
			"error|constraints.max_instances|must be at least 1",
		}},

		// Round of an encounter (api-contracts.md §3.1).
		{"round timeout is a duration", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Round.Timeout = "a minute" }, nil, []string{
			`error|round.timeout|"a minute" is not a duration`,
		}},
		{"round timeout is positive", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Round.Timeout = "-1s" }, nil, []string{
			`error|round.timeout|"-1s" is not a positive duration`,
		}},
		{"round without a timeout takes the default", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Round = &agent.RoundCfg{} }, nil, nil},

		// Rule 7.
		{"narrative phase required", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2 = nil }, nil, []string{
			"error|llm.phase2|required for role group-narrator",
		}},
		{"tick required", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LLM.Tick = nil }, nil, []string{
			"error|llm.tick|required for role global-gm",
		}},
		{"tick of a region required", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.LLM.Tick = nil }, nil, []string{
			"error|llm.tick|required for role region-gm",
		}},
		{"a role of another level demands nothing", "global-world.md", func(bp *agent.AgentBlueprint) { bp.Role = "personal-gm" }, nil, []string{
			`error|role|role "personal-gm" belongs to level "task", not "global"`,
		}},
		{"phase 1 modes", "encounter.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase1.Mode = "script" }, nil, []string{
			`error|## system|required: the blueprint calls a model`,
			`error|llm.phase1.max_tokens|must be greater than 0`,
			`error|llm.phase1.mode|unknown mode "script": want rules or llm`,
			`error|llm.phase1.model|required`,
			`error|llm.phase1.schema_ref|required`,
		}},
		{"phase 1 on a model", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.LLM.Phase1 = &agent.PhaseLLM{Mode: "llm", Model: testModel, Temperature: ptr(0.0), MaxTokens: 64, SchemaRef: "schemas/agent/narrative.json"}
			bp.Prompts.System = "Ты — встреча."
		}, nil, nil},
		{"phase 2 has no rules mode", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Mode = "rules" }, nil, []string{
			`error|llm.phase2.mode|unknown mode "rules": phase 2 always calls a model, only phase 1 has mode rules`,
		}},
		{"tick LOD", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LLM.Tick.LODDefault = "full" }, nil, []string{
			`error|llm.tick.lod_default|unknown LOD "full": want rule-only or basic`,
		}},
		{"tick of rules only", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LLM.Tick.LODDefault = "rule-only" }, nil, nil},
		{"tick incomplete", "global-world.md", func(bp *agent.AgentBlueprint) {
			bp.LLM.Tick.Model, bp.LLM.Tick.MaxTokens, bp.LLM.Tick.SchemaRef, bp.LLM.Tick.Temperature = "", -1, "", ptr(-0.1)
		}, nil, []string{
			"error|llm.tick.max_tokens|must be greater than 0",
			"error|llm.tick.model|required",
			"error|llm.tick.schema_ref|required",
			"error|llm.tick.temperature|-0.1 is outside [0, 2]",
		}},
		{"temperature bounds are inclusive", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Temperature = ptr(2.0) }, nil, nil},
		{"temperature zero", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Temperature = ptr(0.0) }, nil, nil},
		{"temperature NaN", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Temperature = ptr(math.NaN()) }, nil, []string{
			"error|llm.phase2.temperature|NaN is outside [0, 2]",
		}},
		{"forbidden 72b", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Phase2.Model = "qwen:72b" }, nil, []string{
			`error|llm.phase2.model|model "qwen:72b" is not allowed (NFR-071)`,
		}},
		{"unknown fallback", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Fallback = "silence" }, nil, []string{
			`error|llm.fallback|unknown fallback "silence": want template or rules`,
		}},
		{"negative retries", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.LLM.Retries = ptr(-1) }, nil, []string{
			"error|llm.retries|must not be negative",
		}},
		{"no phase, no fallback", "encounter.md", func(bp *agent.AgentBlueprint) { bp.LLM = agent.LLMSettings{} }, nil, nil},
		{"a model per phase, each asked", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.LLM.Phase1 = &agent.PhaseLLM{Model: "other", MaxTokens: 1, SchemaRef: "schemas/agent/narrative.json"}
		}, nil, []string{`error|llm.phase1.model|model "other" is not offered by the provider`}},

		// Rule 8.
		{"event type of the registry, not of the role", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.AllowedEventTypes = append(bp.AllowedEventTypes, "narrative.output", "entity.create.proposed")
		}, nil, []string{`error|allowed_event_types[4]|"narrative.output" is not allowed for role region-gm of level domain`}},
		{"a narrator owns nothing", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.OwnedEntityTypes = []string{"player", "encounter"} }, nil, []string{
			`error|owned_entity_types[0]|entity type "player": role group-narrator proposes no entity changes`,
			`error|owned_entity_types[1]|entity type "encounter": role group-narrator proposes no entity changes`,
		}},
		{"a reserved role owns nothing", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.Level, bp.Role, bp.ScopeBinding.Type = "monitor", "guardian-monitor", []string{"world"}
			bp.AllowedEventTypes, bp.OwnedEntityTypes, bp.RulesRef, bp.AbsoluteLimitsRef = nil, []string{"world"}, "", ""
		}, nil, []string{
			"info|level|reserved level, spawn disabled",
			`error|owned_entity_types[0]|entity type "world": role guardian-monitor proposes no entity changes`,
		}},
		{"white list of a mismatched pair is not guessed", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.Level, bp.Parent = "domain", &agent.ParentReference{Name: "global-test-world"}
			bp.ScopeBinding.ID, bp.TTL, bp.Constraints.MaxInstances = "x", "", 1
		}, nil, []string{
			"error|## description|required for level domain",
			"error|background_events|required for level domain: at least one event",
			"error|encounter|required for level domain",
			"error|laws_ref|required for level domain",
			"error|npc_table|required for level domain: at least one row",
			"error|respawn_ttl|required for level domain",
			`error|role|role "encounter" belongs to level "task", not "domain"`,
		}},

		// Rule 9.
		{"tool name required", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Tools = []agent.ToolReference{{Owner: "core"}} }, nil, []string{
			"error|tools[0].name|required",
		}},
		{"a registered tool", "personal-gm.md", func(bp *agent.AgentBlueprint) { bp.Tools = []agent.ToolReference{{Name: "dice"}} },
			func(env *agent.ValidationEnv) { env.Tools = agent.NewSet("dice") }, nil},

		// Rule 10.
		{"laws of the world required", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "" }, nil, []string{
			"error|laws_ref|required for level global",
		}},
		{"a version of the laws without a file", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "laws/dark-forest-world@v2" }, nil, []string{
			`error|laws_ref|"laws/dark-forest-world@v2": file laws/dark-forest-world.v2.yaml does not exist`,
		}},
		{"a file path is not a laws reference", "global-world.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "config/absolute-limits.yaml" }, nil, []string{
			`error|laws_ref|"config/absolute-limits.yaml" is not a laws reference laws/<world>@vN`,
		}},
		{"a laws reference lives under laws/", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.LawsRef = "rules/dark-forest@v1" }, nil, []string{
			`error|laws_ref|"rules/dark-forest@v1" is not a laws reference laws/<world>@vN`,
		}},
		{"a laws reference is not a rules file", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.RulesRef = "laws/dark-forest-world@v1" }, nil, []string{
			"error|rules_ref|file laws/dark-forest-world@v1 does not exist",
		}},
		{"no laws file without FileExists", "global-world.md", nil, func(env *agent.ValidationEnv) { env.FileExists = nil }, []string{
			`error|laws_ref|"laws/dark-forest-world@v1": file laws/dark-forest-world.v1.yaml does not exist`,
		}},
		{"rules of a region required", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.RulesRef = "" }, nil, []string{
			"error|rules_ref|required for level domain",
		}},
		{"absolute limits of a narrator", "group-narrator.md", func(bp *agent.AgentBlueprint) { bp.AbsoluteLimitsRef = "config/none.yaml" }, nil, []string{
			"error|absolute_limits_ref|file config/none.yaml does not exist",
		}},
		{"no file without FileExists", "encounter.md", nil, func(env *agent.ValidationEnv) { env.FileExists = nil }, []string{
			"error|absolute_limits_ref|file config/absolute-limits.yaml does not exist",
			"error|rules_ref|file rules/dark-forest.yaml does not exist",
		}},

		// Rule 11.
		{"region without respawn, encounter, background", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.RespawnTTL, bp.Encounter, bp.BackgroundEvents = "", nil, nil
		}, nil, []string{
			"error|background_events|required for level domain: at least one event",
			"error|encounter|required for level domain",
			"error|respawn_ttl|required for level domain",
		}},
		{"respawn is a duration, child required", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.RespawnTTL, bp.Encounter.ChildBlueprint = "a day", ""
		}, nil, []string{
			"error|encounter.child_blueprint|required",
			`error|respawn_ttl|"a day" is not a duration`,
		}},

		// Rule 12.
		{"world without budget and invariants", "global-world.md", func(bp *agent.AgentBlueprint) { bp.Budget, bp.Invariants = nil, nil }, nil, []string{
			"error|budget.background_calls_per_hour_world|must be greater than 0 for level global",
			"error|invariants|required for level global: at least one invariant",
		}},
		{"invariant check required", "global-world.md", func(bp *agent.AgentBlueprint) { bp.Invariants[0].Check = "" }, nil, []string{
			"error|invariants[0].check|required",
		}},

		// Rule 13.
		{"prompts of a YAML file are keys", "personal-gm.yaml", func(bp *agent.AgentBlueprint) {
			bp.Prompts.System = ""
			bp.Prompts.Phase2 += " {player.mood} { events }"
		}, nil, []string{
			"error|prompts.phase2|unknown placeholder {player.mood}",
			"warning|prompts.phase2|{ events } looks like a placeholder but is not one: the prompt goes to the model with it as is",
			"error|prompts.system|required: the blueprint calls a model",
		}},
		{"canon and description are checked too", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.Prompts.Canon = append(bp.Prompts.Canon, "Ведьма живёт у {swamp.edge}.")
			bp.Prompts.Description += " {игрок}"
		}, nil, []string{
			"error|## canon|unknown placeholder {swamp.edge}",
			"warning|## description|{игрок} looks like a placeholder but is not one: the prompt goes to the model with it as is",
		}},
		{"a role of rules only needs no system prompt", "encounter.md", nil, nil, nil},

		// Rule 14.
		{"city-gm is a reserved role", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.Role, bp.AllowedEventTypes, bp.OwnedEntityTypes = "city-gm", nil, nil
		}, nil, []string{"info|role|reserved role, spawn disabled"}},
		// The rules of the level apply to the reserved role (decision 2): a
		// region broken by rules 10 and 11 is broken as a city too.
		{"the rules of domain apply to city-gm", "domain-region.md", func(bp *agent.AgentBlueprint) {
			bp.Role, bp.AllowedEventTypes, bp.OwnedEntityTypes = "city-gm", nil, nil
			bp.RulesRef, bp.NPCTable = "", nil
		}, nil, []string{
			"error|npc_table|required for level domain: at least one row",
			"info|role|reserved role, spawn disabled",
			"error|rules_ref|required for level domain",
		}},
		// With the white list empty a city publishes and owns nothing.
		{"city-gm publishes nothing", "domain-region.md", func(bp *agent.AgentBlueprint) { bp.Role = "city-gm" }, nil, []string{
			`error|allowed_event_types[0]|"region.event_occurred" is not allowed for role city-gm of level domain`,
			`error|allowed_event_types[1]|"npc.moved" is not allowed for role city-gm of level domain`,
			`error|allowed_event_types[2]|"npc.spawned" is not allowed for role city-gm of level domain`,
			`error|allowed_event_types[3]|"encounter.started" is not allowed for role city-gm of level domain`,
			`error|owned_entity_types[0]|entity type "region": role city-gm proposes no entity changes`,
			`error|owned_entity_types[1]|entity type "npc": role city-gm proposes no entity changes`,
			`error|owned_entity_types[2]|entity type "encounter": role city-gm proposes no entity changes`,
			"info|role|reserved role, spawn disabled",
		}},
		// A role refused by rule 1 is not said to be reserved as well.
		{"a mismatched city-gm is not reserved", "encounter.md", func(bp *agent.AgentBlueprint) { bp.Role = "city-gm" }, nil, []string{
			`error|role|role "city-gm" belongs to level "domain", not "task"`,
		}},
		{"object is reserved", "encounter.md", func(bp *agent.AgentBlueprint) {
			bp.Level, bp.Role, bp.ScopeBinding.Type = "object", "entity-actor", nil
			bp.AllowedEventTypes, bp.OwnedEntityTypes, bp.RulesRef, bp.AbsoluteLimitsRef = nil, nil, "", ""
		}, nil, []string{
			"info|level|reserved level, spawn disabled",
			"error|scope_binding.type|required",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bp := parseValid(t, tc.file)
			if tc.change != nil {
				tc.change(bp)
			}
			env := testEnv()
			if tc.env != nil {
				tc.env(&env)
			}
			assertIssues(t, agent.Validate(bp, env), tc.want)
		})
	}
}

// valueBlueprint puts one untyped YAML value into a condition and into an
// operation of a background event.
func valueBlueprint(t *testing.T, value string) *agent.AgentBlueprint {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(validDir, "encounter.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	text = strings.Replace(text, "trigger: { type: event, event_name: encounter.started }\n",
		"trigger: { type: event, event_name: encounter.started, conditions: [{ field: f, operator: eq, value: "+value+" }] }\n"+
			"background_events:\n  - { kind: k, weight: 1, ops: [{ path: p, value: "+value+" }] }\n", 1)
	bp, err := agent.ParseBytes("values.md", []byte(text))
	if err != nil {
		t.Fatalf("value %s: %v", value, err)
	}
	return bp
}

// C-02 v1.6: values State refuses only when it applies them are warnings.
// The bound 2^53-1 itself is a safe integer.
func TestUntypedValues(t *testing.T) {
	const outside = "is outside +-(2^53-1): State refuses it when applying"
	const date = "an unquoted YAML date is a timestamp and reaches JSON as a string: quote it"
	cases := []struct {
		value  string
		suffix string // appended to the field ...value
		reason string // "" for no warning
	}{
		{"9007199254740991", "", ""},
		{"-9007199254740991", "", ""},
		{"9007199254740991.0", "", ""},
		{"0.5", "", ""},
		{"text", "", ""},
		{`"2001-01-01"`, "", ""},
		{"9007199254740992", "", "9007199254740992 " + outside},
		{"-9007199254740992", "", "-9007199254740992 " + outside},
		{"18446744073709551615", "", "18446744073709551615 " + outside},
		{"9007199254740992.0", "", "9.007199254740992e+15 " + outside},
		{"-1e300", "", "-1e+300 " + outside},
		{".nan", "", "NaN is not a JSON number"},
		{"-.inf", "", "-Inf is not a JSON number"},
		{"2001-01-01", "", date},
		{"!!timestamp 2001-01-02", "", date},
		{"[1, 9007199254740992]", "[1]", "9007199254740992 " + outside},
		{"{a: {b: 2001-01-01}}", ".a.b", date},
		{"{~: 9007199254740992, z: 1}", ".<nil>", "9007199254740992 " + outside},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			var want []string
			if tc.reason != "" {
				want = []string{
					"warning|background_events[0].ops[0].value" + tc.suffix + "|" + tc.reason,
					"warning|trigger.conditions[0].value" + tc.suffix + "|" + tc.reason,
				}
			}
			assertIssues(t, agent.Validate(valueBlueprint(t, tc.value), testEnv()), want)
		})
	}
}

func TestEnvFromProject(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	valid, err := os.ReadFile(filepath.Join(validDir, "encounter.md"))
	if err != nil {
		t.Fatal(err)
	}
	pure, err := os.ReadFile(filepath.Join(validDir, "personal-gm.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	write("blueprints/encounter-wolf.md", string(valid))
	write("blueprints/player-gm.YAML", string(pure))
	write("blueprints/README.md", "# Blueprints\n")
	// A README that parses as a blueprint still gives no name (IsBlueprintFile).
	write("blueprints/README.ru.yaml", strings.Replace(string(pure), "name: player-test-gm", "name: readme-test-gm", 1))
	write("blueprints/notes.txt", "name: not-a-blueprint\n")
	write("blueprints/nested/region.md", string(valid))
	write("blueprints/folder.md/inner.md", string(valid))
	write("schemas/agent/narrative.json", "{}")
	write("schemas/agent/tick/region.JSON", "{}")
	write("schemas/agent/readme.txt", "")
	write("schemas/events/player.said.v1.json", "{}")
	write("laws/dark-forest-world.v1.yaml", "laws: []\n")

	env, err := agent.EnvFromProject(root, []string{"a.b"}, ownedFromContracts, []string{"inv"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := sortedSet(env.Blueprints); !slices.Equal(got, []string{"encounter-test-wolf", "player-test-gm"}) {
		t.Errorf("Blueprints = %q", got)
	}
	if got := sortedSet(env.Schemas); !slices.Equal(got, []string{"schemas/agent/narrative.json", "schemas/agent/tick/region.JSON"}) {
		t.Errorf("Schemas = %q", got)
	}
	if !env.EventTypes.Has("a.b") || !env.Invariants.Has("inv") || len(env.Tools) != 0 || env.Tools == nil {
		t.Errorf("sets: types %v, invariants %v, tools %v", env.EventTypes, env.Invariants, env.Tools)
	}
	if env.Models != nil {
		t.Errorf("Models = %v, want nil: models not checked", env.Models)
	}
	if got := env.OwnedEntityTypes("global", "global-gm"); !slices.Equal(got, []string{"world"}) {
		t.Errorf("OwnedEntityTypes is not the one given: %q", got)
	}
	for rel, want := range map[string]bool{
		"laws/dark-forest-world.v1.yaml": true,
		"laws":                           false,
		"laws/none.yaml":                 false,
		"../" + filepath.Base(root) + "/laws/dark-forest-world.v1.yaml": false,
		filepath.Join(root, "laws", "dark-forest-world.v1.yaml"):        false,
	} {
		if got := env.FileExists(rel); got != want {
			t.Errorf("FileExists(%q) = %v, want %v", rel, got, want)
		}
	}

	env, err = agent.EnvFromProject(root, nil, nil, nil, []string{})
	if err != nil {
		t.Fatal(err)
	}
	if env.Models == nil || len(env.Models) != 0 {
		t.Errorf("Models = %#v, want an empty set", env.Models)
	}

	// A project without blueprints and schemas yet is an empty one, not an error.
	env, err = agent.EnvFromProject(t.TempDir(), nil, nil, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(env.Blueprints) != 0 || len(env.Schemas) != 0 {
		t.Errorf("empty project: blueprints %v, schemas %v", env.Blueprints, env.Schemas)
	}
}

// Acceptance of T-222 (review #1 N-3, question 1 of the developer): one
// function tells a blueprint from the other entries of a directory of
// blueprints, a README among them.
func TestIsBlueprintFile(t *testing.T) {
	dir := t.TempDir()
	want := map[string]bool{
		"player-gm.md":      true,
		"region.YAML":       true,
		"world.yml":         true,
		"readme-gm.md":      true,
		"my.readme.md":      true,
		"README.md":         false,
		"readme.yaml":       false,
		"ReadMe.ru.md":      false,
		"README":            false,
		"notes.txt":         false,
		"player-gm.md.orig": false,
	}
	for name := range want {
		if err := os.WriteFile(filepath.Join(dir, name), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(dir, "folder.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	want["folder.md"] = false
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != len(want) {
		t.Fatalf("%d entries, want %d: the file system folded names", len(entries), len(want))
	}
	for _, entry := range entries {
		if got := agent.IsBlueprintFile(entry); got != want[entry.Name()] {
			t.Errorf("IsBlueprintFile(%q) = %v, want %v", entry.Name(), got, want[entry.Name()])
		}
	}
}

// A file where a directory of the project is expected is an error, not an
// empty project.
func TestEnvFromProjectFileInsteadOfDirectory(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{"blueprints", "read blueprints: "},
		{"schemas/agent", "read schemas: "},
	} {
		t.Run(tc.file, func(t *testing.T) {
			root := t.TempDir()
			p := filepath.Join(root, filepath.FromSlash(tc.file))
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, nil, 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := agent.EnvFromProject(root, nil, nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) || !strings.Contains(err.Error(), "is not a directory") {
				t.Fatalf("error = %v, want %q… is not a directory", err, tc.want)
			}
		})
	}
}

func sortedSet(s agent.Set) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
