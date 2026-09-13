package main

// Factories of the contexts of EPIC-003. The owner changes the values on the
// right and nothing else; the names and the start order stay in contexts.go.
//
// llm, laws and swarm of one process are to share one gateway of the LLM and
// one service of laws, or the budget windows and the accounting split between
// two instances (ADR-001 addendum 2026-09-13 p. 7). runtime.Deps carries no
// such instance, so the stack is built once, in this file.
//
// swarm is the hook of I1-α until T-256: newSwarm of fake_contexts.go.
var (
	newLLMContext   = newStub("llm")
	newLawsContext  = newStub("laws")
	newSwarmContext = newSwarm
)
