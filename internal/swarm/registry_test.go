package swarm_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/swarm"
	"multiverse-core.io/internal/swarm/blueprintenv"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/runtime"
)

const projectRoot = "../.."

// modelE is the model of every phase of the blueprints of MVP-1 (T-203).
const modelE = "Qwen3.8-27B-UD-Q3_K_XL"

var mvp1Blueprints = []string{"domain-dark-forest", "encounter-wolf", "global-dark-forest-world", "group-narrator", "player-gm"}

// limitsFile is referenced by three blueprints of MVP-1 and delivered by T-216.
const limitsFile = "config/absolute-limits.yaml"

func readProjectFile(t *testing.T, rel string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(t *testing.T, root, rel string, data []byte) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

// projectCopy lays out a project in a temporary directory: the five blueprints
// of MVP-1 and every file they reference. The absolute limits of T-216 are
// copied when the checkout has them; until then a placeholder stands in for
// them, since the file is only checked to exist.
func projectCopy(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, name := range mvp1Blueprints {
		writeFile(t, root, "blueprints/"+name+".md", readProjectFile(t, "blueprints/"+name+".md"))
	}
	schemas, err := filepath.Glob(filepath.Join(projectRoot, "schemas", "agent", "*.json"))
	if err != nil || len(schemas) == 0 {
		t.Fatalf("schemas/agent: %v %v", schemas, err)
	}
	for _, s := range schemas {
		writeFile(t, root, "schemas/agent/"+filepath.Base(s), readProjectFile(t, "schemas/agent/"+filepath.Base(s)))
	}
	for _, rel := range []string{"laws/dark-forest-world.v1.yaml", "rules/dark-forest.yaml"} {
		writeFile(t, root, rel, readProjectFile(t, rel))
	}
	if data, err := os.ReadFile(filepath.Join(projectRoot, filepath.FromSlash(limitsFile))); err == nil {
		writeFile(t, root, limitsFile, data)
	} else {
		writeFile(t, root, limitsFile, []byte("# placeholder of the test: the file comes with T-216\n"))
	}
	return root
}

// blueprintText returns a blueprint of MVP-1 with LF line ends, edited by
// replacing each old string with the new one; an anchor that is not there
// fails the test rather than leaving the file unchanged.
func blueprintText(t *testing.T, name string, replace ...string) []byte {
	t.Helper()
	text := strings.ReplaceAll(string(readProjectFile(t, "blueprints/"+name+".md")), "\r\n", "\n")
	for i := 0; i+1 < len(replace); i += 2 {
		if !strings.Contains(text, replace[i]) {
			t.Fatalf("%s.md has no %q: update the anchor of the test", name, replace[i])
		}
		text = strings.Replace(text, replace[i], replace[i+1], 1)
	}
	return []byte(text)
}

func load(t *testing.T, root string, models []string) *swarm.Registry {
	t.Helper()
	r, err := swarm.LoadDir(filepath.Join(root, "blueprints"), swarm.LoadConfig{Root: root, Models: models})
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func states(r *swarm.Registry) map[string]swarm.BlueprintState {
	out := map[string]swarm.BlueprintState{}
	for _, report := range r.Reports() {
		out[filepath.Base(report.File)] = report.State
	}
	return out
}

func renderIssues(issues []agent.Issue) []string {
	out := make([]string, 0, len(issues))
	for _, issue := range issues {
		out = append(out, string(issue.Severity)+"|"+issue.Field+"|"+issue.Code+"|"+issue.Reason)
	}
	return out
}

func reportOf(t *testing.T, r *swarm.Registry, base string) swarm.FileReport {
	t.Helper()
	for _, report := range r.Reports() {
		if filepath.Base(report.File) == base {
			return report
		}
	}
	t.Fatalf("no report on %s", base)
	return swarm.FileReport{}
}

func assertHealth(t *testing.T, r *swarm.Registry, want runtime.Status) {
	t.Helper()
	got := r.Health()
	if got.Status != want.Status || len(got.Details) != len(want.Details) {
		t.Fatalf("Health() = %+v, want %+v", got, want)
	}
	for key, value := range want.Details {
		switch v := value.(type) {
		case []string:
			if g, ok := got.Details[key].([]string); !ok || !slices.Equal(g, v) {
				t.Errorf("Health().Details[%s] = %#v, want %#v", key, got.Details[key], v)
			}
		default:
			if got.Details[key] != value {
				t.Errorf("Health().Details[%s] = %#v, want %#v", key, got.Details[key], value)
			}
		}
	}
}

// TestLoadDirOfTheProject loads blueprints/ of the checkout the way the
// process does. Every blueprint is active except the ones naming the absolute
// limits while T-216 has not delivered them: those are rejected for exactly
// that missing file, a blueprint whose encounter child is one of them is
// rejected for that reference (review #1 Mi-1), and Health says which files.
func TestLoadDirOfTheProject(t *testing.T) {
	r, err := swarm.LoadDir(filepath.Join(projectRoot, "blueprints"), swarm.LoadConfig{Root: projectRoot, Models: []string{modelE}})
	if err != nil {
		t.Fatal(err)
	}
	_, statErr := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(limitsFile)))
	limitsPresent := statErr == nil

	pending := map[string]bool{}
	for _, report := range r.Reports() {
		if bp, err := agent.ParseFile(report.File); err == nil && !limitsPresent && bp.AbsoluteLimitsRef == limitsFile {
			pending[bp.Name] = true
		}
	}
	var wantActive, wantRejected []string
	for _, report := range r.Reports() {
		base := filepath.Base(report.File)
		bp, err := agent.ParseFile(report.File)
		if err != nil {
			t.Fatalf("%s: %v", base, err)
		}
		if bp.Encounter != nil && pending[bp.Encounter.ChildBlueprint] {
			wantRejected = append(wantRejected, base)
			want := []string{`error|encounter.child_blueprint||unknown blueprint "` + bp.Encounter.ChildBlueprint + `"; that blueprint is in the directory, but rejected`}
			if got := renderIssues(report.Issues); !slices.Equal(got, want) {
				t.Errorf("%s: issues %q, want only the reference to a rejected blueprint %q", base, got, want)
			}
			continue
		}
		if pending[bp.Name] {
			wantRejected = append(wantRejected, base)
			want := []string{"error|absolute_limits_ref|file_missing|file config/absolute-limits.yaml does not exist"}
			if got := renderIssues(report.Issues); !slices.Equal(got, want) {
				t.Errorf("%s: issues %q, want only the file of T-216 %q", base, got, want)
			}
			continue
		}
		wantActive = append(wantActive, bp.Name)
		if len(report.Issues) != 0 || report.State != swarm.BlueprintActive {
			t.Errorf("%s: %s with %q, want active without a finding", base, report.State, renderIssues(report.Issues))
		}
	}
	if len(r.Reports()) != len(mvp1Blueprints) {
		t.Errorf("reports on %d files, want the %d blueprints of MVP-1", len(r.Reports()), len(mvp1Blueprints))
	}
	slices.Sort(wantActive)
	if got := r.Names(); !slices.Equal(got, wantActive) {
		t.Errorf("Names() = %q, want %q", got, wantActive)
	}
	if len(wantRejected) == 0 {
		assertHealth(t, r, runtime.OK())
	} else {
		t.Logf("rejected until T-216 delivers %s (with the region naming such a blueprint): %v", limitsFile, wantRejected)
		assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"blueprints": wantRejected}})
	}
}

