package app

import "database/sql"

// Snapshot each existing rule's effective classification without changing the
// merchant, ledger, grants, versions or connection consent. Append-only migration.
func migrateMerchantRuleClassification(tx *sql.Tx, _ migrationOrigin) error {
	for _, statement := range []string{
		"ALTER TABLE merchant_rules ADD COLUMN category_id INTEGER REFERENCES categories(id)",
		"ALTER TABLE merchant_rules ADD COLUMN spending_group_id INTEGER REFERENCES spending_groups(id)",
		"UPDATE merchant_rules SET category_id=(SELECT category_id FROM merchants WHERE id=merchant_rules.merchant_id),spending_group_id=(SELECT spending_group_id FROM merchants WHERE id=merchant_rules.merchant_id)",
	} {
		if _, err := tx.Exec(statement); err != nil {
			return err
		}
	}
	return nil
}
