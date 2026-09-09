package main

import (
	"context"

	"multiverse-core.io/shared/runtime"
)

// Wave 0 registers empty contexts so that --contexts=all and the compose
// profiles work before internal/* exists. Each owner replaces its stub with a
// real runtime.Register in its own branch (design.md §4.1):
// state, mechanics, replay — EPIC-002; swarm, llm, laws — EPIC-003;
// gateway — EPIC-004; memory — EPIC-005.
//
// The order below is the documented start order of the platform
// (foundation.md §2): contexts without dependencies keep it as a tie-break.
var stubContexts = []string{"state", "laws", "mechanics", "llm", "swarm", "gateway", "memory"}

func init() {
	for _, name := range stubContexts {
		runtime.Register(name, newStub(name))
	}
}

func newStub(name string) func() runtime.Context {
	return func() runtime.Context { return stub{name: name} }
}

// stub is a context that exists only to answer /health until its owner
// implements it.
type stub struct{ name string }

func (s stub) Name() string                              { return s.name }
func (s stub) DependsOn() []string                       { return nil }
func (s stub) Start(context.Context, runtime.Deps) error { return nil }
func (s stub) Stop(context.Context) error                { return nil }
func (s stub) Health() runtime.Status                    { return runtime.OK() }
