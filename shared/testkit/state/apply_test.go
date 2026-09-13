package state_test

import (
	"context"
	"encoding/json"
	"testing"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit/state"
)

// apply hands one event to the stub directly. Everything about a decision is
// tested this way: the wiring is covered once, by the cycle test, and a
// decision does not need a goroutine to be checked.
func apply(t *testing.T, s *state.FakeState, ev eventbus.Event) {
	t.Helper()
	if err := s.Apply(context.Background(), ev); err != nil {
		t.Fatalf("apply %s: %v", ev.Type, err)
	}
}

// applyAt is the same call for an event that arrived at a known offset of
// system_events, the way a subscription hands one over.
func applyAt(t *testing.T, s *state.FakeState, offset int64, ev eventbus.Event) {
	t.Helper()
	ctx := eventbus.ContextWithPosition(context.Background(),
		eventbus.Position{Topic: eventbus.TopicSystemEvents, Offset: offset})
	if err := s.Apply(ctx, ev); err != nil {
		t.Fatalf("apply %s: %v", ev.Type, err)
	}
}

// refusalsOf reads every entity.update.rejected published so far.
func refusalsOf(t *testing.T, bus busReader) []eventbus.Event {
	t.Helper()
	return ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeRejected)
}

// factsOf reads every entity.updated published so far.
func factsOf(t *testing.T, bus busReader) []eventbus.Event {
	t.Helper()
	return ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeUpdated)
}

// --- the matrix of refusals ---

// TestRejectionMatrix walks the reasons the double answers without the laws of
// the world; level_violation of the transition into abandoned and law_violation
// have tests of their own below. The ownership table is not the double's
// (package doc), so a proposal that only the table would refuse is applied here.
func TestRejectionMatrix(t *testing.T) {
	t.Run(state.ReasonUnknownEntity, func(t *testing.T) {
		fake, bus, _ := world(t)
		apply(t, fake, proposal("prop-1", "move", true,
			changeSet(playerRef("player-Z"), "", nil,
				entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))
		assertRefusal(t, bus, "prop-1", state.ReasonUnknownEntity, "player-Z", nil)
	})

	t.Run(state.ReasonVersionConflict, func(t *testing.T) {
		fake, bus, _ := world(t)
		stale := int64(99)
		apply(t, fake, proposal("prop-2", "combat", true,
			changeSet(playerRef(playerA), "Вася", &stale,
				entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})))
		assertRefusal(t, bus, "prop-2", state.ReasonVersionConflict, playerA, map[string]float64{
			"expected_version": 99, "actual_version": 1,
		})
		if hp, _ := mustGet(t, fake, playerA).HP(); hp != 10 {
			t.Errorf("hp %d: a refused proposal changed the world", hp)
		}
	})

	t.Run(state.ReasonInvalidOp, func(t *testing.T) {
		fake, bus, _ := world(t)
		// version is a field of the entity, not an attribute: only State moves
		// it, and an operation that tries is malformed however well-formed the
		// JSON is (entity.ReservedPaths).
		apply(t, fake, proposal("prop-3", "combat", true,
			changeSet(playerRef(playerA), "Вася", nil,
				entity.Op{Op: entity.OpSet, Path: "version", Value: 5})))
		assertRefusal(t, bus, "prop-3", state.ReasonInvalidOp, playerA, nil)
	})

	t.Run(state.ReasonDuplicateEntity, func(t *testing.T) {
		fake, bus, _ := world(t)
		apply(t, fake, createProposal("prop-4", playerA, "Вася"))
		assertRefusal(t, bus, "prop-4", state.ReasonDuplicateEntity, playerA, nil)
		if len(ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeCreated)) != 0 {
			t.Error("a duplicate create still announced an entity")
		}
	})

	t.Run(state.ReasonDeadEntity, func(t *testing.T) {
		fake, bus, _ := world(t)
		kill(t, fake, npcID, entity.TypeNPC)
		apply(t, fake, proposal("prop-5", "combat", true,
			changeSet(entity.Ref{ID: npcID, Type: entity.TypeNPC}, "", nil,
				entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})))
		assertRefusal(t, bus, "prop-5", state.ReasonDeadEntity, npcID, nil)
	})
}

