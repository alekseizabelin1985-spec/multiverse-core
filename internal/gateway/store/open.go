// Package store opens the two SQLite files of the gateway, brings their schema
// up to date and compacts links.db after a deletion (ADR-019, component
// gateway-and-bot.md section 4).
//
// links.db is the only copy of external messenger IDs: it is written with
// synchronous=FULL, and secure_delete plus incremental auto-vacuum let a
// deleted ID be wiped from the file for real. gateway.db holds everything else
// and trades the last milliseconds of a crash for speed (synchronous=NORMAL),
// which the idempotent consumer covers by replaying the bus.
//
// Each database is served by one connection: the gateway is the only writer
// (ADR-004), and one connection rules out SQLITE_BUSY inside the process.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	// The CGO-free SQLite driver, registered as "sqlite" (ADR-004, ADR-019 p. 1).
	_ "modernc.org/sqlite"

	"multiverse-core.io/shared/env"
)

// File names inside MV_GATEWAY_DATA_DIR.
const (
	LinksFile   = "links.db"
	GatewayFile = "gateway.db"
)

const (
	dirMode  fs.FileMode = 0o700
	fileMode fs.FileMode = 0o600

	busyTimeoutMillis = 5000
)

// DataDir returns MV_GATEWAY_DATA_DIR from src; a nil src is the process
// environment. An empty value is an error rather than the working directory:
// the files would land wherever the process happened to start.
func DataDir(src env.Source) (string, error) {
	dir := env.GatewayDataDir.StringFrom(src)
	if dir == "" {
		return "", fmt.Errorf("store: %s is empty", env.GatewayDataDir.Name())
	}
	return dir, nil
}

// LinksPath is the path of links.db inside dir.
func LinksPath(dir string) string { return filepath.Join(dir, LinksFile) }

// GatewayPath is the path of gateway.db inside dir.
func GatewayPath(dir string) string { return filepath.Join(dir, GatewayFile) }

// settings are the PRAGMAs of one database. They go into the DSN rather than
// into statements after Open because the driver applies DSN parameters to
// every new connection: if database/sql replaces a broken connection, the new
// one gets the same PRAGMAs.
type settings struct {
	synchronous  string // FULL or NORMAL
	incremental  bool   // auto_vacuum=INCREMENTAL
	secureDelete bool
}

type pragmaValue struct {
	pragma string
	value  string
}

var (
	linksSettings   = settings{synchronous: "FULL", incremental: true, secureDelete: true}
	gatewaySettings = settings{synchronous: "NORMAL"}
)

// OpenLinks opens (creating if needed) links.db at path with the PRAGMAs of
// component section 4.1 and verifies that they took effect.
func OpenLinks(ctx context.Context, path string) (*sql.DB, error) {
	return open(ctx, path, linksSettings)
}

// OpenGateway opens (creating if needed) gateway.db at path with the PRAGMAs
// of component section 4.2 and verifies that they took effect.
func OpenGateway(ctx context.Context, path string) (*sql.DB, error) {
	return open(ctx, path, gatewaySettings)
}

func open(ctx context.Context, path string, s settings) (*sql.DB, error) {
	if path == "" {
		return nil, errors.New("store: empty database path")
	}
	// The driver splits the DSN at the first '?': a path with one would be
	// opened as a different file with the rest taken for parameters.
	if strings.ContainsRune(path, '?') {
		return nil, fmt.Errorf("store: database path %q contains '?'", path)
	}
	if err := prepareDir(filepath.Dir(path)); err != nil {
		return nil, err
	}
	if err := prepareFile(path); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path+"?"+s.query())
	if err != nil {
		return nil, fmt.Errorf("store: open %s: %w", path, err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := s.verify(ctx, db); err != nil {
		return nil, errors.Join(fmt.Errorf("store: open %s: %w", path, err), db.Close())
	}
	return db, nil
}

func (s settings) query() string {
	q := url.Values{}
	q.Set("_busy_timeout", strconv.Itoa(busyTimeoutMillis))
	q.Set("_foreign_keys", "1")
	q.Set("_journal_mode", "WAL")
	q.Set("_synchronous", s.synchronous)
	if s.incremental {
		// The driver applies auto_vacuum before journal_mode: once page 1
		// exists the setting is locked in.
		q.Set("_auto_vacuum", "INCREMENTAL")
	}
	if s.secureDelete {
		q.Add("_pragma", "secure_delete(1)")
	}
	return q.Encode()
}

// verify reads the PRAGMAs back. SQLite does not fail when a PRAGMA cannot
// apply: auto_vacuum on a file that already has tables, or WAL on a file
// system without shared memory, silently keep the old value. A links.db that
// cannot wipe a forgotten ID must stop the start, not be discovered by a scan.
func (s settings) verify(ctx context.Context, db *sql.DB) error {
	// PRAGMA synchronous and auto_vacuum read back as numbers.
	want := []pragmaValue{
		{"journal_mode", "wal"},
		{"synchronous", map[string]string{"NORMAL": "1", "FULL": "2"}[s.synchronous]},
		{"foreign_keys", "1"},
		{"busy_timeout", strconv.Itoa(busyTimeoutMillis)},
	}
	if s.incremental {
		want = append(want, pragmaValue{"auto_vacuum", "2"})
	}
	if s.secureDelete {
		want = append(want, pragmaValue{"secure_delete", "1"})
	}
	for _, w := range want {
		var got string
		if err := db.QueryRowContext(ctx, "PRAGMA "+w.pragma).Scan(&got); err != nil {
			return fmt.Errorf("read PRAGMA %s: %w", w.pragma, err)
		}
		if !strings.EqualFold(got, w.value) {
			return fmt.Errorf("PRAGMA %s is %s, want %s", w.pragma, got, w.value)
		}
	}
	return nil
}

// prepareDir creates dir with 0700 and refuses one that is open to the group
// or others (ADR-019 p. 1). MkdirAll leaves the mode of an existing directory
// alone, so the check is what catches a volume created with 0755.
func prepareDir(dir string) error {
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("store: create data directory: %w", err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("store: data directory: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("store: data directory %s is not a directory", dir)
	}
	return checkMode(dir, info.Mode(), dirMode)
}

// prepareFile creates the database file with 0600 before SQLite does, so the
// file never exists with the process umask, and refuses a wider existing one.
// SQLite creates the -wal and -shm files with the mode of the database file.
func prepareFile(path string) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, fileMode)
	if err != nil {
		return fmt.Errorf("store: create database file: %w", err)
	}
	info, err := f.Stat()
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("store: database file: %w", err)
	}
	return checkMode(path, info.Mode(), fileMode)
}

func checkMode(path string, got, limit fs.FileMode) error {
	// Windows has no POSIX permission bits: Go reports 0666 or 0444 whatever
	// the ACL is. The check runs where the gateway is deployed (Linux) and in
	// CI; on Windows it would refuse every file.
	if runtime.GOOS == "windows" {
		return nil
	}
	if modeTooWide(got, limit) {
		return fmt.Errorf("store: %s has mode %04o, want %04o or narrower", path, got.Perm(), limit)
	}
	return nil
}

// modeTooWide reports whether got grants a permission bit that limit does
// not. It knows nothing of the operating system, so its test runs everywhere,
// Windows included, while checkMode decides where the comparison applies.
func modeTooWide(got, limit fs.FileMode) bool {
	return got.Perm()&^limit.Perm() != 0
}
