package store

import (
	"context"
	"database/sql"
	"fmt"
)

// CompactLinks removes what a DELETE leaves of a row in links.db outside the
// live pages (SEC-04, SEC-05, ADR-019 addendum p. 1). secure_delete zeroes the
// deleted cells, but in WAL mode the zeroed page goes to the WAL while the
// database file still holds the old one, and the WAL still holds the frame
// that inserted the row. So, in one call:
//
//  1. PRAGMA incremental_vacuum returns the free pages to the file system;
//  2. PRAGMA wal_checkpoint(TRUNCATE) copies the current pages into the file
//     and truncates the WAL to zero bytes.
//
// The vacuum goes first because it writes to the WAL itself: with the
// checkpoint last, the WAL ends empty and the file ends shrunk.
//
// links.Store.Forget calls it right after the deleting transaction, before
// answering the client, and the hourly sweeper calls it as a safety net.
func CompactLinks(ctx context.Context, db *sql.DB) error {
	// The driver steps the statement to completion: incremental_vacuum frees
	// one page per step, and a driver that stopped at the first row would free
	// one page.
	if _, err := db.ExecContext(ctx, "PRAGMA incremental_vacuum"); err != nil {
		return fmt.Errorf("store: incremental vacuum: %w", err)
	}
	var busy, walFrames, checkpointed int
	if err := db.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &walFrames, &checkpointed); err != nil {
		return fmt.Errorf("store: wal checkpoint: %w", err)
	}
	// A blocked checkpoint is not an error for SQLite, but it leaves the
	// deleted bytes in the WAL: the caller must not report the data as gone.
	if busy != 0 {
		return fmt.Errorf("store: wal checkpoint blocked: %d of %d frames checkpointed", checkpointed, walFrames)
	}
	return nil
}
