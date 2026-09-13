// Package blueprints_test guards the five blueprints of MVP-1 and the schemas
// of schemas/agent they name (T-203, swarm-llm-laws.md §13.3–§13.4).
//
// A blueprint is data the swarm loads as it is, so the test reads the files of
// the checkout through the same parser and validator the runtime and mvctl
// blueprint validate use (shared/agent), with the environment built the way a
// caller builds it: event types and ownership from shared/contracts, invariant
// checks from internal/mechanics, files and schemas from the project tree.
//
// The numbers of the narrative cap are asserted once, in the row of the
// configuration that is in force (swarm-llm-laws.md §13.4.1, config E), and
// then checked against each other: a hand edit of one number without the
// others turns the test red even when the new number looks plausible.
package blueprints_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"gopkg.in/yaml.v3"

	"multiverse-core.io/internal/laws"
	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/contracts"
)

const projectRoot = ".."

// modelE is the starting configuration E of every phase that calls a model
// (ADR-005 add. 2 p. 5, add. 3; ops/metrics/baseline.md §5). Switching the
// configuration is an edit of the YAML and of this constant, never of Go code
// that reads the blueprints.
const modelE = "Qwen3.8-27B-UD-Q3_K_XL"

// phaseTemperature is the non-thinking profile of the gateway; the thresholds
// of baseline.md were measured with it (swarm-llm-laws.md §13.3).
const phaseTemperature = 0.7

var mvp1Blueprints = []string{
	"global-dark-forest-world",
	"domain-dark-forest",
	"encounter-wolf",
	"player-gm",
	"group-narrator",
}

// pendingFiles are references of the blueprints to files another task
// delivers. While such a file is absent, the validator reports exactly the
// missing file and nothing else is tolerated; once it lands, the exception
// stops applying by itself. The task that delivers the last pending file
// removes this map together with pendingReference (T-216).
var pendingFiles = map[string]string{
	"config/absolute-limits.yaml": "T-216",
}

// narrativeCap is a row of the table of swarm-llm-laws.md §13.4.1.
type narrativeCap struct {
	config              string
	maxTokensPlayerGM   int
	maxTokensGroup      int
	textMaxLength       int
	askedLength         int // L of ## phase2
	mentionsMaxItems    int
	backgroundRefsItems int
}

// configE is the row in force until T-438 (ops/metrics/baseline.md §5).
var configE = narrativeCap{
	config:              "E",
	maxTokensPlayerGM:   160,
	maxTokensGroup:      160,
	textMaxLength:       185,
	askedLength:         140,
	mentionsMaxItems:    4,
	backgroundRefsItems: 2,
}

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

func projectEnv(t *testing.T, models []string) agent.ValidationEnv {
	t.Helper()
	env, err := agent.EnvFromProject(projectRoot, registryTypes(), ownedFromContracts, mechanics.InvariantIDs(), models)
	if err != nil {
		t.Fatalf("environment of the project: %v", err)
	}
	return env
}

func blueprintPath(name string) string {
	return filepath.Join(projectRoot, "blueprints", name+".md")
}

func parseBlueprint(t *testing.T, name string) *agent.AgentBlueprint {
	t.Helper()
	bp, err := agent.ParseFile(blueprintPath(name))
	if err != nil {
		t.Fatalf("parse %s: %v", name, err)
	}
	return bp
}

func readFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// frontmatter returns the YAML between the two lines "---" of a .md blueprint,
// with CRLF line ends dropped the way the parser drops them.
func frontmatter(t *testing.T, name string) string {
	t.Helper()
	text := strings.ReplaceAll(string(readFile(t, "blueprints/"+name+".md")), "\r\n", "\n")
	parts := strings.SplitN(text, "---\n", 3)
	if len(parts) != 3 || parts[0] != "" {
		t.Fatalf("%s: no frontmatter", name)
	}
	return parts[1]
}

