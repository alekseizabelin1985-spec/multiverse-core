package gateway_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/testkit"
	"multiverse-core.io/shared/testkit/gateway"
	"multiverse-core.io/shared/testkit/membus"
	"multiverse-core.io/shared/testkit/state"
)

// This file holds the one property the two combat actions of the harness rest
// on: an action somebody else answers returns when the world has come to rest
// around it — a fact for EVERY entity the change named — and not one fact
// earlier.
//
// It is proved without a State on the bus, and that is the point. State applies
// an atomic package whole and publishes a fact for everything in it, so a
// harness that returned on the first fact would look right in most runs and
// wrong in a few: the guard would be a race itself, and the mutation "return as
// soon as one fact is in" survives it (review of T-400, Ma-3). What answers the
// action here publishes the package and then exactly the facts the case asks
// for, so a harness that gives up early gives up in every run and a harness
// that waits waits in every run. There is no timing in either direction.

// TestABlowWaitsForAFactAboutEveryEntityItsChangeNamed is that proof.
//
// The package of a blow names two — the character and whoever it swung at —
// because a fight moves both. Answered for one, the action must not return: the
// version the harness holds for the other is the version the world has already
// left, and the next proposal of a script pinned to it would be refused
// (ADR-013 p. 1). It waits, and says how far the answer got.
func TestABlowWaitsForAFactAboutEveryEntityItsChangeNamed(t *testing.T) {
	cases := map[string]struct {
		answered []string
		want     string
	}{
		"both of the two the change named": {[]string{playerA, npcID}, ""},
		"the character only": {
			[]string{playerA},
			"named 2 entities and State has published a fact for 1 of them",
		},
		"the wolf only": {
			[]string{npcID},
			"named 2 entities and State has published a fact for 1 of them",
		},
		"neither": {
			nil,
			"named 2 entities and State has published a fact for 0 of them",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bus := newBus(t)
			h := knowing(t, bus, nil)
			answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
				proposal := packageNaming(t, action, playerA, npcID)
				if err := bus.Publish(ctx, proposal); err != nil {
					return err
				}
				for _, id := range tc.answered {
					if err := bus.Publish(ctx, factFor(t, proposal, id)); err != nil {
						return err
					}
				}
				return nil
			})

			err := h.Attack(t.Context(), playerA, npcID)

			if tc.want == "" {
				if err != nil {
					t.Fatalf("a blow answered for everything its change named: %v", err)
				}
				for _, id := range []string{playerA, npcID} {
					if version, known := h.Version(id); !known || version != answeredVersion {
						t.Errorf("the harness thinks %s is at version %d, the fact said %d",
							id, version, answeredVersion)
					}
				}
				return
			}
			if err == nil {
				t.Fatal("a blow answered for one of the two entities its change named was " +
					"reported as success: the version of the other is the one the world left")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the error is %q, want it to say %q", err, tc.want)
			}
		})
	}
}

// TestAChangeThatNamesNobodyIsNotAnAnswer is the other end of the same
// property: a package the harness could not read a single entity out of must
// not count as "everything it named has a fact" — which is exactly what an
// empty set of names means to a loop over it.
//
// The package is folded into the harness instead of published, and it has to
// be: C-02 requires changes[].entity.entity.id and the bus would refuse this
// one. That is the reason to hold the case now rather than when it becomes
// reachable — the path the harness reads is a choice the harness made, and a
// revision of C-02 that moved it would turn a loud failure into a silent
// success. The harness would report a blow nobody resolved as resolved.
func TestAChangeThatNamesNobodyIsNotAnAnswer(t *testing.T) {
	bus := newBus(t)
	h := knowing(t, bus, nil)
	answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
		return h.Observe(ctx, eventbus.Derive(action, gateway.TypeUpdateProposed,
			contracts.SourceTestkitSwarm, map[string]any{
				"proposal_id": "enc-nobody",
				"atomic":      true,
				"cause":       "combat",
				"changes":     []map[string]any{{"ops": []any{}}},
			}))
	})

	err := h.Attack(t.Context(), playerA, npcID)

	if err == nil {
		t.Fatal("a blow whose change named nobody was reported as success: " +
			"the action returned without a single fact behind it")
	}
	if !strings.Contains(err.Error(), "named no entity the harness can read") {
		t.Errorf("the error is %q; it should say that the package named nobody", err)
	}
}

