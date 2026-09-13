package flow

// HeldChats counts the entries the flow holds — dialogs, cached players and
// pending wipes — without the lazy expiry of dialogOf and playerOf, so that a
// test sees what the sweep let go of (N-2 of review #1 of T-311).
func HeldChats(f *Flow) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.dialogs) + len(f.players) + len(f.forgetting)
}
