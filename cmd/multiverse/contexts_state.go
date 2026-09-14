package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
)

// stateContext is the name the context of internal/state answers to.
const stateContext = state.Name

// Factories of the contexts of EPIC-002. The owner changes the value on the
// right and nothing else; the name and the start order stay in contexts.go.
var (
	// The worlds of state come from MV_STATE_WORLDS when the process starts it.
	newStateContext     = newStateWithTheLaws
	newMechanicsContext = newStub("mechanics")
)

// newStateWithTheLaws builds the context state held to the laws of the rule
// book MV_RULES_PATH names (NFR-020, state-and-mechanics.md §4.5 p. 8; EPIC-002
// design.md §4.3). State without the laws does not start: a book that cannot be
// read or compiled gives a context that refuses at Start and says which file,
// read from where, and why (T-471). Running on without the laws would let a
// proposal through that breaks them — a value of the wrong kind at the root of
// hp, a character moved to a region that does not exist — on a process whose
// /health is ok.
//
// The book is read here, when serve builds the contexts, and not in an init,
// for the reason newSwarm gives: the value is the operator's, not the linker's.
func newStateWithTheLaws() runtime.Context {
	rules, err := loadTheRules(env.RulesPath.String())
	if err != nil {
		return lawless{err: err}
	}
	return state.New(state.Config{Invariants: rules.Invariants()})
}

// loadTheRules loads the rule book of the process. A relative path is relative
// to the working directory of the process, and the refusal names that directory
// too: "open rules/dark-forest.yaml" alone does not say where it was looked for.
func loadTheRules(path string) (*mechanics.Rules, error) {
	rules, err := mechanics.Load(path)
	if err == nil {
		return rules, nil
	}
	where := ""
	if !filepath.IsAbs(path) {
		if wd, wdErr := os.Getwd(); wdErr == nil {
			where = fmt.Sprintf(" (relative to the working directory %s)", wd)
		}
	}
	return nil, fmt.Errorf("state: the laws of the world are not loaded from %s=%q%s, "+
		"and state does not run without them: %w", env.RulesPath.Name(), path, where, err)
}

// lawless stands in for the context state whose rule book could not be loaded.
// It refuses to start and reports the refusal on /health.
type lawless struct{ err error }

func (l lawless) Name() string        { return stateContext }
func (l lawless) DependsOn() []string { return nil }

// Start returns the error as it is: runtime.StartAll names the context, and the
// error names the variable, the path and the cause.
func (l lawless) Start(context.Context, runtime.Deps) error { return l.err }
func (l lawless) Stop(context.Context) error                { return nil }
func (l lawless) Health() runtime.Status {
	return runtime.Status{Status: runtime.StatusFail, Details: map[string]any{"err": l.err.Error()}}
}

var _ runtime.Context = lawless{}