// TestASecondChangeForOneActionIsSaidOutLoud pins what the harness does with a
// package it did not ask for. Returning by the first is what the decision of
// 2026-09-11 allows until C-04/C-05 say an action may be answered twice —
// doing it quietly is what turns a half-applied change into a green run.
//
// The second package carries an identifier of its own, and that is what makes
// it a second answer rather than the first one offered again: a publisher
// retrying after a version conflict keeps the identifier, because it is still
// answering the one action once (see the retry cases below).
func TestASecondChangeForOneActionIsSaidOutLoud(t *testing.T) {
	bus := newBus(t)
	said := &journal{}
	h := knowing(t, bus, slog.New(slog.NewTextHandler(said, nil)))
	answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
		first := packageNaming(t, action, playerA)
		if err := bus.Publish(ctx, first); err != nil {
			return err
		}
		if err := bus.Publish(ctx,
			packageUnder(t, action, "enc-second-"+action.ID, npcID)); err != nil {
			return err
		}
		return bus.Publish(ctx, factFor(t, first, playerA))
	})

	if err := h.Attack(t.Context(), playerA, npcID); err != nil {
		t.Fatalf("attack: %v", err)
	}

	waitFor(t, "the harness to say that it ignored a package", func() bool {
		return strings.Contains(said.String(), "a second proposal for one action was ignored")
	})
	if version, known := h.Version(npcID); known {
		t.Errorf("the harness folded the second package after all: it holds %s at %d",
			npcID, version)
	}
}

// TestAPackageRefusedForAVersionConflictIsWaitedThroughItsRetry is the other
// half of the decision of 2026-09-11 on the residual race: the publisher offers
// the package again, and the harness has to still be there when it does.
//
// A version conflict is not an answer to the action. It says that somebody
// moved an entity under the package — here the harness itself, which moves the
// character it plays while the encounter answers from a view of its own — and
// the answer to it belongs to the publisher: fold in what arrived and offer the
// same package again, under the same identifier. A wait that ended on the
// refusal would report a package arriving a millisecond later as a package that
// never arrived, which is exactly what made this stand red about one run in ten.
//
// The second offer names one entity where the first named two: the package is
// recomputed against the world it will now be applied to, and a target that
// fell in the meantime is not struck again. So the wait has to expect what the
// LAST offer named — a harness that kept the first set waits for a fact about a
// corpse nobody is going to move.
func TestAPackageRefusedForAVersionConflictIsWaitedThroughItsRetry(t *testing.T) {
	bus := newBus(t)
	said := &journal{}
	h := knowing(t, bus, slog.New(slog.NewTextHandler(said, nil)))
	answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
		// Both packages are built by packageNaming, which names a proposal
		// after the action: the identifier is the same because this is one
		// answer to one action, offered twice.
		first := packageNaming(t, action, playerA, npcID)
		if err := bus.Publish(ctx, first); err != nil {
			return err
		}
		if err := bus.Publish(ctx,
			refusalOf(t, first, gateway.ReasonVersionConflict)); err != nil {
			return err
		}
		again := packageNaming(t, action, playerA)
		if err := bus.Publish(ctx, again); err != nil {
			return err
		}
		return bus.Publish(ctx, factFor(t, again, playerA))
	})

	if err := h.Attack(t.Context(), playerA, npcID); err != nil {
		t.Fatalf("a blow whose package was refused for a version conflict and offered "+
			"again: %v", err)
	}

	if version, known := h.Version(playerA); !known || version != answeredVersion {
		t.Errorf("the harness thinks %s is at version %d, the fact of the second offer "+
			"said %d", playerA, version, answeredVersion)
	}
	if strings.Contains(said.String(), "a second proposal for one action was ignored") {
		t.Error("the harness took the retry for a second package: a package offered again " +
			"under the same identifier is the same answer to the same action")
	}
	waitFor(t, "the harness to say that the package was offered again", func() bool {
		return strings.Contains(said.String(), "was offered again")
	})
}

