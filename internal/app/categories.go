package app

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"
)

func categoryDeps(q queryer, id int64) (map[string]any, error) {
	rules, e := data(q, "SELECT COUNT(*) n FROM rules WHERE category_id=? AND enabled=1", id)
	if e != nil {
		return nil, e
	}
	built, e := data(q, "SELECT COUNT(*) n FROM builtin_rules WHERE category_id=? AND enabled=1", id)
	if e != nil {
		return nil, e
	}
	budgets, e := data(q, "SELECT p.id,p.name,COUNT(*) entries FROM group_targets g JOIN periods p ON p.id=g.period_id WHERE g.category_id=? AND g.included=1 AND g.carry_forward=1 GROUP BY p.id ORDER BY p.start_date,p.id", id)
	if e != nil {
		return nil, e
	}
	return map[string]any{"active_rules": num(rules[0]["n"]) + num(built[0]["n"]), "carry_forward_periods": budgets}, nil
}
func (a *App) categoryDependencies(w http.ResponseWriter, r *http.Request) error {
	if e := requireMember(Current(r)); e != nil {
		return e
	}
	id := parseID(r)
	if queryInt(a.DB, "SELECT COUNT(*) FROM categories WHERE id=?", id) != 1 {
		return fail(404, "Category not found")
	}
	// Restricted dependencies are reported without private account counts or content.
	v, e := categoryDeps(a.DB, id)
	if e != nil {
		return e
	}
	visible := queryInt(a.DB, "SELECT COUNT(*) FROM rules x JOIN accounts a ON a.id=x.account_id WHERE x.category_id=? AND x.enabled=1 AND "+accountAccessSQL(Current(r)), id, Current(r).Member, Current(r).ID) + queryInt(a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE category_id=? AND enabled=1", id)
	v["restricted_rules"] = num(v["active_rules"]) > visible
	if v["restricted_rules"] == true {
		v["active_rules"] = visible
		v["blocked"] = true
	}
	send(w, v)
	return nil
}
func (a *App) updateCategory(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if e := requireMember(u); e != nil {
		return e
	}
	var b categoryUpdateInput
	if e := decode(r, &b); e != nil {
		return e
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 1 || utf8.RuneCountInString(b.Name) > 100 {
		return fail(400, "Use a category name of 1–100 characters")
	}
	id := parseID(r)
	e := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireMember(u); err != nil {
			return err
		}
		return updateCategoryTx(tx, u, id, b)
	})
	if e != nil {
		return e
	}
	success(w)
	return nil
}

type categoryUpdateInput struct {
	Name     string `json:"name"`
	Archived bool   `json:"archived"`
	Version  int64  `json:"version,omitempty"`
}

func updateCategoryTx(tx *sql.Tx, u User, id int64, b categoryUpdateInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 1 || utf8.RuneCountInString(b.Name) > 100 {
		return fail(400, "Use a category name of 1–100 characters")
	}
	var oldName string
	var oldArchived int
	var version int64
	if tx.QueryRow("SELECT name,archived,version FROM categories WHERE id=?", id).Scan(&oldName, &oldArchived, &version) != nil {
		return fail(404, "Category not found")
	}
	if version != b.Version {
		return fail(409, "Category changed; reload it")
	}
	if b.Name != oldName && queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id!=? AND finance_normalize(name)=finance_normalize(?)", id, b.Name) > 0 {
		return fail(409, "Category name already exists")
	}
	if b.Archived && oldArchived == 0 {
		deps, e := categoryDeps(tx, id)
		if e != nil {
			return e
		}
		if deps["active_rules"].(int64) > 0 || len(deps["carry_forward_periods"].([]map[string]any)) > 0 {
			return fail(409, "Replace or pause active rules and stop budget carry-forward before archiving")
		}
	}
	res, e := tx.Exec("UPDATE categories SET name=?,archived=?,version=version+1 WHERE id=? AND version=?", b.Name, b.Archived, id, b.Version)
	if e != nil {
		return e
	}
	if e = affected(res); e != nil {
		return e
	}
	return audit(tx, u, nil, "category", id, "updated", map[string]any{"before": map[string]any{"name": oldName, "archived": oldArchived}, "after": b})
}
