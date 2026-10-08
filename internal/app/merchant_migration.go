package app

import "database/sql"

// Rebuild only the catalogue/rule schema; preserve IDs and every ledger reference.
// The migration runner owns the transaction and foreign-key mode.
func migrateGlobalMerchants(tx *sql.Tx) error {
	columns, err := data(tx, "PRAGMA table_info(merchants)")
	if err != nil {
		return err
	}
	required := false
	for _, c := range columns {
		if c["name"] == "account_id" && num(c["notnull"]) == 1 {
			required = true
		}
	}
	if !required {
		return nil
	}
	statements := []string{
		"CREATE TABLE merchants_new(id INTEGER PRIMARY KEY,account_id INTEGER REFERENCES accounts(id),name TEXT NOT NULL COLLATE NOCASE,logo_data TEXT NOT NULL DEFAULT '',category_id INTEGER REFERENCES categories(id),spending_group_id INTEGER REFERENCES spending_groups(id),version INTEGER NOT NULL DEFAULT 1,UNIQUE(account_id,name))",
		"INSERT INTO merchants_new SELECT id,account_id,name,logo_data,category_id,spending_group_id,version FROM merchants",
		"DROP TABLE merchants", "ALTER TABLE merchants_new RENAME TO merchants",
		"CREATE UNIQUE INDEX global_merchant_name ON merchants(name COLLATE NOCASE) WHERE account_id IS NULL",
		"CREATE TABLE merchant_rules_new(id INTEGER PRIMARY KEY,account_id INTEGER REFERENCES accounts(id),merchant_id INTEGER NOT NULL REFERENCES merchants(id),pattern TEXT NOT NULL,direction TEXT NOT NULL DEFAULT 'any' CHECK(direction IN ('any','debit','credit')),priority INTEGER NOT NULL DEFAULT 0,enabled INTEGER NOT NULL DEFAULT 1,version INTEGER NOT NULL DEFAULT 1)",
		"INSERT INTO merchant_rules_new SELECT * FROM merchant_rules", "DROP TABLE merchant_rules", "ALTER TABLE merchant_rules_new RENAME TO merchant_rules",
	}
	for _, s := range statements {
		if _, err = tx.Exec(s); err != nil {
			return err
		}
	}
	violations, err := data(tx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fail(500, "Merchant migration has invalid references")
	}
	return nil
}
