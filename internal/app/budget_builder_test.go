package app

import (
	"encoding/json"
	"testing"
)

func TestBudgetBuilderUpcomingAndOneTime(t *testing.T) {
	e := setup(t)
	for _, statement := range []string{
		"INSERT INTO categories(id,name,group_name,kind) VALUES(3,'One-time fee','','expense')",
		"INSERT INTO periods(id,name,start_date,end_date) VALUES(2,'Next','2026-11-20','2026-12-19'),(3,'Later','2026-12-20','2027-01-19')",
		"INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents) VALUES(2,1,7,99999)",
		"INSERT INTO targets VALUES(2,1,99999)",
		"INSERT INTO budget_groups VALUES(2,7)",
	} {
		if _, err := e.a.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	b := map[string]any{"version": 1, "groups": []any{map[string]any{"group_id": 7, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 35000, "carry_forward": true, "apply_upcoming": true}, map[string]any{"category_id": 3, "amount_cents": 20000, "carry_forward": false}}}}}
	status(t, e.req(t, 2, "/api/periods/1/budget", "PUT", b), 403)
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", b), 200)
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM targets WHERE period_id=1") != 55000 {
		t.Fatal("group budget must inherit the sum of its categories")
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=2 AND category_id=1") != 99999 || queryInt(e.a.DB, "SELECT version FROM periods WHERE id=2") != 1 {
		t.Fatal("existing future budget overwritten")
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=3 AND category_id=1") != 35000 || queryInt(e.a.DB, "SELECT version FROM periods WHERE id=3") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM group_targets WHERE period_id>1 AND category_id=3") != 0 {
		t.Fatal("upcoming option or one-time category incorrect")
	}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", b), 409)
	preview := e.req(t, 1, "/api/periods/new/preview", "POST", map[string]any{"name": "Future", "start_date": "2027-01-20", "end_date": "2027-02-19"})
	status(t, preview, 200)
	var changes map[string]any
	json.Unmarshal(preview.Body.Bytes(), &changes)
	status(t, e.req(t, 1, "/api/periods", "POST", map[string]any{"name": "Future", "start_date": "2027-01-20", "end_date": "2027-02-19", "preview_token": changes["preview_token"]}), 200)
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM targets WHERE period_id=4") != 35000 || queryInt(e.a.DB, "SELECT COUNT(*) FROM group_targets WHERE period_id=4 AND category_id=3") != 0 {
		t.Fatal("new budget copied a one-time entry")
	}
	// A failure after a valid group/amount must roll back everything.
	bad := map[string]any{"version": 2, "groups": []any{map[string]any{"group_id": 7, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 500, "carry_forward": true, "apply_upcoming": true}, map[string]any{"category_id": 2, "amount_cents": 500}}}}}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", bad), 400)
	if queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1") != 2 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM targets WHERE period_id=1") != 55000 {
		t.Fatal("invalid builder save changed finances")
	}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", map[string]any{"version": 2, "groups": []any{map[string]any{"group_id": 7, "remove": true}}}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM targets WHERE period_id=1") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM budget_groups WHERE period_id=1") != 0 || queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=2") != 99999 {
		t.Fatal("group removal affected future budgets")
	}
}

func TestBudgetBuilderSparseReadsAndZeroEntry(t *testing.T) {
	e := setup(t)
	b := map[string]any{"version": 1, "groups": []any{map[string]any{"group_id": 1, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 0, "carry_forward": false}}}, map[string]any{"group_id": 7, "targets": []any{}}}}
	status(t, e.req(t, 1, "/api/periods/1/budget", "PUT", b), 200)
	w := e.req(t, 1, "/api/periods/1/budget-groups?page=0", "GET", nil)
	status(t, w, 200)
	var d map[string]any
	json.Unmarshal(w.Body.Bytes(), &d)
	if num(d["total"]) != 2 {
		t.Fatal("empty explicitly added group lost")
	}
	w = e.req(t, 1, "/api/periods/1/budget-groups?page=0&id=1", "GET", nil)
	status(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &d)
	if num(d["total"]) != 1 {
		t.Fatal("existing group lookup lost its budget")
	}
	w = e.req(t, 1, "/api/periods/1/targets?group=1&budget_only=1&page=0", "GET", nil)
	status(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &d)
	if num(d["total"]) != 1 {
		t.Fatal("builder listed unselected categories or excluded explicit zero entry")
	}
	w = e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &d)
	if len(d["spending_groups"].([]any)) != 1 {
		t.Fatal("zero budget category not visible")
	}
}