// DoD of T-222: a directory with one invalid blueprint activates the others
// and names the invalid file in Health.
func TestLoadDirSkipsTheInvalidBlueprint(t *testing.T) {
	root := projectCopy(t)
	full := load(t, root, []string{modelE})
	if got := full.Names(); !slices.Equal(got, mvp1Blueprints) {
		t.Fatalf("the copy of the project: Names() = %q, want %q", got, mvp1Blueprints)
	}
	assertHealth(t, full, runtime.OK())

	// A copy of player-gm under its own name, publishing what its role may not.
	writeFile(t, root, "blueprints/rogue-gm.md", blueprintText(t, "player-gm",
		"name: player-gm\n", "name: rogue-gm\n",
		"allowed_event_types: [narrative.output]\n", "allowed_event_types: [narrative.output, world.weather_changed]\n"))
	r := load(t, root, []string{modelE})

	if got := r.Names(); !slices.Equal(got, mvp1Blueprints) {
		t.Errorf("Names() = %q, want the valid blueprints %q", got, mvp1Blueprints)
	}
	for _, name := range mvp1Blueprints {
		if bp, ok := r.Get(name); !ok || bp.Name != name {
			t.Errorf("Get(%q) = %v, %v", name, bp, ok)
		}
	}
	if _, ok := r.Get("rogue-gm"); ok {
		t.Error("Get(rogue-gm): an invalid blueprint is not activated")
	}
	report := reportOf(t, r, "rogue-gm.md")
	if report.State != swarm.BlueprintRejected || report.Name != "rogue-gm" {
		t.Errorf("report %+v, want rejected rogue-gm", report)
	}
	want := []string{`error|allowed_event_types[1]||"world.weather_changed" is not allowed for role personal-gm of level task`}
	if got := renderIssues(report.Issues); !slices.Equal(got, want) {
		t.Errorf("issues %q, want %q", got, want)
	}
	if got, want := r.Rejected(), []string{filepath.Join(root, "blueprints", "rogue-gm.md")}; !slices.Equal(got, want) {
		t.Errorf("Rejected() = %q, want %q", got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"blueprints": []string{"rogue-gm.md"}}})
}

