package sqlitedir_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"multiverse-core.io/shared/testkit/gateway/sqlitedir"
)

func TestTempIsRemovedWithItsFilesWhenTheTestEnds(t *testing.T) {
	var dir string
	t.Run("user", func(t *testing.T) {
		dir = sqlitedir.Temp(t)
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			t.Fatalf("Temp = %s: %v", dir, err)
		}
		if err := os.WriteFile(filepath.Join(dir, "links.db-wal"), []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("%s after the test = %v, want removed", dir, err)
	}
}
