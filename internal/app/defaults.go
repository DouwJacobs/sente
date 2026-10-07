package app

import "database/sql"

// General starter labels and descriptions, never personal financial fixtures.
var defaultCategories = []struct{ Name, Kind string }{
	{"Groceries", "expense"}, {"Eating out", "expense"}, {"Transport & fuel", "expense"},
	{"Bank charges", "expense"}, {"Interest paid", "expense"}, {"Housing", "expense"},
	{"Utilities", "expense"}, {"Insurance", "expense"}, {"Phone & internet", "expense"},
	{"Medical", "expense"}, {"Personal care", "expense"}, {"Pets", "expense"},
	{"Clothing", "expense"}, {"Entertainment", "expense"}, {"Subscriptions", "expense"},
	{"Home & garden", "expense"}, {"Gifts & donations", "expense"}, {"Education", "expense"},
	{"Travel", "expense"}, {"Salary", "income"}, {"Interest earned", "income"},
}
var defaultRules = []struct{ Pattern, Category, Group, Direction string }{
	{"Monthly Account Fee", "Bank charges", "Bank Fees", "debit"},
	{"Monthly Credit Fee", "Bank charges", "Bank Fees", "debit"},
	{"Monthly Service Fee", "Bank charges", "Bank Fees", "debit"},
	{"Eft Charge-Fnb To Other", "Bank charges", "Bank Fees", "debit"},
	{"Service Fees", "Bank charges", "Bank Fees", "debit"},
	{"Int On Credit Balance", "Interest earned", "Income", "credit"},
	{"Int On Debit Balance", "Interest paid", "Recurring", "debit"},
	{"Salary", "Salary", "Income", "credit"},
	{"Checkers", "Groceries", "Day-to-day", "debit"},
	{"Pick N Pay Asap", "Groceries", "Day-to-day", "debit"},
	{"Pnp Fam", "Groceries", "Day-to-day", "debit"},
	{"Woolworths Food", "Groceries", "Day-to-day", "debit"},
	{"Spar ", "Groceries", "Day-to-day", "debit"},
	{"McDonald's", "Eating out", "Day-to-day", "debit"},
	{"Mcd ", "Eating out", "Day-to-day", "debit"},
	{"Nando", "Eating out", "Day-to-day", "debit"},
	{"Steers", "Eating out", "Day-to-day", "debit"},
	{"Kfc ", "Eating out", "Day-to-day", "debit"},
	{"Engen", "Transport & fuel", "Day-to-day", "debit"},
	{"Shell ", "Transport & fuel", "Day-to-day", "debit"},
	{"Sasol", "Transport & fuel", "Day-to-day", "debit"},
	{"Petshop Science", "Pets", "Day-to-day", "debit"},
	{"Fnbconnect", "Phone & internet", "Communications", "debit"},
	{"Web Africa", "Phone & internet", "Communications", "debit"},
}

func seedClassification(tx *sql.Tx) error {
	ids := map[string]int64{}
	for _, category := range defaultCategories {
		matches, err := data(tx, "SELECT id,kind FROM categories WHERE lower(name)=lower(?)", category.Name)
		if err != nil {
			return err
		}
		if len(matches) == 1 && matches[0]["kind"] == category.Kind {
			ids[category.Name] = num(matches[0]["id"])
			continue
		}
		// Preserve conflicting kinds/legacy duplicates instead of merging financial history.
		if len(matches) > 0 {
			continue
		}
		res, err := tx.Exec("INSERT INTO categories(name,group_name,kind) VALUES(?,'',?)", category.Name, category.Kind)
		if err != nil {
			return err
		}
		ids[category.Name], _ = res.LastInsertId()
	}
	for _, rule := range defaultRules {
		cid := ids[rule.Category]
		if cid == 0 {
			continue
		}
		var gid *int64
		var group int64
		err := tx.QueryRow("SELECT id FROM spending_groups WHERE name=? COLLATE NOCASE", rule.Group).Scan(&group)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		if err == nil {
			gid = &group
		}
		// Existing group edits/removals are preserved; missing groups remain unassigned.
		if _, err := tx.Exec("INSERT INTO builtin_rules(pattern,category_id,spending_group_id,direction) VALUES(?,?,?,?)", rule.Pattern, cid, gid, rule.Direction); err != nil {
			return err
		}
	}
	return nil
}
func (a *App) writeBuiltinRule(tx *sql.Tx, u User, id int64, b ruleInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	if queryInt(tx, "SELECT COUNT(*) FROM builtin_rules WHERE id=?", -id) != 1 {
		return fail(404, "Rule not found")
	}
	if b.AccountID != 0 {
		return fail(400, "Built-in rules apply to all enabled accounts")
	}
	if err := b.validate(tx); err != nil {
		return err
	}
	res, err := tx.Exec("UPDATE builtin_rules SET pattern=?,category_id=?,spending_group_id=?,direction=?,priority=?,enabled=?,version=version+1 WHERE id=? AND version=?", b.Pattern, b.CategoryID, b.SpendingGroupID, b.Direction, b.Priority, b.active(), -id, b.Version)
	if err != nil {
		return err
	}
	if err := affected(res); err != nil {
		return err
	}
	return audit(tx, u, nil, "builtin_rule", -id, "updated", b)
}
func (a *App) deleteBuiltinRule(tx *sql.Tx, u User, id, version int64) error {
	if err := requireMember(u); err != nil {
		return err
	}
	if queryInt(tx, "SELECT COUNT(*) FROM builtin_rules WHERE id=?", -id) != 1 {
		return fail(404, "Rule not found")
	}
	res, err := tx.Exec("DELETE FROM builtin_rules WHERE id=? AND version=?", -id, version)
	if err != nil {
		return err
	}
	if err := affected(res); err != nil {
		return err
	}
	return audit(tx, u, nil, "builtin_rule", -id, "deleted", nil)
}

func (a *App) createBuiltinRule(tx *sql.Tx, u User, id *int64, b ruleInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	if b.AccountID != 0 {
		return fail(400, "Built-in rules apply to all enabled accounts")
	}
	if err := b.validate(tx); err != nil {
		return err
	}
	res, err := tx.Exec("INSERT INTO builtin_rules(pattern,category_id,spending_group_id,direction,priority,enabled) VALUES(?,?,?,?,?,?)", b.Pattern, b.CategoryID, b.SpendingGroupID, b.Direction, b.Priority, b.active())
	if err != nil {
		return err
	}
	*id, err = res.LastInsertId()
	if err != nil {
		return err
	}
	return audit(tx, u, nil, "builtin_rule", *id, "created", b)
}
