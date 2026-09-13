package main

import (
	"strings"
	"testing"

	"multiverse-core.io/shared/env"
)

// The variable of an owner is found by the name of its slot, not by its
// position: a factory put under the wrong name — laws building llm — would pass
// the order test and start the wrong context in the right place.
func TestEveryFactoryBuildsTheContextOfItsName(t *testing.T) {
	clearVar(t, env.SwarmFake.Name())
	for _, name := range platformContexts {
		if got := factoryOf(name)().Name(); got != name {
			t.Errorf("the factory registered as %q builds %q", name, got)
		}
	}
}

// A name of the start order without a variable of its owner stops the binary
// at start, with the name, instead of registering a nil factory that panics
// only when --contexts asks for it.
func TestANameWithoutAFactoryStopsTheStart(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("factoryOf(unknown) returned, want a panic")
		}
		if msg, _ := r.(string); !strings.Contains(msg, `"replay"`) {
			t.Errorf("panic = %v, want it to name the context", r)
		}
	}()
	factoryOf("replay")
}
