package app

import (
	"database/sql"
	"fmt"
)

const schemaVersion = 9

func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys=ON;PRAGMA journal_mode=WAL;PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migrations'") > 0 && queryInt(db, "SELECT MAX(version) FROM migrations") > schemaVersion {
		return fmt.Errorf("database schema is newer than this application")
	}
	// Upgrade the initial development schema without replacing any source data.
	for _, column := range []struct{ table, name, definition string }{
		{"rules", "direction", "TEXT NOT NULL DEFAULT 'any' CHECK(direction IN ('any','debit','credit'))"},
		{"rules", "spending_group_id", "INTEGER REFERENCES spending_groups(id)"},
		{"rules", "enabled", "INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1))"},
		{"rules", "version", "INTEGER NOT NULL DEFAULT 1"},
		{"fnb_connections", "debug_browser", "INTEGER NOT NULL DEFAULT 0"},
		{"fnb_connections", "last_diagnostics", "TEXT NOT NULL DEFAULT '{}'"},
		{"fnb_connections", "last_skipped", "INTEGER NOT NULL DEFAULT 0"},
		{"accounts", "sync_hidden", "INTEGER NOT NULL DEFAULT 0"},
		{"transactions", "spending_group_id", "INTEGER REFERENCES spending_groups(id)"},
		{"users", "disabled", "INTEGER NOT NULL DEFAULT 0"}, {"users", "version", "INTEGER NOT NULL DEFAULT 1"}, {"transactions", "source_key", "TEXT NOT NULL DEFAULT ''"},
	} {
		if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", column.table) == 0 {
			continue
		}
		columns, err := data(db, "PRAGMA table_info("+column.table+")")
		if err != nil {
			return err
		}
		found := false
		for _, c := range columns {
			if c["name"] == column.name {
				found = true
			}
		}
		if !found {
			if _, err := db.Exec("ALTER TABLE " + column.table + " ADD COLUMN " + column.name + " " + column.definition); err != nil {
				return err
			}
		}
	}
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	rows, err := data(db, "SELECT id,source_date,source_amount,source_description FROM transactions WHERE source_key=''")
	if err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, row := range rows {
		key := fingerprint(SourceRow{Date: row["source_date"].(string), Amount: num(row["source_amount"]), Description: row["source_description"].(string)})
		if _, err := tx.Exec("UPDATE transactions SET source_key=? WHERE id=?", key, row["id"]); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 3 {
		for _, group := range [][2]string{{"Day-to-day", "blue"}, {"Recurring", "amber"}, {"Invest-save-repay", "purple"}, {"Exceptions", "orange"}, {"Income", "teal"}, {"Transfer", "slate"}, {"Bank Fees", "orange"}, {"Communications", "purple"}, {"Debt", "rose"}, {"Utilities", "blue"}, {"Insurance", "teal"}} {
			if _, err := tx.Exec("INSERT INTO spending_groups(name,color) VALUES(?,?)", group[0], group[1]); err != nil {
				return err
			}
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 9 {
		if err := seedClassification(tx); err != nil {
			return err
		}
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO migrations VALUES(?)", schemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}
