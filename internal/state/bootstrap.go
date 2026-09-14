package state

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"multiverse-core.io/shared/clock"
	"multiverse-core.io/shared/contracts"
	"multiverse-core.io/shared/entity"
	"multiverse-core.io/shared/eventbus"
	"multiverse-core.io/shared/runtime"
)

// The bootstrap of a world (state-and-mechanics.md §4.10): the one way a world
// comes to be on MVP-1 is the fixtures, proposed to State one entity at a time
// as the operator proposes them — source mvctl, the proposer author, cause
// init, actor_kind system (C-02 v1.5). mvctl world init calls it, and so does a
// harness that runs State in its own process: it plays the operator.

// CauseInit is the cause of every proposal of a bootstrap.
const CauseInit = "init"

// BootstrapTimeout is how long the bootstrap waits for the answer to one
// proposal (§4.10).
const BootstrapTimeout = 10 * time.Second

// FixtureFiles are the entity files of a fixtures directory in the order a
// bootstrap proposes them, and the one type each file holds: a world before its
// regions, a region before what stands in it (§4.10). The laws of the world
// read that order — an NPC placed in a region that does not exist yet breaks
// inv-10.
var FixtureFiles = []FixtureFile{
	{Name: "world.json", Type: entity.TypeWorld},
	{Name: "region.json", Type: entity.TypeRegion},
	{Name: "npc.json", Type: entity.TypeNPC},
	{Name: "players.json", Type: entity.TypePlayer},
}

// FixtureFile is one entity file of the fixtures and the type of its entities.
type FixtureFile struct {
	Name string
	Type string
}

// ErrFixtures is a fixtures directory a world cannot be created from.
var ErrFixtures = errors.New("state: fixtures")

// ErrBootstrap is a bootstrap that did not bring the world into being: a
// proposal refused, or not answered in time.
var ErrBootstrap = errors.New("state: bootstrap")

// BootstrapResult is what a bootstrap did, in the order it proposed.
type BootstrapResult struct {
	World string `json:"world"`
	// Created are the entities State created on this run.
	Created []entity.Ref `json:"created"`
	// Skipped are the entities already there: created by an earlier bootstrap
	// whose fact is in the journal, or refused duplicate_entity.
	Skipped []entity.Ref `json:"skipped"`
}

// BootstrapOption changes how a bootstrap waits.
type BootstrapOption func(*bootstrapOptions)

type bootstrapOptions struct {
	timers  clock.Timers
	timeout time.Duration
}

// WithAnswerTimers sets the timers the wait for an answer is measured on. The
// default is the real timers, whatever the mode: the wait is transport time,
// not domain time, and a NullTimers pause of replay would never end (the same
// reason as Config.Timers). A test passes manual timers.
func WithAnswerTimers(timers clock.Timers) BootstrapOption {
	return func(o *bootstrapOptions) { o.timers = timers }
}

// BootstrapProposalID is the deterministic proposal_id of the entity of a
// bootstrap, bootstrap:{world}:{type}/{id} (§4.10, C-02 v1.6): a second
// bootstrap proposes under the same identifier, and State recognises it.
func BootstrapProposalID(worldID string, ref entity.Ref) string {
	return "bootstrap:" + worldID + ":" + ref.Type + "/" + ref.ID
}

// LoadFixtures reads the entities of a world out of a fixtures directory in the
// order of FixtureFiles.
//
// A fixture that does not describe worldID is refused rather than proposed:
// world.json holds exactly the world itself, every entity names the world, a
// file holds only its type, and no identifier appears twice. Unknown fields are
// refused too — a misspelled attribute would otherwise create an entity that
// quietly lacks it.
func LoadFixtures(worldID, dir string) ([]*entity.Entity, error) {
	if worldID == "" {
		return nil, fmt.Errorf("%w: no world", ErrFixtures)
	}
	var all []*entity.Entity
	seen := map[string]string{}
	for _, file := range FixtureFiles {
		path := filepath.Join(dir, file.Name)
		batch, err := readFixtureFile(path)
		if err != nil {
			return nil, err
		}
		if len(batch) == 0 {
			return nil, fmt.Errorf("%w: %s holds no entities", ErrFixtures, path)
		}
		for _, e := range batch {
			if err := checkFixture(worldID, path, file, e, seen); err != nil {
				return nil, err
			}
		}
		if file.Type == entity.TypeWorld && (len(batch) != 1 || batch[0].ID != worldID) {
			return nil, fmt.Errorf("%w: %s must hold the world %s and nothing else", ErrFixtures, path, worldID)
		}
		all = append(all, batch...)
	}
	return all, nil
}