// TestACorpseStillAcceptsTheFourPathsOfDeath covers the exception of §4.5 p. 5:
// without it a trophy could never be handed out, because the NPC it is taken
// from is dead by definition (inv-03).
func TestACorpseStillAcceptsTheFourPathsOfDeath(t *testing.T) {
	fake, bus, _ := world(t)
	kill(t, fake, npcID, entity.TypeNPC)
	before := len(factsOf(t, bus))

	apply(t, fake, proposal("prop-loot", "loot", true,
		changeSet(entity.Ref{ID: npcID, Type: entity.TypeNPC}, "", nil,
			entity.Op{Op: entity.OpSet, Path: "loot_claimed_by", Value: playerA},
			entity.Op{Op: entity.OpSet, Path: entity.AttrKilledBy, Value: playerA})))

	if refusals := refusalsOf(t, bus); len(refusals) > 0 {
		reason, _ := refusals[len(refusals)-1].Path().GetString("reason")
		t.Fatalf("the trophy of a dead wolf was refused: %s", reason)
	}
	if have := len(factsOf(t, bus)); have != before+1 {
		t.Fatalf("%d facts, want %d", have, before+1)
	}
	if claimed, _ := mustGet(t, fake, npcID).AttrString("loot_claimed_by"); claimed != playerA {
		t.Errorf("loot_claimed_by %q, want %q", claimed, playerA)
	}
}

// --- the abandoned character (C-02 v1.2, З-2) ---

// TestForgetAbandonsALivingCharacterOnce is the addendum of сведение 3: the
// gateway proposes the one transition alive -> abandoned in the cascade of
// /forget, the stub applies it, and the status is terminal afterwards — a
// second attempt is refused like an attempt on a corpse.
//
// The stub does not publish narrative.output kind=death for it, and this test
// says so: an abandoned character leaves the world without an obituary
// (C-02 v1.2).
func TestForgetAbandonsALivingCharacterOnce(t *testing.T) {
	fake, bus, _ := world(t)
	version := versionOf(t, fake, playerA)

	apply(t, fake, forget("prop-forget", playerA, version))

	facts := factsOf(t, bus)
	if len(facts) != 1 {
		t.Fatalf("%d facts, want one entity.updated", len(facts))
	}
	fact := facts[0]
	if err := contracts.Validate(fact); err != nil {
		t.Fatalf("the fact is not valid: %v", err)
	}
	if cause, _ := fact.Path().GetString("cause"); cause != "forget" {
		t.Errorf("cause %q, want forget", cause)
	}
	changed, ok := fact.Path().GetSlice("changed")
	if !ok || len(changed) != 1 {
		t.Fatalf("changed %v, want the one path of the transition", changed)
	}
	entry, _ := changed[0].(map[string]any)
	if entry["path"] != entity.AttrStatus ||
		entry["old"] != entity.StatusAlive || entry["new"] != entity.StatusAbandoned {
		t.Errorf("changed %+v, want status %s -> %s", entry, entity.StatusAlive, entity.StatusAbandoned)
	}
	if status, _ := mustGet(t, fake, playerA).Status(); status != entity.StatusAbandoned {
		t.Errorf("status %q after /forget", status)
	}
	if narratives := events(t, bus, eventbus.TopicNarrativeOutput); len(narratives) != 0 {
		t.Errorf("%d narratives: an abandoned character gets no kind=death (C-02 v1.2)", len(narratives))
	}

	// Terminal means terminal: the second /forget is refused, and so would be
	// any other change outside the four paths of a corpse.
	apply(t, fake, forget("prop-forget-again", playerA, versionOf(t, fake, playerA)))
	assertRefusal(t, bus, "prop-forget-again", state.ReasonDeadEntity, playerA, nil)
	if have := len(factsOf(t, bus)); have != 1 {
		t.Errorf("%d facts after the second /forget, want the one from the first", have)
	}
}

// TestForgetOverADeadCharacterIsRefused covers the decision of З-2 against the
// recommendation it replaced: dead does not become abandoned. dead is terminal
// by FR-023 and is already out of every scope and target list, so the rule of
// dead_entity is left alone.
func TestForgetOverADeadCharacterIsRefused(t *testing.T) {
	fake, bus, _ := world(t)
	kill(t, fake, playerA, entity.TypePlayer)

	apply(t, fake, forget("prop-forget-dead", playerA, versionOf(t, fake, playerA)))
	assertRefusal(t, bus, "prop-forget-dead", state.ReasonDeadEntity, playerA, nil)
	if status, _ := mustGet(t, fake, playerA).Status(); status != entity.StatusDead {
		t.Errorf("status %q: a dead character stayed dead", status)
	}
}

