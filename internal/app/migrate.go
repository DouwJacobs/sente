package app

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
)

const schemaVersion = 27
const migrationBaseline = 23

type migrationOrigin struct {
	version int
	fresh   bool
}

type schemaMigration struct {
	version       int
	name          string
	rebuildTables bool
	apply         func(*sql.Tx, migrationOrigin) error
}

// Append immutable, consecutive entries here. Never edit an applied migration
// or the frozen schema snapshot to introduce a new application schema change.
var schemaMigrations = []schemaMigration{
	{23, "adopt sequential migrations", true, migrateLegacyBaseline},
	{24, "persistent notification foundation", false, migrateNotifications},
	{25, "notification producers diagnostics and push", false, migrateNotificationAlerts},
	{26, "independent notification channels and evaluation generations", false, migrateNotificationChannels},
	{27, "change-triggered notification evaluation", false, migrateNotificationSchedule},
}

func migrate(db *sql.DB) error {
	return runSchemaMigrations(db, schemaMigrations, schemaVersion)
}

func validateSchemaMigrations(steps []schemaMigration, target int) error {
	if len(steps) == 0 {
		return errors.New("no schema migrations registered")
	}
	for i, step := range steps {
		if step.version != migrationBaseline+i || step.name == "" || step.apply == nil {
			return fmt.Errorf("invalid schema migration registration at position %d", i)
		}
	}
	if steps[len(steps)-1].version != target {
		return errors.New("schema version does not match migration registry")
	}
	return nil
}

func runSchemaMigrations(db *sql.DB, steps []schemaMigration, target int) (result error) {
	if err := validateSchemaMigrations(steps, target); err != nil {
		return err
	}
	ctx := context.Background()
	// PRAGMAs belong to a connection. Pin it through rebuild, integrity checks,
	// commit/rollback and restoration, even if the caller configures a pool.
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	var hasHistory int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migrations'").Scan(&hasHistory); err != nil {
		return err
	}
	origin := migrationOrigin{}
	if hasHistory != 0 {
		var minimum, count int
		if err := conn.QueryRowContext(ctx, "SELECT COALESCE(MAX(version),0), COALESCE(MIN(version),0), COUNT(*) FROM migrations").Scan(&origin.version, &minimum, &count); err != nil {
			return err
		}
		if count > 0 && minimum < 1 {
			return errors.New("invalid database migration history")
		}
	}
	if origin.version > target {
		return errors.New("database schema is newer than this application")
	}
	if origin.version >= migrationBaseline {
		var count int
		if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM migrations WHERE version>=?", migrationBaseline).Scan(&count); err != nil {
			return err
		}
		if count != origin.version-migrationBaseline+1 {
			return errors.New("database migration history has gaps")
		}
	}
	var tables int
	if err := conn.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name<>'migrations' AND name NOT LIKE 'sqlite_%'").Scan(&tables); err != nil {
		return err
	}
	origin.fresh = tables == 0
	if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON; PRAGMA journal_mode=WAL; PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	if origin.version == target {
		return nil
	}
	rebuild := false
	for _, step := range steps {
		if step.version > origin.version && step.rebuildTables {
			rebuild = true
		}
	}
	if rebuild {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=OFF"); err != nil {
			return err
		}
		defer func() {
			if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
				result = errors.Join(result, fmt.Errorf("restore migration foreign keys: %w", err))
				// Never return a connection with disabled enforcement to the pool.
				_ = conn.Raw(func(any) error { return driver.ErrBadConn })
			}
		}()
	}
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, step := range steps {
		if step.version <= origin.version {
			continue
		}
		if err := step.apply(tx, origin); err != nil {
			return fmt.Errorf("migration %d (%s): %w", step.version, step.name, err)
		}
		if _, err := tx.Exec("INSERT INTO migrations(version) VALUES(?)", step.version); err != nil {
			return fmt.Errorf("record migration %d: %w", step.version, err)
		}
	}
	rows, err := tx.Query("PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	invalid := rows.Next()
	rowErr := rows.Err()
	closeErr := rows.Close()
	if rowErr != nil || closeErr != nil {
		return errors.Join(rowErr, closeErr)
	}
	if invalid {
		return errors.New("schema migration has invalid foreign-key references")
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migrations: %w", err)
	}
	return nil
}