// Files that are not blueprints by name (agent.IsBlueprintFile), a README
// among them, are not read; a file that does not parse is rejected with the
// parse error, including a file over the size limit of the parser.
func TestLoadDirFilesThatDoNotParse(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/notes.txt", []byte("name: not-a-blueprint\n"))
	writeFile(t, root, "blueprints/README.md", []byte("# Blueprints\n\nHow to write a region without Go.\n"))
	writeFile(t, root, "blueprints/nested/extra.md", blueprintText(t, "player-gm", "name: player-gm\n", "name: nested-gm\n"))
	if err := os.MkdirAll(filepath.Join(root, "blueprints", "folder.md"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, "blueprints/broken.yaml", []byte("name: [unclosed\n"))
	writeFile(t, root, "blueprints/unknown-key.md", blueprintText(t, "player-gm",
		"name: player-gm\n", "name: unknown-key-gm\n", "locale: ru\n", "locale: ru\nprovider: openai_compat\n"))
	writeFile(t, root, "blueprints/huge.md", []byte(strings.Repeat("x", agent.MaxBlueprintSize+1)))

	r := load(t, root, []string{modelE})
	wantStates := map[string]swarm.BlueprintState{
		"broken.yaml": swarm.BlueprintRejected, "huge.md": swarm.BlueprintRejected, "unknown-key.md": swarm.BlueprintRejected,
	}
	for _, name := range mvp1Blueprints {
		wantStates[name+".md"] = swarm.BlueprintActive
	}
	got := states(r)
	if len(got) != len(wantStates) {
		t.Errorf("reports %v, want %v", got, wantStates)
	}
	for file, state := range wantStates {
		if got[file] != state {
			t.Errorf("%s: %q, want %q", file, got[file], state)
		}
	}
	if got := renderIssues(reportOf(t, r, "unknown-key.md").Issues); !slices.Equal(got, []string{"error|provider||unknown field"}) {
		t.Errorf("unknown-key.md: %q", got)
	}
	if issues := reportOf(t, r, "huge.md").Issues; len(issues) != 1 || !strings.Contains(issues[0].Reason, "larger than") {
		t.Errorf("huge.md: %q", renderIssues(issues))
	}
	if report := reportOf(t, r, "broken.yaml"); report.Name != "" || len(report.Issues) == 0 || report.Issues[0].Severity != agent.SeverityError {
		t.Errorf("broken.yaml: %+v", report)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{
		"blueprints": []string{"broken.yaml", "huge.md", "unknown-key.md"},
	}})
}