// TestAbandonedNeedsTheCauseOfForget covers the other half of the rule C-02
// v1.2 and З-2 give the transition alive -> abandoned: it is the /forget of the
// gateway, and nothing else ends a living character that way. The pair
// "status, not forget" is not the proposer's to write, so it is level_violation
// (C-02 v1.5; the stand-in of T-017 answered invalid_op, "Код расходится").
//
// The status the matrix does not know at all (data-model.md §3.3, checked
// through entity.StatusTransitionAllowed) stays invalid_op: it is a move no
// proposer could make.
func TestAbandonedNeedsTheCauseOfForget(t *testing.T) {
	t.Run("abandoned with another cause", func(t *testing.T) {
		fake, bus, _ := world(t)
		apply(t, fake, proposal("prop-abandon-combat", "combat", true,
			changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
				entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusAbandoned})))

		assertRefusal(t, bus, "prop-abandon-combat", state.ReasonLevelViolation, playerA, nil)
		if facts := factsOf(t, bus); len(facts) != 0 {
			t.Errorf("%d facts: a character was abandoned by a fight", len(facts))
		}
		if status, _ := mustGet(t, fake, playerA).Status(); status != entity.StatusAlive {
			t.Errorf("status %q, want the character still alive", status)
		}
	})

	t.Run("a status the matrix does not know", func(t *testing.T) {
		fake, bus, _ := world(t)
		apply(t, fake, proposal("prop-status-invented", "combat", true,
			changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
				entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: "sleeping"})))

		assertRefusal(t, bus, "prop-status-invented", state.ReasonInvalidOp, playerA, nil)
		if status, _ := mustGet(t, fake, playerA).Status(); status != entity.StatusAlive {
			t.Errorf("status %q: a status outside data-model.md §3.3 was applied", status)
		}
	})
}

// --- one entity, one change set ---

// TestOneEntityNamedTwiceIsRefused is the answer to a package that names an
// entity twice (review #1 of T-017, Major-1). Applied one after the other the
// two sets publish two entity.updated under one version, which C-02 forbids
// (the version moves strictly by one per entity) and §4.5 p. 12 forbids again
// (one fact per entity); applied as one, the first set is silently lost. The
// stub refuses the package instead, whether or not it is atomic: the proposer
// has to merge the operations into a single change set.
func TestOneEntityNamedTwiceIsRefused(t *testing.T) {
	for _, atomic := range []bool{true, false} {
		t.Run(map[bool]string{true: "atomic", false: "loose"}[atomic], func(t *testing.T) {
			fake, bus, _ := world(t)
			before := fake.StateHash()
			version := versionOf(t, fake, playerA)
			hpBefore, _ := mustGet(t, fake, playerA).HP()

			apply(t, fake, proposal("prop-twice", "combat", atomic,
				changeSet(playerRef(playerA), "Вася", version,
					entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1}),
				changeSet(playerRef(playerA), "Вася", version,
					entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -2})))

			assertRefusal(t, bus, "prop-twice", state.ReasonInvalidOp, playerA, nil)
			if refusals := refusalsOf(t, bus); len(refusals) != 1 {
				t.Errorf("%d refusals, want one for the package", len(refusals))
			}
			if facts := factsOf(t, bus); len(facts) != 0 {
				t.Errorf("%d facts: one entity got more than one entity.updated per proposal", len(facts))
			}
			if hp, _ := mustGet(t, fake, playerA).HP(); hp != hpBefore {
				t.Errorf("hp %d, want %d: half of a refused package was applied", hp, hpBefore)
			}
			if after := fake.StateHash(); after != before {
				t.Errorf("the world moved under a refused package\n  before: %s\n  after:  %s",
					before, after)
			}
			if applied := fake.AppliedProposals(); len(applied) != 0 {
				t.Errorf("applied proposals %v: a refused proposal was remembered", applied)
			}
		})
	}
}

// --- one world per stub ---

// TestAProposalOfAnotherWorldIsPassedOver covers what a second world on the
// same bus must not do to this one: State is a worker per world
// (state-and-mechanics.md §7.1), and a proposal addressed elsewhere is neither
// applied nor refused — answering it would answer for a State that is not this
// one.
func TestAProposalOfAnotherWorldIsPassedOver(t *testing.T) {
	fake, bus, _ := world(t)
	before := fake.StateHash()

	const offset = 41
	applyAt(t, fake, offset, proposalIn("some-other-world", "prop-elsewhere", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -5})))

	// Passed over is not the same as unread. A cursor that stopped at the
	// foreign message would hand every read-model catching up from this
	// snapshot the same message again (review #2 of T-017, Minor-8).
	pointer, err := fake.Snapshot(context.Background(), state.ReasonAdmin)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if cursor := pointer.Snapshot.Cursor[eventbus.TopicSystemEvents]; cursor != offset+1 {
		t.Errorf("cursor %d, want %d: the message of another world was left unread",
			cursor, offset+1)
	}

	if facts := factsOf(t, bus); len(facts) != 0 {
		t.Errorf("%d facts under a world the stub does not own", len(facts))
	}
	if refusals := refusalsOf(t, bus); len(refusals) != 0 {
		t.Errorf("%d refusals: the stub answered for another world", len(refusals))
	}
	if hp, _ := mustGet(t, fake, playerA).HP(); hp != 10 {
		t.Errorf("hp %d, want 10: another world changed this one", hp)
	}
	if after := fake.StateHash(); after != before {
		t.Errorf("the world moved under a proposal of another world\n  before: %s\n  after:  %s",
			before, after)
	}
}

