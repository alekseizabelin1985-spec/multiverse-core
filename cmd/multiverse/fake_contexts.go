package main

// This file is the temporary hook of I1-α and is removed by T-256 (the
// criterion of the mvp-1/i1 tag).
//
// It is the only file of the binary that imports shared/testkit/swarm, and it
// is a file of its own for that reason: .golangci.yml lets exactly this file
// import exactly that package (ADR-001 addendum p. 8, T-255).

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"multiverse-core.io/shared/env"
	"multiverse-core.io/shared/runtime"
	"multiverse-core.io/shared/testkit/swarm"
)

// newSwarm builds the context swarm. With MV_SWARM_FAKE=true it is the Phase 1
// stub of shared/testkit/swarm — FakeEncounter and FakeNarrator under the name
// swarm; otherwise it is what swarm is without the flag.
//
// The flag is read here, when serve builds the contexts of the process, and not
// in an init: a value read at init is the value of whoever linked the binary,
// and a test or an operator setting the variable afterwards would be ignored.
//
// Today "without the flag" is the /health stub of contexts.go. Once
// internal/swarm exists its constructor takes the place of that stub on the
// last line, and internal/swarm must not register itself while this hook does:
// runtime.Register panics on a second registration of one name.
//
// The stub gets no fixture directory, because the process has none: the world
// is initialised from testdata/fixtures by mvctl world init --fixtures
// (EPIC-002, state-and-mechanics.md §4.10), and the stub learns it from the
// entity.created facts on the bus, as the swarm will. Its rule book is the
// default of the package, rules/dark-forest.yaml under the working directory.
func newSwarm() runtime.Context {
	fake, err := env.SwarmFake.Bool()
	if err != nil {
		return refused{name: swarmContext, err: err}
	}
	if fake {
		return flagged{swarm.NewFakeContext(swarm.ContextConfig{})}
	}
	return stub{name: swarmContext}
}

// flagged is the fake as the flag mounts it: the fake itself, plus a refusal
// that names the setting which asked for it. What the fake reads at start is
// rules/dark-forest.yaml under the working directory, not the book
// MV_RULES_PATH names for state; the image of the platform carries it there
// (T-471), a process started elsewhere may not, and "open
// rules/dark-forest.yaml" alone does not tell an operator that it is
// MV_SWARM_FAKE=true that needs it (review #1 of T-255, Mi-1).
type flagged struct{ *swarm.FakeContext }

func (f flagged) Start(ctx context.Context, deps runtime.Deps) error {
	err := f.FakeContext.Start(ctx, deps)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, fs.ErrNotExist):
		return fmt.Errorf("%s=true: the stub reads rules/dark-forest.yaml from the working directory of the "+
			"process, not the path %s names, and the file is not there: %w",
			env.SwarmFake.Name(), env.RulesPath.Name(), err)
	default:
		return fmt.Errorf("%s=true: %w", env.SwarmFake.Name(), err)
	}
}

// refused stands in for a context whose configuration cannot be read. A flag
// that says neither true nor false is refused at start rather than read as
// false: an operator who asked for the fake and got the stub in silence would
// see a healthy process that never fights.
type refused struct {
	name string
	err  error
}

func (r refused) Name() string        { return r.name }
func (r refused) DependsOn() []string { return nil }

// Start returns the error as it is: runtime.StartAll already names the
// context, and the error of shared/env names the variable and its value.
func (r refused) Start(context.Context, runtime.Deps) error { return r.err }
func (r refused) Stop(context.Context) error                { return nil }
func (r refused) Health() runtime.Status {
	return runtime.Status{Status: runtime.StatusFail, Details: map[string]any{"err": r.err.Error()}}
}

var (
	_ runtime.Context = flagged{}
	_ runtime.Context = refused{}
)
