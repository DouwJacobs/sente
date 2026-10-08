package app

import "database/sql"

func migratePWA(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`CREATE TABLE app_identity (id INTEGER PRIMARY KEY CHECK(id=1), logo TEXT NOT NULL DEFAULT '', pwa_logo TEXT NOT NULL DEFAULT '', pwa_name TEXT NOT NULL DEFAULT 'Sente', use_branding_name INTEGER NOT NULL DEFAULT 0 CHECK(use_branding_name IN (0,1)), use_branding_logo INTEGER NOT NULL DEFAULT 1 CHECK(use_branding_logo IN (0,1)), version INTEGER NOT NULL DEFAULT 1); INSERT INTO app_identity(id) VALUES(1)`)
	return err
}
