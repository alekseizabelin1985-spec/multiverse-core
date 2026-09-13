package contracts

// Types of EPIC-005. The owner adds the line of its type here, reviewed by the
// system architect (contracts.md §16 p. 8); the order of the lists is in
// registry.go.
var opsDefinitions = []Spec{
	// The operator CLI publishes what its audit found (C-10). Nothing here is
	// read during a replay.
	analyticsEvent("analytics.consistency.violated", OwnerOps,
		[]string{SourceMvctl}, []string{SourceMvctl}),
}
