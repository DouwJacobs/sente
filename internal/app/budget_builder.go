package app

import (
	"database/sql"
	"net/http"
)

type budgetCategoryEdit struct {
	CategoryID   int64 `json:"category_id"`
	Amount       int64 `json:"amount_cents"`
	CarryForward bool  `json:"carry_forward"`
	Upcoming     bool  `json:"apply_upcoming"`
	Remove       bool  `json:"remove"`
}
type budgetGroupEdit struct {
	GroupID int64                `json:"group_id"`
	Remove  bool                 `json:"remove"`
	Targets []budgetCategoryEdit `json:"targets"`
}
type budgetBuilderInput struct {
	Version int64             `json:"version"`
	Groups  []budgetGroupEdit `json:"groups"`
}

func (a *App) budgetGroupPages(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	id := parseID(r)
	if queryInt(a.DB, "SELECT COUNT(*) FROM periods WHERE id=?", id) != 1 {
		return fail(404, "Period not found")
	}
	return a.metadataPage(w, r, `SELECT CAST(COALESCE(bg.spending_group_id,0) AS INTEGER) id,COALESCE(g.name,'No spending group') name,g.color,(SELECT version FROM periods WHERE id=bg.period_id) version,
 COALESCE((SELECT SUM(amount_cents) FROM group_targets gt WHERE gt.period_id=bg.period_id AND gt.spending_group_id IS bg.spending_group_id AND gt.included=1),0) target_cents
 FROM budget_groups bg LEFT JOIN spending_groups g ON g.id=bg.spending_group_id WHERE bg.period_id=?`, []any{id}, "name", "name,id")
}

func syncBudgetTotalsTx(tx *sql.Tx, id int64) error {
	if queryInt(tx, "SELECT COUNT(*) FROM (SELECT category_id FROM group_targets WHERE period_id=? AND included=1 GROUP BY category_id HAVING SUM(amount_cents)>900000000000000)", id) > 0 {
		return fail(400, "Combined category budgets exceed the supported limit")
	}
	if _, err := tx.Exec("DELETE FROM targets WHERE period_id=?", id); err != nil {
		return err
	}
	_, err := tx.Exec("INSERT INTO targets SELECT period_id,category_id,SUM(amount_cents) FROM group_targets WHERE period_id=? AND included=1 GROUP BY period_id,category_id", id)
	return err
}

func (a *App) updateBudgetBuilder(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b budgetBuilderInput
	if err := decode(r, &b); err != nil {
		return err
	}
	id := parseID(r)
	if len(b.Groups) == 0 || len(b.Groups) > 1000 {
		return fail(400, "Choose budget groups to save")
	}
	err := a.write(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE periods SET version=version+1 WHERE id=? AND version=?", id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		var start string
		if err := tx.QueryRow("SELECT start_date FROM periods WHERE id=?", id).Scan(&start); err != nil {
			return err
		}
		seen := map[int64]bool{}
		futureChanged := map[int64]bool{}
		for _, g := range b.Groups {
			if g.GroupID < 0 || seen[g.GroupID] || g.GroupID > 0 && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", g.GroupID) != 1 {
				return fail(400, "Choose unique spending groups")
			}
			seen[g.GroupID] = true
			var group any
			if g.GroupID > 0 {
				group = g.GroupID
			}
			if g.Remove {
				if len(g.Targets) > 0 {
					return fail(400, "A removed group cannot contain category edits")
				}
				if _, err := tx.Exec("DELETE FROM group_targets WHERE period_id=? AND spending_group_id IS ?", id, group); err != nil {
					return err
				}
				if _, err := tx.Exec("DELETE FROM budget_groups WHERE period_id=? AND spending_group_id IS ?", id, group); err != nil {
					return err
				}
				continue
			}
			if _, err := tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
				return err
			}
			cats := map[int64]bool{}
			for _, c := range g.Targets {
				if cats[c.CategoryID] || c.Amount < 0 || c.Amount > 900000000000000 || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND kind='expense'", c.CategoryID) != 1 || c.Upcoming && !c.CarryForward {
					return fail(400, "Choose expense categories, valid amounts and an upcoming-budget option")
				}
				if !c.Remove && queryInt(tx, "SELECT archived FROM categories WHERE id=?", c.CategoryID) == 1 && (c.CarryForward || c.Upcoming || queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND spending_group_id IS ? AND included=1", id, c.CategoryID, group) == 0) {
					return fail(400, "Restore the category before adding or carrying its budget")
				}
				cats[c.CategoryID] = true
				if c.Remove {
					if _, err := tx.Exec("DELETE FROM group_targets WHERE period_id=? AND category_id=? AND spending_group_id IS ?", id, c.CategoryID, group); err != nil {
						return err
					}
					continue
				}
				if _, err := tx.Exec(`INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward,included) VALUES(?,?,?,?,?,1) ON CONFLICT DO UPDATE SET amount_cents=excluded.amount_cents,carry_forward=excluded.carry_forward,included=1`, id, c.CategoryID, group, c.Amount, c.CarryForward); err != nil {
					return err
				}
				if !c.Upcoming {
					continue
				}
				future, err := data(tx, "SELECT id FROM periods WHERE start_date>? ORDER BY start_date,id", start)
				if err != nil {
					return err
				}
				for _, p := range future {
					pid := num(p["id"])
					if queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND spending_group_id IS ? AND included=1", pid, c.CategoryID, group) > 0 {
						continue
					}
					if _, err := tx.Exec(`INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward,included) VALUES(?,?,?,?,1,1) ON CONFLICT DO UPDATE SET amount_cents=excluded.amount_cents,carry_forward=1,included=1`, pid, c.CategoryID, group, c.Amount); err != nil {
						return err
					}
					if _, err := tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", pid, group); err != nil {
						return err
					}
					futureChanged[pid] = true
				}
			}
		}
		if err := syncBudgetTotalsTx(tx, id); err != nil {
			return err
		}
		for pid := range futureChanged {
			if _, err := tx.Exec("UPDATE periods SET version=version+1 WHERE id=?", pid); err != nil {
				return err
			}
			if err := syncBudgetTotalsTx(tx, pid); err != nil {
				return err
			}
			if err := audit(tx, u, nil, "period", pid, "upcoming_budget_categories_added", b); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "period", id, "budget_built", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