// TestARefusalThatIsNotARaceEndsTheWaitAtOnce is the line the case above must
// not wash away. Four of the five reasons of C-02 are defects of whoever
// proposed the change, not races: nothing about them gets better on a second
// offer, and a wait that sat through them would turn a defect visible now into
// a defect visible one timeout later — or, if the publisher does retry, into a
// defect that happens three times and then disappears.
//
// The check is on the text and not merely on "there is an error", because a
// wait that ran out ends in an error too: what says the refusal was reported at
// once rather than waited out is that the message is the refusal and names the
// reason State gave.
func TestARefusalThatIsNotARaceEndsTheWaitAtOnce(t *testing.T) {
	for _, reason := range []string{
		state.ReasonUnknownEntity, state.ReasonInvalidOp,
		state.ReasonDeadEntity, state.ReasonDuplicateEntity,
	} {
		t.Run(reason, func(t *testing.T) {
			bus := newBus(t)
			h := knowing(t, bus, nil)
			answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
				proposal := packageNaming(t, action, playerA, npcID)
				if err := bus.Publish(ctx, proposal); err != nil {
					return err
				}
				return bus.Publish(ctx, refusalOf(t, proposal, reason))
			})

			err := h.Attack(t.Context(), playerA, npcID)

			if err == nil {
				t.Fatal("a blow whose change State refused was reported as success")
			}
			if want := "was refused: " + reason; !strings.Contains(err.Error(), want) {
				t.Errorf("the error is %q, want it to say %q — a defect is worth seeing "+
					"the moment it happens, not one timeout later", err, want)
			}
		})
	}
}

// TestAPublisherThatRanOutOfRetriesIsCaughtByTheDeadline is the answer to the
// obvious objection to waiting through a conflict: that the wait now has no
// end. It has exactly the end it had before — the deadline of the action — and
// what changed is only what the failure says.
//
// The two ends of that deadline are two different defects in two different
// places. Nobody answered at all: the action never reached an encounter, and
// the next reader should be looking at whoever was supposed to publish. The
// publisher kept losing the version race until it gave up (testkit/swarm allows
// an action three attempts and then logs "gave up"): the action did reach an
// encounter, the fight decided something, and none of it is in State. A message
// that read the same for both would send half the readers to the wrong half of
// the bus.
func TestAPublisherThatRanOutOfRetriesIsCaughtByTheDeadline(t *testing.T) {
	cases := map[string]struct {
		conflicts int
		want      string
	}{
		"nobody answered the action at all": {0, "none arrived"},
		"the publisher gave up after one conflict": {
			1, "ran out of retries and gave up, not silence on the bus (version conflicts: 1",
		},
		"the publisher gave up after three": {
			3, "ran out of retries and gave up, not silence on the bus (version conflicts: 3",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			bus := newBus(t)
			h := knowing(t, bus, nil)
			answerFirstAction(t, bus, func(ctx context.Context, action eventbus.Event) error {
				for range tc.conflicts {
					proposal := packageNaming(t, action, playerA, npcID)
					if err := bus.Publish(ctx, proposal); err != nil {
						return err
					}
					if err := bus.Publish(ctx,
						refusalOf(t, proposal, gateway.ReasonVersionConflict)); err != nil {
						return err
					}
				}
				return nil
			})

			err := h.Attack(t.Context(), playerA, npcID)

			if err == nil {
				t.Fatal("an action nothing ever answered was reported as success: " +
					"waiting through a version conflict must not be waiting forever")
			}
			if !strings.Contains(err.Error(), "nobody resolved") {
				t.Errorf("the error is %q; the wait should have ended at its deadline", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("the error is %q, want it to say %q", err, tc.want)
			}
		})
	}
}

// --- a bus with nobody on it but the harness ---

// answeredVersion is the version the stand-in State publishes its facts at. It
// is one past the version the character was created at, which is all any of
// this needs: what matters is that the harness holds what the fact said.
const answeredVersion = 2

// knowing is a started harness on a bus with no State and no encounter, which
// already knows the character the way a created one would — impatient, because
// half of these cases end in a wait that has to run out.
func knowing(t *testing.T, bus *membus.Bus, log *slog.Logger) *gateway.Harness {
	t.Helper()
	h, err := gateway.NewHarness(bus, loadFixtures(t))
	if err != nil {
		t.Fatalf("harness: %v", err)
	}
	h.WithTimeout(200 * time.Millisecond).WithLog(log)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(func() { cancel(); _ = h.Wait() })
	if err := h.Observe(ctx, born(t, playerA)); err != nil {
		t.Fatalf("observe: %v", err)
	}
	if err := h.Start(ctx); err != nil {
		t.Fatalf("start: %v", err)
	}
	return h
}

