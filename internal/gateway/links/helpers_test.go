package links_test

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"multiverse-core.io/internal/gateway/links"
	"multiverse-core.io/internal/gateway/store"
	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

// externalID is shaped like a Telegram user id; it is not a real account.
const externalID = "7391846205"

// t0 is the time of the tests; the store takes time as an argument.
var t0 = time.Date(2026, 9, 13, 10, 0, 0, 0, time.UTC)

// sequence is the id source of --id-source=sequence: the same calls give the
// same ids in every run.
func sequence() links.IDSource {
	var mu sync.Mutex
	n := 0
	return func() string {
		mu.Lock()
		defer mu.Unlock()
		n++
		return "seq-" + strconv.Itoa(n)
	}
}

type fixture struct {
	db    *sql.DB
	store *links.SQLite
	path  string
}

func newFixture(t *testing.T, hooks ...links.ForgetHooks) fixture {
	t.Helper()
	dir := sqlitedir.Temp(t)
	path := store.LinksPath(dir)
	db, err := store.OpenLinks(context.Background(), path)
	if err != nil {
		t.Fatalf("OpenLinks: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if _, err := store.MigrateLinks(context.Background(), db); err != nil {
		t.Fatalf("MigrateLinks: %v", err)
	}
	s, err := links.NewSQLite(db, sequence(), hooks...)
	if err != nil {
		t.Fatalf("NewSQLite: %v", err)
	}
	return fixture{db: db, store: s, path: path}
}

func (f fixture) resolve(t *testing.T, id string, now time.Time) links.Resolution {
	t.Helper()
	res, err := f.store.Resolve(context.Background(), links.PlatformTelegram, id, now)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return res
}

func complete(shownAt time.Time) links.ConsentForm {
	return links.ConsentForm{NoticeShown: true, Consent: true, AgeConfirmed: true, ShownAt: shownAt}
}

func (f fixture) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := f.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// occurrences counts needle in the file at path; a missing file has none.
func occurrences(t *testing.T, path, needle string) int {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return 0
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return bytes.Count(data, []byte(needle))
}
