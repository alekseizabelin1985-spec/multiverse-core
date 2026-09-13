package state

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
)

// Apply handles one event of system_events. It is the handler Start subscribes
// with, and it is exported so that a test can drive the stub without a
// goroutine: publishing a proposal and waiting for a fact tests the wiring,
// calling Apply tests the decision.
//
// Everything that is not a proposal is ignored, the facts of the stub itself
// included: system_events carries entity.created and entity.updated, and a
// stub that answered its own facts would loop.
//
// The error it returns is a failure of the stub, not a refusal of the
// proposal: a refusal is an entity.update.rejected on the bus and a nil error,
// because the proposal was handled. Returning an error would make the bus
// retry the proposal three times and then park it in dead_letters, which is
// what a broken stub deserves and a rejected proposal does not.
func (s *FakeState) Apply(ctx context.Context, ev eventbus.Event) error {
	if pos, ok := eventbus.PositionFromContext(ctx); ok && pos.Topic == eventbus.TopicSystemEvents {
		s.mu.Lock()
		s.cursor = pos.Offset + 1
		s.mu.Unlock()
	}
	// State is a worker per world (state-and-mechanics.md §7.1), and the stub
	// owns the one world Config.WorldID names. A proposal addressed to another
	// world is passed over without a fact and without a refusal: it was not
	// addressed to this State, and a refusal would answer for a State that is
	// not this one. Two worlds on one membus therefore no longer apply each
	// other's proposals (review #1 of T-017, Minor-4).
	if world := eventbus.GetWorldIDFromEvent(ev); world != "" && world != s.worldID {
		s.log.Debug("proposal of another world passed over",
			"event_id", ev.ID, "type", ev.Type, "event_world_id", world)
		return nil
	}
	switch ev.Type {
	case TypeCreateProposed:
		return s.applyCreate(ctx, ev)
	case TypeUpdateProposed:
		return s.applyUpdate(ctx, ev)
	default:
		return nil
	}
}

// updateProposal is the payload of entity.update.proposed as C-02 defines it.
// The change sets are the model type, so that the operations the stub applies
// are literally the operations entity.ApplyOps understands.
type updateProposal struct {
	ProposalID string             `json:"proposal_id"`
	Changes    []entity.ChangeSet `json:"changes"`
	Atomic     bool               `json:"atomic"`
	Cause      string             `json:"cause"`
}

// createProposal is the payload of entity.create.proposed. proposal_id is
// required there as in an update (C-02 v1.6).
type createProposal struct {
	ProposalID string          `json:"proposal_id"`
	Entity     eventbus.Entity `json:"entity"`
	Attributes map[string]any  `json:"attributes"`
	Cause      string          `json:"cause"`
}

// decodePayload reads a payload into a typed proposal. It goes through JSON
// rather than through a type assertion because that is how the payload
// travelled: whoever published it wrote a map, and a number in it is a float64
// whichever way the publisher meant it.
func decodePayload(payload map[string]any, dst any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, dst)
}

// applyCreate answers entity.create.proposed with entity.created or with
// entity.update.rejected reason=duplicate_entity (§4.5), or invalid_op for a
// create that names no entity or carries attributes no reader of the bus could
// hold.
//
// What the real State does here and the stub does not: the attributes required
// by the type are not checked, ownership is not checked and no invariant runs.
// A create that would be refused by internal/state for one of those three
// reasons is accepted here (design.md §5).
func (s *FakeState) applyCreate(ctx context.Context, ev eventbus.Event) error {
	var p createProposal
	if err := decodePayload(ev.Payload, &p); err != nil {
		return s.refuseMalformed(ctx, ev, p.ProposalID)
	}
	if p.ProposalID == "" {
		return s.refuseMalformed(ctx, ev, "")
	}
	ref := entity.RefFrom(p.Entity.Entity)
	if ref.ID == "" || ref.Type == "" {
		return s.reject(ctx, ev, p.ProposalID, ReasonInvalidOp, nil, nil)
	}
	if !entity.JSONCompatible(p.Attributes) {
		// The attributes of a create are held to the rule of the value of an
		// operation (C-02 v1.6): a number of magnitude 2^53 or more would be
		// stored as one number and read back from the fact and the snapshot as
		// another.
		return s.reject(ctx, ev, p.ProposalID, ReasonInvalidOp, &ref, nil)
	}

	s.mu.Lock()
	if s.seen(p.ProposalID) {
		s.mu.Unlock()
		s.log.Debug("proposal already applied", "proposal_id", p.ProposalID)
		return nil
	}
	if _, exists := s.entities[ref.ID]; exists {
		s.mu.Unlock()
		return s.reject(ctx, ev, p.ProposalID, ReasonDuplicateEntity, &ref, nil)
	}
	created := entity.New(ref, s.worldID, p.Entity.Name, p.Attributes, ev.Timestamp)
	s.entities[ref.ID] = created
	s.remember(p.ProposalID)
	s.mu.Unlock()

	fact := eventbus.Derive(ev, TypeCreated, Source, map[string]any{
		"entity":      entityRefPayload(created),
		"version":     created.Version,
		"attributes":  created.Attributes,
		"proposal_id": p.ProposalID,
	})
	if err := s.bus.Publish(ctx, fact); err != nil {
		return fmt.Errorf("testkit/state: publish %s for %s: %w", TypeCreated, ref, err)
	}

	s.mu.Lock()
	if e, ok := s.entities[ref.ID]; ok {
		e.SetFactEventID(fact.ID)
	}
	s.mu.Unlock()
	s.log.Info("entity created", "entity_id", ref.ID, "entity_type", ref.Type, "event_id", fact.ID)
	return nil
}