// --- all or nothing ---

// TestAtomicPackageIsAllOrNothing is the guarantee of C-02 that a consumer
// leans on hardest: after a refusal there is no half-moved group and no
// half-fought round. The check is not "the world is right afterwards" but "the
// world did not move at all", hash included.
func TestAtomicPackageIsAllOrNothing(t *testing.T) {
	fake, bus, _ := world(t)
	before := fake.StateHash()
	versionA := versionOf(t, fake, playerA)
	versionB := versionOf(t, fake, "player-B")

	// The valid change sets come first and the broken one last, so that a stub
	// which applied as it went would have applied two entities by the time it
	// found the problem.
	apply(t, fake, proposal("prop-atomic", "move", true,
		changeSet(playerRef(playerA), "Вася", versionA,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-B"), "Лена", versionB,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-Z"), "", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))

	if facts := factsOf(t, bus); len(facts) != 0 {
		t.Errorf("%d facts from a refused atomic package", len(facts))
	}
	refusals := refusalsOf(t, bus)
	if len(refusals) != 1 {
		t.Fatalf("%d refusals, want one for the whole package", len(refusals))
	}
	if id, _ := refusals[0].Path().GetString("entity.entity.id"); id != "player-Z" {
		t.Errorf("the refusal blames %q, want the first entity that failed", id)
	}
	if after := fake.StateHash(); after != before {
		t.Errorf("the world moved under a refused atomic package\n  before: %s\n  after:  %s",
			before, after)
	}
	for _, id := range []string{playerA, "player-B"} {
		if position, _ := mustGet(t, fake, id).Position(); position == regionID {
			t.Errorf("%s moved although the package was refused", id)
		}
	}
}

// TestNonAtomicPackageRefusesOnlyWhatFailed is the other half of the same
// switch (§4.5 p. 9): one refusal per broken change set, and everything else
// applied.
func TestNonAtomicPackageRefusesOnlyWhatFailed(t *testing.T) {
	fake, bus, _ := world(t)

	apply(t, fake, proposal("prop-loose", "move", false,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-Z"), "", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))

	if facts := factsOf(t, bus); len(facts) != 1 {
		t.Errorf("%d facts, want the one entity that could move", len(facts))
	}
	if refusals := refusalsOf(t, bus); len(refusals) != 1 {
		t.Errorf("%d refusals, want the one entity that could not", len(refusals))
	}
	if position, _ := mustGet(t, fake, playerA).Position(); position != regionID {
		t.Errorf("position %q: the change set that was fine did not apply", position)
	}
}

// TestFactsComeOutInIdentifierOrder covers the order §4.5 p. 12 fixes, which
// is what makes a recording of a package comparable between runs.
func TestFactsComeOutInIdentifierOrder(t *testing.T) {
	fake, bus, _ := world(t)
	sets := []entity.ChangeSet{}
	for _, id := range []string{"player-C", playerA, "player-B"} {
		sets = append(sets, changeSet(playerRef(id), "", versionOf(t, fake, id),
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}))
	}
	apply(t, fake, proposal("prop-order", "move", true, sets...))

	var have []string
	for _, fact := range factsOf(t, bus) {
		id, _ := fact.Path().GetString("entity.entity.id")
		have = append(have, id)
	}
	want := []string{playerA, "player-B", "player-C"}
	if len(have) != len(want) {
		t.Fatalf("%d facts, want %d", len(have), len(want))
	}
	for i := range want {
		if have[i] != want[i] {
			t.Fatalf("facts came out %v, want %v", have, want)
		}
	}
}

// --- the deduplication window ---

// TestARepeatedProposalIsAppliedOnce covers the at-least-once delivery of the
// bus: a consumer that publishes twice, or a bus that delivers twice, must not
// move the world twice (C-01, C-02).
func TestARepeatedProposalIsAppliedOnce(t *testing.T) {
	fake, bus, _ := world(t)
	ev := proposal("prop-once", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -3}))

	apply(t, fake, ev)
	apply(t, fake, ev)

	if facts := factsOf(t, bus); len(facts) != 1 {
		t.Errorf("%d facts from one proposal delivered twice", len(facts))
	}
	if hp, _ := mustGet(t, fake, playerA).HP(); hp != 7 {
		t.Errorf("hp %d, want 7: the proposal was applied twice", hp)
	}
	if applied := fake.AppliedProposals(); len(applied) != 1 || applied[0] != "prop-once" {
		t.Errorf("applied proposals %v, want one entry", applied)
	}
}

