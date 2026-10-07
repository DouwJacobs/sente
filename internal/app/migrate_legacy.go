package app

import (
	"database/sql"
	"fmt"
)

// Versions 1–22 used additive schema reconciliation and sparse version records.
// This frozen bridge accepts those installations without inventing historical
// schemas. After version 23, only explicitly registered migrations may run.
func migrateLegacyBaseline(tx *sql.Tx, origin migrationOrigin) error {
	if err := addLegacyColumns(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(schema); err != nil {
		return err
	}
	if origin.fresh {
		return nil
	} // New installs must not regain historical seeds.
	if err := migrateGlobalMerchants(tx); err != nil {
		return err
	}
	if err := backfillLegacySourceKeys(tx); err != nil {
		return err
	}
	steps := []struct {
		version int
		name    string
		apply   func(*sql.Tx) error
	}{
		{3, "spending groups", seedLegacySpendingGroups},
		{9, "classification defaults", seedClassification},
		{10, "category acceptance and personal seen state", migrateLegacyAcceptance},
		{14, "group budget limits", migrateLegacyBudgetLimits},
		{15, "explicit budget inclusion", migrateLegacyBudgetInclusion},
		{18, "merchant rule consolidation", migrateLegacyMerchantRules},
	}
	for _, step := range steps {
		if origin.version < step.version {
			if err := step.apply(tx); err != nil {
				return fmt.Errorf("legacy migration %d (%s): %w", step.version, step.name, err)
			}
		}
	}
	// The old runner reconciled these on every startup, including version 22.
	// Apply them once during adoption, then respect the recorded baseline.
	if err := migrateLegacyMCPPermissions(tx); err != nil {
		return err
	}
	return migrateLegacyCategoryGroups(tx)
}

func addLegacyColumns(tx *sql.Tx) error {
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
		if queryInt(tx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", column.table) == 0 {
			continue
		}
		columns, err := data(tx, "PRAGMA table_info("+column.table+")")
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
			if _, err := tx.Exec("ALTER TABLE " + column.table + " ADD COLUMN " + column.name + " " + column.definition); err != nil {
				return err
			}
		}
	}

	return nil
}
