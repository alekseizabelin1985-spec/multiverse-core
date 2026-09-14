package contracts

import (
	"slices"
	"testing"
)

// TestOwnershipRulesCoverEveryProposer checks the ownership table against
// state-and-mechanics.md §4.6: the three agent levels, the three non-agent
// proposers, and the two levels C-13 reserves without giving them anything.
//
// The table in ownership.go is the only source (ADR-025): there is no second
// table and no equality test, so this test is what keeps the table honest.
func TestOwnershipRulesCoverEveryProposer(t *testing.T) {
	rules := OwnershipRules()

	seen := map[string]int{}
	for _, rule := range rules {
		seen[rule.Proposer]++
	}
	for _, proposer := range []string{
		ProposerGateway, ProposerTask, ProposerDomain, ProposerGlobal,
		ProposerAuthor, ProposerSystem, ProposerObject, ProposerMonitor,
	} {
		if seen[proposer] == 0 {
			t.Errorf("%s has no row in the ownership table", proposer)
		}
	}

	// The encounter agent and the personal GM are both level task; the
	// personal GM proposes nothing, so the task rows are the encounter ones.
	for _, rule := range rules {
		if rule.Proposer != ProposerTask {
			continue
		}
		if len(rule.Paths) == 0 || len(rule.Causes) == 0 {
			t.Errorf("task rule for %v is empty", rule.EntityTypes)
		}
	}

	// object and monitor are reserved and empty: an empty rule refuses
	// everything, which is what a disabled spawn flag must mean. system is
	// reserved and empty as well (C-02 v1.8 p. 6): it has no publisher in
	// MVP-1, the bootstrap of a world proposes as the author, and a row of "*"
	// would hand the rights of the author to the first envelope matched to it.
	reserved := map[string]string{
		ProposerObject:  "until C-13 is implemented",
		ProposerMonitor: "until C-13 is implemented",
		ProposerSystem:  "until a system mechanism brings a row of its own (C-02 v1.8 p. 6)",
	}
	for _, rule := range rules {
		until, ok := reserved[rule.Proposer]
		if !ok {
			continue
		}
		if len(rule.EntityTypes) != 0 || len(rule.Paths) != 0 || len(rule.Causes) != 0 || rule.Create {
			t.Errorf("%s must stay empty %s: %+v", rule.Proposer, until, rule)
		}
	}
}

// TestGatewayRow pins the row of C-02 v1.2: the gateway is the only proposer
// of the transition alive -> abandoned and of the leader of a group.
func TestGatewayRow(t *testing.T) {
	player, ok := ruleFor(ProposerGateway, "player")
	if !ok {
		t.Fatal("the gateway has no rule for a player")
	}
	if !slices.Contains(player.Paths, "status") {
		t.Error("the gateway cannot set status: the /forget cascade (C-02 v1.2) would be refused")
	}
	if !slices.Contains(player.Causes, "forget") {
		t.Error("cause=forget is missing from the gateway rule (C-02 v1.2)")
	}
	if !slices.Contains(player.Causes, "rest") || !slices.Contains(player.Paths, "hp") {
		t.Error("the gateway proposes rest with set hp = hp_max (C-02 v1.1)")
	}
	if !player.Create {
		t.Error("the gateway creates players")
	}

	group, ok := ruleFor(ProposerGateway, "group")
	if !ok {
		t.Fatal("the gateway has no rule for a group")
	}
	// leader_id, including the value null when no living member is left
	// (З-4), is covered by the wildcard of §4.6.
	if !slices.Contains(group.Paths, AnyPath) {
		t.Errorf("the group rule lost its wildcard: %v", group.Paths)
	}
	if !slices.Contains(group.Causes, "forget") {
		t.Error("cause=forget is missing from the group rule (C-04 v1.1)")
	}
}

// TestOwnershipRulesAreACopy makes sure a caller cannot rewrite the table
// through the slice it is given: State reads this copy on every proposal.
func TestOwnershipRulesAreACopy(t *testing.T) {
	first := OwnershipRules()
	first[0].Proposer = "attacker"
	first[0].Paths[0] = "*"

	second := OwnershipRules()
	if second[0].Proposer != ProposerGateway {
		t.Errorf("the table was rewritten through the returned slice: %s", second[0].Proposer)
	}
	if second[0].Paths[0] != "position" {
		t.Errorf("the paths were rewritten through the returned slice: %v", second[0].Paths)
	}
}

// TestCauseEnumCoversOwnershipRules is the sentinel of ОВ-20: the ownership
// table and the schemas of the same package must not disagree. A cause a row
// of §4.6 allows but the enum of §2.3.4 omits means State is allowed to
// propose what the registry then refuses to publish — a deadlock nobody sees
// until the run.
//
// The check is one-directional on purpose: the enum is the union of the two
// documents, so it also carries causes no rule names (bootstrap).
func TestCauseEnumCoversOwnershipRules(t *testing.T) {
	// The types of C-02 whose payload carries cause; entity.created and
	// entity.update.rejected have none (api-contracts §2.3.4).
	types := []string{"entity.create.proposed", "entity.update.proposed", "entity.updated"}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			spec, ok := Lookup(typ)
			if !ok {
				t.Fatalf("%s is not registered", typ)
			}
			doc := readSchemaDoc(t, SchemaFile(typ, spec.SchemaVersion))
			properties, _ := doc["properties"].(map[string]any)
			cause, ok := properties["cause"].(map[string]any)
			if !ok {
				t.Fatalf("%s: no cause field", typ)
			}
			values, ok := cause["enum"].([]any)
			if !ok {
				t.Fatalf("%s: cause has no enum", typ)
			}
			allowed := make([]string, 0, len(values))
			for _, value := range values {
				allowed = append(allowed, value.(string))
			}
			for _, rule := range OwnershipRules() {
				for _, want := range rule.Causes {
					if want == AnyCause {
						continue
					}
					if !slices.Contains(allowed, want) {
						t.Errorf("proposer %s may use cause=%s, the schema of %s refuses it", rule.Proposer, want, typ)
					}
				}
			}
		})
	}
}

func ruleFor(proposer, entityType string) (OwnershipRule, bool) {
	for _, rule := range OwnershipRules() {
		if rule.Proposer == proposer && slices.Contains(rule.EntityTypes, entityType) {
			return rule, true
		}
	}
	return OwnershipRule{}, false
}