// refusal is one proposal that cannot be applied: the reason C-02 names, the
// entity it is about, and the numbers that explain a version conflict.
type refusal struct {
	reason  string
	ref     entity.Ref
	details map[string]any
}

// applyUpdate answers entity.update.proposed with one entity.updated per
// applied entity or with entity.update.rejected (§4.5).
//
// The checks are steps 3, 4, 5 and 7 of §4.5 — existence, version, terminal
// status, well-formed operations — and no others: ownership (step 6) and the
// invariants (step 8) are what the stub does not have. atomic=true refuses the
// whole package on the first problem and leaves the world exactly as it was;
// atomic=false refuses only the change sets that failed and applies the rest.
func (s *FakeState) applyUpdate(ctx context.Context, ev eventbus.Event) error {
	var p updateProposal
	if err := decodePayload(ev.Payload, &p); err != nil {
		return s.refuseMalformed(ctx, ev, p.ProposalID)
	}
	if p.ProposalID == "" {
		return s.refuseMalformed(ctx, ev, "")
	}
	if len(p.Changes) == 0 {
		return s.reject(ctx, ev, p.ProposalID, ReasonInvalidOp, nil, nil)
	}
	// One entity, one change set. A package that names an entity twice has no
	// answer C-02 allows: applied one after the other the two sets publish two
	// entity.updated for one entity, and both under the same version, while
	// C-02 says the version moves strictly by one per entity and §4.5 p. 12
	// says one entity.updated per entity; applied as one, whatever the first
	// set asked for is silently dropped. So the stub refuses the package with
	// the reason §4.5 p. 1 gives a malformed proposal and says in the log what
	// to do instead — merge the operations of that entity into a single set.
	//
	// The rule lives here rather than in entity.update.proposed.v1.json
	// because JSON Schema cannot require uniqueness over a nested field
	// (uniqueItems compares whole items, and two sets over one entity differ
	// in their operations). The registry therefore accepts such a package and
	// every implementation of State has to refuse it — T-056 meets the same
	// proposal (review #1 of T-017, Major-1).
	if ref, twice := repeatedEntity(p.Changes); twice {
		s.log.Info("proposal names one entity twice",
			"proposal_id", p.ProposalID, "entity_id", ref.ID,
			"remedy", "merge the operations of one entity into a single change set")
		return s.reject(ctx, ev, p.ProposalID, ReasonInvalidOp, &ref, nil)
	}

	s.mu.Lock()
	if s.seen(p.ProposalID) {
		s.mu.Unlock()
		s.log.Debug("proposal already applied", "proposal_id", p.ProposalID)
		return nil
	}

	// Everything happens on copies, and the originals are replaced only once
	// the whole package has been decided: that is what makes atomic=true an
	// all-or-nothing change of the world rather than a rollback of one
	// (§4.5 p. 7, p. 12).
	applied := make([]*entity.Entity, 0, len(p.Changes))
	changedBy := make(map[string][]entity.Change, len(p.Changes))
	var refusals []refusal
	for _, set := range p.Changes {
		copyOf, changes, bad := s.evaluate(set, p.Cause)
		if bad != nil {
			refusals = append(refusals, *bad)
			if p.Atomic {
				break
			}
			continue
		}
		applied = append(applied, copyOf)
		changedBy[copyOf.ID] = changes
	}

	if p.Atomic && len(refusals) > 0 {
		s.mu.Unlock()
		r := refusals[0]
		return s.reject(ctx, ev, p.ProposalID, r.reason, &r.ref, r.details)
	}

	// The facts come out in ascending identifier order, the order §4.5 p. 12
	// gives the writes they follow.
	slices.SortFunc(applied, func(a, b *entity.Entity) int { return cmp.Compare(a.ID, b.ID) })
	for _, e := range applied {
		changes := changedBy[e.ID]
		e.Commit(e.Attributes, changes, entity.LastChange{
			ProposalID:      p.ProposalID,
			ProposalEventID: ev.ID,
			Cause:           p.Cause,
			AppliedAt:       ev.Timestamp,
			Atomic:          p.Atomic,
			BatchSize:       len(p.Changes),
		})
		s.entities[e.ID] = e
	}
	if len(applied) > 0 {
		s.remember(p.ProposalID)
	}
	s.mu.Unlock()

	for _, e := range applied {
		fact := eventbus.Derive(ev, TypeUpdated, Source, map[string]any{
			"entity":      entityRefPayload(e),
			"version":     e.Version,
			"changed":     changedPayload(changedBy[e.ID]),
			"cause":       p.Cause,
			"proposal_id": p.ProposalID,
			"applied_at":  ev.Timestamp,
		})
		if err := s.bus.Publish(ctx, fact); err != nil {
			return fmt.Errorf("testkit/state: publish %s for %s: %w", TypeUpdated, e.Ref(), err)
		}
		s.mu.Lock()
		e.SetFactEventID(fact.ID)
		s.mu.Unlock()
		s.log.Info("entity updated", "entity_id", e.ID, "version", e.Version,
			"cause", p.Cause, "event_id", fact.ID)
	}

	for _, r := range refusals {
		if err := s.reject(ctx, ev, p.ProposalID, r.reason, &r.ref, r.details); err != nil {
			return err
		}
	}
	return nil
}