// DoD of T-222 (acceptance of T-202): a second blueprint with a name already
// taken is rejected, the first one stays active.
func TestLoadDirRejectsASecondBlueprintOfTheSameName(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/zz-player-gm-copy.md", blueprintText(t, "player-gm", "ttl: 45m\n", "ttl: 30m\n"))
	r := load(t, root, []string{modelE})

	first := filepath.Join(root, "blueprints", "player-gm.md")
	if bp, ok := r.Get("player-gm"); !ok || bp.SourceFile != first || bp.TTL != "45m" {
		t.Errorf("Get(player-gm) = %+v, %v; want the blueprint of %s", bp, ok, first)
	}
	report := reportOf(t, r, "zz-player-gm-copy.md")
	want := []string{`error|name||blueprint name "player-gm" is already taken by ` + first}
	if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, want) {
		t.Errorf("second file: %s %q, want rejected %q", report.State, got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"blueprints": []string{"zz-player-gm-copy.md"}}})

	// Acceptance of T-222 (review #1 N-2, question 6 of the developer): the
	// name stays with the first file even when that file is rejected, so a
	// valid second file does not take the name over. Both are rejected.
	writeFile(t, root, "blueprints/player-gm.md", blueprintText(t, "player-gm",
		"allowed_event_types: [narrative.output]\n", "allowed_event_types: [narrative.output, world.weather_changed]\n"))
	writeFile(t, root, "blueprints/zz-player-gm-copy.md", blueprintText(t, "player-gm"))
	r = load(t, root, []string{modelE})
	for _, base := range []string{"player-gm.md", "zz-player-gm-copy.md"} {
		if state := states(r)[base]; state != swarm.BlueprintRejected {
			t.Errorf("%s: %q, want rejected", base, state)
		}
	}
	if bp, ok := r.Get("player-gm"); ok {
		t.Errorf("Get(player-gm) = %s: the name stays with the rejected first file", bp.SourceFile)
	}
	if got := renderIssues(reportOf(t, r, "zz-player-gm-copy.md").Issues); !slices.Equal(got, want) {
		t.Errorf("second file: %q, want %q", got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"blueprints": []string{"player-gm.md", "zz-player-gm-copy.md"}}})
}

