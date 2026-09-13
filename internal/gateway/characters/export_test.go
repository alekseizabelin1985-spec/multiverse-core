package characters

import "time"

// SetStatusTimeout shortens the deadline of the reading in Status.
func SetStatusTimeout(s *Service, d time.Duration) { s.statusTimeout = d }
