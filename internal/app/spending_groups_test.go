package app

import (
	"database/sql"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestIndependentSpendingGroups(t *testing.T) {
	e := setup(t)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM spending_groups") != 11 {
		t.Fatal("missing default groups")
	}
	p := e.stage(t, "groups.ofx", ofx(ofxRow("group-test", "20261021", "-10.00", "Shop")))
	status(t, e.commit(t, p.ID, nil, true), 200)
	id := queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='group-test'")
	body := map[string]any{"version": 1, "date": "2026-10-21", "amount_cents": -1000, "description": "Shop", "allocations": []map[string]any{{"category_id": 1, "amount_cents": -1000, "note": ""}}, "is_transfer": false, "assignment": "auto", "spending_group_id": 1}
	path := "/api/transactions/" + strconv.FormatInt(id, 10)
	status(t, e.req(t, 3, path, "PUT", body), 403)
	status(t, e.req(t, 1, path, "PUT", body), 200)
	// A complete category is already accepted after the edit.
	before := e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, before, 200)
	body["version"] = 2
	body["spending_group_id"] = 6 // The Transfer label does not designate a ledger transfer.
	status(t, e.req(t, 1, path, "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT is_transfer FROM transactions WHERE id=?", id) != 0 {
		t.Fatal("label changed transfer behavior")
	}
	var state string
	e.a.DB.QueryRow("SELECT review_state FROM transactions WHERE id=?", id).Scan(&state)
	if state != "approved" {
		t.Fatal("categorized group edit must remain accepted")
	}
	if queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", id) != 1 {
		t.Fatal("group edit changed category")
	}
	status(t, e.req(t, 1, path, "PUT", body), 409)
	body["version"] = 3
	body["spending_group_id"] = 999
	status(t, e.req(t, 1, path, "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 3 {
		t.Fatal("invalid group changed transaction")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='transaction' AND details LIKE '%spending_group_id%'") != 2 {
		t.Fatal("missing classification audit")
	}
	// Group metadata survives renames without replacing the transaction reference.
	rename := map[string]any{"name": "Transfers and moves", "color": "slate", "version": 1}
	status(t, e.req(t, 3, "/api/spending-groups/6", "PUT", rename), 403)
	status(t, e.req(t, 1, "/api/spending-groups/6", "PUT", rename), 200)
	status(t, e.req(t, 1, "/api/spending-groups/6", "PUT", rename), 409)
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM spending_groups") != 11 || queryInt(e.a.DB, "SELECT COUNT(*) FROM spending_groups WHERE name='Transfers and moves'") != 1 {
		t.Fatal("restart replaced defaults")
	}
	w := e.req(t, 1, "/api/transactions?id="+strconv.FormatInt(id, 10), "GET", nil)
	status(t, w, 200)
	var result struct {
		Items []map[string]any `json:"items"`
	}
	json.Unmarshal(w.Body.Bytes(), &result)
	if result.Items[0]["spending_group_name"] != "Transfers and moves" {
		t.Fatal("rename not reflected")
	}
}

func TestCategoryPopularityRespectsPermissions(t *testing.T) {
	e := setup(t)
	for _, s := range []string{
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) VALUES(1,1,'2026-10-21',-100,'Shared','2026-10-21',-100,'Shared','{}'),(2,2,'2026-10-21',-100,'Private','2026-10-21',-100,'Private','{}')",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-50),(1,1,-50),(2,1,-100)",
	} {
		if _, err := e.a.DB.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	counts := func(w *httptest.ResponseRecorder) float64 {
		status(t, w, 200)
		var rows []map[string]any
		json.Unmarshal(w.Body.Bytes(), &rows)
		for _, row := range rows {
			if row["name"] == "Groceries" {
				return row["usage_count"].(float64)
			}
		}
		t.Fatal("missing category")
		return -1
	}
	if counts(e.req(t, 1, "/api/categories", "GET", nil)) != 2 {
		t.Fatal("owner count must include accessible accounts, counting splits once")
	}
	if counts(e.req(t, 3, "/api/categories", "GET", nil)) != 1 {
		t.Fatal("viewer count included private history")
	}
	if counts(e.req(t, 2, "/api/categories", "GET", nil)) != 0 {
		t.Fatal("ungranted user saw usage")
	}
}