func checkFixture(worldID, path string, file FixtureFile, e *entity.Entity, seen map[string]string) error {
	switch {
	case e == nil || e.ID == "":
		return fmt.Errorf("%w: %s holds an entity without an id", ErrFixtures, path)
	case e.Type != file.Type:
		return fmt.Errorf("%w: %s holds %s of type %q, want %s", ErrFixtures, path, e.ID, e.Type, file.Type)
	case e.WorldID != worldID:
		return fmt.Errorf("%w: %s: %s belongs to the world %q, not to %s", ErrFixtures, path, e.ID, e.WorldID, worldID)
	}
	if other, twice := seen[e.ID]; twice {
		return fmt.Errorf("%w: %s: the id %s is already used in %s", ErrFixtures, path, e.ID, other)
	}
	seen[e.ID] = path
	return nil
}

func readFixtureFile(path string) ([]*entity.Entity, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // the fixtures directory is the operator's argument
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrFixtures, err)
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var batch []*entity.Entity
	if err := dec.Decode(&batch); err != nil {
		return nil, fmt.Errorf("%w: decode %s: %w", ErrFixtures, path, err)
	}
	if err := dec.Decode(new(json.RawMessage)); !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("%w: %s holds more than one JSON document", ErrFixtures, path)
	}
	return batch, nil
}

// BootstrapProposal is entity.create.proposed of one fixture entity as the
// bootstrap publishes it (§4.10, C-02 v1.5): a root event of the world from
// mvctl, the actor system, cause init and the deterministic proposal_id. The
// attributes are the fixture's; version, timestamps and the commit record are
// State's to write.
func BootstrapProposal(worldID string, e *entity.Entity) eventbus.Event {
	named := map[string]any{"entity": map[string]any{"id": e.ID, "type": e.Type}}
	if e.Name != "" {
		named["name"] = e.Name
	}
	attributes := e.Attributes
	if attributes == nil {
		attributes = map[string]any{}
	}
	return eventbus.NewRoot(TypeCreateProposed, contracts.SourceMvctl, worldID, nil, eventbus.ActorSystem,
		map[string]any{
			"proposal_id": BootstrapProposalID(worldID, e.Ref()),
			"entity":      named,
			"attributes":  attributes,
			"cause":       CauseInit,
		})
}

// Bootstrap creates the world worldID from the fixtures in fixturesDir through
// State (§4.10): it publishes entity.create.proposed for every fixture entity
// in the order of FixtureFiles and waits for the answer to each, reading
// system_events through the journal, before it proposes the next.
//
// It is idempotent. An entity whose entity.created of the same proposal_id is
// already in the journal is not proposed again: State drops a repeat of an
// applied proposal without an answer (§4.5 p. 2), and there would be nothing to
// wait for. An entity State refuses duplicate_entity — there already, the
// window and its commit record no longer naming the bootstrap — is skipped as
// well. Any other refusal, or no answer within BootstrapTimeout, ends the
// bootstrap with ErrBootstrap; what was created stays.
//
// deps needs Bus and Journal, one transport for both.
func Bootstrap(ctx context.Context, deps runtime.Deps, worldID, fixturesDir string, opts ...BootstrapOption) (BootstrapResult, error) {
	options := bootstrapOptions{timers: clock.RealTimers{}, timeout: BootstrapTimeout}
	for _, opt := range opts {
		opt(&options)
	}
	result := BootstrapResult{World: worldID, Created: []entity.Ref{}, Skipped: []entity.Ref{}}
	if deps.Bus == nil || deps.Journal == nil {
		return result, fmt.Errorf("%w: a bus and a journal are needed", ErrBootstrap)
	}
	fixtures, err := LoadFixtures(worldID, fixturesDir)
	if err != nil {
		return result, err
	}
	end, err := deps.Journal.End(ctx, eventbus.TopicSystemEvents)
	if err != nil {
		return result, fmt.Errorf("%w: the end of %s: %w", ErrBootstrap, eventbus.TopicSystemEvents, err)
	}
	created, err := createdBefore(ctx, deps.Journal, worldID, end)
	if err != nil {
		return result, err
	}

	answers := newAnswerBoard(worldID)
	for _, e := range fixtures {
		if !created[BootstrapProposalID(worldID, e.Ref())] {
			answers.expect(BootstrapProposalID(worldID, e.Ref()))
		}
	}
	tailCtx, stopTail := context.WithCancel(ctx)
	tail := &tailRun{done: make(chan struct{})}
	go func() {
		defer close(tail.done)
		tail.err = deps.Journal.Tail(tailCtx, eventbus.TopicSystemEvents, end, answers.take)
	}()
	defer func() {
		stopTail()
		<-tail.done
	}()

	for _, e := range fixtures {
		ref := e.Ref()
		id := BootstrapProposalID(worldID, ref)
		if created[id] {
			result.Skipped = append(result.Skipped, ref)
			continue
		}
		if err := deps.Bus.Publish(ctx, BootstrapProposal(worldID, e)); err != nil {
			return result, fmt.Errorf("%w: publish the proposal of %s/%s: %w", ErrBootstrap, ref.Type, ref.ID, err)
		}
		answer, err := answers.wait(ctx, id, options, tail)
		if err != nil {
			return result, fmt.Errorf("%w: %s/%s: %w", ErrBootstrap, ref.Type, ref.ID, err)
		}
		if answer.Type == TypeCreated {
			result.Created = append(result.Created, ref)
			continue
		}
		reason, _ := answer.Path().GetString("reason")
		if reason != string(ReasonDuplicateEntity) {
			return result, fmt.Errorf("%w: %s/%s refused %s (event %s)", ErrBootstrap, ref.Type, ref.ID, reason, answer.ID)
		}
		result.Skipped = append(result.Skipped, ref)
	}
	return result, nil
}

