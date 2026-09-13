package agent_test

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"multiverse-core.io/shared/agent"
)

const validDir = "testdata/blueprints/valid"

var contentHashForm = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func parseValid(t *testing.T, file string) *agent.AgentBlueprint {
	t.Helper()
	bp, err := agent.ParseFile(filepath.Join(validDir, file))
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", file, err)
	}
	return bp
}

func parseError(t *testing.T, err error) *agent.ParseError {
	t.Helper()
	if err == nil {
		t.Fatal("parse succeeded, want an error")
	}
	var pe *agent.ParseError
	if !errors.As(err, &pe) {
		t.Fatalf("error %T %q is not a *agent.ParseError", err, err)
	}
	return pe
}

func TestParseFileParsesTheValidCorpus(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(validDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	var md, yamlFiles int
	for _, f := range files {
		switch filepath.Ext(f) {
		case ".md":
			md++
		case ".yaml", ".yml":
			yamlFiles++
		}
		t.Run(filepath.Base(f), func(t *testing.T) {
			bp, err := agent.ParseFile(f)
			if err != nil {
				t.Fatalf("ParseFile: %v", err)
			}
			if bp.Name == "" || bp.Version == "" || bp.Level == "" || bp.Role == "" {
				t.Errorf("name/version/level/role not filled: %q %q %q %q", bp.Name, bp.Version, bp.Level, bp.Role)
			}
			if bp.SourceFile != f {
				t.Errorf("SourceFile = %q, want %q", bp.SourceFile, f)
			}
			if !contentHashForm.MatchString(bp.ContentHash) {
				t.Errorf("ContentHash = %q, want the form sha256:<64 hex>", bp.ContentHash)
			}
			if len(bp.ParseIssues) != 0 {
				t.Errorf("a valid v2 blueprint has no parse issues, got %+v", bp.ParseIssues)
			}
		})
	}
	if md < 5 || yamlFiles < 1 {
		t.Fatalf("corpus has %d .md and %d pure YAML files, want at least 5 and 1", md, yamlFiles)
	}
}

func TestParseMarkdownFillsEverySection(t *testing.T) {
	bp := parseValid(t, "domain-region.md")

	want := agent.Prompts{
		System: "Ты — мастер региона {region.name}.\n\n```text\n## not a section: an example inside a fence\n```",
		Tick:   "Опиши, что происходит в регионе: {events}",
		Canon: []string{
			"В лесу живёт стая волков.",
			"Альфа-волк не уходит от логова\nдальше старой мельницы.",
			"Ночью туман скрывает тропы.",
		},
		Description: "Старый лес на краю мира. Тропы зарастают быстрее, чем их протаптывают.",
	}
	if !reflect.DeepEqual(bp.Prompts, want) {
		t.Fatalf("Prompts =\n%#v\nwant\n%#v", bp.Prompts, want)
	}

	personal := parseValid(t, "personal-gm.md")
	if personal.Prompts.System == "" || personal.Prompts.Phase2 == "" {
		t.Fatalf("personal GM sections missing: %+v", personal.Prompts)
	}
	global := parseValid(t, "global-world.md")
	if global.Prompts.Tick == "" || global.Prompts.System == "" {
		t.Fatalf("global GM sections missing: %+v", global.Prompts)
	}
}

func TestParseMarkdownFillsTheV2Fields(t *testing.T) {
	bp := parseValid(t, "domain-region.md")

	if got, want := bp.ScopeBinding, (agent.ScopeBinding{Type: agent.ScopeTypes{"region"}, ID: "test-forest-01"}); !reflect.DeepEqual(got, want) {
		t.Errorf("ScopeBinding = %+v, want %+v", got, want)
	}
	if bp.Parent == nil || bp.Parent.Name != "global-test-world" {
		t.Errorf("Parent = %+v", bp.Parent)
	}
	if bp.Trigger.Type != "timer" || bp.Trigger.Intervals == nil || *bp.Trigger.Intervals != (agent.Intervals{Idle: "30m", Active: "60s"}) {
		t.Errorf("Trigger = %+v", bp.Trigger)
	}
	if bp.Constraints != (agent.BlueprintConstraints{MaxInstances: 1, Priority: 50}) {
		t.Errorf("Constraints = %+v", bp.Constraints)
	}
	if bp.LLM.Phase1 == nil || bp.LLM.Phase1.Mode != "rules" {
		t.Errorf("LLM.Phase1 = %+v", bp.LLM.Phase1)
	}
	tick := bp.LLM.Tick
	if tick == nil || tick.Model != "Qwen3.8-27B-UD-Q3_K_XL" || tick.LODDefault != "basic" || tick.MaxTokens != 512 ||
		tick.Temperature == nil || *tick.Temperature != 0.7 || tick.Thinking || tick.SchemaRef != "schemas/agent/tick-region.json" {
		t.Errorf("LLM.Tick = %+v", tick)
	}
	if bp.LLM.Fallback != "rules" || bp.LLM.Retries != nil {
		t.Errorf("LLM fallback/retries = %q/%v", bp.LLM.Fallback, bp.LLM.Retries)
	}
	wantNPC := []agent.NPCRow{{NPCID: "wolf-alpha", Kind: "wolf", Name: "Альфа-волк", StatsRef: "wolf", Count: 1, Spawn: agent.NPCSpawn{On: "tick", Chance: "0.25"}}}
	if !reflect.DeepEqual(bp.NPCTable, wantNPC) {
		t.Errorf("NPCTable = %+v", bp.NPCTable)
	}
	if bp.Encounter == nil || *bp.Encounter != (agent.EncounterCfg{DetectOn: "tick", Perception: "all", Chance: "0.5", ChildBlueprint: "encounter-test-wolf"}) {
		t.Errorf("Encounter = %+v", bp.Encounter)
	}
	if bp.RespawnTTL != "24h" || bp.LawsRef != "laws/dark-forest-world@v1" || bp.RulesRef != "rules/dark-forest.yaml" || bp.Locale != "ru" {
		t.Errorf("refs = %q %q %q %q", bp.RespawnTTL, bp.LawsRef, bp.RulesRef, bp.Locale)
	}
	if !reflect.DeepEqual(bp.ImmediateBroadcast, []string{"region.event_occurred"}) {
		t.Errorf("ImmediateBroadcast = %v", bp.ImmediateBroadcast)
	}

	global := parseValid(t, "global-world.md")
	if global.Budget == nil || global.Budget.BackgroundCallsPerHourWorld != 4 {
		t.Errorf("Budget = %+v", global.Budget)
	}
	if !reflect.DeepEqual(global.Invariants, []agent.InvariantRef{{ID: "inv-01", Check: "dead_does_not_act"}, {ID: "inv-02", Check: "hp_within_bounds"}}) {
		t.Errorf("Invariants = %+v", global.Invariants)
	}
	wantOps := []agent.OpTemplate{{Path: "weather", Value: "{next_weather}"}}
	if len(global.BackgroundEvents) != 2 || !reflect.DeepEqual(global.BackgroundEvents[0].Ops, wantOps) || global.BackgroundEvents[1].Weight != 10 {
		t.Errorf("BackgroundEvents = %+v", global.BackgroundEvents)
	}
	if global.LLM.Retries == nil || *global.LLM.Retries != 1 {
		t.Errorf("Retries = %v, want 1", global.LLM.Retries)
	}

	enc := parseValid(t, "encounter.md")
	if !reflect.DeepEqual(enc.ScopeBinding.Type, agent.ScopeTypes{"solo", "group"}) {
		t.Errorf("scope_binding.type \"solo|group\" = %v, want [solo group]", enc.ScopeBinding.Type)
	}
	if enc.Round == nil || *enc.Round != (agent.RoundCfg{Timeout: "60s", IdleAfterMissed: 2}) {
		t.Errorf("Round = %+v", enc.Round)
	}
	if enc.AbsoluteLimitsRef != "config/absolute-limits.yaml" || enc.TTL != "30m" {
		t.Errorf("AbsoluteLimitsRef/TTL = %q/%q", enc.AbsoluteLimitsRef, enc.TTL)
	}
	if !reflect.DeepEqual(enc.Prompts, agent.Prompts{}) {
		t.Errorf("a blueprint without sections has empty prompts, got %+v", enc.Prompts)
	}
}

// The pure YAML file and the .md file describe one blueprint with keys in a
// different order, flow and block style, and prompts as sections or as keys:
// the parser gives one value and one hash.
func TestParsePureYAMLEqualsMarkdown(t *testing.T) {
	md := parseValid(t, "personal-gm.md")
	yml := parseValid(t, "personal-gm.yaml")

	if md.ContentHash != yml.ContentHash {
		t.Errorf("ContentHash differs: md %s, yaml %s", md.ContentHash, yml.ContentHash)
	}
	md.SourceFile, yml.SourceFile = "", ""
	if !reflect.DeepEqual(md, yml) {
		t.Fatalf("pure YAML and markdown differ:\nmd   %+v\nyaml %+v", md, yml)
	}
	if md.LLM.Phase2 == nil || md.LLM.Phase2.Model != "Qwen3.8-27B-UD-Q3_K_XL" || md.LLM.Retries == nil || *md.LLM.Retries != 2 {
		t.Fatalf("Phase2 = %+v, retries %v", md.LLM.Phase2, md.LLM.Retries)
	}
	if md.OwnedEntityTypes == nil || len(md.OwnedEntityTypes) != 0 {
		t.Errorf("owned_entity_types: [] = %#v, want an empty non-nil list", md.OwnedEntityTypes)
	}
}

func TestParseFileMissingFile(t *testing.T) {
	_, err := agent.ParseFile(filepath.Join(validDir, "no-such-blueprint.md"))
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v, want os.ErrNotExist", err)
	}
}

