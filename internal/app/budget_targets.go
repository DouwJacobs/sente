package app

import (
	"database/sql"
	"fmt"
	"net/http"
	"strings"
)

func (a *App) updateTargets(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	id := parseID(r)
	var b targetsInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := a.write(func(tx *sql.Tx) error { return updateTargetsTx(tx, u, id, b) }); err != nil {
		return err
	}
	success(w)
	return nil
}

type targetItem struct {
	CategoryID        int64  `json:"category_id"`
	Amount            int64  `json:"amount_cents"`
	SpendingGroupID   *int64 `json:"spending_group_id,omitempty"`
	GroupID           *int64 `json:"group_id,omitempty"`
	SpendingGroupName string `json:"spending_group_name,omitempty"`
}

type targetGroupItem struct {
	GroupID           *int64       `json:"group_id,omitempty"`
	SpendingGroupID   *int64       `json:"spending_group_id,omitempty"`
	SpendingGroupName string       `json:"spending_group_name,omitempty"`
	Targets           []targetItem `json:"targets"`
}

type targetsInput struct {
	Version         int64             `json:"version"`
	GroupID         *int64            `json:"group_id,omitempty"`
	SpendingGroupID *int64            `json:"spending_group_id,omitempty"`
	Merge           bool              `json:"merge,omitempty"`
	Targets         []targetItem      `json:"targets"`
	Groups          []targetGroupItem `json:"groups,omitempty"`
}

// canonicalTargetsTx freezes names to IDs and preserves the independent budget scope.
// Omitted scopes are legacy No spending group only; category/rule metadata is never a budget scope.
func canonicalTargetsTx(tx *sql.Tx, id int64, b targetsInput) (targetsInput, error) {
	resolve := func(first, second *int64, name string) (*int64, error) {
		if first != nil && second != nil && *first != *second {
			return nil, fail(400, "Conflicting spending groups")
		}
		value := first
		if value == nil {
			value = second
		}
		if strings.TrimSpace(name) != "" {
			var gid int64
			if err := tx.QueryRow("SELECT id FROM spending_groups WHERE name=? COLLATE NOCASE", strings.TrimSpace(name)).Scan(&gid); err != nil {
				return nil, fail(400, "Choose an existing spending group")
			}
			if value != nil && *value != gid {
				return nil, fail(400, "Conflicting spending groups")
			}
			value = &gid
		}
		if value != nil {
			gid := *value
			if gid < 0 || gid > 0 && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", gid) != 1 {
				return nil, fail(400, "Choose an existing spending group")
			}
			value = &gid
		}
		return value, nil
	}
	root, err := resolve(b.GroupID, b.SpendingGroupID, "")
	if err != nil {
		return b, err
	}
	targets := append([]targetItem(nil), b.Targets...)
	canonicalGroups := []targetGroupItem{}
	groupSeen := map[int64]bool{}
	for _, g := range b.Groups {
		gid, e := resolve(g.GroupID, g.SpendingGroupID, g.SpendingGroupName)
		if e != nil {
			return b, e
		}
		if gid == nil {
			return b, fail(400, "Specify the spending group for each budget group")
		}
		if groupSeen[*gid] {
			return b, fail(400, "Each budget group can appear only once")
		}
		groupSeen[*gid] = true
		canonicalGroups = append(canonicalGroups, targetGroupItem{GroupID: gid, Targets: []targetItem{}})
		for _, t := range g.Targets {
			own, e := resolve(t.GroupID, t.SpendingGroupID, t.SpendingGroupName)
			if e != nil {
				return b, e
			}
			if own != nil && *own != *gid {
				return b, fail(400, "Conflicting spending groups")
			}
			t.GroupID = gid
			t.SpendingGroupID = nil
			t.SpendingGroupName = ""
			targets = append(targets, t)
		}
	}
	if root != nil && len(b.Groups) > 0 {
		return b, fail(400, "Choose a root group or grouped targets, not both")
	}
	legacy := false
	for i, t := range targets {
		gid, e := resolve(t.GroupID, t.SpendingGroupID, t.SpendingGroupName)
		if e != nil {
			return b, e
		}
		if root != nil {
			if gid != nil && *gid != *root {
				return b, fail(400, "Conflicting spending groups")
			}
			gid = root
		}
		if gid == nil {
			legacy = true
			if queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND spending_group_id IS NOT NULL AND included=1", id, t.CategoryID) > 0 {
				return b, fail(400, "Specify the spending group for this category budget")
			}
			zero := int64(0)
			gid = &zero
		}
		targets[i] = targetItem{CategoryID: t.CategoryID, Amount: t.Amount, GroupID: gid}
	}
	if root == nil && !b.Merge && (legacy || len(targets) == 0 && len(b.Groups) == 0) && queryInt(tx, "SELECT COUNT(*) FROM budget_groups WHERE period_id=? AND spending_group_id IS NOT NULL", id) > 0 {
		return b, fail(400, "Specify explicit group budgets before replacing a grouped budget")
	}
	b.GroupID = root
	b.SpendingGroupID = nil
	b.Groups = canonicalGroups
	b.Targets = targets
	return b, nil
}