// Decision 1 of swarm-llm-laws.md §13.2 in both modes: the validator — and so
// mvctl blueprint validate — reports a model the provider does not offer as
// an error with the code model_missing; the runtime lowers that finding by
// its code, activates the agent and names the blueprint and the phase.
func TestModelMissingInTheRuntimeAndInTheValidator(t *testing.T) {
	root := projectCopy(t)

	validationEnv, err := blueprintenv.ProjectEnv(root, []string{"other-model"})
	if err != nil {
		t.Fatal(err)
	}
	bp, err := agent.ParseFile(filepath.Join(root, "blueprints", "player-gm.md"))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := renderIssues(agent.Validate(bp, validationEnv)), []string{
		`error|llm.phase2.model|model_missing|model "Qwen3.8-27B-UD-Q3_K_XL" is not offered by the provider`,
	}; !slices.Equal(got, want) {
		t.Errorf("validator: %q, want %q", got, want)
	}

	r := load(t, root, []string{"other-model"})
	if got := r.Names(); !slices.Equal(got, mvp1Blueprints) {
		t.Errorf("runtime: Names() = %q, want every blueprint active %q", got, mvp1Blueprints)
	}
	if got, want := renderIssues(reportOf(t, r, "player-gm.md").Issues), []string{
		`warning|llm.phase2.model|model_missing|model "Qwen3.8-27B-UD-Q3_K_XL" is not offered by the provider`,
	}; !slices.Equal(got, want) {
		t.Errorf("runtime: %q, want %q", got, want)
	}
	wantMissing := []swarm.ModelMissing{
		{Blueprint: "domain-dark-forest", Phase: "tick", Field: "llm.tick.model"},
		{Blueprint: "global-dark-forest-world", Phase: "tick", Field: "llm.tick.model"},
		{Blueprint: "group-narrator", Phase: "phase2", Field: "llm.phase2.model"},
		{Blueprint: "player-gm", Phase: "phase2", Field: "llm.phase2.model"},
	}
	got := r.ModelsMissing()
	if len(got) != len(wantMissing) {
		t.Fatalf("ModelsMissing() = %+v, want %+v", got, wantMissing)
	}
	for i, m := range got {
		want := wantMissing[i]
		want.File = filepath.Join(root, "blueprints", want.Blueprint+".md")
		if m != want {
			t.Errorf("ModelsMissing()[%d] = %+v, want %+v", i, m, want)
		}
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"llm": "model_missing"}})

	// Only the code is lowered: a forbidden model of the same field stays an
	// error and keeps the blueprint out.
	writeFile(t, root, "blueprints/player-gm.md", blueprintText(t, "player-gm", "model: "+modelE+"\n", "model: qwen:7b\n"))
	r = load(t, root, []string{"other-model"})
	if state := states(r)["player-gm.md"]; state != swarm.BlueprintRejected {
		t.Errorf("player-gm.md with a forbidden model: %q, want rejected", state)
	}

	// Without the models of the provider nothing is missing: an info, not a
	// degradation.
	r = load(t, projectCopy(t), nil)
	if len(r.ModelsMissing()) != 0 {
		t.Errorf("models not checked: ModelsMissing() = %+v", r.ModelsMissing())
	}
	if got := renderIssues(reportOf(t, r, "player-gm.md").Issues); !slices.Equal(got, []string{"info|llm|models_not_checked|models not checked"}) {
		t.Errorf("models not checked: %q", got)
	}
	assertHealth(t, r, runtime.OK())
}

// Acceptance of T-222 (review #1 Mi-2): the runtime lowers the finding of one
// code only. A file missing from the project (file_missing) keeps the
// blueprint out while a missing model of the same blueprint is lowered. The
// test lays the error out itself, so it does not depend on which files the
// checkout has.
func TestLoadDirLowersOnlyModelMissing(t *testing.T) {
	root := projectCopy(t)
	if err := os.Remove(filepath.Join(root, "rules", "dark-forest.yaml")); err != nil {
		t.Fatal(err)
	}
	r := load(t, root, []string{"other-model"})

	missingRules := "error|rules_ref|file_missing|file rules/dark-forest.yaml does not exist"
	for base, want := range map[string][]string{
		"encounter-wolf.md": {missingRules},
		"domain-dark-forest.md": {
			`warning|llm.tick.model|model_missing|model "` + modelE + `" is not offered by the provider`,
			missingRules,
		},
	} {
		report := reportOf(t, r, base)
		if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, want) {
			t.Errorf("%s: %s %q, want rejected %q", base, report.State, got, want)
		}
	}
	for _, base := range []string{"global-dark-forest-world.md", "group-narrator.md", "player-gm.md"} {
		report := reportOf(t, r, base)
		got := renderIssues(report.Issues)
		if report.State != swarm.BlueprintActive || len(got) != 1 || report.Issues[0].Severity != agent.SeverityWarning || report.Issues[0].Code != agent.CodeModelMissing {
			t.Errorf("%s: %s %q, want active with one model_missing warning", base, report.State, got)
		}
	}
	if got, want := r.Names(), []string{"global-dark-forest-world", "group-narrator", "player-gm"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{
		"blueprints": []string{"domain-dark-forest.md", "encounter-wolf.md"},
		"llm":        "model_missing",
	}})
}

