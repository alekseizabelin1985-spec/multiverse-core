package turns

// SetListed puts f between the reading of the active sessions and the cleanup
// of the reserved numbers in Sweep.
func SetListed(t *Tracker, f func()) { t.listed = f }

// Reserves is how many sessions hold a reserved number in the process.
func Reserves(t *Tracker) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.next)
}
