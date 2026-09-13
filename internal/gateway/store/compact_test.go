package store_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// forgottenID is a Telegram user id shaped test value; it is not a real
// account.
const forgottenID = "7391846205"

// TestCompactLinksWipesAForgottenIDFromTheFileAndTheWAL is the physical
// deletion check of SEC-04, SEC-05 and NFR-042: after DELETE and CompactLinks
// the ID is found neither in links.db nor in its -wal and -shm files. The scan
// is a byte search in Go, the strings(1) of the task card without the shell.
func TestCompactLinksWipesAForgottenIDFromTheFileAndTheWAL(t *testing.T) {
	ctx := context.Background()
	dir := sqlitedir.Temp(t)
	db := openLinks(t, dir)
	path := store.LinksPath(dir)

	// The forgotten ID sorts before the fillers, so it shares the first leaf
	// page with links that stay: the page is rewritten, not freed, and only
	// secure_delete can wipe the cell. The deleted half of the fillers frees
	// whole pages for the vacuum.
	insertLink(t, db, "telegram", forgottenID, "link-forgotten", "player-forgotten")
	exec(t, db, `INSERT INTO character_requests (link_id, action_key, player_id, status_code, response_json, expires_at)
		VALUES ('link-forgotten', 'k-1', 'player-forgotten', 201, '{}', '2026-09-14T10:00:00Z')`)
	const fillers = 120
	for i := range fillers {
		insertLink(t, db, "telegram", fmt.Sprintf("filler-%04d-%s", i, strings.Repeat("x", 400)),
			fmt.Sprintf("link-%04d", i), fmt.Sprintf("player-%04d", i))
	}
	// Move the rows into the database file itself: the ID must be wiped from
	// the file, not only dropped with the WAL.
	if err := store.CompactLinks(ctx, db); err != nil {
		t.Fatalf("CompactLinks after the inserts: %v", err)
	}
	if n := occurrences(t, path); n == 0 {
		t.Fatal("control: the ID is not in links.db after the insert, the scan below would prove nothing")
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		"DELETE FROM links WHERE external_platform = 'telegram' AND external_id = '" + forgottenID + "'",
		fmt.Sprintf("DELETE FROM links WHERE external_id >= 'filler-%04d'", fillers/2),
	} {
		if _, err := tx.ExecContext(ctx, q); err != nil {
			_ = tx.Rollback()
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if n := queryInt(t, db, "SELECT COUNT(*) FROM character_requests"); n != 0 {
		t.Fatalf("character_requests after the delete: %d rows, want 0", n)
	}
	// The committed DELETE, secure_delete included, is not enough: the zeroed
	// page sits in the WAL and links.db itself still holds the ID. This fails if
	// SQLite checkpointed on its own (wal_autocheckpoint) and the file were
	// already clean, in which case the scan below would not prove the
	// checkpoint of CompactLinks.
	if n := occurrences(t, path); n == 0 {
		t.Fatal("control: links.db no longer holds the ID right after the DELETE, the scan below would not prove CompactLinks")
	}
	if info, err := os.Stat(path + "-wal"); err != nil || info.Size() == 0 {
		t.Fatalf("control: the DELETE left nothing in links.db-wal (%v), CompactLinks has no checkpoint to do", err)
	}
	if n := queryInt(t, db, "PRAGMA freelist_count"); n == 0 {
		t.Fatal("control: the DELETE freed no page, the vacuum is not exercised")
	}

	if err := store.CompactLinks(ctx, db); err != nil {
		t.Fatalf("CompactLinks: %v", err)
	}

	for _, name := range []string{path, path + "-wal", path + "-shm"} {
		if n := occurrences(t, name); n != 0 {
			t.Errorf("%s: the forgotten ID occurs %d times after CompactLinks", name, n)
		}
	}
	if n := queryInt(t, db, "PRAGMA freelist_count"); n != 0 {
		t.Errorf("freelist_count = %d after CompactLinks, want 0", n)
	}
	if info, err := os.Stat(path + "-wal"); err == nil && info.Size() != 0 {
		t.Errorf("links.db-wal is %d bytes after CompactLinks, want 0", info.Size())
	}
	if n := queryInt(t, db, "SELECT COUNT(*) FROM links"); n != fillers/2 {
		t.Errorf("links left: %d, want %d", n, fillers/2)
	}
}

// A reader holding an old snapshot blocks wal_checkpoint(TRUNCATE). SQLite
// answers with busy=1 and no error; CompactLinks must turn that into an
// error, or /forget would report the ID gone while the WAL still holds it.
func TestCompactLinksReportsABlockedCheckpoint(t *testing.T) {
	ctx := context.Background()
	dir := sqlitedir.Temp(t)
	db := openLinks(t, dir)
	insertLink(t, db, "telegram", forgottenID, "link-forgotten", "player-forgotten")

	reader, err := store.OpenLinks(ctx, store.LinksPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reader.Close() })
	tx, err := reader.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	var n int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM links").Scan(&n); err != nil {
		t.Fatal(err)
	}

	exec(t, db, "DELETE FROM links")
	// Do not wait five seconds for the reader: the one connection of db keeps
	// the setting for the call below.
	exec(t, db, "PRAGMA busy_timeout = 0")

	err = store.CompactLinks(ctx, db)
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("CompactLinks with a reader on an old snapshot: err = %v, want a blocked checkpoint", err)
	}
}

// occurrences counts forgottenID in the file at path; a missing file has none.
func occurrences(t *testing.T, path string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.Count(data, []byte(forgottenID))
}
