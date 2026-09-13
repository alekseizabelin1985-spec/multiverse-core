package actions

// PlayerLocks is the number of entries in the table of player locks: an entry
// lives while a request holds or waits for the lock of its player, and goes
// with its last user.
func PlayerLocks(s *Service) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.players)
}
