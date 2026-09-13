package memstore_test

import (
	"testing"
	"time"

	"multiverse-core.io/internal/state/memstore"
	"multiverse-core.io/shared/entity"
)

func player(id string, hp int) *entity.Entity {
	return entity.New(entity.Ref{ID: id, Type: entity.TypePlayer}, "w", id, map[string]any{"hp": hp}, time.Time{})
}

// What goes in and what comes out are copies: a caller that goes on changing
// the entity it put, or the one it got, changes nothing in the world.
func TestTheStoreHandsOutCopies(t *testing.T) {
	s := memstore.New()
	put := player("player-A", 10)
	if err := s.Put("w", put); err != nil {
		t.Fatal(err)
	}
	put.Attributes["hp"] = 1

	got, ok := s.Get("w", "player-A")
	if !ok || got.Attributes["hp"] != 10 {
		t.Fatalf("Get = %v %v, want hp 10 as put", got, ok)
	}
	got.Attributes["hp"] = 2
	for _, e := range s.List("w") {
		e.Attributes["hp"] = 3
	}
	if again, _ := s.Get("w", "player-A"); again.Attributes["hp"] != 10 {
		t.Errorf("hp %v, want 10: a copy handed out reached the world", again.Attributes["hp"])
	}
}

// A put that cannot be written whole writes nothing.
func TestAPutIsAllOrNothing(t *testing.T) {
	s := memstore.New()
	if err := s.Put("w", player("player-A", 10), &entity.Entity{}); err == nil {
		t.Fatal("Put accepted an entity without an identifier")
	}
	if err := s.Put("w", player("player-A", 10), nil); err == nil {
		t.Fatal("Put accepted a nil entity")
	}
	if err := s.Put("", player("player-A", 10)); err == nil {
		t.Fatal("Put accepted no world")
	}
	if n := s.Len("w"); n != 0 {
		t.Errorf("%d entities in the world after refused puts, want 0", n)
	}
}

// Worlds are apart, and a list is in the (type, id) order of a snapshot.
func TestWorldsAreApartAndListedInSnapshotOrder(t *testing.T) {
	s := memstore.New()
	npc := entity.New(entity.Ref{ID: "a-wolf", Type: entity.TypeNPC}, "w", "", nil, time.Time{})
	if err := s.Put("w", player("player-B", 1), player("player-A", 1), npc); err != nil {
		t.Fatal(err)
	}
	if err := s.Put("other", player("player-C", 1)); err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, e := range s.List("w") {
		order = append(order, e.Type+"/"+e.ID)
	}
	if want := []string{"npc/a-wolf", "player/player-A", "player/player-B"}; len(order) != 3 ||
		order[0] != want[0] || order[1] != want[1] || order[2] != want[2] {
		t.Errorf("List = %v, want %v", order, want)
	}
	if _, ok := s.Get("w", "player-C"); ok {
		t.Error("an entity of another world is visible")
	}
	if s.Len("other") != 1 || s.Len("nowhere") != 0 || len(s.List("nowhere")) != 0 {
		t.Error("Len or List of a world is wrong")
	}
}
