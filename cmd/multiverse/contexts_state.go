package main

import (
	"multiverse-core.io/internal/state"
	"multiverse-core.io/shared/runtime"
)

// stateContext is the name the context of internal/state answers to.
const stateContext = state.Name

// newStateContext is the factory of the context state: the worlds it serves
// come from MV_STATE_WORLDS when the process starts it, not when the binary is
// linked.
func newStateContext() runtime.Context { return state.New(state.Config{}) }
