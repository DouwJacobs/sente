package app

import (
	"encoding/json"
	"testing"
)

func TestGroupCategoryBudgetsIndependentAndAtomic(t *testing.T) {
	e := setup(t)
	write := func(group, version, amount int64) map[string]any {
		return map[string]any{"group_id": group, "version": version, "merge": true, "targets": []map[string]any{{"category_id": 1, "amount_cents": amount}}}
	}
	status(t, e.req(t, 1, "/api/targets/1", "PUT", write(1, 1, 100000)), 200)
	status(t, e.req(t, 1, "/api/targets/1", "PUT", write(4, 2, 30000)), 200)
	if _, err := e.a.DB.Exec("INSERT INTO categories(id,name,group_name,kind) VALUES(3,'Other fee','','expense')"); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{"group_id": 1, "version": 3, "merge": true, "targets": []map[string]any{{"category_id": 3, "amount_cents": 20000}}}), 200)

	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1 AND category_id=1") != 130000 {
		t.Fatal("category total duplicated or lost")
	}
	for group, amount := range map[int64]int64{1: 100000, 4: 30000} {
		if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id=?", group) != amount {
			t.Fatal("independent category limit changed")
		}
	}
	status(t, e.req(t, 1, "/api/targets/1", "PUT", write(1, 2, 50000)), 409)
	status(t, e.req(t, 2, "/api/targets/1", "PUT", write(1, 4, 50000)), 403)
	status(t, e.req(t, 1, "/api/targets/1", "PUT", write(999, 4, 50000)), 400)
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{"version": 4, "targets": []map[string]any{{"category_id": 1, "amount_cents": 50000}}}), 400)
	if queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1") != 4 || queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1 AND category_id=1") != 130000 {
		t.Fatal("failed update modified budgets")
	}
	w := e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, w, 200)
	var d map[string]any
	json.Unmarshal(w.Body.Bytes(), &d)
	if num(d["budget_cents"]) != 150000 || len(d["spending_groups"].([]any)) != 2 {
		t.Fatal("budget-only groups excluded")
	}
	for _, row := range d["spending_groups"].([]any) {
		g := row.(map[string]any)
		if num(g["target_cents"]) != map[int64]int64{1: 120000, 4: 30000}[num(g["id"])] || num(g["spent_cents"]) != 0 {
			t.Fatal("group budget wrong")
		}
	}
	preview := e.req(t, 1, "/api/periods/new/preview", "POST", map[string]any{"name": "Next", "start_date": "2026-11-20", "end_date": "2026-12-19"})
	status(t, preview, 200)
	var changes map[string]any
	json.Unmarshal(preview.Body.Bytes(), &changes)
	status(t, e.req(t, 1, "/api/periods", "POST", map[string]any{"name": "Next", "start_date": "2026-11-20", "end_date": "2026-12-19", "preview_token": changes["preview_token"]}), 200)
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM group_targets WHERE period_id=2") != 150000 || queryInt(e.a.DB, "SELECT COUNT(*) FROM group_targets WHERE period_id=2") != 3 {
		t.Fatal("new period did not copy group/category limits")
	}
}

func TestGroupBudgetMigrationPreservesLegacyLimits(t *testing.T) {
	e := setup(t)
	removePostBaselineFixtureTables(t, e.a.DB)
	for _, statement := range []string{"DELETE FROM migrations WHERE version>=14", "INSERT OR IGNORE INTO migrations VALUES(13)", "INSERT INTO targets VALUES(1,1,200000)"} {
		if _, err := e.a.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM group_targets WHERE period_id=1") != 1 || queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND spending_group_id IS NULL") != 200000 || queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1") != 200000 {
		t.Fatal("migration changed or guessed a budget")
	}
}