// TestARefusedProposalIsNotRemembered is why the window records only what was
// applied (§4.5 p. 12): a proposer that fixes its proposal and resends it under
// the same identifier has to be answered, not ignored.
func TestARefusedProposalIsNotRemembered(t *testing.T) {
	fake, bus, _ := world(t)
	stale := int64(99)

	apply(t, fake, proposal("prop-retry", "combat", true,
		changeSet(playerRef(playerA), "Вася", &stale,
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})))
	apply(t, fake, proposal("prop-retry", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})))

	if refusals := refusalsOf(t, bus); len(refusals) != 1 {
		t.Errorf("%d refusals, want the first attempt alone", len(refusals))
	}
	if facts := factsOf(t, bus); len(facts) != 1 {
		t.Errorf("%d facts: the corrected proposal was swallowed by the window", len(facts))
	}
}

// --- what the stub answers to what it does not handle ---

// TestFactsOfTheStubAreIgnored guards against the loop the subscription would
// otherwise be: system_events carries entity.created and entity.updated, and
// the stub reads its own topic.
func TestFactsOfTheStubAreIgnored(t *testing.T) {
	fake, bus, _ := world(t)
	apply(t, fake, proposal("prop-echo", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1})))

	fact := factsOf(t, bus)[0]
	apply(t, fake, fact)

	if have := len(factsOf(t, bus)); have != 1 {
		t.Errorf("%d facts after the stub was handed its own: it answers itself", have)
	}
}

// --- the form of changed[] and the proposal_id of a create (C-02 v1.6, T-448) ---

// TestChangedCarriesOldAndNewByPresence publishes the three shapes of an entry
// of changed[]: an append has no old, a remove of a key has no new, and a set of
// an existing path has both. A null in place of a missing key is the defect the
// form closes: a consumer that catches up would write it.
func TestChangedCarriesOldAndNewByPresence(t *testing.T) {
	fake, bus, _ := world(t)
	apply(t, fake, proposal("prop-mark", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: "mark", Value: "fox"},
			entity.Op{Op: entity.OpAppend, Path: "tags", Value: "hunted"})))
	apply(t, fake, proposal("prop-unmark", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpRemove, Path: "mark"})))
	apply(t, fake, proposal("prop-move", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))

	facts := factsOf(t, bus)
	if len(facts) != 3 {
		t.Fatalf("%d facts, want 3", len(facts))
	}
	want := []map[string][2]bool{ // path -> {old present, new present}
		{"mark": {false, true}, "tags[0]": {false, true}},
		{"mark": {true, false}},
		{entity.AttrPosition: {true, true}},
	}
	for i, fact := range facts {
		if err := contracts.Validate(fact); err != nil {
			t.Fatalf("fact %d is not valid: %v", i, err)
		}
		changed, _ := fact.Path().GetSlice("changed")
		if len(changed) != len(want[i]) {
			t.Fatalf("fact %d: changed %v, want %d entries", i, changed, len(want[i]))
		}
		for _, raw := range changed {
			entry, _ := raw.(map[string]any)
			path, _ := entry["path"].(string)
			presence, known := want[i][path]
			if !known {
				t.Fatalf("fact %d: unexpected entry %v", i, entry)
			}
			_, hasOld := entry["old"]
			_, hasNew := entry["new"]
			if hasOld != presence[0] || hasNew != presence[1] {
				t.Errorf("fact %d: %v has old = %v, new = %v; want %v, %v",
					i, entry, hasOld, hasNew, presence[0], presence[1])
			}
		}
	}
}

// TestChangedCarriesAPresentNull is the same form where the value is null: a
// null is a value, so presence is decided by the path and not by the value. A
// set of a missing path to null carries new: null, a set of an existing path to
// null carries both, and a remove of a key that held null carries old: null and
// no new. Written as "new only when the value is not nil", the stub would publish
// a set to null as a remove, and a consumer that catches up would delete the key
// State kept (C-02 v1.6).
func TestChangedCarriesAPresentNull(t *testing.T) {
	fake, bus, _ := world(t)
	apply(t, fake, proposal("prop-null-new", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: "nul", Value: nil},
			entity.Op{Op: entity.OpSet, Path: "mark", Value: "fox"})))
	apply(t, fake, proposal("prop-null-over", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: "mark", Value: nil})))
	apply(t, fake, proposal("prop-null-gone", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpRemove, Path: "nul"})))

	facts := factsOf(t, bus)
	if len(facts) != 3 {
		t.Fatalf("%d facts, want 3", len(facts))
	}
	type presence struct{ old, new bool }
	want := []map[string]presence{
		{"nul": {old: false, new: true}, "mark": {old: false, new: true}},
		{"mark": {old: true, new: true}},
		{"nul": {old: true, new: false}},
	}
	for i, fact := range facts {
		if err := contracts.Validate(fact); err != nil {
			t.Fatalf("fact %d is not valid: %v", i, err)
		}
		changed, _ := fact.Path().GetSlice("changed")
		if len(changed) != len(want[i]) {
			t.Fatalf("fact %d: changed %v, want %d entries", i, changed, len(want[i]))
		}
		for _, raw := range changed {
			entry, _ := raw.(map[string]any)
			path, _ := entry["path"].(string)
			expected, known := want[i][path]
			if !known {
				t.Fatalf("fact %d: unexpected entry %v", i, entry)
			}
			oldValue, hasOld := entry["old"]
			newValue, hasNew := entry["new"]
			if hasOld != expected.old || hasNew != expected.new {
				t.Errorf("fact %d: %v has old = %v, new = %v; want %v, %v",
					i, entry, hasOld, hasNew, expected.old, expected.new)
			}
			if path == "nul" && (oldValue != nil || newValue != nil) {
				t.Errorf("fact %d: %v, want null on the side that is present", i, entry)
			}
		}
	}
}

