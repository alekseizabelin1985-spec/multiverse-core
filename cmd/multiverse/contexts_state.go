package main

// Factories of the contexts of EPIC-002. The owner changes the value on the
// right and nothing else; the name and the start order stay in contexts.go.
var (
	newStateContext     = newStub("state")
	newMechanicsContext = newStub("mechanics")
)