// evaluate runs one change set against the world and reports either the copy
// to commit or the reason to refuse it. The caller holds the lock.
func (s *FakeState) evaluate(set entity.ChangeSet, cause string) (*entity.Entity, []entity.Change, *refusal) {
	ref := set.Ref()
	if ref.ID == "" || len(set.Ops) == 0 {
		return nil, nil, &refusal{reason: ReasonInvalidOp, ref: ref}
	}
	current, ok := s.entities[ref.ID]
	if !ok {
		return nil, nil, &refusal{reason: ReasonUnknownEntity, ref: ref}
	}
	if err := current.CheckVersion(set.ExpectedVersion); err != nil {
		var conflict entity.ErrVersionConflict
		if !errors.As(err, &conflict) {
			return nil, nil, &refusal{reason: ReasonInvalidOp, ref: ref}
		}
		return nil, nil, &refusal{reason: ReasonVersionConflict, ref: ref, details: map[string]any{
			"expected_version": conflict.Expected,
			"actual_version":   conflict.Actual,
		}}
	}
	if current.IsTerminal() && !onlyCorpsePaths(set.Ops) {
		// The rule that makes abandoned terminal: over dead, abandoned and
		// ascended_final nothing but the four paths of a corpse is allowed, so
		// a second /forget over an already abandoned character is refused here
		// (§4.5 p. 5, C-02 v1.2, З-2).
		return nil, nil, &refusal{reason: ReasonDeadEntity, ref: ref}
	}

	copyOf := entity.Clone(current)
	attrs, changes, err := entity.ApplyOps(copyOf, set.Ops)
	if err != nil {
		return nil, nil, &refusal{reason: ReasonInvalidOp, ref: ref}
	}
	if bad := statusRefusal(changes, cause, ref); bad != nil {
		return nil, nil, bad
	}
	// The attributes travel on the copy so that Commit has them when the whole
	// package has been decided; nothing is visible to a reader until the copy
	// replaces the original.
	copyOf.Attributes = attrs
	return copyOf, changes, nil
}

// repeatedEntity reports the first entity a package names more than once. The
// identifier alone decides: two sets differing only in the type they claim for
// one identifier are still two sets over one entity.
func repeatedEntity(sets []entity.ChangeSet) (entity.Ref, bool) {
	seen := make(map[string]struct{}, len(sets))
	for _, set := range sets {
		ref := set.Ref()
		if _, twice := seen[ref.ID]; twice {
			return ref, true
		}
		seen[ref.ID] = struct{}{}
	}
	return entity.Ref{}, false
}