func TestParseUnknownKeyNamesTheFieldAndTheFile(t *testing.T) {
	cases := []struct {
		name, file, content string
		field               string
		line                int
	}{
		{
			name:    "top-level typo in frontmatter",
			file:    "typo.md",
			content: "---\nname: x\nversion: \"1.0\"\nalowed_event_types: [narrative.output]\n---\n",
			field:   "alowed_event_types",
			line:    4,
		},
		{
			name:    "provider in a phase (SEC-21)",
			file:    "provider.md",
			content: "---\nname: x\nllm:\n  phase2:\n    model: m\n    provider: ollama\n---\n",
			field:   "llm.phase2.provider",
			line:    6,
		},
		{
			name:    "sampling key the phase format does not have",
			file:    "top-p.yaml",
			content: "name: x\nllm:\n  tick: { model: m, top_p: 0.8 }\n",
			field:   "llm.tick.top_p",
			line:    3,
		},
		{
			name:    "as-is shared_resources",
			file:    "resources.md",
			content: "---\nname: x\nconstraints:\n  shared_resources:\n    - name: chroma\n---\n",
			field:   "constraints.shared_resources",
			line:    4,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := agent.ParseBytes(tc.file, []byte(tc.content))
			pe := parseError(t, err)
			if pe.File != tc.file || pe.Field != tc.field || pe.Line != tc.line || pe.Reason != "unknown field" {
				t.Fatalf("ParseError = %+v, want file %q field %q line %d reason \"unknown field\"", pe, tc.file, tc.field, tc.line)
			}
			if msg := err.Error(); !strings.Contains(msg, tc.file) || !strings.Contains(msg, tc.field) {
				t.Fatalf("message %q does not name the file and the field", msg)
			}
		})
	}
}

