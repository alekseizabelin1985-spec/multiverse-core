package links

// SetCompacted puts f between a successful checkpoint of Compact and the
// clearing of the mark "compaction pending".
func SetCompacted(s *SQLite, f func()) { s.compacted = f }
