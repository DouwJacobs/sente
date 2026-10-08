package app

import "database/sql"

// Migration 25 was already applied by the watched development service. Extend
// it through a new step rather than changing an installed schema definition.
func migrateNotificationChannels(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`ALTER TABLE notifications ADD COLUMN in_app_enabled INTEGER NOT NULL DEFAULT 1 CHECK(in_app_enabled IN (0,1));
 ALTER TABLE notification_conditions ADD COLUMN evaluated_run INTEGER NOT NULL DEFAULT 0;
 CREATE TABLE notification_evaluation_runs(id INTEGER PRIMARY KEY CHECK(id=1),run INTEGER NOT NULL);
 INSERT INTO notification_evaluation_runs VALUES(1,0);`)
	return err
}
