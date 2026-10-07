package app

import (
	"net/http"
	"strconv"
)

func (a *App) targetPages(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	id := parseID(r)
	if queryInt(a.DB, "SELECT COUNT(*) FROM periods WHERE id=?", id) == 0 {
		return fail(404, "Period not found")
	}
	if r.URL.Query().Has("group") {
		gid, err := strconv.ParseInt(r.URL.Query().Get("group"), 10, 64)
		if err != nil || gid < 0 || gid > 0 && queryInt(a.DB, "SELECT COUNT(*) FROM spending_groups WHERE id=?", gid) != 1 {
			return fail(400, "Choose a spending group")
		}
		query := "SELECT c.id,c.name,c.kind,COALESCE(t.amount_cents,0) amount_cents,t.carry_forward FROM categories c LEFT JOIN group_targets t ON t.category_id=c.id AND t.period_id=? AND COALESCE(t.spending_group_id,0)=? WHERE c.kind='expense'"
		if r.URL.Query().Get("budget_only") == "1" {
			query += " AND t.included=1"
		}
		return a.metadataPage(w, r, query, []any{id, gid}, "name", "name,id")
	}
	return a.metadataPage(w, r, "SELECT c.id,c.name,c.kind,COALESCE(t.amount_cents,0) amount_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? WHERE c.kind='expense'", []any{id}, "name", "name,id")
}
