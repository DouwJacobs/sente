package app

import "database/sql"

// Allocation overrides retain a real foreign key, with a separate flag to distinguish
// explicit No spending group from legacy/default parent inheritance.
func migrateAllocationGroups(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`ALTER TABLE allocations ADD COLUMN spending_group_id INTEGER REFERENCES spending_groups(id);
 ALTER TABLE allocations ADD COLUMN group_override INTEGER NOT NULL DEFAULT 0 CHECK(group_override IN (0,1));`)
	return err
}

func allocationGroupSQL(allocation, transaction string) string {
	return "(CASE WHEN " + allocation + ".group_override=1 THEN " + allocation + ".spending_group_id ELSE " + transaction + ".spending_group_id END)"
}

func allocationGroupFromRow(row map[string]any) *int64 {
	if num(row["group_override"]) == 0 {
		return nil
	}
	id := num(row["spending_group_id"])
	return &id
}
