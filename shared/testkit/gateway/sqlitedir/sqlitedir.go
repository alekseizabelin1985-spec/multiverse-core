// Package sqlitedir gives a test a temporary directory for the SQLite files of
// the gateway (MV_GATEWAY_DATA_DIR, links.db, gateway.db). It has no
// dependency beyond the standard library, so the tests of the gateway store
// itself import it without pulling in the harness of the parent package.
package sqlitedir

import (
	"os"
	"testing"
	"time"
)

// Temp is t.TempDir for a directory that holds SQLite databases. On Windows
// the -wal and -shm files that SQLite deletes on the last Close, or that a
// child process that just exited held, can stay "delete pending" for a moment
// while another process (the indexer, an antivirus) still has them open, and
// the removal by t.TempDir then fails the test with "The directory is not
// empty". The removal is retried instead, for up to five seconds.
func Temp(t testing.TB) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gateway-sqlite-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		var err error
		for range 50 {
			if err = os.RemoveAll(dir); err == nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		t.Errorf("remove %s: %v", dir, err)
	})
	return dir
}