// TestCreateWithoutProposalIDIsPassedOver: proposal_id is required in
// entity.create.proposed (C-02 v1.6), so the stub no longer makes one up from
// the event id. There is nothing to refuse such a proposal by — the refusal
// requires the proposal_id too — and nothing is created.
func TestCreateWithoutProposalIDIsPassedOver(t *testing.T) {
	fake, bus, _ := world(t)
	before := len(events(t, bus, eventbus.TopicSystemEvents))

	ev := createProposal("", "player-Q", "Кью")
	delete(ev.Payload, "proposal_id")
	apply(t, fake, ev)

	if _, ok := fake.Get("player-Q"); ok {
		t.Error("an entity was created from a proposal without proposal_id")
	}
	if after := len(events(t, bus, eventbus.TopicSystemEvents)); after != before {
		t.Errorf("%d events published, want none", after-before)
	}

	apply(t, fake, createProposal("prop-q", "player-Q", "Кью"))
	created := ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeCreated)
	if len(created) != 1 {
		t.Fatalf("%d entity.created, want one for the proposal with an id", len(created))
	}
	if id, _ := created[0].Path().GetString("proposal_id"); id != "prop-q" {
		t.Errorf("entity.created proposal_id %q, want prop-q", id)
	}
}

// TestUpdateWithoutProposalIDIsPassedOver is the same rule for an update, which
// the stub used to refuse under the id of the event.
func TestUpdateWithoutProposalIDIsPassedOver(t *testing.T) {
	fake, bus, _ := world(t)
	before := len(events(t, bus, eventbus.TopicSystemEvents))

	apply(t, fake, proposal("", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))

	if after := len(events(t, bus, eventbus.TopicSystemEvents)); after != before {
		t.Errorf("%d events published, want none", after-before)
	}
}

// TestNumberPastTwoToTheFiftyThirdIsInvalidOp: a value no reader of the bus
// can hold is refused rather than stored (C-02 v1.6). The proposal travels the
// way a proposal on the bus does — encoded and decoded — so the literal
// 2^53+1 reaches the stub as the float64 2^53. That is why the bound excludes
// 2^53: a bound that let 2^53 in would apply 2^53+1 as a different number.
func TestNumberPastTwoToTheFiftyThirdIsInvalidOp(t *testing.T) {
	fake, bus, _ := world(t)
	ev := overTheBus(t, proposal("prop-big", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("9007199254740993")})))
	if value, _ := ev.Path().GetAny("changes[0].ops[0].value"); value != float64(1<<53) {
		t.Fatalf("value after the bus = %#v, want the float64 2^53", value)
	}

	apply(t, fake, ev)

	assertRefusal(t, bus, "prop-big", state.ReasonInvalidOp, playerA, nil)
	if _, has := mustGet(t, fake, playerA).Attributes["seed"]; has {
		t.Error("the refused value was stored")
	}
	if facts := factsOf(t, bus); len(facts) != 0 {
		t.Errorf("%d facts from a refused proposal", len(facts))
	}
}

// TestTheLargestSafeNumberIsApplied is the other side of the bound: 2^53-1 comes
// through the bus as itself and is stored.
func TestTheLargestSafeNumberIsApplied(t *testing.T) {
	fake, bus, _ := world(t)
	apply(t, fake, overTheBus(t, proposal("prop-safe", "move", true,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: "seed", Value: json.Number("-9007199254740991")}))))

	if refusals := refusalsOf(t, bus); len(refusals) != 0 {
		t.Fatalf("%d refusals, want -(2^53-1) applied", len(refusals))
	}
	if seed := mustGet(t, fake, playerA).Attributes["seed"]; seed != float64(-(1<<53 - 1)) {
		t.Errorf("seed = %#v, want -(2^53-1)", seed)
	}
}

