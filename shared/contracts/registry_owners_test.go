package contracts

import (
	"slices"
	"testing"
)

// Every list of registry_<owner>.go holds the types of its owner only (T-446,
// contracts.md §16 p. 8): a line added to the file of another epic would be
// reviewed and merged by the wrong team, and the next owner looking for its
// type would not find it where the rule says it is.
func TestOwnerFilesHoldTheirOwnTypes(t *testing.T) {
	lists := []struct {
		file  string
		owner string
		specs []Spec
	}{
		{"registry_gateway.go (gatewayDefinitions)", OwnerGateway, gatewayDefinitions},
		{"registry_gateway.go (gatewayAnalyticsDefinitions)", OwnerGateway, gatewayAnalyticsDefinitions},
		{"registry_state.go", OwnerState, stateDefinitions},
		{"registry_swarm.go", OwnerSwarm, swarmDefinitions},
		{"registry_ops.go", OwnerOps, opsDefinitions},
		{"registry.go (legacyDefinitions)", OwnerFoundation, legacyDefinitions},
	}
	total := 0
	for _, list := range lists {
		for _, spec := range list.specs {
			if spec.Owner != list.owner {
				t.Errorf("%s: %s is owned by %s, want %s", list.file, spec.Type, spec.Owner, list.owner)
			}
		}
		total += len(list.specs)
	}
	if total != len(definitions) {
		t.Errorf("the owner lists hold %d types and the registry %d: a list is missing from definitions", total, len(definitions))
	}
}

// All() keeps the declaration order the registry had before the split into
// owner files, which is also the order of the findings of mvctl contracts
// check: the lists follow one another in the order below, and each keeps its
// own (review #1 of T-446, N-1). The order is pinned by the lists and not by a
// golden list of every type: an owner adds the line of its type to its own
// file and nothing else (ownership.md v0.6 §3 p. 6), and a golden list here
// would make each such line an edit of a test of EPIC-001 in three branches at
// once.
func TestAllKeepsTheOrderOfTheOwnerLists(t *testing.T) {
	var want []string
	for _, list := range [][]Spec{
		gatewayDefinitions,
		stateDefinitions,
		swarmDefinitions,
		gatewayAnalyticsDefinitions,
		opsDefinitions,
		legacyDefinitions,
	} {
		for _, spec := range list {
			want = append(want, spec.Type)
		}
	}
	var got []string
	for _, spec := range Default().All() {
		got = append(got, spec.Type)
	}
	if !slices.Equal(got, want) {
		t.Errorf("All() lists the types in the order\n%q\nwant the owner lists one after another\n%q", got, want)
	}
}
