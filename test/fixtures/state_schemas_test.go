package fixtures_test

// The dictionaries and forms of the EPIC-002 schemas (T-052)
//
// A pair of payload fixtures refuses one document per type. What C-02 v1.4
// and C-14 v1.2 settled is wider than one document: a whole list of reasons,
// two lists of modes and components, and one spelling of a state hash in three
// fields. The checks below read those straight from the schema files, for the
// same reason the dictionaries of the LLM record are checked that way in
// events_test.go: an eighth reason or a hash field without its pattern would
// pass every fixture in the tree.
//
// The lists are written out here on purpose. Comparing a schema with itself
// proves nothing; what is under test is that the file says what the contract
// decided.
//
// What no schema here checks, and no fixture can: the rules of C-02 that span
// more than one field or more than one element. A package naming one entity
// twice (v1.3), the pairing of a path with a cause and a status set to the
// value it already has (v1.4) are decided by State (T-056) — JSON Schema
// cannot demand uniqueness by a nested field, and a status set twice is a
// valid proposal with an empty fact, not a malformed one.

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/shared/entity"
)

// rejectionReasonsC02 are the seven values of entity.update.rejected.reason
// (C-02 v1.4, FR-034). A broken invariant is law_violation with
// details.invariant_id; there is no reason called invariant.
var rejectionReasonsC02 = []string{
	"version_conflict", "unknown_entity", "level_violation", "law_violation",
	"invalid_op", "dead_entity", "duplicate_entity",
}

// stateHashPattern is the one spelling of a state hash on the wire: what
// entity.StateHash produces and what a recovery compares as a string
// (ADR-011 p. 2, decision 3 of system-architect#1, 2026-09-13).
const stateHashPattern = "^sha256:[0-9a-f]{64}$"

func TestStateRejectionReasonsAreTheSevenOfC02(t *testing.T) {
	got := schemaEnum(t, "entity.update.rejected.v1.json", "properties", "reason", "enum")
	if !slices.Equal(slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(rejectionReasonsC02))) {
		t.Errorf("reason = %v, want the seven values of C-02 v1.4 %v", got, rejectionReasonsC02)
	}
}

func TestReplayModesAndSnapshotComponents(t *testing.T) {
	modes := schemaEnum(t, "analytics.replay.completed.v1.json", "properties", "mode", "enum")
	if want := []string{"recovery", "test"}; !slices.Equal(slices.Sorted(slices.Values(modes)), want) {
		t.Errorf("mode = %v, want %v (C-14)", modes, want)
	}
	components := schemaEnum(t, "snapshot.created.v1.json", "properties", "component", "enum")
	if want := []string{"gateway", "state", "swarm"}; !slices.Equal(slices.Sorted(slices.Values(components)), want) {
		t.Errorf("component = %v, want %v (C-14 v1.2)", components, want)
	}
}

// TestReplayKeepsTheFieldsOfModeTest guards the fields both modes of the
// catch-up rely on. events_hash_match is the field EPIC-005 adds together with
// mode=test (ownership.md §1), so EPIC-002 may not drop or rename it while
// revising its own. identical and state_hash_before belong to the file owner:
// State publishes them in mode=recovery (state-and-mechanics.md §4.4), and the
// harness of EPIC-005 reads them in both modes.
func TestReplayKeepsTheFieldsOfModeTest(t *testing.T) {
	replay, ok := schemaNode(t, "analytics.replay.completed.v1.json", "properties", "replay", "properties").(map[string]any)
	if !ok {
		t.Fatal("analytics.replay.completed.v1.json: properties.replay.properties is not an object")
	}
	for _, field := range []string{"events_hash_match", "identical", "state_hash_before"} {
		if _, ok := replay[field]; !ok {
			t.Errorf("replay.%s is gone: both modes of the catch-up read it", field)
		}
	}
}

// TestStateHashesCarryTheOneForm holds every hash field of the EPIC-002
// schemas to the pattern, and the pattern to the code that fills them. The
// second half is what keeps the two from drifting: a schema that demanded a
// spelling entity.StateHash does not produce would refuse every snapshot.
func TestStateHashesCarryTheOneForm(t *testing.T) {
	fields := []struct {
		file string
		path []string
	}{
		{"snapshot.created.v1.json", []string{"properties", "snapshot", "properties", "state_hash", "pattern"}},
		{"analytics.replay.completed.v1.json", []string{"properties", "replay", "properties", "state_hash_before", "pattern"}},
		{"analytics.replay.completed.v1.json", []string{"properties", "replay", "properties", "state_hash_after", "pattern"}},
	}
	for _, field := range fields {
		got, ok := schemaNode(t, field.file, field.path...).(string)
		if !ok || got != stateHashPattern {
			t.Errorf("%s: %s = %v, want %q", field.file, strings.Join(field.path, "."), got, stateHashPattern)
		}
	}

	pattern := regexp.MustCompile(stateHashPattern)
	world := []*entity.Entity{{ID: "player-A", Type: "player", Version: 1, Attributes: map[string]any{"hp": 10}}}
	for _, hash := range []string{entity.StateHash(nil), entity.StateHash(world)} {
		if !pattern.MatchString(hash) {
			t.Errorf("entity.StateHash = %q does not match %s", hash, stateHashPattern)
		}
	}
}

// schemaNode reads one node of a payload schema by the path of its keys.
func schemaNode(t *testing.T, file string, path ...string) any {
	t.Helper()
	var node any
	if err := json.Unmarshal(readLF(t, repoPath("schemas", "events", file)), &node); err != nil {
		t.Fatalf("decode %s: %v", file, err)
	}
	for i, key := range path {
		object, ok := node.(map[string]any)
		if !ok {
			t.Fatalf("%s: %s is not an object", file, strings.Join(path[:i], "."))
		}
		if node, ok = object[key]; !ok {
			t.Fatalf("%s: no %s", file, strings.Join(path[:i+1], "."))
		}
	}
	return node
}
