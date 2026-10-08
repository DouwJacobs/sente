package app

import (
	"database/sql"
	"fmt"
)

var notificationSourceTables = []string{"transactions", "allocations", "targets", "group_targets", "periods", "categories", "accounts", "grants", "users"}

func migrateNotificationSchedule(tx *sql.Tx, _ migrationOrigin) error {
	if _, err := tx.Exec(`CREATE TABLE notification_source_revision(id INTEGER PRIMARY KEY CHECK(id=1),revision INTEGER NOT NULL); INSERT INTO notification_source_revision VALUES(1,0); CREATE INDEX notification_diagnostics_scope ON notification_diagnostics(user_id,type,key_hash,channel,id);`); err != nil {
		return err
	}
	// Source revisions commit or roll back with imports, edits and access changes.
	for _, table := range notificationSourceTables {
		for _, operation := range []string{"INSERT", "UPDATE", "DELETE"} {
			if _, err := tx.Exec(fmt.Sprintf("CREATE TRIGGER notification_source_%s_%s AFTER %s ON %s BEGIN UPDATE notification_source_revision SET revision=revision+1 WHERE id=1; END", table, operation, operation, table)); err != nil {
				return err
			}
		}
	}
	return nil
}
