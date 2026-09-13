// Package swarm is the swarm of game-master agents (swarm-llm-laws.md, EPIC-003).
package swarm

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"multiverse-core.io/internal/swarm/blueprintenv"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// Health detail keys and values of the registry (swarm-llm-laws.md §13.2,
// UC-029 E1).
const (
	healthBlueprints   = "blueprints"
	healthLLM          = "llm"
	healthModelMissing = agent.CodeModelMissing
)

// BlueprintState is what the registry did with one file of the directory.
type BlueprintState string

const (
	// BlueprintActive: the blueprint is valid and agents of it are spawned.
	BlueprintActive BlueprintState = "active"
	// BlueprintReserved: the blueprint is valid, but its level or role is
	// reserved (rule 14), so no agent of it is spawned in MVP-1.
	BlueprintReserved BlueprintState = "reserved"
	// BlueprintRejected: the file does not parse, has an error, or repeats the
	// name of a blueprint before it; it is not activated.
	BlueprintRejected BlueprintState = "rejected"
)

// FileReport is the verdict of the registry on one file.
type FileReport struct {
	// File is the path the file was read from.
	File string
	// Name is the name of the blueprint, empty when the file does not parse.
	Name  string
	State BlueprintState
	// Issues are the findings as the runtime reads them: a model the provider
	// does not offer is a warning here, not an error (CodeModelMissing).
	Issues []agent.Issue
}

// ModelMissing is a phase of an active blueprint whose model the provider does
// not offer. The agent runs, and the calls of the phase go to the template
// until the model appears (swarm-llm-laws.md §13.2, decision 1).
type ModelMissing struct {
	Blueprint string
	File      string
	// Phase is the phase of llm: phase1, phase2 or tick.
	Phase string
	Field string
}

// Registry is the BlueprintRegistry of the swarm (swarm-llm-laws.md §2): the
// blueprints of one directory, validated once at load. It is not changed after
// LoadDir returns, so it is safe for concurrent readers; the blueprints it
// hands out are shared and must be treated as read-only.
type Registry struct {
	active        map[string]*agent.AgentBlueprint
	names         []string
	reports       []FileReport
	modelsMissing []ModelMissing
}

// LoadConfig is what LoadDir needs besides the directory.
type LoadConfig struct {
	// Root is the project root the references of a blueprint (laws_ref,
	// rules_ref, absolute_limits_ref) and the schemas under schemas/agent are
	// resolved against.
	Root string
	// Models are the models of the provider (Provider.Models()); nil means the
	// provider is not reachable and the models are not checked.
	Models []string
}

// LoadFromEnv loads the directory of MV_SWARM_BLUEPRINTS_DIR, with the working
// directory of the process as the project root.
func LoadFromEnv(models []string) (*Registry, error) {
	return LoadDir(env.SwarmBlueprintsDir.String(), LoadConfig{Root: ".", Models: models})
}