func TestParseRejectsMalformedBlueprints(t *testing.T) {
	const fm = "---\nname: x\n---\n"
	cases := []struct {
		name, file, content string
		field, reason       string
		line                int
	}{
		{"unsupported extension", "bp.txt", "name: x\n", "", "unsupported blueprint extension", 0},
		{"no frontmatter", "bp.md", "name: x\n", "", "missing YAML frontmatter", 1},
		{"frontmatter not closed", "bp.md", "---\nname: x\n", "", "not closed", 1},
		{"empty frontmatter", "bp.md", "---\n---\n", "", "empty YAML", 2},
		{"empty yaml file", "bp.yaml", "\n\n", "", "empty YAML", 1},
		{"frontmatter of one comment", "bp.md", "---\n# note\n---\n", "", "empty YAML: only comments", 2},
		{"yaml file of comments", "bp.yaml", "# one\n# two\n", "", "empty YAML: only comments", 1},
		{"frontmatter null", "bp.md", "---\n~\n---\n", "", "empty YAML: the document sets no field", 2},
		{"yaml file of an empty mapping", "bp.yaml", "{}\n", "", "empty YAML: the document sets no field", 1},
		{"unclosed fence swallows the next section", "bp.md", fm + "## system\nS\n```\n## phase2\nP\n", "## system", "code fence ``` is not closed", 6},
		{"a fence is closed only by its own marker", "bp.md", fm + "## tick\n~~~\n```\n", "## tick", "code fence ~~~ is not closed", 5},
		{"too large", "bp.md", fm + "## system\n" + strings.Repeat("a", agent.MaxBlueprintSize), "", "larger than 1048576 bytes", 0},
		{"unknown section", "bp.md", fm + "\n## phase3\ntext\n", "## phase3", "unknown section", 5},
		{"repeated section", "bp.md", fm + "## system\na\n## system\nb\n", "## system", "section repeated (first at line 4)", 6},
		{"text before the sections", "bp.md", fm + "# Title\n## system\na\n", "body", "text outside sections", 4},
		{"prompts key in markdown", "bp.md", "---\nname: x\nprompts: { system: a }\n---\n", "prompts", "gives its prompts as sections", 0},
		{"canon is not a list", "bp.md", fm + "## canon\nплоский текст\n", "## canon", "the section is a list", 5},
		{"empty canon item", "bp.md", fm + "## canon\n- факт\n-\n", "## canon", "empty list item", 6},
		{"two yaml documents", "bp.yaml", "name: x\n---\nname: y\n", "", "more than one YAML document", 2},
		{"type error names the field", "bp.md", "---\nname: x\nconstraints:\n  max_instances: many\n---\n", "constraints.max_instances", "cannot unmarshal", 4},
		{"type error without a value names the deepest field", "bp.md", "---\nname: x\ntrigger: { type: [timer] }\n---\n", "trigger.type", "cannot unmarshal !!seq", 3},
		{"type error in a list", "bp.yaml", "name: x\nnpc_table:\n  - { npc_id: w, count: lots }\n", "npc_table[0].count", "cannot unmarshal", 3},
		{"yaml syntax error", "bp.md", "---\nname: x\n  bad: [\n---\n", "", "", 3},
		{"role and its alias type differ", "bp.md", "---\nrole: personal-gm\ntype: narrator\n---\n", "type", "alias of role", 0},
		{"inline schema of the as-is format", "bp.md", "---\nllm:\n  schema: { type: object }\n---\n", "llm.schema", "inline schema is not supported", 0},
		{"flat model and phase2 model", "bp.md", "---\nllm:\n  model: a\n  phase2: { model: b }\n---\n", "llm.model", "given twice", 0},
		{"flat temperature and phase2 temperature", "bp.md", "---\nllm:\n  temperature: 0.5\n  phase2: { temperature: 0.7 }\n---\n", "llm.temperature", "given twice", 0},
		{"flat max_tokens and phase2 max_tokens", "bp.md", "---\nllm:\n  max_tokens: 5\n  phase2: { max_tokens: 7 }\n---\n", "llm.max_tokens", "given twice", 0},
		{"phase1_prompt and section", "bp.md", "---\nphase1_prompt: a\n---\n## phase1\nb\n", "phase1_prompt", "given twice", 0},
		{"phase2_prompt and prompts.phase2", "bp.yaml", "phase2_prompt: a\nprompts: { phase2: b }\n", "phase2_prompt", "given twice", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bp, err := agent.ParseBytes(tc.file, []byte(tc.content))
			if bp != nil {
				t.Errorf("a failed parse returns a nil blueprint, got %+v", bp)
			}
			pe := parseError(t, err)
			if pe.File != tc.file || pe.Field != tc.field || pe.Line != tc.line || !strings.Contains(pe.Reason, tc.reason) {
				t.Fatalf("ParseError = %+v, want file %q field %q line %d reason containing %q", pe, tc.file, tc.field, tc.line, tc.reason)
			}
		})
	}
}