func TestDirectoryHoldsTheBlueprintsOfMVP1(t *testing.T) {
	env := projectEnv(t, nil)
	if got, want := sortedSet(env.Blueprints), slices.Sorted(slices.Values(mvp1Blueprints)); !slices.Equal(got, want) {
		t.Errorf("blueprints of the project: got %v, want %v", got, want)
	}
	for _, name := range mvp1Blueprints {
		if bp := parseBlueprint(t, name); bp.Name != name {
			t.Errorf("%s.md: name %q, want the name of the file", name, bp.Name)
		}
	}
	// The domain blueprint replaces the as-is example of shared/agent, which
	// T-201 moved to services/_archive.
	if _, err := os.Stat(filepath.Join(projectRoot, "shared", "agent", "examples", "domain-dark-forest.md")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("shared/agent/examples/domain-dark-forest.md is back: %v", err)
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

// TestBlueprintsValidate runs agent.Validate on every blueprint with the
// provider offering configuration E: no finding at all, not even a warning.
func TestBlueprintsValidate(t *testing.T) {
	env := projectEnv(t, []string{modelE})
	for _, name := range mvp1Blueprints {
		t.Run(name, func(t *testing.T) {
			for _, issue := range agent.Validate(parseBlueprint(t, name), env) {
				if task, ok := pendingReference(issue, env); ok {
					t.Logf("%s|%s|%s (the file comes with %s)", issue.Severity, issue.Field, issue.Reason, task)
					continue
				}
				t.Errorf("%s|%s|%s", issue.Severity, issue.Field, issue.Reason)
			}
		})
	}
}

// pendingReference reports whether an issue is the missing file of another
// task, and which task: an error on absolute_limits_ref naming a pending file
// that is absent from the checkout.
func pendingReference(issue agent.Issue, env agent.ValidationEnv) (string, bool) {
	if issue.Severity != agent.SeverityError || issue.Field != "absolute_limits_ref" {
		return "", false
	}
	for file, task := range pendingFiles {
		if !env.FileExists(file) && issue.Reason == "file "+file+" does not exist" {
			return task, true
		}
	}
	return "", false
}

// TestBlueprintsValidateOffline is mvctl --offline and a runtime without its
// provider: the models are not checked, which is an info on the blueprints that
// name a model and nothing on the one that does not.
func TestBlueprintsValidateOffline(t *testing.T) {
	env := projectEnv(t, nil)
	for _, name := range mvp1Blueprints {
		t.Run(name, func(t *testing.T) {
			bp := parseBlueprint(t, name)
			var infos int
			for _, issue := range agent.Validate(bp, env) {
				_, pending := pendingReference(issue, env)
				switch {
				case pending:
				case issue.Severity == agent.SeverityInfo && issue.Field == "llm":
					infos++
				default:
					t.Errorf("%s|%s|%s", issue.Severity, issue.Field, issue.Reason)
				}
			}
			wantInfos := 0
			if bp.LLM.Phase2 != nil || bp.LLM.Tick != nil {
				wantInfos = 1
			}
			if infos != wantInfos {
				t.Errorf("infos on llm: got %d, want %d", infos, wantInfos)
			}
		})
	}
}

// phaseKeys are the keys of the format of a phase (api-contracts.md §3,
// swarm-llm-laws.md §13.1). phase1 in mode rules calls no model and has only
// mode; a tick adds lod_default.
var phaseKeys = []string{"max_tokens", "model", "schema_ref", "temperature", "thinking"}

func TestPhasesAreConfigurationE(t *testing.T) {
	for _, name := range mvp1Blueprints {
		t.Run(name, func(t *testing.T) {
			text := frontmatter(t, name)
			var doc struct {
				LLM map[string]any `yaml:"llm"`
			}
			if err := yaml.Unmarshal([]byte(text), &doc); err != nil {
				t.Fatal(err)
			}
			var modelPhases int
			for _, phase := range []string{"phase1", "phase2", "tick"} {
				raw, ok := doc.LLM[phase]
				if !ok {
					continue
				}
				fields, ok := raw.(map[string]any)
				if !ok {
					t.Fatalf("llm.%s is not a mapping", phase)
				}
				if phase == "phase1" && fields["mode"] == "rules" {
					if keys := sortedKeys(fields); !slices.Equal(keys, []string{"mode"}) {
						t.Errorf("llm.phase1 in mode rules has %v, want only mode", keys)
					}
					continue
				}
				modelPhases++
				want := phaseKeys
				if phase == "tick" {
					want = slices.Sorted(slices.Values(append(slices.Clone(phaseKeys), "lod_default")))
				}
				if keys := sortedKeys(fields); !slices.Equal(keys, want) {
					t.Errorf("llm.%s keys: got %v, want %v", phase, keys, want)
				}
				if fields["model"] != modelE {
					t.Errorf("llm.%s.model: got %v, want %s", phase, fields["model"], modelE)
				}
				if fields["temperature"] != phaseTemperature {
					t.Errorf("llm.%s.temperature: got %v, want %v", phase, fields["temperature"], phaseTemperature)
				}
				if fields["thinking"] != false {
					t.Errorf("llm.%s.thinking: got %v, want false", phase, fields["thinking"])
				}
			}
			if got := strings.Count(text, "# model: per ops/metrics/baseline.md"); got != modelPhases {
				t.Errorf("pointer comments to baseline.md next to a model: got %d, want %d", got, modelPhases)
			}
		})
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

// TestSamplingAndProviderAreNotBlueprintFields: the rest of the sampling
// profile lives in the gateway and the provider is chosen by the operator
// (SEC-21), so each of these keys is a parse error, not a setting.
func TestSamplingAndProviderAreNotBlueprintFields(t *testing.T) {
	source := strings.ReplaceAll(string(readFile(t, "blueprints/player-gm.md")), "\r\n", "\n")
	const phaseAnchor = "    thinking: false\n"
	const topAnchor = "locale: ru\n"
	if !strings.Contains(source, phaseAnchor) || !strings.Contains(source, topAnchor) {
		t.Fatal("player-gm.md changed its layout: update the anchors of this test")
	}
	inPhase := func(line string) string { return strings.Replace(source, phaseAnchor, phaseAnchor+"    "+line+"\n", 1) }
	cases := []struct {
		field string // the dotted path the parser names
		text  string
	}{
		{"llm.phase2.top_p", inPhase("top_p: 0.8")},
		{"llm.phase2.top_k", inPhase("top_k: 20")},
		{"llm.phase2.min_p", inPhase("min_p: 0")},
		{"llm.phase2.presence_penalty", inPhase("presence_penalty: 1.5")},
		{"provider", strings.Replace(source, topAnchor, "provider: openai_compat\n"+topAnchor, 1)},
	}
	if _, err := agent.ParseBytes("player-gm.md", []byte(source)); err != nil {
		t.Fatalf("the unmodified file does not parse: %v", err)
	}
	for _, c := range cases {
		t.Run(c.field, func(t *testing.T) {
			_, err := agent.ParseBytes("player-gm.md", []byte(c.text))
			// Only the unknown field itself counts: a YAML syntax error of a
			// broken insertion is a parse error too, and would pass unnoticed.
			var pe *agent.ParseError
			if !errors.As(err, &pe) || pe.Field != c.field || pe.Reason != "unknown field" {
				t.Fatalf("got %v, want the parse error %q: unknown field", err, c.field)
			}
		})
	}
}

// schemaURL is where a schema of schemas/agent is registered with the
// compiler: its $id, so that no file URL of the checkout is involved.
func schemaURL(file string) string {
	return "https://multiverse-core.io/schemas/agent/" + file
}

func compileSchema(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	doc, err := jsonschema.UnmarshalJSON(strings.NewReader(string(readFile(t, "schemas/agent/"+file))))
	if err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	if err := c.AddResource(schemaURL(file), doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	sch, err := c.Compile(schemaURL(file))
	if err != nil {
		t.Fatalf("%s does not compile: %v", file, err)
	}
	return sch
}

func schemaDoc(t *testing.T, file string) map[string]any {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(readFile(t, "schemas/agent/"+file), &doc); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
	return doc
}

func TestAgentSchemasCompile(t *testing.T) {
	files, err := filepath.Glob(filepath.Join(projectRoot, "schemas", "agent", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range files {
		names = append(names, filepath.Base(f))
	}
	if want := []string{"breach.json", "narrative.json", "tick-global.json", "tick-region.json"}; !slices.Equal(names, want) {
		t.Errorf("schemas/agent: got %v, want %v", names, want)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			compileSchema(t, name)
			doc := schemaDoc(t, name)
			if doc["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Errorf("$schema: got %v, want draft 2020-12", doc["$schema"])
			}
			if doc["$id"] != schemaURL(name) {
				t.Errorf("$id: got %v, want %s", doc["$id"], schemaURL(name))
			}
		})
	}
}

func instance(t *testing.T, raw string) any {
	t.Helper()
	v, err := jsonschema.UnmarshalJSON(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("instance is not JSON: %v\n%s", err, raw)
	}
	return v
}

type schemaCase struct {
	name  string
	raw   string
	valid bool
}

func checkInstances(t *testing.T, sch *jsonschema.Schema, cases []schemaCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := sch.Validate(instance(t, c.raw))
			switch {
			case c.valid && err != nil:
				t.Errorf("want valid: %v", err)
			case !c.valid && err == nil:
				t.Error("want invalid")
			}
		})
	}
}

func quoted(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestNarrativeSchemaInstances(t *testing.T) {
	sch := compileSchema(t, "narrative.json")
	text := func(n int) string { return quoted(strings.Repeat("ё", n)) }
	answer := func(text, mentions, refs string) string {
		return `{"text":` + text + `,"mentions":` + mentions + `,"background_refs":` + refs + `}`
	}
	checkInstances(t, sch, []schemaCase{
		{"minimal", answer(`"Волк отступает."`, `[]`, `[]`), true},
		{"full arrays and tone", `{"text":"Волк отступает.","mentions":["e1","e2","e10","e99"],"background_refs":["b1","b20"],"tone":"grim"}`, true},
		{"text at maxLength in Cyrillic", answer(text(185), `[]`, `[]`), true},
		{"text over maxLength", answer(text(186), `[]`, `[]`), false},
		{"empty text", answer(`""`, `[]`, `[]`), false},
		{"mentions over maxItems", answer(`"x"`, `["e1","e2","e3","e4","e5"]`, `[]`), false},
		{"background_refs over maxItems", answer(`"x"`, `[]`, `["b1","b2","b3"]`), false},
		{"label e0", answer(`"x"`, `["e0"]`, `[]`), false},
		{"label e100", answer(`"x"`, `["e100"]`, `[]`), false},
		{"background label in mentions", answer(`"x"`, `["b1"]`, `[]`), false},
		{"entity label in background_refs", answer(`"x"`, `[]`, `["e1"]`), false},
		{"id instead of a label", answer(`"x"`, `["wolf-alpha"]`, `[]`), false},
		{"object reference of the old form", answer(`"x"`, `[{"entity":{"id":"wolf-alpha","type":"npc"}}]`, `[]`), false},
		{"missing background_refs", `{"text":"x","mentions":[]}`, false},
		{"unknown tone", `{"text":"x","mentions":[],"background_refs":[],"tone":"happy"}`, false},
		{"extra field", `{"text":"x","mentions":[],"background_refs":[],"events":[]}`, false},
	})
}

func TestTickGlobalSchemaInstances(t *testing.T) {
	sch := compileSchema(t, "tick-global.json")
	event := `{"type":"world.weather_changed","summary":"Туман рассеивается.","ops":[{"path":"weather","value":"clear"}]}`
	checkInstances(t, sch, []schemaCase{
		{"no events", `{"events":[]}`, true},
		{"two events", `{"events":[` + event + `,{"type":"world.time_advanced","summary":"Светает.","ops":[{"path":"time_of_day","value":"dawn"},{"path":"day","value":2}]}]}`, true},
		{"three events", `{"events":[` + event + `,` + event + `,` + event + `]}`, false},
		{"type of another level", `{"events":[{"type":"npc.moved","summary":"x","ops":[]}]}`, false},
		{"path outside the world", `{"events":[{"type":"world.event_occurred","summary":"x","ops":[{"path":"hp","value":1}]}]}`, false},
		{"summary over 300", `{"events":[{"type":"world.event_occurred","summary":` + quoted(strings.Repeat("ё", 301)) + `,"ops":[]}]}`, false},
		{"missing ops", `{"events":[{"type":"world.event_occurred","summary":"x"}]}`, false},
		{"extra field of an event", `{"events":[{"type":"world.event_occurred","summary":"x","ops":[],"extra":1}]}`, false},
		{"extra field of an op", `{"events":[{"type":"world.event_occurred","summary":"x","ops":[{"path":"day","value":2,"extra":1}]}]}`, false},
		{"extra field of the answer", `{"events":[],"extra":1}`, false},
	})
}

// TestTickRegionSchemaInstances holds the limit of an id at 128, shared with
// llm.output.rejected.ref (C-07 v1.5): 128 passes, 129 is refused wherever an
// id stands in the answer.
func TestTickRegionSchemaInstances(t *testing.T) {
	sch := compileSchema(t, "tick-region.json")
	id128, id129 := quoted(strings.Repeat("w", 128)), quoted(strings.Repeat("w", 129))
	event := func(entity, affects string) string {
		return `{"events":[{"type":"npc.moved","summary":"Волк уходит к мельнице.","entity":{"id":` + entity + `},"affects":[{"id":` + affects + `}],"ops":[{"path":"position","value":"mill"}]}]}`
	}
	three := `{"type":"region.event_occurred","summary":"Вой.","ops":[]}`
	checkInstances(t, sch, []schemaCase{
		{"ids of 128", event(id128, id128), true},
		{"entity id of 129", event(id129, `"wolf-alpha"`), false},
		{"affects id of 129", event(`"wolf-alpha"`, id129), false},
		{"empty id", event(`""`, `"wolf-alpha"`), false},
		{"three events without references", `{"events":[` + three + `,` + three + `,` + three + `]}`, true},
		{"four events", `{"events":[` + three + `,` + three + `,` + three + `,` + three + `]}`, false},
		{"encounter.started is not proposed by a tick", `{"events":[{"type":"encounter.started","summary":"x","ops":[]}]}`, false},
		{"entity with a type", `{"events":[{"type":"npc.spawned","summary":"x","entity":{"id":"wolf-alpha","type":"npc"},"ops":[]}]}`, false},
		{"affects with a type", `{"events":[{"type":"npc.moved","summary":"x","affects":[{"id":"wolf-alpha","type":"npc"}],"ops":[]}]}`, false},
		{"extra field of an event", `{"events":[{"type":"region.event_occurred","summary":"x","ops":[],"extra":1}]}`, false},
		{"extra field of an op", `{"events":[{"type":"region.event_occurred","summary":"x","ops":[{"path":"position","value":"mill","extra":1}]}]}`, false},
		{"extra field of the answer", `{"events":[],"extra":1}`, false},
	})
}

// publishedByDesign is what each blueprint of MVP-1 publishes by the design,
// written out from the documents rather than from shared/agent/levels.go, so
// that a narrowed list shows up even when the white lists agree with it.
var publishedByDesign = map[string][]string{
	// api-contracts.md §2.4 (white list of global, entity.update.proposed(world)),
	// swarm-llm-laws.md §13.4 (tick-global.json).
	"global-dark-forest-world": {"entity.update.proposed", "world.event_occurred", "world.time_advanced", "world.weather_changed"},
	// swarm-llm-laws.md §15.2 (npc.moved, entity.update.proposed of a tick),
	// ADR-028 p. 1 and C-05 p. 4 (entity.create.proposed, then encounter.started),
	// §13.4 (tick-region.json).
	"domain-dark-forest": {"encounter.started", "entity.create.proposed", "entity.update.proposed", "npc.moved", "npc.spawned", "region.event_occurred"},
	// swarm-llm-laws.md §15.1 (dice.rolled ×4, combat.decided ×2, the package
	// entity.update.proposed), C-05 p. 1–2 and p. 5 (encounter.ended).
	"encounter-wolf": {"combat.decided", "dice.rolled", "encounter.ended", "entity.update.proposed"},
	// swarm-llm-laws.md §15.1, C-05 (narrative.output of a task level).
	"player-gm":      {"narrative.output"},
	"group-narrator": {"narrative.output"},
}

// TestBlueprintsListWhatTheirRolesPublish: Emitter refuses a publication whose
// type is not in allowed_event_types of the blueprint, and an entity proposal
// whose entity type is not in owned_entity_types (swarm-llm-laws.md §5.4), so
// both lists cover everything the role does by the design. The validator does
// not see a narrowed list: rule 8 compares it with the white list of the role,
// where narrower is fine (review #1 of T-203, Ma-1).
//
// The three sets of event types — the design, the white list of the role in
// levels.go and allowed_event_types of the blueprint — are compared for
// equality, not inclusion: a publication added to the white list turns the
// test red until the table and the blueprint follow it with a reference to the
// design, and so does a right granted to a role and its blueprint together
// (acceptance of T-203, review #2 N-4). In MVP-1 every role has one blueprint;
// a future blueprint that narrows its role on purpose writes the narrower list
// into the table. A blueprint that proposes entity changes owns exactly the
// entity types the ownership table gives its level (review #2 Mi-4).
func TestBlueprintsListWhatTheirRolesPublish(t *testing.T) {
	if got := slices.Sorted(maps.Keys(publishedByDesign)); !slices.Equal(got, slices.Sorted(slices.Values(mvp1Blueprints))) {
		t.Fatalf("the table covers %v, want the blueprints of MVP-1", got)
	}
	isProposal := func(eventType string) bool {
		ok, _ := agent.MatchEventType("entity.*.proposed", eventType)
		return ok
	}
	for _, name := range mvp1Blueprints {
		t.Run(name, func(t *testing.T) {
			bp := parseBlueprint(t, name)
			design := slices.Sorted(slices.Values(publishedByDesign[name]))
			if whiteList := slices.Sorted(slices.Values(agent.AllowedEventTypes(bp.Level, bp.Role))); !slices.Equal(whiteList, design) {
				t.Errorf("white list of %s/%s: got %v, want %v of the design", bp.Level, bp.Role, whiteList, design)
			}
			if listed := slices.Sorted(slices.Values(bp.AllowedEventTypes)); !slices.Equal(listed, design) {
				t.Errorf("allowed_event_types: got %v, want %v of the design", listed, design)
			}

			owned := slices.Sorted(slices.Values(bp.OwnedEntityTypes))
			if !slices.ContainsFunc(design, isProposal) {
				if len(owned) > 0 {
					t.Errorf("owned_entity_types %v, but the role proposes no entity change", owned)
				}
				return
			}
			want := slices.Compact(slices.Sorted(slices.Values(ownedFromContracts(bp.Level, bp.Role))))
			if len(want) == 0 {
				t.Fatalf("the ownership table gives level %s no entity type, but the role proposes entity changes", bp.Level)
			}
			if !slices.Equal(owned, want) {
				t.Errorf("owned_entity_types: got %v, want %v of contracts.OwnershipRules for level %s", owned, want, bp.Level)
			}
		})
	}
}

// TestTickEnumsFollowTheWhiteLists: the enum of a tick schema is the full set
// a model of the level may propose (TL2-7). Every type of it is in the white
// list of the role and in allowed_event_types of the blueprint, so narrowing
// the enum to the blueprint at runtime drops nothing; what the role publishes
// besides — entity proposals, and encounter.started from the rules of
// detection — is not the tick's to propose.
func TestTickEnumsFollowTheWhiteLists(t *testing.T) {
	notByTick := func(eventType string) bool {
		proposal, _ := agent.MatchEventType("entity.*.proposed", eventType)
		return proposal || eventType == "encounter.started"
	}
	registry := agent.NewSet(registryTypes()...)
	for _, name := range []string{"global-dark-forest-world", "domain-dark-forest"} {
		t.Run(name, func(t *testing.T) {
			bp := parseBlueprint(t, name)
			if bp.LLM.Tick == nil {
				t.Fatal("no tick phase")
			}
			doc := schemaDoc(t, strings.TrimPrefix(bp.LLM.Tick.SchemaRef, "schemas/agent/"))
			enum := tickTypeEnum(t, doc)

			var want []string
			for _, eventType := range agent.AllowedEventTypes(bp.Level, bp.Role) {
				if !notByTick(eventType) {
					want = append(want, eventType)
				}
			}
			if got := slices.Sorted(slices.Values(enum)); !slices.Equal(got, want) {
				t.Errorf("enum of %s: got %v, want %v", bp.LLM.Tick.SchemaRef, got, want)
			}
			for _, eventType := range enum {
				if !slices.Contains(bp.AllowedEventTypes, eventType) {
					t.Errorf("%s is in the enum but not in allowed_event_types", eventType)
				}
				if !registry.Has(eventType) {
					t.Errorf("%s is not a type of the registry", eventType)
				}
			}
		})
	}
}

func tickTypeEnum(t *testing.T, doc map[string]any) []string {
	t.Helper()
	raw, err := dig(doc, "properties", "events", "items", "properties", "type", "enum")
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, v := range raw.([]any) {
		out = append(out, v.(string))
	}
	return out
}

func dig(doc map[string]any, path ...string) (any, error) {
	var cur any = doc
	for i, key := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s is not an object", strings.Join(path[:i], "."))
		}
		if cur, ok = m[key]; !ok {
			return nil, fmt.Errorf("no %s", strings.Join(path[:i+1], "."))
		}
	}
	return cur, nil
}

func intAt(t *testing.T, doc map[string]any, path ...string) int {
	t.Helper()
	v, err := dig(doc, path...)
	if err != nil {
		t.Fatal(err)
	}
	f, ok := v.(float64)
	if !ok || f != math.Trunc(f) {
		t.Fatalf("%s is not an integer: %v", strings.Join(path, "."), v)
	}
	return int(f)
}

var capCommentRe = regexp.MustCompile(`^narrative cap: config=(\w+); c=([0-9.]+); c_min=([0-9.]+); overhead=([0-9]+); `)

// askedLengthRe reads L of ## phase2, the length asked of the model.
var askedLengthRe = regexp.MustCompile(`до ~(\d+) символов`)

// TestNarrativeCap checks the numbers of swarm-llm-laws.md §13.4.1 in the
// files: the row of configuration E, and the three invariants between the
// max_tokens of a narrator, the limits of narrative.json and L. c and c_min
// are read from the $comment of the schema, the service fields are counted
// from its maxItems.
func TestNarrativeCap(t *testing.T) {
	doc := schemaDoc(t, "narrative.json")
	comment, _ := doc["$comment"].(string)
	m := capCommentRe.FindStringSubmatch(comment)
	if m == nil {
		t.Fatalf("$comment of narrative.json: %q, want the form %q", comment, "narrative cap: config=E; c=2.5; c_min=2.0; overhead=66; …")
	}
	config := m[1]
	c, errC := strconv.ParseFloat(m[2], 64)
	cMin, errMin := strconv.ParseFloat(m[3], 64)
	overhead, errOverhead := strconv.Atoi(m[4])
	if err := errors.Join(errC, errMin, errOverhead); err != nil {
		t.Fatal(err)
	}
	for _, pointer := range []string{"ops/metrics/baseline.md §5", "§13.4.1", "ADR-029"} {
		if !strings.Contains(comment, pointer) {
			t.Errorf("$comment of narrative.json does not point at %s", pointer)
		}
	}

	maxLength := intAt(t, doc, "properties", "text", "maxLength")
	mentions := intAt(t, doc, "properties", "mentions", "maxItems")
	refs := intAt(t, doc, "properties", "background_refs", "maxItems")

	narrators := map[string]int{}
	asked := map[string]int{}
	for _, name := range []string{"player-gm", "group-narrator"} {
		bp := parseBlueprint(t, name)
		if bp.LLM.Phase2 == nil {
			t.Fatalf("%s: no phase2", name)
		}
		if bp.LLM.Phase2.SchemaRef != "schemas/agent/narrative.json" {
			t.Errorf("%s: schema_ref %q, want the one narrative schema of both narrators", name, bp.LLM.Phase2.SchemaRef)
		}
		narrators[name] = bp.LLM.Phase2.MaxTokens
		found := askedLengthRe.FindAllStringSubmatch(bp.Prompts.Phase2, -1)
		if len(found) != 1 {
			t.Fatalf("%s: ## phase2 has %d phrases «до ~L символов», want exactly one", name, len(found))
		}
		asked[name], _ = strconv.Atoi(found[0][1])
	}

	t.Run("row "+configE.config, func(t *testing.T) {
		got := narrativeCap{
			config:              config,
			maxTokensPlayerGM:   narrators["player-gm"],
			maxTokensGroup:      narrators["group-narrator"],
			textMaxLength:       maxLength,
			askedLength:         asked["player-gm"],
			mentionsMaxItems:    mentions,
			backgroundRefsItems: refs,
		}
		if got != configE {
			t.Errorf("got %+v, want %+v", got, configE)
		}
		// One L for both narrators while they share one schema (§13.4.1, «Группа»).
		if l := asked["group-narrator"]; l != configE.askedLength {
			t.Errorf("group-narrator: L = %d, want %d", l, configE.askedLength)
		}
	})

	t.Run("overhead", func(t *testing.T) {
		// 30 tokens of frame, 6 per label in quotes with its comma.
		if want := 30 + 6*(mentions+refs); overhead != want {
			t.Errorf("overhead in $comment: %d, want 30 + 6 × (%d + %d) = %d", overhead, mentions, refs, want)
		}
	})

	for name, n := range narrators {
		t.Run("invariants/"+name, func(t *testing.T) {
			if need := overhead + int(math.Ceil(float64(maxLength)/cMin)); need > n {
				t.Errorf("1: overhead + ⌈maxLength / c_min⌉ = %d + ⌈%d / %v⌉ = %d > max_tokens %d", overhead, maxLength, cMin, need, n)
			}
			if limit := float64(n-overhead) * 0.9 * c; float64(maxLength) > limit {
				t.Errorf("2: maxLength %d > (max_tokens − overhead) × 0.9 × c = %v", maxLength, limit)
			}
			if l := asked[name]; float64(l) > 0.8*float64(maxLength) {
				t.Errorf("3: L %d > 0.8 × maxLength = %v", l, 0.8*float64(maxLength))
			}
		})
	}
}

// TestGlobalInvariantsFollowTheLaws: the invariants of the world blueprint
// repeat, id and check, the invariants of the laws version it names whose
// check mechanics implements (swarm-llm-laws.md §13.3). inv-11 is the
// guardian's and is not listed.
func TestGlobalInvariantsFollowTheLaws(t *testing.T) {
	bp := parseBlueprint(t, "global-dark-forest-world")
	m := regexp.MustCompile(`^laws/([^/@]+)@(v[1-9][0-9]*)$`).FindStringSubmatch(bp.LawsRef)
	if m == nil {
		t.Fatalf("laws_ref %q", bp.LawsRef)
	}
	file := laws.FileName(m[1], m[2])
	doc, err := laws.Parse(file, readFile(t, "laws/"+file), nil)
	if err != nil {
		t.Fatal(err)
	}
	mechanicsChecks := mechanics.InvariantIDs()
	var want []agent.InvariantRef
	for _, law := range doc.Laws {
		if law.Kind == laws.KindInvariant && slices.Contains(mechanicsChecks, law.Check) {
			want = append(want, agent.InvariantRef{ID: law.ID, Check: law.Check})
		}
	}
	if len(want) != len(mechanicsChecks) {
		t.Fatalf("laws %s cover %d of %d invariants of mechanics", file, len(want), len(mechanicsChecks))
	}
	if !slices.Equal(bp.Invariants, want) {
		t.Errorf("invariants:\n  got  %v\n  want %v", bp.Invariants, want)
	}
}

// labelInTextRe is the check of the parser for a label in text (ADR-029 p. 4).
var labelInTextRe = regexp.MustCompile(`(^|[^0-9A-Za-z_])[eb][1-9][0-9]?([^0-9A-Za-z_]|$)`)

// TestLLMOutputFixturesAnswerByNarrativeSchema: the payload fixtures of
// llm.output record a narrative answer of the current format — labels, not
// ids, and no label in text (ADR-029 p. 7) — with the cap of the narrator, and
// the length and hash of exactly that answer.
func TestLLMOutputFixturesAnswerByNarrativeSchema(t *testing.T) {
	sch := compileSchema(t, "narrative.json")
	maxTokens := parseBlueprint(t, "player-gm").LLM.Phase2.MaxTokens
	for _, kind := range []string{"valid", "invalid"} {
		t.Run(kind, func(t *testing.T) {
			var fixture struct {
				Phase        string `json:"phase"`
				ResponseRaw  string `json:"response_raw"`
				ResponseHash string `json:"response_hash"`
				ResponseLen  int    `json:"response_len"`
				Params       struct {
					MaxTokens int `json:"max_tokens"`
				} `json:"params"`
			}
			if err := json.Unmarshal(readFile(t, "testdata/fixtures/events/llm.output.v1."+kind+".json"), &fixture); err != nil {
				t.Fatal(err)
			}
			if fixture.Phase != "narrative" {
				t.Fatalf("phase %q, want narrative", fixture.Phase)
			}
			if err := sch.Validate(instance(t, fixture.ResponseRaw)); err != nil {
				t.Errorf("response_raw is not an answer by narrative.json: %v", err)
			}
			var answer struct {
				Text string `json:"text"`
			}
			if err := json.Unmarshal([]byte(fixture.ResponseRaw), &answer); err != nil {
				t.Fatal(err)
			}
			if labelInTextRe.MatchString(answer.Text) {
				t.Errorf("text carries a label: %q", answer.Text)
			}
			if fixture.Params.MaxTokens != maxTokens {
				t.Errorf("params.max_tokens %d, want %d of player-gm", fixture.Params.MaxTokens, maxTokens)
			}
			if fixture.ResponseLen != len(fixture.ResponseRaw) {
				t.Errorf("response_len %d, want %d bytes of response_raw", fixture.ResponseLen, len(fixture.ResponseRaw))
			}
			sum := sha256.Sum256([]byte(fixture.ResponseRaw))
			if want := "sha256:" + hex.EncodeToString(sum[:]); fixture.ResponseHash != want {
				t.Errorf("response_hash %s, want %s", fixture.ResponseHash, want)
			}
		})
	}
}
