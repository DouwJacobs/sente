package app

import "database/sql"

func updateGroupTargetsTx(tx *sql.Tx, u User, id int64, b targetsInput) error {
	gid := *b.GroupID
	if gid < 0 || gid > 0 && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", gid) != 1 {
		return fail(400, "Choose a spending group")
	}
	seen := map[int64]bool{}
	for _, t := range b.Targets {
		if t.Amount < 0 || t.Amount > 900000000000000 || seen[t.CategoryID] || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND kind='expense'", t.CategoryID) != 1 {
			return fail(400, "Choose unique expense categories and nonnegative limits")
		}
		if queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 1 && queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND COALESCE(spending_group_id,0)=? AND category_id=? AND included=1", id, gid, t.CategoryID) == 0 {
			return fail(400, "Archived categories cannot receive new budgets")
		}
		seen[t.CategoryID] = true
	}
	if !b.Merge {
		if _, err := tx.Exec("DELETE FROM group_targets WHERE period_id=? AND COALESCE(spending_group_id,0)=?", id, gid); err != nil {
			return err
		}
	}
	var group any
	if gid != 0 {
		group = gid
	}
	for _, t := range b.Targets {
		if _, err := tx.Exec("INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward) VALUES(?,?,?,?,?) ON CONFLICT DO UPDATE SET amount_cents=excluded.amount_cents,included=1", id, t.CategoryID, group, t.Amount, queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 0); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT COUNT(*) FROM (SELECT category_id FROM group_targets WHERE period_id=? GROUP BY category_id HAVING SUM(amount_cents)>900000000000000)", id) > 0 {
		return fail(400, "Combined category budgets exceed the supported limit")
	}
	if _, err := tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
		return err
	}
	if err := syncBudgetTotalsTx(tx, id); err != nil {
		return err
	}
	return audit(tx, u, nil, "period", id, "group_limits_updated", b)
}