// answerFirstAction runs answer for the first combat action the harness
// publishes, and only for the first: most of these cases are about one action.
func answerFirstAction(t *testing.T, bus *membus.Bus,
	answer func(context.Context, eventbus.Event) error) {
	t.Helper()
	var once sync.Once
	answerEveryAction(t, bus, func(ctx context.Context, ev eventbus.Event) error {
		var err error
		once.Do(func() { err = answer(ctx, ev) })
		return err
	})
}

// answerEveryAction runs answer for every combat action the harness publishes.
func answerEveryAction(t *testing.T, bus *membus.Bus,
	answer func(context.Context, eventbus.Event) error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	var alive sync.WaitGroup
	alive.Add(1)
	go func() {
		defer alive.Done()
		_ = bus.Subscribe(ctx, eventbus.TopicPlayerEvents, "test-answer",
			func(ctx context.Context, ev eventbus.Event) error {
				if ev.Type != gateway.TypeAttacked && ev.Type != gateway.TypeFleeAttempted {
					return nil
				}
				return answer(ctx, ev)
			})
	}()
	t.Cleanup(func() { cancel(); alive.Wait() })
}

// opens a fight around the characters given and waits until the harness has
// heard of it. It goes over the bus rather than into Observe, because the
// harness reads world_events for exactly this and a payload built by hand is
// not the JSON a subscriber is handed.
func opens(t *testing.T, bus *membus.Bus, h *gateway.Harness, encounterID string,
	players ...string) {
	t.Helper()
	fixtures := loadFixtures(t)
	participants := make([]any, 0, len(players))
	for _, id := range players {
		participants = append(participants, named(pick(t, fixtures, id)))
	}
	publish(t, bus, eventbus.NewRoot(gateway.TypeEncounterStarted, contracts.SourceTestkitSwarm,
		worldID, nil, eventbus.ActorSystem, map[string]any{
			"encounter":    ref(encounterID, entity.TypeEncounter),
			"region":       named(pick(t, fixtures, regionID)),
			"participants": participants,
			"npcs":         []any{named(pick(t, fixtures, npcID))},
		}, eventbus.WithAgent(agent())))
	waitFor(t, "the harness to hear that the fight opened", func() bool {
		_, _, known := h.Fight(players[0])
		return known
	})
}

// ended is the end of a fight as C-05 publishes it, as a root of its own: an
// end derived from an action belongs to that action, and this one belongs to
// none.
func ended(t *testing.T, encounterID, reason string) eventbus.Event {
	t.Helper()
	return eventbus.NewRoot(gateway.TypeEncounterEnded, contracts.SourceTestkitSwarm, worldID,
		nil, eventbus.ActorSystem, map[string]any{
			"encounter": ref(encounterID, entity.TypeEncounter),
			"reason":    reason,
			"rounds":    1,
		}, eventbus.WithAgent(agent()))
}

// falls publishes the fact of a death and waits until the harness has folded
// it: a change of status is how the harness learns that somebody is past
// acting (C-02, changed[]).
func falls(t *testing.T, bus *membus.Bus, h *gateway.Harness, id string) {
	t.Helper()
	who := pick(t, loadFixtures(t), id)
	publish(t, bus, eventbus.NewRoot(gateway.TypeUpdated, contracts.SourceTestkitState, worldID,
		nil, entity.ActorKindCI, map[string]any{
			"entity":  named(who),
			"version": answeredVersion,
			"changed": []any{
				map[string]any{"path": entity.AttrHP, "old": 3, "new": 0},
				map[string]any{"path": entity.AttrStatus,
					"old": entity.StatusAlive, "new": entity.StatusDead},
			},
			"cause":       "combat",
			"proposal_id": "fell-" + id,
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
		}))
	waitFor(t, "the harness to fold the death of "+id, func() bool {
		status, known := h.Status(id)
		return known && status == entity.StatusDead
	})
}

