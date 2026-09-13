package store

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"

	"multiverse-core.io/internal/gateway/migrations"
)

// MigrateLinks applies the pending migrations of links.db and returns how many
// it applied; zero means the schema was already current. Each database keeps
// its own goose_db_version table inside its own file (ADR-019 p. 2).
func MigrateLinks(ctx context.Context, db *sql.DB) (int, error) {
	return migrate(ctx, db, LinksFile, migrations.Links)
}

// MigrateGateway applies the pending migrations of gateway.db and returns how
// many it applied.
func MigrateGateway(ctx context.Context, db *sql.DB) (int, error) {
	return migrate(ctx, db, GatewayFile, migrations.Gateway)
}

func migrate(ctx context.Context, db *sql.DB, name string, source func() (fs.FS, error)) (int, error) {
	fsys, err := source()
	if err != nil {
		return 0, fmt.Errorf("store: migrate %s: %w", name, err)
	}
	// The global registry holds Go migrations registered by init functions
	// anywhere in the binary; the gateway has none and must not pick up
	// someone else's.
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, fsys, goose.WithDisableGlobalRegistry(true))
	if err != nil {
		return 0, fmt.Errorf("store: migrate %s: %w", name, err)
	}
	// Provider.Close is not called: it closes db, which belongs to the caller.
	results, err := provider.Up(ctx)
	if err != nil {
		return 0, fmt.Errorf("store: migrate %s: %w", name, err)
	}
	return len(results), nil
}