// statusRefusal checks the status transition a change set asks for against the
// matrix entity.StatusTransitionAllowed holds (data-model.md §3.3, §9.1;
// C-02 v1.2), and against the one cause that may end a living character
// without killing them: alive -> abandoned is the /forget of the gateway and
// carries cause=forget. Nothing else may reach abandoned, so a swarm that
// proposes it after a fight is refused here instead of passing on the stub and
// failing against the real State.
//
// What the stub still does not check is who proposed it. C-02 v1.2 and З-2
// give the transition to the gateway alone, and a proposal carrying meta.agent
// is answered by the real State with level_violation — a reason that needs the
// ownership table, which v0 does not have (design.md §5). The gap is named in
// the dev-log of T-017; the matrix of the stub stays at five reasons.
//
// A set that leaves the status where it was produces no change at all
// (entity.ApplyOps drops it), so a no-op is neither a transition nor a
// refusal.
func statusRefusal(changes []entity.Change, cause string, ref entity.Ref) *refusal {
	for _, c := range changes {
		if c.Path != entity.AttrStatus {
			continue
		}
		from, _ := c.Old.(string)
		to, _ := c.New.(string)
		if !entity.StatusTransitionAllowed(from, to) {
			return &refusal{reason: ReasonInvalidOp, ref: ref}
		}
		if to == entity.StatusAbandoned && cause != CauseForget {
			return &refusal{reason: ReasonInvalidOp, ref: ref}
		}
	}
	return nil
}

// corpsePaths are the four attributes an entity keeps accepting after it is
// gone: how it died, who killed it, who took the trophy and which encounter it
// fell in (§4.5 p. 5). Without them a trophy could never be handed out, since
// the NPC it is taken from is dead by definition (inv-03).
//
// (T-011 defines the other three); see the open questions of T-017.
var corpsePaths = []string{
	entity.AttrDiedAt, entity.AttrKilledBy, entity.AttrLootClaimedBy, entity.AttrEncounterID,
}

func onlyCorpsePaths(ops []entity.Op) bool {
	for _, op := range ops {
		if !slices.Contains(corpsePaths, op.Path) {
			return false
		}
	}
	return len(ops) > 0
}

// reject publishes entity.update.rejected. It answers both proposal types: the
// name of the event is the name C-02 gives the refusal of either
// (duplicate_entity is a refusal of a create).
func (s *FakeState) reject(ctx context.Context, cause eventbus.Event, proposalID, reason string,
	ref *entity.Ref, details map[string]any) error {
	payload := map[string]any{"proposal_id": proposalID, "reason": reason}
	if ref != nil && ref.ID != "" {
		payload["entity"] = map[string]any{
			"entity": map[string]any{"id": ref.ID, "type": ref.Type},
		}
	}
	if len(details) > 0 {
		payload["details"] = details
	}
	ev := eventbus.Derive(cause, TypeRejected, Source, payload)
	if err := s.bus.Publish(ctx, ev); err != nil {
		return fmt.Errorf("testkit/state: publish %s (%s): %w", TypeRejected, reason, err)
	}
	s.log.Info("proposal rejected", "proposal_id", proposalID, "reason", reason, "event_id", ev.ID)
	return nil
}

// refuseMalformed answers a proposal that could not be read. With a
// proposal_id it is refused as invalid_op like any other malformed proposal.
//
// Without one there is nothing to refuse: entity.update.rejected requires the
// proposal_id, both proposal schemas require it too (C-02 v1.6), and the stub
// no longer makes one up from the event id — T-055 would have no rule to
// repeat it by. A bus that validates on read parks such an event in
// dead_letters before it reaches the stub, so this branch is reached only past
// a lenient bus or by a direct call, and it says so in the log.
func (s *FakeState) refuseMalformed(ctx context.Context, ev eventbus.Event, proposalID string) error {
	if proposalID == "" {
		s.log.Warn("proposal without proposal_id passed over: nothing to refuse it by",
			"event_id", ev.ID, "type", ev.Type)
		return nil
	}
	return s.reject(ctx, ev, proposalID, ReasonInvalidOp, nil, nil)
}

// entityRefPayload is an entity as _common.json#/$defs/EntityWithName carries
// it.
func entityRefPayload(e *entity.Entity) map[string]any {
	out := map[string]any{
		"entity": map[string]any{"id": e.ID, "type": e.Type},
	}
	if e.Name != "" {
		out["name"] = e.Name
	}
	return out
}

// changedPayload is the changed list of entity.updated. It is always an array,
// never null: an empty list means the turn counted and the version did not
// move, and the schema requires the field either way (C-02).
//
// old is written only when the path existed before and new only when it exists
// after (C-02 v1.6): an append carries no old and a remove of a key no new, so
// a consumer that catches up deletes the key instead of writing a null into it.
func changedPayload(changes []entity.Change) []map[string]any {
	out := make([]map[string]any, 0, len(changes))
	for _, c := range changes {
		entry := map[string]any{"path": c.Path}
		if c.HasOld {
			entry["old"] = c.Old
		}
		if c.HasNew {
			entry["new"] = c.New
		}
		out = append(out, entry)
	}
	return out
}