// Acceptance of T-222 (review #1 Mi-1): a blueprint naming a rejected one is
// rejected too. Here a region whose encounter child is broken: the region
// never opens an encounter the registry has no agent for.
func TestLoadDirRejectsWhatNamesARejectedBlueprint(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/encounter-wolf.md", blueprintText(t, "encounter-wolf",
		"rules_ref: rules/dark-forest.yaml\n", "rules_ref: rules/missing.yaml\n"))
	r := load(t, root, []string{modelE})

	if got := renderIssues(reportOf(t, r, "encounter-wolf.md").Issues); !slices.Equal(got, []string{
		"error|rules_ref|file_missing|file rules/missing.yaml does not exist",
	}) {
		t.Errorf("encounter-wolf.md: %q", got)
	}
	report := reportOf(t, r, "domain-dark-forest.md")
	want := []string{`error|encounter.child_blueprint||unknown blueprint "encounter-wolf"; that blueprint is in the directory, but rejected`}
	if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, want) {
		t.Errorf("domain-dark-forest.md: %s %q, want rejected %q", report.State, got, want)
	}
	if _, ok := r.Get("domain-dark-forest"); ok {
		t.Error("Get(domain-dark-forest): a region whose encounter is rejected is not activated")
	}
	if got, want := r.Names(), []string{"global-dark-forest-world", "group-narrator", "player-gm"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{
		"blueprints": []string{"domain-dark-forest.md", "encounter-wolf.md"},
	}})
}

// The rejection runs down a chain until nothing changes: a broken world takes
// its region out in the second pass and the encounter of the region in the
// third. The narrators name their parent by role (instance: dynamic) and stay.
func TestLoadDirRejectsAChainOfReferences(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/global-dark-forest-world.md", blueprintText(t, "global-dark-forest-world",
		"laws_ref: laws/dark-forest-world@v1\n", "laws_ref: laws/missing-world@v1\n"))
	r := load(t, root, []string{modelE})

	if report := reportOf(t, r, "global-dark-forest-world.md"); report.State != swarm.BlueprintRejected || !slices.ContainsFunc(report.Issues, func(i agent.Issue) bool {
		return i.Field == "laws_ref" && i.Code == agent.CodeFileMissing
	}) {
		t.Errorf("global-dark-forest-world.md: %s %q, want rejected for laws_ref", report.State, renderIssues(report.Issues))
	}
	for base, parent := range map[string]string{"domain-dark-forest.md": "global-dark-forest-world", "encounter-wolf.md": "domain-dark-forest"} {
		report := reportOf(t, r, base)
		want := []string{`error|parent.name||unknown blueprint "` + parent + `"; that blueprint is in the directory, but rejected`}
		if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, want) {
			t.Errorf("%s: %s %q, want rejected %q", base, report.State, got, want)
		}
	}
	if got, want := r.Names(), []string{"group-narrator", "player-gm"}; !slices.Equal(got, want) {
		t.Errorf("Names() = %q, want %q", got, want)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{
		"blueprints": []string{"domain-dark-forest.md", "encounter-wolf.md", "global-dark-forest-world.md"},
	}})
}

