package store_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/env"
)

func TestDataDirReadsTheManifestVariable(t *testing.T) {
	cases := []struct {
		name    string
		src     map[string]string
		want    string
		wantErr bool
	}{
		{name: "unset takes the default", src: map[string]string{}, want: "/data"},
		{name: "set", src: map[string]string{"MV_GATEWAY_DATA_DIR": "/srv/gateway"}, want: "/srv/gateway"},
		{name: "empty is refused", src: map[string]string{"MV_GATEWAY_DATA_DIR": ""}, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := store.DataDir(env.MapSource(tc.src))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DataDir = %q, want an error", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("DataDir = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	if got, want := store.LinksPath("dir"), filepath.Join("dir", "links.db"); got != want {
		t.Errorf("LinksPath = %q, want %q", got, want)
	}
	if got, want := store.GatewayPath("dir"), filepath.Join("dir", "gateway.db"); got != want {
		t.Errorf("GatewayPath = %q, want %q", got, want)
	}
}

// TestOpenAppliesThePragmas checks the PRAGMAs of component section 4.1/4.2 on
// a new file and again on the existing file after a reopen: auto_vacuum is
// stored in the file and would be lost only on the second opening.
func TestOpenAppliesThePragmas(t *testing.T) {
	cases := []struct {
		name   string
		open   func(t *testing.T, dir string) *sql.DB
		path   func(dir string) string
		reopen func(ctx context.Context, path string) (*sql.DB, error)
		want   map[string]string
	}{
		{
			name: "links.db", open: openLinks, path: store.LinksPath, reopen: store.OpenLinks,
			want: map[string]string{
				"journal_mode": "wal", "synchronous": "2", "foreign_keys": "1",
				"busy_timeout": "5000", "secure_delete": "1", "auto_vacuum": "2",
			},
		},
		{
			name: "gateway.db", open: openGateway, path: store.GatewayPath, reopen: store.OpenGateway,
			want: map[string]string{
				"journal_mode": "wal", "synchronous": "1", "foreign_keys": "1", "busy_timeout": "5000",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tempDir(t)
			db := tc.open(t, dir)
			checkPragmas(t, db, tc.want)
			if got := db.Stats().MaxOpenConnections; got != 1 {
				t.Errorf("MaxOpenConnections = %d, want 1 (single writer, ADR-019)", got)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}

			again, err := tc.reopen(context.Background(), tc.path(dir))
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			t.Cleanup(func() { _ = again.Close() })
			checkPragmas(t, again, tc.want)
		})
	}
}

func checkPragmas(t *testing.T, db *sql.DB, want map[string]string) {
	t.Helper()
	for pragma, value := range want {
		var got string
		if err := db.QueryRowContext(context.Background(), "PRAGMA "+pragma).Scan(&got); err != nil {
			t.Fatalf("PRAGMA %s: %v", pragma, err)
		}
		if !strings.EqualFold(got, value) {
			t.Errorf("PRAGMA %s = %s, want %s", pragma, got, value)
		}
	}
}

// A links.db created elsewhere without auto_vacuum cannot be switched to it
// by a PRAGMA once it has tables, and SQLite says nothing. Such a file would
// keep a forgotten ID in its free pages, so Open must refuse it.
func TestOpenLinksRefusesAFileWithoutIncrementalVacuum(t *testing.T) {
	path := store.LinksPath(tempDir(t))
	plain, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := plain.Exec("CREATE TABLE t (x TEXT)"); err != nil {
		t.Fatal(err)
	}
	if err := plain.Close(); err != nil {
		t.Fatal(err)
	}

	db, err := store.OpenLinks(context.Background(), path)
	if err == nil {
		_ = db.Close()
		t.Fatal("OpenLinks accepted a links.db without auto_vacuum=INCREMENTAL")
	}
	if !strings.Contains(err.Error(), "auto_vacuum") {
		t.Errorf("error = %v, want it to name auto_vacuum", err)
	}
}

func TestOpenRefusesABadPath(t *testing.T) {
	cases := []struct{ path, reason string }{
		{"", "empty database path"},
		// Checked before the file system sees the path: on Windows '?' is not a
		// valid file name character at all, on Linux it would create a file
		// the driver never opens.
		{filepath.Join(tempDir(t), "links.db?mode=ro"), "contains '?'"},
	}
	for _, tc := range cases {
		db, err := store.OpenLinks(context.Background(), tc.path)
		if err == nil {
			_ = db.Close()
			t.Errorf("OpenLinks(%q) succeeded, want an error", tc.path)
			continue
		}
		if !strings.Contains(err.Error(), tc.reason) {
			t.Errorf("OpenLinks(%q): err = %v, want it to say %q", tc.path, err, tc.reason)
		}
	}
}

func TestOpenCreatesThePrivateDirectoryAndFiles(t *testing.T) {
	skipWithoutPOSIXModes(t)
	dir := filepath.Join(tempDir(t), "data")
	links := openLinks(t, dir)
	insertLink(t, links, "ci", "player-A", "link-a", "player-a")
	openGateway(t, dir)

	checkMode(t, dir, 0o700)
	for _, name := range []string{"links.db", "links.db-wal", "links.db-shm", "gateway.db", "gateway.db-wal"} {
		checkMode(t, filepath.Join(dir, name), 0o600)
	}
}

func TestOpenRefusesWiderModes(t *testing.T) {
	skipWithoutPOSIXModes(t)

	t.Run("directory", func(t *testing.T) {
		dir := filepath.Join(tempDir(t), "data")
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if db, err := store.OpenLinks(context.Background(), store.LinksPath(dir)); err == nil {
			_ = db.Close()
			t.Fatal("OpenLinks accepted a data directory with mode 0755")
		}
	})
	t.Run("file", func(t *testing.T) {
		dir := tempDir(t)
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		path := store.GatewayPath(dir)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(path, 0o644); err != nil {
			t.Fatal(err)
		}
		if db, err := store.OpenGateway(context.Background(), path); err == nil {
			_ = db.Close()
			t.Fatal("OpenGateway accepted a database file with mode 0644")
		}
	})
}

// skipWithoutPOSIXModes skips on Windows: it has no POSIX permission bits, Go
// reports 0666 whatever the ACL is, and store does not check modes there. The
// Linux job of CI runs these tests.
func skipWithoutPOSIXModes(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("no POSIX file modes on Windows; checked by the Linux CI job")
	}
}

func checkMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("%s: mode %04o, want %04o", filepath.Base(path), got, want)
	}
}
