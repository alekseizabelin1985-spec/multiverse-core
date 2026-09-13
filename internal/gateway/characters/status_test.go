package characters_test

import (
	"context"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/characters"
	"multiverse-core.io/shared/clock"
)

// The reading of pending_characters in Status has a deadline: a transaction
// that holds the only connection of gateway.db does not hold the answer to
// resolve with it (acceptance of T-306, Mi-2).
func TestStatusDoesNotWaitForAHeldConnection(t *testing.T) {
	f := newFixture(t)
	characters.SetStatusTimeout(f.svc, 100*time.Millisecond)
	tx, err := f.gatewayDB.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback() }()

	began := clock.Real{}.Now()
	got := f.svc.Status("player-unknown")
	if took := (clock.Real{}).Now().Sub(began); took > 2*time.Second {
		t.Errorf("Status waited %s for the held connection", took)
	}
	if got != characters.StatusCreating {
		t.Errorf("Status under a held connection = %s, want creating, the answer to a reading that failed", got)
	}
}