func updateTargetsTx(tx *sql.Tx, u User, id int64, b targetsInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	var err error
	b, err = canonicalTargetsTx(tx, id, b)
	if err != nil {
		return err
	}
	seen := map[string]bool{}
	for _, t := range b.Targets {
		gid := *t.GroupID
		key := fmt.Sprintf("%d:%d", gid, t.CategoryID)
		if t.Amount < 0 || t.Amount > 900000000000000 || seen[key] || queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND kind='expense'", t.CategoryID) != 1 {
			return fail(400, "Choose unique expense categories and nonnegative limits")
		}
		if queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 1 && queryInt(tx, "SELECT COUNT(*) FROM group_targets WHERE period_id=? AND category_id=? AND COALESCE(spending_group_id,0)=? AND included=1", id, t.CategoryID, gid) == 0 {
			return fail(400, "Archived categories cannot receive new budgets")
		}
		seen[key] = true
	}
	res, err := tx.Exec("UPDATE periods SET version=version+1 WHERE id=? AND version=?", id, b.Version)
	if err != nil {
		return err
	}
	if err = affected(res); err != nil {
		return err
	}
	// Preserve undistributed legacy aggregate rows before rebuilding totals, just as period copying does.
	// Never infer a category-owned group or overwrite any existing canonical entry.
	legacy, err := data(tx, "SELECT t.category_id,c.name category_name,t.amount_cents FROM targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id) ORDER BY t.category_id", id)
	if err != nil {
		return err
	}
	if len(legacy) > 0 {
		if _, err = tx.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward,included) SELECT t.period_id,t.category_id,t.amount_cents,CASE WHEN c.kind='expense' AND c.archived=0 AND t.amount_cents>0 THEN 1 ELSE 0 END,1 FROM targets t JOIN categories c ON c.id=t.category_id WHERE t.period_id=? AND NOT EXISTS(SELECT 1 FROM group_targets g WHERE g.period_id=t.period_id AND g.category_id=t.category_id)", id); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,NULL)", id); err != nil {
			return err
		}
		if err = audit(tx, u, nil, "period", id, "legacy_limits_preserved", legacy); err != nil {
			return err
		}
	}
	// Replacement preserves recurrence for retained entries rather than restarting it.
	oldRows, err := data(tx, "SELECT COALESCE(spending_group_id,0) group_id,category_id,carry_forward FROM group_targets WHERE period_id=?", id)
	if err != nil {
		return err
	}
	carry := map[string]bool{}
	for _, row := range oldRows {
		carry[fmt.Sprintf("%d:%d", num(row["group_id"]), num(row["category_id"]))] = num(row["carry_forward"]) != 0
	}
	if !b.Merge {

		if b.GroupID != nil {
			_, err = tx.Exec("DELETE FROM group_targets WHERE period_id=? AND COALESCE(spending_group_id,0)=?", id, *b.GroupID)
		} else {
			_, err = tx.Exec("DELETE FROM group_targets WHERE period_id=?", id)
			if err == nil {
				_, err = tx.Exec("DELETE FROM budget_groups WHERE period_id=?", id)
			}
		}
		if err != nil {
			return err
		}
	}
	for _, t := range b.Targets {
		var group any
		if *t.GroupID != 0 {
			group = *t.GroupID
		}
		recurring := queryInt(tx, "SELECT archived FROM categories WHERE id=?", t.CategoryID) == 0
		if saved, ok := carry[fmt.Sprintf("%d:%d", *t.GroupID, t.CategoryID)]; ok {
			recurring = saved
		}
		if _, err = tx.Exec("INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents,carry_forward,included) VALUES(?,?,?,?,?,1) ON CONFLICT DO UPDATE SET amount_cents=excluded.amount_cents,included=1", id, t.CategoryID, group, t.Amount, recurring); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	for _, g := range b.Groups {
		var group any
		if *g.GroupID != 0 {
			group = *g.GroupID
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	if b.GroupID != nil {
		var group any
		if *b.GroupID != 0 {
			group = *b.GroupID
		}
		if _, err = tx.Exec("INSERT OR IGNORE INTO budget_groups VALUES(?,?)", id, group); err != nil {
			return err
		}
	}
	if queryInt(tx, "SELECT COUNT(*) FROM (SELECT category_id FROM group_targets WHERE period_id=? AND included=1 GROUP BY category_id HAVING SUM(amount_cents)>900000000000000)", id) > 0 {
		return fail(400, "Combined category budgets exceed the supported limit")
	}
	if err = syncBudgetTotalsTx(tx, id); err != nil {
		return err
	}
	return audit(tx, u, nil, "period", id, "limits_updated", b)
}