// TestCreateWithANumberPastTheRangeIsInvalidOp: the attributes of a create are
// held to the same rule as the value of an operation (C-02 v1.6), or a create
// would store what an update may not.
func TestCreateWithANumberPastTheRangeIsInvalidOp(t *testing.T) {
	fake, bus, _ := world(t)
	ev := createProposal("prop-seed", "player-Q", "Кью")
	ev.Payload["attributes"] = map[string]any{
		entity.AttrStatus: entity.StatusAlive,
		"rolls":           map[string]any{"seed": float64(1 << 53)},
	}

	apply(t, fake, ev)

	assertRefusal(t, bus, "prop-seed", state.ReasonInvalidOp, "player-Q", nil)
	if _, ok := fake.Get("player-Q"); ok {
		t.Error("an entity was created with a number past the range")
	}
	if created := ofType(events(t, bus, eventbus.TopicSystemEvents), state.TypeCreated); len(created) != 0 {
		t.Errorf("%d entity.created for a refused create", len(created))
	}
}

// TestTheAttributesOfACreateAreCheckedBeforeTheWorld: a number past the range is
// a defect of the form of the proposal (§4.5 p. 1), so it is refused as
// invalid_op before the stub looks at the world — even for an entity that is
// already there, where the world alone would answer duplicate_entity.
func TestTheAttributesOfACreateAreCheckedBeforeTheWorld(t *testing.T) {
	fake, bus, _ := world(t)
	ev := createProposal("prop-seed-dup", playerA, "Вася")
	ev.Payload["attributes"] = map[string]any{
		entity.AttrStatus: entity.StatusAlive,
		"seed":            float64(-(1 << 53)),
	}

	apply(t, fake, ev)

	assertRefusal(t, bus, "prop-seed-dup", state.ReasonInvalidOp, playerA, nil)
	if _, has := mustGet(t, fake, playerA).Attributes["seed"]; has {
		t.Error("the refused attributes reached the entity")
	}
}

// overTheBus is an event as a subscriber of the bus holds it: the payload
// encoded by the publisher and decoded, every number a float64.
func overTheBus(t *testing.T, ev eventbus.Event) eventbus.Event {
	t.Helper()
	encoded, err := json.Marshal(ev.Payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal payload %s: %v", encoded, err)
	}
	ev.Payload = decoded
	return ev
}

// TestWithInvariantsHoldsTheLawsOfTheWorld: with WithInvariants a character
// sent somewhere the world does not know breaks inv-10 (one entity, one
// position) and is refused law_violation naming the law; nothing is published
// but the refusal and the version does not move. Without it the same proposal
// is applied: the double checks the laws only when a test asks.
func TestWithInvariantsHoldsTheLawsOfTheWorld(t *testing.T) {
	nowhere := func(fake *state.FakeState) eventbus.Event {
		return proposal("prop-nowhere", "move", true,
			changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
				entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: "nowhere-at-all"}))
	}

	t.Run("with invariants", func(t *testing.T) {
		fake, bus, _ := world(t)
		fake.WithInvariants()
		before := versionOf(t, fake, playerA)

		apply(t, fake, nowhere(fake))

		assertRefusal(t, bus, "prop-nowhere", state.ReasonLawViolation, playerA, nil)
		if law, _ := refusalsOf(t, bus)[0].Path().GetString("details.invariant_id"); law != "inv-10" {
			t.Errorf("details.invariant_id %q, want inv-10", law)
		}
		if facts := factsOf(t, bus); len(facts) != 0 {
			t.Errorf("%d facts from a proposal that breaks a law", len(facts))
		}
		if v := versionOf(t, fake, playerA); *v != *before {
			t.Errorf("version %d, want %d", *v, *before)
		}
	})

	t.Run("without", func(t *testing.T) {
		fake, bus, _ := world(t)
		apply(t, fake, nowhere(fake))
		if refusals := refusalsOf(t, bus); len(refusals) != 0 {
			t.Errorf("%d refusals: the laws were checked without WithInvariants", len(refusals))
		}
		if position, _ := mustGet(t, fake, playerA).Position(); position != "nowhere-at-all" {
			t.Errorf("position %q, want the proposal applied", position)
		}
	})
}

// TestAProposalWithoutAWorldIsPassedOver: a proposal whose envelope names no
// world is neither applied nor refused (C-02 v1.7). The stand-in of T-017
// applied it to its own world; the registry refuses such a proposal on Publish
// now, so it reaches the double only by a direct call.
func TestAProposalWithoutAWorldIsPassedOver(t *testing.T) {
	fake, bus, _ := world(t)
	before := fake.StateHash()
	ev := proposal("prop-worldless", "combat", true,
		changeSet(playerRef(playerA), "Вася", versionOf(t, fake, playerA),
			entity.Op{Op: entity.OpInc, Path: entity.AttrHP, Value: -1}))
	ev.World = nil

	apply(t, fake, ev)

	if all := events(t, bus, eventbus.TopicSystemEvents); len(all) != 0 {
		t.Errorf("%d events published for a proposal without a world", len(all))
	}
	if fake.StateHash() != before {
		t.Error("the world moved under a proposal without a world")
	}
	if err := bus.Publish(context.Background(), ev); err == nil {
		t.Error("the bus published a proposal without a world: the policy of the registry is not WorldRequired")
	}
}

