package migrations_test

import (
	"io/fs"
	"slices"
	"strings"
	"testing"

	"multiverse-core.io/internal/gateway/migrations"
)

// goose reads the migrations from the root of the FS it is given: a file left
// in a subdirectory, or a database given the other one's files, would be
// silently skipped or applied to the wrong file.
func TestEachDatabaseSeesOnlyItsOwnMigrationsAtTheRoot(t *testing.T) {
	cases := []struct {
		name   string
		source func() (fs.FS, error)
		marker string
	}{
		{"links", migrations.Links, "CREATE TABLE links ("},
		{"gateway", migrations.Gateway, "CREATE TABLE sessions ("},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fsys, err := tc.source()
			if err != nil {
				t.Fatal(err)
			}
			names, err := fs.Glob(fsys, "*.sql")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(names, []string{"0001_init.sql"}) {
				t.Fatalf("migrations at the root = %v, want [0001_init.sql]", names)
			}
			data, err := fs.ReadFile(fsys, "0001_init.sql")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(data), tc.marker) {
				t.Errorf("0001_init.sql of %s does not create its own table (%q)", tc.name, tc.marker)
			}
		})
	}
}