func TestParseMigratesTheFlatLLMKeysWithWarnings(t *testing.T) {
	content := "---\nname: legacy\nversion: \"1.0\"\ntype: personal-gm\nllm:\n  model: Qwen3.8-27B-UD-Q3_K_XL\n  temperature: 0\n  max_tokens: 160\n  fallback: template\nphase2_prompt: |\n  Опиши {events}\n---\n"
	bp, err := agent.ParseBytes("legacy.md", []byte(content))
	if err != nil {
		t.Fatalf("a flat llm.model must not fail the parse: %v", err)
	}
	p2 := bp.LLM.Phase2
	if p2 == nil || p2.Model != "Qwen3.8-27B-UD-Q3_K_XL" || p2.Temperature == nil || *p2.Temperature != 0 || p2.MaxTokens != 160 {
		t.Fatalf("Phase2 = %+v, want the flat values moved there", p2)
	}
	if bp.LLM.LegacyModel != nil || bp.LLM.LegacyTemperature != nil || bp.LLM.LegacyMaxTokens != nil || bp.LegacyType != "" || bp.LegacyPhase2Prompt != nil {
		t.Fatalf("legacy fields are left filled: %+v", bp)
	}
	if bp.Role != "personal-gm" {
		t.Errorf("Role = %q, want the alias type moved there", bp.Role)
	}
	if bp.Prompts.Phase2 != "Опиши {events}" {
		t.Errorf("Prompts.Phase2 = %q", bp.Prompts.Phase2)
	}
	wantFields := []string{"phase2_prompt", "llm.model", "llm.temperature", "llm.max_tokens"}
	if len(bp.ParseIssues) != len(wantFields) {
		t.Fatalf("ParseIssues = %+v, want one warning per moved key %v", bp.ParseIssues, wantFields)
	}
	for i, issue := range bp.ParseIssues {
		if issue.Field != wantFields[i] || issue.Severity != agent.SeverityWarning || issue.File != "legacy.md" || issue.Reason == "" {
			t.Errorf("issue %d = %+v, want a warning on %q in legacy.md", i, issue, wantFields[i])
		}
	}

	// Migration keeps a phase2 that the blueprint already has.
	bp, err = agent.ParseBytes("mixed.yaml", []byte("llm:\n  model: m\n  phase2: { schema_ref: s.json }\nphase1_prompt: a\n"))
	if err != nil {
		t.Fatal(err)
	}
	if bp.LLM.Phase2.Model != "m" || bp.LLM.Phase2.SchemaRef != "s.json" || bp.Prompts.Phase1 != "a" {
		t.Fatalf("Phase2 = %+v, Phase1 prompt %q", bp.LLM.Phase2, bp.Prompts.Phase1)
	}

	// The same blueprint written in v2 has the same content, hence the same hash.
	v2 := "---\nname: legacy\nversion: \"1.0\"\nrole: personal-gm\nllm:\n  phase2: { model: Qwen3.8-27B-UD-Q3_K_XL, temperature: 0, max_tokens: 160 }\n  fallback: template\n---\n## phase2\nОпиши {events}\n"
	legacy, _ := agent.ParseBytes("legacy.md", []byte(content))
	current, err := agent.ParseBytes("v2.md", []byte(v2))
	if err != nil {
		t.Fatal(err)
	}
	if legacy.ContentHash != current.ContentHash {
		t.Fatalf("migrated and v2 hashes differ: %s vs %s", legacy.ContentHash, current.ContentHash)
	}
}