// Decision 2 of §13.2: a valid blueprint of a reserved role or level is not
// spawned, and it does not degrade the swarm either.
func TestLoadDirKeepsReservedBlueprintsOut(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/city-gm.md", blueprintText(t, "domain-dark-forest",
		"name: domain-dark-forest\n", "name: city-dark-forest\n",
		"role: region-gm\n", "role: city-gm\n",
		"allowed_event_types: [region.event_occurred, npc.moved, npc.spawned, encounter.started, entity.create.proposed, entity.update.proposed]\n", "allowed_event_types: []\n",
		"owned_entity_types: [region, npc, encounter]\n", "owned_entity_types: []\n"))
	r := load(t, root, []string{modelE})

	report := reportOf(t, r, "city-gm.md")
	if got := renderIssues(report.Issues); report.State != swarm.BlueprintReserved || !slices.Equal(got, []string{"info|role||reserved role, spawn disabled"}) {
		t.Errorf("city-gm.md: %s %q, want reserved with one info", report.State, got)
	}
	if _, ok := r.Get("city-dark-forest"); ok {
		t.Error("Get(city-dark-forest): a reserved role is not spawned")
	}
	if _, ok := r.ContentHash("city-dark-forest"); ok {
		t.Error("ContentHash(city-dark-forest): a reserved blueprint is not in the registry")
	}
	if got := r.Names(); !slices.Equal(got, mvp1Blueprints) {
		t.Errorf("Names() = %q", got)
	}
	assertHealth(t, r, runtime.OK())

	// Acceptance of T-222 (review #1 Mi-3): a reserved blueprint with an error
	// is rejected, not reserved, and Health names it. The rules of domain
	// apply to the reserved role, rule 10 among them.
	writeFile(t, root, "blueprints/city-gm-broken.md", blueprintText(t, "domain-dark-forest",
		"name: domain-dark-forest\n", "name: city-dark-forest-broken\n",
		"role: region-gm\n", "role: city-gm\n",
		"allowed_event_types: [region.event_occurred, npc.moved, npc.spawned, encounter.started, entity.create.proposed, entity.update.proposed]\n", "allowed_event_types: []\n",
		"owned_entity_types: [region, npc, encounter]\n", "owned_entity_types: []\n",
		"rules_ref: rules/dark-forest.yaml\n", ""))
	r = load(t, root, []string{modelE})
	report = reportOf(t, r, "city-gm-broken.md")
	if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, []string{
		"info|role||reserved role, spawn disabled", "error|rules_ref||required for level domain",
	}) {
		t.Errorf("city-gm-broken.md: %s %q, want rejected with the error and the info", report.State, got)
	}
	if state := states(r)["city-gm.md"]; state != swarm.BlueprintReserved {
		t.Errorf("city-gm.md: %q, want reserved", state)
	}
	assertHealth(t, r, runtime.Status{Status: runtime.StatusDegraded, Details: map[string]any{"blueprints": []string{"city-gm-broken.md"}}})
}

// DoD of T-222 (acceptance of T-202): LoadDir checks owned_entity_types
// against the ownership view of blueprintenv.OwnedEntityTypes, the builder of
// mvctl blueprint validate (its own test is in blueprintenv).
func TestLoadDirChecksTheOwnershipRowOfTheLevel(t *testing.T) {
	root := projectCopy(t)
	writeFile(t, root, "blueprints/encounter-wolf.md", blueprintText(t, "encounter-wolf",
		"owned_entity_types: [encounter, npc, player]\n", "owned_entity_types: [encounter, npc, player, world]\n"))
	r := load(t, root, []string{modelE})
	report := reportOf(t, r, "encounter-wolf.md")
	want := []string{`error|owned_entity_types[3]||entity type "world" is outside the ownership row of level task`}
	if got := renderIssues(report.Issues); report.State != swarm.BlueprintRejected || !slices.Equal(got, want) {
		t.Errorf("encounter-wolf.md: %s %q, want rejected %q", report.State, got, want)
	}
}

