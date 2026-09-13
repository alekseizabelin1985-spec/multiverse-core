package store_test

import (
	"context"
	"database/sql"
	"testing"

	"multiverse-core.io/internal/gateway/store"
)

// openLinks opens links.db in dir, applies the migrations and closes the
// database when the test ends. t.Cleanup runs in reverse order, so the file is
// closed before sqlitedir.Temp removes it (Windows refuses to remove an open file).
func openLinks(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := store.OpenLinks(context.Background(), store.LinksPath(dir))
	if err != nil {
		t.Fatalf("OpenLinks: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateLinks(context.Background(), db); err != nil {
		t.Fatalf("MigrateLinks: %v", err)
	}
	return db
}

// openGateway is openLinks for gateway.db.
func openGateway(t *testing.T, dir string) *sql.DB {
	t.Helper()
	db, err := store.OpenGateway(context.Background(), store.GatewayPath(dir))
	if err != nil {
		t.Fatalf("OpenGateway: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateGateway(context.Background(), db); err != nil {
		t.Fatalf("MigrateGateway: %v", err)
	}
	return db
}

func exec(t *testing.T, db *sql.DB, query string, args ...any) {
	t.Helper()
	if _, err := db.ExecContext(context.Background(), query, args...); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
}

func queryInt(t *testing.T, db *sql.DB, query string, args ...any) int {
	t.Helper()
	var n int
	if err := db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// insertLink adds a consented link with the given external ID.
func insertLink(t *testing.T, db *sql.DB, platform, externalID, linkID, playerID string) {
	t.Helper()
	exec(t, db, `INSERT INTO links
		(link_id, external_platform, external_id, player_id, world_id, status,
		 notice_shown_at, consent_at, age_confirmed_at, last_seen_at, created_at)
		VALUES (?, ?, ?, ?, 'world-dark-forest', 'consented',
		 '2026-09-13T10:00:00Z', '2026-09-13T10:00:01Z', '2026-09-13T10:00:01Z',
		 '2026-09-13T10:00:02Z', '2026-09-13T10:00:00Z')`,
		linkID, platform, externalID, playerID)
}
