package blueprintenv_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/internal/swarm/blueprintenv"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/contracts"
)

const projectRoot = "../../.."

// DoD of T-222 (acceptance of T-202): the ownership view comes from
// contracts.OwnershipRules through one builder, the one ProjectEnv gives the
// validator.
func TestOwnedEntityTypesIsTheRowOfTheLevel(t *testing.T) {
	for _, level := range agent.LevelNames() {
		var want []string
		for _, rule := range contracts.OwnershipRules() {
			if rule.Proposer == level {
				for _, typ := range rule.EntityTypes {
					if typ != contracts.AnyType && !slices.Contains(want, typ) {
						want = append(want, typ)
					}
				}
			}
		}
		slices.Sort(want)
		got := blueprintenv.OwnedEntityTypes(level, "")
		if !slices.Equal(got, want) {
			t.Errorf("OwnedEntityTypes(%q) = %q, want %q", level, got, want)
		}
		if slices.Contains(got, contracts.AnyType) {
			t.Errorf("OwnedEntityTypes(%q) holds the wildcard", level)
		}
	}
	if got := blueprintenv.OwnedEntityTypes("gateway", ""); slices.Contains(got, contracts.AnyType) {
		t.Errorf("OwnedEntityTypes(gateway) = %q: a wildcard is not a type", got)
	}
	if got := blueprintenv.OwnedEntityTypes("author", ""); len(got) != 0 {
		t.Errorf("OwnedEntityTypes(author) = %q: a row of any type owns no named type", got)
	}
	validationEnv, err := blueprintenv.ProjectEnv(projectRoot, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := validationEnv.OwnedEntityTypes("task", "encounter"), blueprintenv.OwnedEntityTypes("task", "encounter"); !slices.Equal(got, want) {
		t.Errorf("ProjectEnv().OwnedEntityTypes = %q, want the builder %q", got, want)
	}
}

// ProjectEnv hands the validator every event type of the registry, every
// invariant of mechanics and the models as given.
func TestProjectEnv(t *testing.T) {
	validationEnv, err := blueprintenv.ProjectEnv(projectRoot, []string{"model-x"})
	if err != nil {
		t.Fatal(err)
	}
	for _, spec := range contracts.All() {
		if !validationEnv.EventTypes.Has(spec.Type) {
			t.Errorf("event type %q of the registry is not in the environment", spec.Type)
		}
	}
	for _, id := range mechanics.InvariantIDs() {
		if !validationEnv.Invariants.Has(id) {
			t.Errorf("invariant %q of mechanics is not in the environment", id)
		}
	}
	if !validationEnv.Models.Has("model-x") || len(validationEnv.Models) != 1 {
		t.Errorf("Models = %v", validationEnv.Models)
	}
	if !validationEnv.Blueprints.Has("player-gm") || !validationEnv.Schemas.Has("schemas/agent/narrative.json") {
		t.Errorf("blueprints %v, schemas %v: want the ones of the checkout", validationEnv.Blueprints, validationEnv.Schemas)
	}

	badRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(badRoot, "blueprints"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := blueprintenv.ProjectEnv(badRoot, nil); err == nil || !strings.Contains(err.Error(), "environment of the blueprint validator") {
		t.Errorf("a root whose blueprints/ is a file: %v", err)
	}
}