func publish(t *testing.T, bus *membus.Bus, ev eventbus.Event) {
	t.Helper()
	if err := bus.Publish(context.Background(), ev); err != nil {
		t.Fatalf("publish %s: %v", ev.Type, err)
	}
}

// agent is the swarm agent everything on world_events travels with (C-01).
func agent() eventbus.AgentRef {
	return eventbus.AgentRef{ID: "test-encounter", Level: "domain", Blueprint: "encounter-wolf"}
}

// packageNaming is the one atomic package an encounter answers an action with,
// naming the entities given and pinning each at the version the fixtures start
// it at.
func packageNaming(t *testing.T, action eventbus.Event, named ...string) eventbus.Event {
	t.Helper()
	return packageUnder(t, action, "enc-"+action.ID, named...)
}

// packageUnder is the same package under an identifier the case chooses,
// because the identifier is what tells the two things a second package can be
// apart: the same attempt offered again after it lost the version race, or a
// second answer to one action.
func packageUnder(t *testing.T, action eventbus.Event, proposalID string,
	named ...string) eventbus.Event {
	t.Helper()
	fixtures := loadFixtures(t)
	changes := make([]entity.ChangeSet, 0, len(named))
	for _, id := range named {
		who := pick(t, fixtures, id)
		version := who.Version
		changes = append(changes, entity.ChangeSet{
			Entity:          eventbus.Entity{Entity: who.Ref().EventRef(), Name: who.Name},
			ExpectedVersion: &version,
			Ops:             []entity.Op{{Op: entity.OpInc, Path: entity.AttrHP, Value: -1}},
		})
	}
	return eventbus.Derive(action, gateway.TypeUpdateProposed, contracts.SourceTestkitSwarm,
		map[string]any{
			"proposal_id": proposalID,
			"changes":     changes,
			"atomic":      true,
			"cause":       "combat",
		})
}

// refusalOf is what State publishes when it turns a whole package down: the
// reason, and the identifier of the package it turned down (C-02 v1.2, one
// rejection per atomic package).
func refusalOf(t *testing.T, proposal eventbus.Event, reason string) eventbus.Event {
	t.Helper()
	proposalID, _ := proposal.Path().GetString("proposal_id")
	return eventbus.Derive(proposal, gateway.TypeRejected, contracts.SourceTestkitState,
		map[string]any{"proposal_id": proposalID, "reason": reason})
}

// factFor is what State would publish for one entity of a package it applied.
func factFor(t *testing.T, proposal eventbus.Event, id string) eventbus.Event {
	t.Helper()
	who := pick(t, loadFixtures(t), id)
	proposalID, _ := proposal.Path().GetString("proposal_id")
	return eventbus.Derive(proposal, gateway.TypeUpdated, contracts.SourceTestkitState,
		map[string]any{
			"entity":      named(who),
			"version":     answeredVersion,
			"changed":     []map[string]any{{"path": entity.AttrHP, "old": 10, "new": 9}},
			"cause":       "combat",
			"proposal_id": proposalID,
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
		})
}

// born is the fact of the creation of a character, for a harness that has to
// know one without a State to tell it.
func born(t *testing.T, id string) eventbus.Event {
	t.Helper()
	who := pick(t, loadFixtures(t), id)
	return eventbus.NewRoot(gateway.TypeCreated, contracts.SourceTestkitState, worldID, nil,
		entity.ActorKindCI, map[string]any{
			"entity":      named(who),
			"version":     who.Version,
			"attributes":  who.Attributes,
			"cause":       gateway.CauseCreate,
			"proposal_id": "born-" + id,
			"applied_at":  testkit.Epoch.Format(time.RFC3339),
		})
}

// named is the EntityWithName of an entity, as a payload built by hand rather
// than marshalled from a struct: a fact folded straight into the harness never
// passes through JSON, and a struct in a payload is not a map the accessor of
// the event can walk into.
func named(who *entity.Entity) map[string]any {
	out := ref(who.ID, who.Type)
	out["name"] = who.Name
	return out
}

// journal collects what the harness logged, from whichever goroutine logged it.
type journal struct {
	mu   sync.Mutex
	said bytes.Buffer
}

func (j *journal) Write(p []byte) (int, error) {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.said.Write(p)
}

func (j *journal) String() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.said.String()
}
