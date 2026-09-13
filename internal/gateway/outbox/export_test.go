package outbox

// HasBell reports whether a long-poll has taken the bell of platform and no
// ring has come since.
func HasBell(n *Notifier, platform string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	_, ok := n.bells[platform]
	return ok
}
