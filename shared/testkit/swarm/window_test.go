package swarm

import "testing"

// TestTheWindowForgetsTheOldestFirst holds the two edges of the window the
// narrator remembers told events in: it is bounded, forgetting the oldest
// event first, and an empty identifier is never remembered, so that an
// envelope without one cannot pass for a duplicate of the last.
func TestTheWindowForgetsTheOldestFirst(t *testing.T) {
	w := newWindow(2)
	for _, id := range []string{"a", "b", "b", "c", ""} {
		w.add(id)
	}
	for id, want := range map[string]bool{"a": false, "b": true, "c": true, "": false} {
		if got := w.has(id); got != want {
			t.Errorf("has(%q) = %v, want %v", id, got, want)
		}
	}
	w.add("d")
	if w.has("b") || !w.has("c") || !w.has("d") {
		t.Error("the window did not forget the oldest event first")
	}
}