func TestByTriggerAndContentHash(t *testing.T) {
	r := load(t, projectCopy(t), []string{modelE})
	cases := map[string][]string{
		"player.attacked":   {"player-gm"},
		"player.said":       {"player-gm"},
		"group.created":     {"group-narrator"},
		"encounter.started": {"encounter-wolf"},
		"tick.fired":        nil,
		"player":            nil,
	}
	for eventType, want := range cases {
		var got []string
		for _, bp := range r.ByTrigger(eventType) {
			got = append(got, bp.Name)
		}
		if !slices.Equal(got, want) {
			t.Errorf("ByTrigger(%q) = %q, want %q", eventType, got, want)
		}
	}
	for _, name := range mvp1Blueprints {
		bp, _ := r.Get(name)
		hash, ok := r.ContentHash(name)
		if !ok || hash != bp.ContentHash || !strings.HasPrefix(hash, "sha256:") {
			t.Errorf("ContentHash(%q) = %q, %v; want %q", name, hash, ok, bp.ContentHash)
		}
	}
	if _, ok := r.ContentHash("bard"); ok {
		t.Error("ContentHash(bard): an unknown blueprint has no hash")
	}

	names := r.Names()
	names[0] = "changed"
	reports := r.Reports()
	reports[0].Issues = append(reports[0].Issues, agent.Issue{Reason: "changed"})
	if r.Names()[0] == "changed" || len(r.Reports()[0].Issues) != 0 {
		t.Error("the registry was changed through a returned slice")
	}
}

func TestLoadDirErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "none")
	if _, err := swarm.LoadDir(missing, swarm.LoadConfig{Root: projectRoot}); err == nil || !strings.Contains(err.Error(), "read blueprints") {
		t.Errorf("missing directory: %v", err)
	}
	notDir := writeFile(t, t.TempDir(), "blueprints", []byte("x"))
	if _, err := swarm.LoadDir(notDir, swarm.LoadConfig{Root: projectRoot}); err == nil {
		t.Error("a file instead of a directory: want an error")
	}
	// A root whose blueprints/ is a file cannot give the environment.
	badRoot := t.TempDir()
	writeFile(t, badRoot, "blueprints", []byte("x"))
	if err := os.MkdirAll(filepath.Join(badRoot, "schemas", "agent"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := swarm.LoadDir(t.TempDir(), swarm.LoadConfig{Root: badRoot}); err == nil || !strings.Contains(err.Error(), "environment of the blueprint validator") {
		t.Errorf("bad root: %v", err)
	}
	// Acceptance of T-222 (review #1 Mi-4): a root without schemas/agent is not
	// a project root. The registry fails instead of rejecting every blueprint
	// of a good directory, here the one of the checkout.
	fileRoot := t.TempDir()
	writeFile(t, fileRoot, "schemas/agent", []byte("x"))
	for name, root := range map[string]string{"no schemas/agent": t.TempDir(), "schemas/agent is a file": fileRoot} {
		_, err := swarm.LoadDir(filepath.Join(projectRoot, "blueprints"), swarm.LoadConfig{Root: root, Models: []string{modelE}})
		if err == nil || !strings.Contains(err.Error(), "is not a project root") {
			t.Errorf("%s: %v, want an error of the root", name, err)
		}
	}
	// An empty directory is an empty registry, not a degraded one.
	r, err := swarm.LoadDir(t.TempDir(), swarm.LoadConfig{Root: projectRoot})
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Names()) != 0 || len(r.Reports()) != 0 {
		t.Errorf("empty directory: %q %+v", r.Names(), r.Reports())
	}
	assertHealth(t, r, runtime.OK())
}

// MV_SWARM_BLUEPRINTS_DIR names the directory LoadFromEnv reads.
func TestLoadFromEnv(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "rogue.md", []byte("not a blueprint\n"))
	t.Setenv("MV_SWARM_BLUEPRINTS_DIR", dir)
	// The root is the working directory of the process: the one of the project.
	t.Chdir(projectRoot)
	r, err := swarm.LoadFromEnv(nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Rejected(); !slices.Equal(got, []string{filepath.Join(dir, "rogue.md")}) {
		t.Errorf("Rejected() = %q, want the file of the directory from the environment", got)
	}

	missing := filepath.Join(t.TempDir(), "none")
	t.Setenv("MV_SWARM_BLUEPRINTS_DIR", missing)
	if _, err := swarm.LoadFromEnv(nil); err == nil || !strings.Contains(err.Error(), "none") {
		t.Errorf("LoadFromEnv with a missing directory: %v", err)
	}
}