// LoadDir reads every blueprint of dir (agent.IsBlueprintFile: .md, .yaml,
// .yml, not a README; not recursive), validates it against the project and
// activates the valid ones. A file that fails is not activated and the others
// are loaded all the same; Health then reports the failed files
// (swarm-llm-laws.md §13.2, UC-029 E1).
//
// The environment of the validator is built the way mvctl blueprint validate
// builds it (blueprintenv.ProjectEnv); the blueprint names it knows are the
// ones of dir. Three differences from the verdict of the command line are the
// runtime's:
//   - a model missing at the provider is a warning (the agent runs on the
//     template);
//   - a second blueprint with a name already taken is rejected; the name is
//     taken by the first file in the order of the files, even when that file
//     is rejected itself, so which blueprint is active never depends on the
//     validity of its neighbour;
//   - a blueprint naming a rejected one — its parent.name or its
//     encounter.child_blueprint (rules 3 and 11) — is rejected as well, and so
//     on until the set of the blueprints left stops changing. A region never
//     opens an encounter whose agent the registry does not have, and a domain
//     never runs without its world (acceptance of T-222, review #1 Mi-1). The
//     names of reserved blueprints stay known: they are valid.
//
// The error is for a directory that cannot be read at all and for a root that
// is not a project root (no schemas/agent directory): with a wrong root every
// reference of every blueprint would fail, and a mistake of configuration
// would look like a directory of bad blueprints (review #1 Mi-4).
func LoadDir(dir string, cfg LoadConfig) (*Registry, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read blueprints: %w", err)
	}
	if err := checkProjectRoot(cfg.Root); err != nil {
		return nil, err
	}
	validationEnv, err := blueprintenv.ProjectEnv(cfg.Root, cfg.Models)
	if err != nil {
		return nil, err
	}

	type parsed struct {
		file string
		bp   *agent.AgentBlueprint
		// issues are the parse errors of a file that does not parse, then the
		// findings of the latest validation.
		issues []agent.Issue
		// first are the findings of the first validation, the one against
		// every name of the directory, as the command line has them.
		first []agent.Issue
		// takenBy is the file that took the name before this one.
		takenBy  string
		rejected bool
	}
	var files []*parsed
	for _, entry := range entries {
		if !agent.IsBlueprintFile(entry) {
			continue
		}
		file := filepath.Join(dir, entry.Name())
		bp, err := agent.ParseFile(file)
		if err != nil {
			files = append(files, &parsed{file: file, issues: parseIssues(file, err), rejected: true})
			continue
		}
		files = append(files, &parsed{file: file, bp: bp})
	}
	slices.SortFunc(files, func(a, b *parsed) int { return strings.Compare(a.file, b.file) })

	takenBy := map[string]string{}
	for _, f := range files {
		if f.bp == nil || f.bp.Name == "" {
			continue
		}
		if first, taken := takenBy[f.bp.Name]; taken {
			f.takenBy = first
		} else {
			takenBy[f.bp.Name] = f.file
		}
	}

	// The set of names only shrinks from one pass to the next, and a smaller
	// set only adds findings, so a rejected file stays rejected and the loop
	// ends after at most one pass per file.
	for pass := 0; ; pass++ {
		validationEnv.Blueprints = agent.NewSet()
		for _, f := range files {
			if !f.rejected && f.bp.Name != "" {
				validationEnv.Blueprints[f.bp.Name] = struct{}{}
			}
		}
		changed := false
		for _, f := range files {
			if f.rejected {
				continue
			}
			issues := runtimeIssues(agent.Validate(f.bp, validationEnv))
			if pass == 0 {
				f.first = issues
			} else {
				issues = markRejectedReferences(f.first, issues)
			}
			if f.takenBy != "" {
				issues = append(issues, agent.Issue{
					File: f.file, Field: "name", Severity: agent.SeverityError,
					Reason: fmt.Sprintf("blueprint name %q is already taken by %s", f.bp.Name, f.takenBy),
				})
			}
			f.issues = issues
			if hasError(issues) {
				f.rejected = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	r := &Registry{active: make(map[string]*agent.AgentBlueprint)}
	for _, f := range files {
		report := FileReport{File: f.file, State: BlueprintRejected, Issues: f.issues}
		if f.bp != nil {
			report.Name = f.bp.Name
		}
		switch {
		case f.rejected:
		case agent.IsReservedLevel(f.bp.Level) || agent.IsReservedRole(f.bp.Role):
			report.State = BlueprintReserved
		default:
			report.State = BlueprintActive
			r.active[f.bp.Name] = f.bp
			r.names = append(r.names, f.bp.Name)
			r.modelsMissing = append(r.modelsMissing, modelsMissing(f.bp, report.Issues)...)
		}
		r.reports = append(r.reports, report)
	}
	slices.Sort(r.names)
	return r, nil
}

// checkProjectRoot fails a root without the directory schemas/agent: the
// references of a blueprint are resolved against the root, and without it
// every one of them would fail.
func checkProjectRoot(root string) error {
	info, err := os.Stat(filepath.Join(root, "schemas", "agent"))
	if err != nil || !info.IsDir() {
		return fmt.Errorf("blueprints: %q is not a project root: no directory schemas/agent (the references of blueprints are resolved against it)", root)
	}
	return nil
}

// reasonRejectedReference ends a finding that appears only once a blueprint of
// the directory is rejected: the reference names a file the registry has, but
// does not activate.
const reasonRejectedReference = "; that blueprint is in the directory, but rejected"

// markRejectedReferences marks the findings of a later pass that the first
// pass did not have. Between the passes only the names of the rejected
// blueprints leave the environment, so each such finding is a reference to a
// rejected blueprint (rules 3 and 11).
func markRejectedReferences(first, issues []agent.Issue) []agent.Issue {
	left := slices.Clone(first)
	out := slices.Clone(issues)
	for i, issue := range out {
		if j := slices.Index(left, issue); j >= 0 {
			left = slices.Delete(left, j, j+1)
			continue
		}
		out[i].Reason += reasonRejectedReference
	}
	return out
}

// parseIssues reports a file the parser refuses: one error per parse error,
// with its field, and the error itself when it is not a parse error (the file
// cannot be read).
func parseIssues(file string, err error) []agent.Issue {
	var errs []error
	if joined, ok := err.(interface{ Unwrap() []error }); ok {
		errs = joined.Unwrap()
	} else {
		errs = []error{err}
	}
	issues := make([]agent.Issue, 0, len(errs))
	for _, e := range errs {
		var pe *agent.ParseError
		if errors.As(e, &pe) {
			issues = append(issues, agent.Issue{File: file, Field: pe.Field, Reason: pe.Reason, Severity: agent.SeverityError})
			continue
		}
		issues = append(issues, agent.Issue{File: file, Reason: e.Error(), Severity: agent.SeverityError})
	}
	return issues
}

// runtimeIssues lowers the findings the runtime tolerates, told by their code:
// a model the provider does not offer.
func runtimeIssues(issues []agent.Issue) []agent.Issue {
	out := slices.Clone(issues)
	for i := range out {
		if out[i].Code == agent.CodeModelMissing && out[i].Severity == agent.SeverityError {
			out[i].Severity = agent.SeverityWarning
		}
	}
	return out
}

func hasError(issues []agent.Issue) bool {
	return slices.ContainsFunc(issues, func(issue agent.Issue) bool { return issue.Severity == agent.SeverityError })
}

func modelsMissing(bp *agent.AgentBlueprint, issues []agent.Issue) []ModelMissing {
	var out []ModelMissing
	for _, issue := range issues {
		if issue.Code != agent.CodeModelMissing {
			continue
		}
		phase := strings.TrimSuffix(strings.TrimPrefix(issue.Field, "llm."), ".model")
		out = append(out, ModelMissing{Blueprint: bp.Name, File: bp.SourceFile, Phase: phase, Field: issue.Field})
	}
	return out
}

// Get returns the active blueprint of a name.
func (r *Registry) Get(name string) (*agent.AgentBlueprint, bool) {
	bp, ok := r.active[name]
	return bp, ok
}

// Names returns the names of the active blueprints, sorted.
func (r *Registry) Names() []string { return slices.Clone(r.names) }

// ByTrigger returns the active blueprints with an event trigger whose
// event_name matches the event type (a glob by segments, C-11), sorted by
// name. The scope binding is not looked at: that is the Router's (§5.1).
func (r *Registry) ByTrigger(eventType string) []*agent.AgentBlueprint {
	var out []*agent.AgentBlueprint
	for _, name := range r.names {
		bp := r.active[name]
		if bp.Trigger.Type != "event" {
			continue
		}
		if ok, _ := agent.MatchEventType(bp.Trigger.EventName, eventType); ok {
			out = append(out, bp)
		}
	}
	return out
}

// ContentHash returns the content hash of an active blueprint
// ("sha256:<64 hex>"), the value of agent.spawned.content_hash.
func (r *Registry) ContentHash(name string) (string, bool) {
	bp, ok := r.active[name]
	if !ok {
		return "", false
	}
	return bp.ContentHash, true
}

// Reports returns the verdict on every file of the directory, sorted by file.
func (r *Registry) Reports() []FileReport {
	out := make([]FileReport, len(r.reports))
	for i, report := range r.reports {
		report.Issues = slices.Clone(report.Issues)
		out[i] = report
	}
	return out
}

// Rejected returns the files that were not activated, sorted.
func (r *Registry) Rejected() []string {
	var files []string
	for _, report := range r.reports {
		if report.State == BlueprintRejected {
			files = append(files, report.File)
		}
	}
	return files
}

// ModelsMissing returns the phases of active blueprints whose model the
// provider does not offer, in the order of the files.
func (r *Registry) ModelsMissing() []ModelMissing { return slices.Clone(r.modelsMissing) }

// Health is degraded while a file of the directory is not activated
// ({blueprints: [file]}, by the base name of the file) or a phase runs on the
// template for want of its model ({llm: model_missing}).
func (r *Registry) Health() runtime.Status {
	details := map[string]any{}
	if rejected := r.Rejected(); len(rejected) > 0 {
		names := make([]string, len(rejected))
		for i, file := range rejected {
			names[i] = filepath.Base(file)
		}
		details[healthBlueprints] = names
	}
	if len(r.modelsMissing) > 0 {
		details[healthLLM] = healthModelMissing
	}
	if len(details) == 0 {
		return runtime.OK()
	}
	return runtime.Status{Status: runtime.StatusDegraded, Details: details}
}
