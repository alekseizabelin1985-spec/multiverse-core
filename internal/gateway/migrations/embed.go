// Package migrations carries the SQL migrations of the two gateway databases
// inside the binary (ADR-019 p. 2): links.db, the only copy of external IDs,
// and gateway.db, everything else the gateway keeps. Each database has its own
// directory and its own version table, so a migration of one never touches the
// other.
//
// The files are embedded here and not next to internal/gateway/store: a
// //go:embed pattern cannot reach a parent directory.
package migrations

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed links/*.sql gateway/*.sql
var files embed.FS

// Links returns the migrations of links.db with the files at the root, the
// layout goose expects.
func Links() (fs.FS, error) { return sub("links") }

// Gateway returns the migrations of gateway.db with the files at the root.
func Gateway() (fs.FS, error) { return sub("gateway") }

func sub(dir string) (fs.FS, error) {
	fsys, err := fs.Sub(files, dir)
	if err != nil {
		return nil, fmt.Errorf("migrations: %s: %w", dir, err)
	}
	return fsys, nil
}
