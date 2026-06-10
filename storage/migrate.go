package storage

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/ducanng/URLShortener/logger"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// newMigrator builds a *migrate.Migrate bound to the given *sql.DB and the
// embedded migrations/ directory. The caller is responsible for calling
// Close() (via the returned function) once migrations are done.
func newMigrator(db *sql.DB) (*migrate.Migrate, func(), error) {
	src, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return nil, nil, fmt.Errorf("iofs source: %w", err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		return nil, nil, fmt.Errorf("postgres driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "postgres", driver)
	if err != nil {
		return nil, nil, fmt.Errorf("new migrate: %w", err)
	}

	cleanup := func() {
		// Migrate.Close() returns (sourceErr, dbErr); we already own the *sql.DB
		// elsewhere, so we only care about the source close.
		_, _ = m.Close()
	}
	return m, cleanup, nil
}

// MigrateUp applies all pending migrations. Returns nil when the schema is
// already at the latest version (migrate.ErrNoChange is treated as success).
func MigrateUp(db *sql.DB, log *logger.Logger) error {
	m, cleanup, err := newMigrator(db)
	if err != nil {
		return err
	}
	defer cleanup()

	before, _, _ := m.Version()
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate up: %w", err)
	}
	after, dirty, _ := m.Version()

	if before == after {
		log.Infof("db schema already at version %d", after)
	} else {
		log.Infof("db schema migrated %d -> %d (dirty=%v)", before, after, dirty)
	}
	return nil
}

// MigrateDown reverts the last applied migration. Used by the migrate-down
// Makefile target / CLI entrypoint, never called automatically.
func MigrateDown(db *sql.DB, steps int) error {
	m, cleanup, err := newMigrator(db)
	if err != nil {
		return err
	}
	defer cleanup()

	if steps <= 0 {
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			return fmt.Errorf("migrate down all: %w", err)
		}
		return nil
	}
	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("migrate down %d: %w", steps, err)
	}
	return nil
}

// MigrateForce sets the version without running any migration. Used to
// recover from a dirty state after a failed migration.
func MigrateForce(db *sql.DB, version int) error {
	m, cleanup, err := newMigrator(db)
	if err != nil {
		return err
	}
	defer cleanup()
	if err := m.Force(version); err != nil {
		return fmt.Errorf("migrate force %d: %w", version, err)
	}
	return nil
}

// MigrateVersion returns the current schema version and dirty flag.
func MigrateVersion(db *sql.DB) (uint, bool, error) {
	m, cleanup, err := newMigrator(db)
	if err != nil {
		return 0, false, err
	}
	defer cleanup()
	v, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return 0, false, fmt.Errorf("migrate version: %w", err)
	}
	return v, dirty, nil
}
