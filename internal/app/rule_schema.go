package app

import "database/sql"

// Preserve IDs, authorship, versions and scope; no financial history is rewritten.
func migrateOptionalRuleCategories(tx *sql.Tx, _ migrationOrigin) error {
	for _, s := range []string{
		"CREATE TABLE rules_next(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),account_id INTEGER NOT NULL REFERENCES accounts(id),pattern TEXT NOT NULL,category_id INTEGER REFERENCES categories(id),priority INTEGER NOT NULL DEFAULT 0,direction TEXT NOT NULL DEFAULT 'any' CHECK(direction IN ('any','debit','credit')),spending_group_id INTEGER REFERENCES spending_groups(id),enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),version INTEGER NOT NULL DEFAULT 1,CHECK(category_id IS NOT NULL OR spending_group_id IS NOT NULL))",
		"INSERT INTO rules_next SELECT id,user_id,account_id,pattern,category_id,priority,direction,spending_group_id,enabled,version FROM rules",
		"DROP TABLE rules", "ALTER TABLE rules_next RENAME TO rules",
		"CREATE TABLE builtin_rules_next(id INTEGER PRIMARY KEY,pattern TEXT NOT NULL,category_id INTEGER REFERENCES categories(id),spending_group_id INTEGER REFERENCES spending_groups(id),direction TEXT NOT NULL CHECK(direction IN ('any','debit','credit')),priority INTEGER NOT NULL DEFAULT -1000,enabled INTEGER NOT NULL DEFAULT 1 CHECK(enabled IN (0,1)),version INTEGER NOT NULL DEFAULT 1,CHECK(category_id IS NOT NULL OR spending_group_id IS NOT NULL))",
		"INSERT INTO builtin_rules_next SELECT id,pattern,category_id,spending_group_id,direction,priority,enabled,version FROM builtin_rules",
		"DROP TABLE builtin_rules", "ALTER TABLE builtin_rules_next RENAME TO builtin_rules",
	} {
		if _, err := tx.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
