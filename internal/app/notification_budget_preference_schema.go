package app

import (
	"database/sql"
	"fmt"
)

func migrateBudgetAlertPreferences(tx *sql.Tx, _ migrationOrigin) error {
	if _, err := tx.Exec(`CREATE TABLE notification_budget_preferences(
 user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 category_id INTEGER NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
 group_id INTEGER NOT NULL CHECK(group_id>=0),
 enabled INTEGER NOT NULL CHECK(enabled IN (0,1)),
 threshold INTEGER CHECK(threshold BETWEEN 1 AND 100),version INTEGER NOT NULL DEFAULT 1,
 PRIMARY KEY(user_id,category_id,group_id));
 CREATE TRIGGER notification_budget_preference_source_insert AFTER INSERT ON notification_budget_preferences BEGIN UPDATE notification_source_revision SET revision=revision+1 WHERE id=1; END;
 CREATE TRIGGER notification_budget_preference_source_update AFTER UPDATE ON notification_budget_preferences BEGIN UPDATE notification_source_revision SET revision=revision+1 WHERE id=1; END;
 CREATE TRIGGER notification_budget_preference_source_delete AFTER DELETE ON notification_budget_preferences BEGIN UPDATE notification_source_revision SET revision=revision+1 WHERE id=1; END;`); err != nil {
		return err
	}
	// Carry active category episodes into each explicit saved budget scope so an
	// upgrade cannot replay an already consumed category crossing. Financial and
	// receipt history remains immutable. Missing scopes retain normal defaults.
	rows, err := data(tx, `SELECT period_id,category_id,COALESCE(spending_group_id,0) group_id FROM group_targets WHERE included=1
 UNION SELECT period_id,category_id,0 FROM targets t WHERE NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id)`)
	if err != nil {
		return err
	}
	for _, row := range rows {
		pid, cid, gid := num(row["period_id"]), num(row["category_id"]), num(row["group_id"])
		old := fmt.Sprintf("household-budget:%d:category:%d", pid, cid)
		for _, suffix := range []string{":threshold:75", ":threshold:90", ":threshold:100", ":overspend", ":projection"} {
			if _, err := tx.Exec(`INSERT OR IGNORE INTO notification_conditions(user_id,type,scope_hash,active,cycle,consumed,last_sent,evaluated_at,evaluated_run)
 SELECT user_id,type,?,active,cycle,consumed,last_sent,evaluated_at,evaluated_run FROM notification_conditions WHERE scope_hash=?`, hash(budgetAlertScope(pid, cid, gid)+suffix), hash(old+suffix)); err != nil {
				return err
			}
		}
	}
	return nil
}

func migrateBudgetAlertDeliveryScopes(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`CREATE TABLE notification_budget_scopes(notification_id INTEGER PRIMARY KEY REFERENCES notifications(id) ON DELETE CASCADE,category_id INTEGER NOT NULL,group_id INTEGER NOT NULL)`)
	return err
}
