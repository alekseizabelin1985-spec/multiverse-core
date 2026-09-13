package readmodel

// PendingWaiters counts the waits still registered; a wait that ended by any
// way out leaves none behind.
func PendingWaiters(m *Model) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := make(map[*Waiter]struct{})
	for _, list := range m.waiters {
		for _, w := range list {
			seen[w] = struct{}{}
		}
	}
	return len(seen)
}

// IndexedEncounters counts the encounters the index holds under a player: only
// open ones, so that the index does not grow with every fight of the process.
func IndexedEncounters(m *Model, playerID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.open[playerID])
}