func TestSpendingGroupMigrationPreservesLedger(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "old.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	old := schema
	for _, definition := range []string{"CREATE TABLE IF NOT EXISTS group_targets(", "CREATE UNIQUE INDEX IF NOT EXISTS group_target_scope", "CREATE TABLE IF NOT EXISTS budget_groups(", "CREATE UNIQUE INDEX IF NOT EXISTS budget_group_scope"} {
		start := strings.Index(old, definition)
		if start >= 0 {
			end := start + strings.Index(old[start:], ";") + 1
			old = old[:start] + old[end:]
		}
	}
	old = strings.ReplaceAll(old, "spending_group_id INTEGER REFERENCES spending_groups(id),", "")
	// A version-2 fixture predates built-in classification entirely.
	builtinStart := strings.Index(old, "CREATE TABLE IF NOT EXISTS builtin_rules(")
	if builtinStart >= 0 {
		builtinEnd := strings.Index(old[builtinStart:], ";") + builtinStart + 1
		old = old[:builtinStart] + old[builtinEnd:]
	}
	start := strings.Index(old, "CREATE TABLE IF NOT EXISTS spending_groups(")
	end := strings.Index(old[start:], ";") + start + 1
	old = old[:start] + old[end:]
	if _, err := db.Exec(old); err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		"INSERT INTO migrations VALUES(2)",
		"INSERT INTO accounts(id,name,bank_id) VALUES(1,'Existing','12345678901')",
		"INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Groceries','Living','expense')",
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,review_state,version) VALUES(1,1,'2026-10-21',-100,'Shop','2026-10-21',-100,'Shop','{}','approved',7)",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-100)",
	} {
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM spending_groups") != 11 || queryInt(db, "SELECT amount_cents FROM transactions WHERE id=1") != -100 || queryInt(db, "SELECT version FROM transactions WHERE id=1") != 7 || queryInt(db, "SELECT COUNT(*) FROM transactions WHERE review_state='approved' AND spending_group_id IS NULL") != 1 || queryInt(db, "SELECT category_id FROM allocations WHERE transaction_id=1") != 1 {
		t.Fatal("migration rewrote existing ledger or classification")
	}
}

func TestFlatCategoryCreation(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 1, "/api/categories", "POST", map[string]any{"name": "Eating Out", "kind": "expense"}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE name='Eating Out' AND group_name=''") != 1 {
		t.Fatal("category should not require or acquire a group")
	}
	status(t, e.req(t, 1, "/api/categories", "POST", map[string]any{"name": "groceries", "kind": "expense", "group_name": "Another group"}), 409)
	status(t, e.req(t, 3, "/api/categories", "POST", map[string]any{"name": "Restricted", "kind": "expense"}), 403)
}

func TestCategorySpendingGroupAndBudgetInheritance(t *testing.T) {
	e := setup(t)
	res := e.req(t, 1, "/api/categories", "POST", map[string]any{"name": "Coffee & Treats", "kind": "expense", "spending_group_name": "Day-to-day"})
	status(t, res, 200)
	var catRes map[string]any
	json.NewDecoder(res.Body).Decode(&catRes)
	cid := num(catRes["id"])

	dayGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Day-to-day'")
	if queryInt(e.a.DB, "SELECT spending_group_id FROM categories WHERE id=?", cid) != dayGroup {
		t.Fatalf("expected category to have spending_group_id=%d", dayGroup)
	}

	listRes := e.req(t, 1, "/api/categories?page=0&page_size=100&id="+strconv.FormatInt(cid, 10), "GET", nil)
	status(t, listRes, 200)
	var listData map[string]any
	json.NewDecoder(listRes.Body).Decode(&listData)
	items := listData["items"].([]any)
	if len(items) == 0 {
		t.Fatal("created category not found in list")
	}
	item := items[0].(map[string]any)
	if num(item["spending_group_id"]) != dayGroup || item["spending_group_name"] != "Day-to-day" {
		t.Fatalf("unexpected category spending group data: %+v", item)
	}

	recGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	ver := queryInt(e.a.DB, "SELECT version FROM categories WHERE id=?", cid)
	status(t, e.req(t, 1, "/api/categories/"+strconv.FormatInt(cid, 10), "PUT", map[string]any{"name": "Coffee & Treats", "archived": false, "spending_group_id": recGroup, "version": ver}), 200)
	if queryInt(e.a.DB, "SELECT spending_group_id FROM categories WHERE id=?", cid) != recGroup {
		t.Fatal("category spending group was not updated")
	}

	pver := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{
		"version": pver,
		"merge":   true,
		"targets": []map[string]any{
			{"category_id": cid, "amount_cents": 45000},
		},
	}), 200)

	gtGroup := queryInt(e.a.DB, "SELECT spending_group_id FROM group_targets WHERE period_id=1 AND category_id=?", cid)
	if gtGroup != recGroup {
		t.Fatalf("expected budget target to inherit spending_group_id=%d, got %d", recGroup, gtGroup)
	}
}

