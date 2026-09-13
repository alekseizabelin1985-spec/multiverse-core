package main

// Factory of the context of EPIC-005. The owner changes the value on the right
// and nothing else; the name and the start order stay in contexts.go.
var newMemoryContext = newStub("memory")
