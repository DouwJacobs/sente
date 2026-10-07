package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

const schemaVersion = 20

func migrate(db *sql.DB) error {
	if _, err := db.Exec("PRAGMA foreign_keys=ON;PRAGMA journal_mode=WAL;PRAGMA busy_timeout=5000"); err != nil {
		return err
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='migrations'") > 0 && queryInt(db, "SELECT MAX(version) FROM migrations") > schemaVersion {
		return fmt.Errorf("database schema is newer than this application")
	}
	// Upgrade the initial development schema without replacing any source data.
	for _, column := range []struct{ table, name, definition string }{
		{"users", "deleted_at", "TEXT"},
		{"imports", "committed_at", "TEXT"},
		{"merchants", "logo_data", "TEXT NOT NULL DEFAULT ''"},
		{"merchants", "version", "INTEGER NOT NULL DEFAULT 1"},
		{"merchants", "category_id", "INTEGER REFERENCES categories(id)"},
		{"merchants", "spending_group_id", "INTEGER REFERENCES spending_groups(id)"},
		{"categories", "archived", "INTEGER NOT NULL DEFAULT 0"},
		{"categories", "version", "INTEGER NOT NULL DEFAULT 1"},
		{"categories", "spending_group_id", "INTEGER REFERENCES spending_groups(id)"},
		{"transactions", "note", "TEXT NOT NULL DEFAULT ''"},
		{"transactions", "merchant_id", "INTEGER REFERENCES merchants(id)"},
		{"group_targets", "carry_forward", "INTEGER NOT NULL DEFAULT 1 CHECK(carry_forward IN (0,1))"},
		{"group_targets", "included", "INTEGER NOT NULL DEFAULT 1 CHECK(included IN (0,1))"},
		{"mcp_tokens", "permissions", "TEXT NOT NULL DEFAULT '{}'"},
		{"mcp_tokens", "permission_version", "INTEGER NOT NULL DEFAULT 1"},
		{"mcp_proposals", "result", "TEXT NOT NULL DEFAULT '{}'"},
		{"mcp_oauth_codes", "resource", "TEXT NOT NULL DEFAULT ''"},
		{"mcp_refresh_tokens", "resource", "TEXT NOT NULL DEFAULT ''"},
		{"mcp_tokens", "client_id", "TEXT NOT NULL DEFAULT ''"},
		{"mcp_tokens", "last_used_at", "INTEGER"},
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
	if err := migrateGlobalMerchants(db); err != nil {
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
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 10 {
		// Preserve the last explicit reviewer as having seen the unchanged entry.
		if _, err := tx.Exec(`INSERT OR IGNORE INTO transaction_seen(user_id,transaction_id,transaction_version,seen_at)
 SELECT reviewed_by,id,version,COALESCE(reviewed_at,CURRENT_TIMESTAMP) FROM transactions WHERE reviewed_by IS NOT NULL AND review_state='approved'`); err != nil {
			return err
		}
		ready := acceptanceSQL
		if _, err := tx.Exec(`INSERT INTO audit(account_id,entity,entity_id,action,details)
 SELECT account_id,'transaction',id,'automatically_accepted','{"reason":"category_acceptance_migration"}' FROM transactions WHERE review_state='pending_review' AND ` + ready); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE transactions SET review_state='approved',reviewed_by=NULL,reviewed_at=NULL,version=version+1 WHERE review_state='pending_review' AND " + ready); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE transactions SET review_state='pending_review',reviewed_by=NULL,reviewed_at=NULL,version=version+1 WHERE review_state='approved' AND NOT " + ready); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 14 {
		if _, err := tx.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents) SELECT period_id,category_id,amount_cents FROM targets"); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 15 {
		if _, err := tx.Exec("UPDATE group_targets SET included=0 WHERE amount_cents=0"); err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO budget_groups(period_id,spending_group_id) SELECT DISTINCT period_id,spending_group_id FROM group_targets WHERE included=1"); err != nil {
			return err
		}
	}
	for _, write := range []bool{false, true} {
		encoded, err := json.Marshal(legacyMCPPermissions(write))
		if err != nil {
			return err
		}
		if _, err = tx.Exec("UPDATE mcp_tokens SET permissions=? WHERE permissions='{}' AND can_write=?", string(encoded), write); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 18 {
		groups, err := data(tx, "SELECT merchant_id, account_id, direction, COUNT(*) cnt FROM merchant_rules GROUP BY merchant_id, account_id, direction HAVING cnt > 1")
		if err != nil {
			return err
		}
		for _, g := range groups {
			var rows []map[string]any
			if g["account_id"] == nil {
				rows, err = data(tx, "SELECT id, pattern FROM merchant_rules WHERE merchant_id=? AND account_id IS NULL AND direction=? ORDER BY priority DESC, id ASC", g["merchant_id"], g["direction"])
			} else {
				rows, err = data(tx, "SELECT id, pattern FROM merchant_rules WHERE merchant_id=? AND account_id=? AND direction=? ORDER BY priority DESC, id ASC", g["merchant_id"], g["account_id"], g["direction"])
			}
			if err != nil {
				return err
			}
			if len(rows) <= 1 {
				continue
			}
			primaryID := num(rows[0]["id"])
			patterns := []string{}
			seen := map[string]bool{}
			var deleteIDs []any
			for i, r := range rows {
				p := strings.TrimSpace(r["pattern"].(string))
				if p != "" && !seen[p] {
					seen[p] = true
					patterns = append(patterns, p)
				}
				if i > 0 {
					deleteIDs = append(deleteIDs, r["id"])
				}
			}
			mergedPattern := strings.Join(patterns, " | ")
			if _, err := tx.Exec("UPDATE merchant_rules SET pattern=?, version=version+1 WHERE id=?", mergedPattern, primaryID); err != nil {
				return err
			}
			for _, delID := range deleteIDs {
				if _, err := tx.Exec("DELETE FROM merchant_rules WHERE id=?", delID); err != nil {
					return err
				}
			}
		}
	}
	if queryInt(tx, "SELECT MAX(version) FROM migrations") < 19 || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE spending_group_id IS NULL") > 0 {
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM builtin_rules WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`)
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM rules WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`)
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM group_targets WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`)
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT spending_group_id FROM merchants WHERE category_id=categories.id AND spending_group_id IS NOT NULL LIMIT 1) WHERE spending_group_id IS NULL`)
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name='Income' LIMIT 1) WHERE kind='income' AND spending_group_id IS NULL`)
		for _, m := range [][2]string{
			{"Groceries", "Day-to-day"},
			{"Eating out", "Day-to-day"},
			{"Transport & fuel", "Day-to-day"},
			{"Personal care", "Day-to-day"},
			{"Pets", "Day-to-day"},
			{"Clothing", "Day-to-day"},
			{"Entertainment", "Day-to-day"},
			{"Bank charges", "Bank Fees"},
			{"Phone & internet", "Communications"},
			{"Utilities", "Utilities"},
			{"Insurance", "Insurance"},
			{"Housing", "Recurring"},
			{"Subscriptions", "Recurring"},
			{"Education", "Recurring"},
			{"Home & garden", "Recurring"},
			{"Interest paid", "Recurring"},
			{"Gifts & donations", "Exceptions"},
			{"Travel", "Exceptions"},
			{"Medical", "Exceptions"},
		} {
			tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name=? COLLATE NOCASE LIMIT 1) WHERE lower(name)=lower(?) AND spending_group_id IS NULL`, m[1], m[0])
		}
		tx.Exec(`UPDATE categories SET spending_group_id=(SELECT id FROM spending_groups WHERE name='Day-to-day' LIMIT 1) WHERE kind='expense' AND spending_group_id IS NULL`)
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO migrations VALUES(?)", schemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}
