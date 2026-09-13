// Package memstore holds the entities of the worlds of State in memory: the
// working set the worker of a world reads a proposal against and swaps its
// result into (state-and-mechanics.md §4.2, §4.3).
//
// Every entity goes in and comes out as a deep copy. The worker owns the
// entities it is deciding about until it hands them over, and a reader —
// /health, a test, later a snapshot — cannot change the world behind its back
// or see a copy the worker is still changing.
//
// It is also the store of the unit tests and of the replacement of
// shared/testkit/state (T-056): a State without object storage. The object
// store behind it (write-through, intents, snapshots) is T-057.
package memstore

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"sync"

	"multiverse-core.io/shared/entity"
)

// Store is the entities of every world State serves, keyed by world and by
// identifier. The identifier alone is unique within a world
// (state-and-mechanics.md §4.2).
type Store struct {
	mu     sync.RWMutex
	worlds map[string]map[string]*entity.Entity
}

// New returns an empty store.
func New() *Store {
	return &Store{worlds: make(map[string]map[string]*entity.Entity)}
}

// Get returns a copy of one entity of a world.
func (s *Store) Get(worldID, id string) (*entity.Entity, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.worlds[worldID][id]
	if !ok {
		return nil, false
	}
	return entity.Clone(e), true
}

// Put writes entities into a world, replacing those with the same identifier.
// It writes all of them or none: an entity without an identifier refuses the
// whole call, because a package that State decided as one must reach the
// working set as one (§4.5 p. 9).
func (s *Store) Put(worldID string, entities ...*entity.Entity) error {
	if worldID == "" {
		return errors.New("memstore: put: no world")
	}
	copies := make([]*entity.Entity, 0, len(entities))
	for i, e := range entities {
		if e == nil || e.ID == "" {
			return fmt.Errorf("memstore: put into %s: entity %d has no identifier", worldID, i)
		}
		copies = append(copies, entity.Clone(e))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	world, ok := s.worlds[worldID]
	if !ok {
		world = make(map[string]*entity.Entity)
		s.worlds[worldID] = world
	}
	for _, e := range copies {
		world[e.ID] = e
	}
	return nil
}

// List returns copies of the entities of a world in the (type, id) order of a
// snapshot and of the state hash (§3.3, §4.4).
func (s *Store) List(worldID string) []*entity.Entity {
	s.mu.RLock()
	defer s.mu.RUnlock()
	world := s.worlds[worldID]
	out := make([]*entity.Entity, 0, len(world))
	for _, e := range world {
		out = append(out, entity.Clone(e))
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int {
		if c := cmp.Compare(a.Type, b.Type); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
	return out
}

// Len is the number of entities of a world.
func (s *Store) Len(worldID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.worlds[worldID])
}
