// Package blueprintenv builds the environment of the blueprint validator for
// a project checkout. It is the one builder of the runtime (the registry of
// internal/swarm) and of mvctl blueprint validate (T-204), so that both reach
// the same verdict (swarm-llm-laws.md §13.2).
//
// It is a leaf package of the swarm on purpose: it imports shared/agent,
// shared/contracts and internal/mechanics and nothing of the runtime of the
// swarm, so the command line does not link the scheduler, the gateway or the
// laws to validate a directory (acceptance of T-222).
package blueprintenv

import (
	"fmt"
	"slices"

	"multiverse-core.io/internal/mechanics"
	"multiverse-core.io/shared/agent"
	"multiverse-core.io/shared/contracts"
)

// ProjectEnv builds the environment of the blueprint validator for a project
// root: the event types of the contract registry, the ownership view of
// OwnedEntityTypes, the invariant checks of mechanics, the blueprints of
// root/blueprints, the schemas of root/schemas/agent and the models of the
// provider (nil: not checked).
func ProjectEnv(root string, models []string) (agent.ValidationEnv, error) {
	specs := contracts.All()
	types := make([]string, 0, len(specs))
	for _, spec := range specs {
		types = append(types, spec.Type)
	}
	validationEnv, err := agent.EnvFromProject(root, types, OwnedEntityTypes, mechanics.InvariantIDs(), models)
	if err != nil {
		return agent.ValidationEnv{}, fmt.Errorf("environment of the blueprint validator: %w", err)
	}
	return validationEnv, nil
}

// OwnedEntityTypes is the view of the ownership table the validator gets: the
// entity types of the rows of contracts.OwnershipRules whose proposer is the
// level, sorted and without repeats. The role does not narrow the row
// (swarm-llm-laws.md §13.2, decision 3), and a wildcard is not a type: no row
// of a level of the swarm has one, and a row that gained it would not become
// "owns every type" through this view.
func OwnedEntityTypes(level, _ string) []string {
	var owned []string
	for _, rule := range contracts.OwnershipRules() {
		if rule.Proposer != level {
			continue
		}
		for _, t := range rule.EntityTypes {
			if t != contracts.AnyType {
				owned = append(owned, t)
			}
		}
	}
	slices.Sort(owned)
	return slices.Compact(owned)
}