// createdBefore are the proposal identifiers of the world that already have
// their entity.created in [0, end) of the journal.
func createdBefore(ctx context.Context, journal eventbus.Journal, worldID string, end int64) (map[string]bool, error) {
	created := map[string]bool{}
	if end <= 0 {
		return created, nil
	}
	_, err := journal.ReadRange(ctx, eventbus.TopicSystemEvents, 0, end, func(_ context.Context, ev eventbus.Event) error {
		if ev.Type == TypeCreated && worldOf(ev) == worldID {
			if id, _ := ev.Path().GetString("proposal_id"); id != "" {
				created[id] = true
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: read %s: %w", ErrBootstrap, eventbus.TopicSystemEvents, err)
	}
	return created, nil
}

// tailRun is the read of the answers: done closes when Tail has returned, and
// err is what it returned.
type tailRun struct {
	done chan struct{}
	err  error
}

// answerBoard hands the answers the journal delivers to the proposal waiting
// for them. The first answer to a proposal is the one kept.
type answerBoard struct {
	worldID string
	mu      sync.Mutex
	slots   map[string]chan eventbus.Event
}

func newAnswerBoard(worldID string) *answerBoard {
	return &answerBoard{worldID: worldID, slots: map[string]chan eventbus.Event{}}
}

func (b *answerBoard) expect(proposalID string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.slots[proposalID] = make(chan eventbus.Event, 1)
}

// take is the handler of the tail: it never fails, so the journal neither
// retries it nor parks an event of somebody else in dead_letters.
func (b *answerBoard) take(_ context.Context, ev eventbus.Event) error {
	if (ev.Type != TypeCreated && ev.Type != TypeRejected) || worldOf(ev) != b.worldID {
		return nil
	}
	id, _ := ev.Path().GetString("proposal_id")
	b.mu.Lock()
	slot, ok := b.slots[id]
	b.mu.Unlock()
	if !ok {
		return nil
	}
	select {
	case slot <- ev:
	default:
	}
	return nil
}

// wait is the answer to one proposal, or the reason there is none.
func (b *answerBoard) wait(ctx context.Context, proposalID string, options bootstrapOptions, tail *tailRun) (eventbus.Event, error) {
	b.mu.Lock()
	slot := b.slots[proposalID]
	b.mu.Unlock()
	timer := options.timers.After(options.timeout)
	defer timer.Stop()
	select {
	case ev := <-slot:
		return ev, nil
	case <-timer.C():
		return eventbus.Event{}, fmt.Errorf("no answer to %s within %s: is the context state serving the world %s?",
			proposalID, options.timeout, b.worldID)
	case <-tail.done:
		err := tail.err
		if err == nil {
			err = errors.New("the journal stopped")
		}
		return eventbus.Event{}, fmt.Errorf("reading the answer to %s: %w", proposalID, err)
	case <-ctx.Done():
		return eventbus.Event{}, ctx.Err()
	}
}