func TestParseTypeAliasAgreeingWithRoleIsAccepted(t *testing.T) {
	bp, err := agent.ParseBytes("bp.md", []byte("---\nrole: encounter\ntype: encounter\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if bp.Role != "encounter" || bp.LegacyType != "" || len(bp.ParseIssues) != 0 {
		t.Fatalf("bp = %+v", bp)
	}
}

func TestParseToleratesCRLFAndBOM(t *testing.T) {
	path := filepath.Join(validDir, "domain-region.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lf := strings.ReplaceAll(string(data), "\r\n", "\n")
	crlf := "\xEF\xBB\xBF" + strings.ReplaceAll(lf, "\n", "\r\n")

	a, err := agent.ParseBytes("a.md", []byte(lf))
	if err != nil {
		t.Fatal(err)
	}
	b, err := agent.ParseBytes("b.md", []byte(crlf))
	if err != nil {
		t.Fatalf("CRLF with BOM: %v", err)
	}
	if a.ContentHash != b.ContentHash {
		t.Fatalf("line ends change the hash: %s vs %s", a.ContentHash, b.ContentHash)
	}
}

// A fence is closed by its own marker only: the other marker inside it is
// text, and so is a heading between them.
func TestParseKeepsSectionHeadingsInsideMixedFences(t *testing.T) {
	cases := map[string]string{
		"backticks around tildes": "```\n~~~\n## tick\n```",
		"tildes around backticks": "~~~\n```\n## tick\n~~~",
	}
	for name, system := range cases {
		t.Run(name, func(t *testing.T) {
			bp, err := agent.ParseBytes("bp.md", []byte("---\nname: x\n---\n## system\n"+system+"\n## phase2\nP\n"))
			if err != nil {
				t.Fatal(err)
			}
			if bp.Prompts.System != system || bp.Prompts.Tick != "" || bp.Prompts.Phase2 != "P" {
				t.Fatalf("Prompts = %+v, want the fence and the heading inside ## system", bp.Prompts)
			}
		})
	}
}

// A flat as-is key is reported by its presence: a zero value is dropped, but
// its author still learns the key moved.
func TestParseWarnsAboutFlatKeysWithZeroValues(t *testing.T) {
	content := "---\nname: x\nllm:\n  model: \"\"\n  max_tokens: 0\nphase1_prompt: \"\"\n---\n"
	bp, err := agent.ParseBytes("zero.md", []byte(content))
	if err != nil {
		t.Fatal(err)
	}
	if bp.LLM.Phase2 != nil || bp.Prompts.Phase1 != "" {
		t.Fatalf("zero values must not create a phase or a prompt: Phase2 %+v, Phase1 %q", bp.LLM.Phase2, bp.Prompts.Phase1)
	}
	wantFields := []string{"phase1_prompt", "llm.model", "llm.max_tokens"}
	if len(bp.ParseIssues) != len(wantFields) {
		t.Fatalf("ParseIssues = %+v, want a warning for each of %v", bp.ParseIssues, wantFields)
	}
	for i, issue := range bp.ParseIssues {
		if issue.Field != wantFields[i] || issue.Severity != agent.SeverityWarning || !strings.Contains(issue.Reason, "obsolete") {
			t.Errorf("issue %d = %+v, want an obsolete-key warning on %q", i, issue, wantFields[i])
		}
	}
	plain, err := agent.ParseBytes("plain.md", []byte("---\nname: x\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if plain.ContentHash != bp.ContentHash {
		t.Fatalf("dropped zero flat keys leave the content of a blueprint without them: %s vs %s", bp.ContentHash, plain.ContentHash)
	}
}

func TestParseFileRefusesAFileOverTheSizeLimit(t *testing.T) {
	head := "---\nname: big\n---\n## system\n"
	dir := t.TempDir()

	atLimit := filepath.Join(dir, "at-limit.md")
	if err := os.WriteFile(atLimit, []byte(head+strings.Repeat("a", agent.MaxBlueprintSize-len(head))), 0o600); err != nil {
		t.Fatal(err)
	}
	if bp, err := agent.ParseFile(atLimit); err != nil || len(bp.Prompts.System) != agent.MaxBlueprintSize-len(head) {
		t.Fatalf("a file of exactly MaxBlueprintSize bytes must parse: %v", err)
	}

	over := filepath.Join(dir, "over.md")
	if err := os.WriteFile(over, []byte(head+strings.Repeat("a", agent.MaxBlueprintSize-len(head)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := agent.ParseFile(over)
	if pe := parseError(t, err); pe.File != over || !strings.Contains(pe.Reason, "larger than") {
		t.Fatalf("ParseError = %+v", pe)
	}
}

func TestParseKeepsSectionHeadingsInsideTildeFences(t *testing.T) {
	bp, err := agent.ParseBytes("bp.md", []byte("---\nname: x\n---\n## system\n~~~\n## tick\n~~~\n"))
	if err != nil {
		t.Fatal(err)
	}
	if bp.Prompts.System != "~~~\n## tick\n~~~" || bp.Prompts.Tick != "" {
		t.Fatalf("Prompts = %+v", bp.Prompts)
	}
}

// N-2 of review #2: a line that starts with inline code in triple backticks is
// text. It neither opens a fence (which would then be reported as not closed)
// nor closes one.
func TestParseTakesInlineTripleBackticksForText(t *testing.T) {
	inFence := "```\n```json``` пример ответа\n## tick is not here\n```"
	cases := map[string]struct {
		body       string
		wantSystem string
		wantTick   string
	}{
		"outside a fence": {"## system\n```json``` пример ответа\n## tick\nT\n", "```json``` пример ответа", "T"},
		"inside a fence":  {"## system\n" + inFence + "\n## tick\nT\n", inFence, "T"},
		"tilde fence":     {"## system\n~~~ `x`\n## a\n~~~\n## tick\nT\n", "~~~ `x`\n## a\n~~~", "T"},
		"four backticks":  {"## system\n````\n```json``` x\n## a\n````\n## tick\nT\n", "````\n```json``` x\n## a\n````", "T"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bp, err := agent.ParseBytes("bp.md", []byte("---\nname: x\n---\n"+tc.body))
			if err != nil {
				t.Fatal(err)
			}
			if bp.Prompts.System != tc.wantSystem || bp.Prompts.Tick != tc.wantTick {
				t.Fatalf("Prompts = %+v", bp.Prompts)
			}
		})
	}
}
