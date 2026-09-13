package laws

import (
	"maps"
	"sync"
)

// Strain counts the pressure on each law: how many times the guardian rejected
// an output with law_violation against it (ADR-008 p. 5). In MVP-1 the counter
// only accumulates — it has no threshold and no consequence — and is rebuilt
// on catch-up from llm.output.rejected reason=law_violation by the swarm
// (swarm-llm-laws.md §12.3). The thresholds and the breach are E-B.
//
// The zero value is an empty counter ready for use.
type Strain struct {
	mu     sync.Mutex
	counts map[string]int64
}

// Inc adds one to the strain of a law. An empty identifier is not a law and is
// not counted.
func (s *Strain) Inc(lawID string) {
	if lawID == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.counts == nil {
		s.counts = make(map[string]int64)
	}
	s.counts[lawID]++
}

// Snapshot returns a copy of the counts, so that a reader cannot change the
// counter and a later Inc does not change what the reader holds.
func (s *Strain) Snapshot() map[string]int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]int64, len(s.counts))
	maps.Copy(out, s.counts)
	return out
}