// TestBatchSizeIsTheNumberApplied: last_change.batch_size of a non-atomic
// package is the number of change sets applied, as in State (§4.5 p. 9); the
// stand-in of T-017 wrote the number proposed (N-3 of review #1 of T-055).
func TestBatchSizeIsTheNumberApplied(t *testing.T) {
	fake, bus, _ := world(t)
	stale := int64(99)

	apply(t, fake, proposal("prop-four", "move", false,
		changeSet(playerRef(playerA), "Вася", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-B"), "Лена", &stale,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-C"), "", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID}),
		changeSet(playerRef("player-Z"), "", nil,
			entity.Op{Op: entity.OpSet, Path: entity.AttrPosition, Value: regionID})))

	if facts, refusals := len(factsOf(t, bus)), len(refusalsOf(t, bus)); facts != 2 || refusals != 2 {
		t.Fatalf("%d facts and %d refusals, want two of each", facts, refusals)
	}
	for _, id := range []string{playerA, "player-C"} {
		if batch := mustGet(t, fake, id).LastChange.BatchSize; batch != 2 {
			t.Errorf("%s last_change.batch_size = %d, want 2 applied of the 4 proposed", id, batch)
		}
	}
}

// --- helpers ---

// busReader is the read side of the in-process bus a test asserts against:
// the raw bodies of a topic, in order, without joining a consumer group and
// competing with the subscription of the stub for a cursor.
type busReader interface {
	Records(string) ([][]byte, error)
}

func mustGet(t *testing.T, s *state.FakeState, id string) *entity.Entity {
	t.Helper()
	e, ok := s.Get(id)
	if !ok {
		t.Fatalf("%s is not in the world", id)
	}
	return e
}

// kill puts an entity into the terminal status the fight leaves behind, so
// that a test of the dead_entity rule starts from a corpse the stub itself
// made rather than from a fixture edited by hand.
func kill(t *testing.T, s *state.FakeState, id, typ string) {
	t.Helper()
	apply(t, s, proposal("prop-kill-"+id, "death", true,
		changeSet(entity.Ref{ID: id, Type: typ}, "", versionOf(t, s, id),
			entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusDead})))
	if status, _ := mustGet(t, s, id).Status(); status != entity.StatusDead {
		t.Fatalf("%s is %q, want dead", id, status)
	}
}

// forget is the proposal the gateway publishes in the /forget cascade
// (C-02 v1.2): one atomic package, cause forget, the version pinned, and no
// meta.agent — a person asked, not an agent.
func forget(proposalID, playerID string, version *int64) eventbus.Event {
	return proposal(proposalID, "forget", true,
		changeSet(playerRef(playerID), "", version,
			entity.Op{Op: entity.OpSet, Path: entity.AttrStatus, Value: entity.StatusAbandoned}))
}

func createProposal(proposalID, id, name string) eventbus.Event {
	return eventbus.NewRoot(state.TypeCreateProposed, contracts.SourceGateway, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"proposal_id": proposalID,
			"entity": map[string]any{
				"entity": map[string]any{"id": id, "type": entity.TypePlayer},
				"name":   name,
			},
			"attributes": map[string]any{entity.AttrStatus: entity.StatusAlive},
			"cause":      "create",
		})
}

// assertRefusal checks the last refusal on the topic: the reason C-02 names,
// the entity it blames, and the numbers a version conflict has to carry.
func assertRefusal(t *testing.T, bus busReader, proposalID, reason, entityID string,
	details map[string]float64) {
	t.Helper()
	refusals := refusalsOf(t, bus)
	if len(refusals) == 0 {
		t.Fatalf("nothing was refused, want %s for %s", reason, proposalID)
	}
	ev := refusals[len(refusals)-1]
	if err := contracts.Validate(ev); err != nil {
		t.Fatalf("the refusal is not a valid %s: %v", state.TypeRejected, err)
	}
	if ev.Source != state.Source {
		t.Errorf("source %q, want %q", ev.Source, state.Source)
	}
	path := ev.Path()
	if id, _ := path.GetString("proposal_id"); id != proposalID {
		t.Errorf("proposal_id %q, want %q", id, proposalID)
	}
	if have, _ := path.GetString("reason"); have != reason {
		t.Errorf("reason %q, want %q", have, reason)
	}
	if have, _ := path.GetString("entity.entity.id"); have != entityID {
		t.Errorf("the refusal blames %q, want %q", have, entityID)
	}
	for key, want := range details {
		have, ok := path.GetFloat("details." + key)
		if !ok || have != want {
			t.Errorf("details.%s = %v, want %v", key, have, want)
		}
	}
}
