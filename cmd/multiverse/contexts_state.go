package main

import (
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/runtime"
)

// stateContext is the name the context of internal/state answers to.
const stateContext = state.Name

// Factories of the contexts of EPIC-002. The owner changes the value on the
// right and nothing else; the name and the start order stay in contexts.go.
var (
	// The worlds of state come from MV_STATE_WORLDS when the process starts it.
	newStateContext     = func() runtime.Context { return state.New(state.Config{}) }
	newMechanicsContext = newStub("mechanics")
)
