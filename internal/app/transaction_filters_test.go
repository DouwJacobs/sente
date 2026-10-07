package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"testing"
)

func filterResult(t *testing.T, e *testEnv, uid int, path string) map[string]any {
	t.Helper()
	w := e.req(t, uid, path, "GET", nil)
	status(t, w, 200)
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}
func TestTransactionClassificationFiltersBeforePaging(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	gid := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	for i := 0; i < 105; i++ {
		seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	}
	missing := seedTransaction(t, e, 1, -200, "2026-10-22", nil)
	split := seedTransaction(t, e, 1, -300, "2026-10-22", &cat)
	e.a.DB.Exec("UPDATE allocations SET amount_cents=-100 WHERE transaction_id=?", split)
	e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,NULL,-200)", split)
	e.a.DB.Exec("UPDATE transactions SET spending_group_id=? WHERE id=?", gid, split)
	transfer := seedTransaction(t, e, 1, -100, "2026-10-23", nil)
	e.a.DB.Exec("UPDATE transactions SET is_transfer=1 WHERE id=?", transfer)
	seedTransaction(t, e, 2, -100, "2026-10-23", nil)
	seedTransaction(t, e, 1, 100, "2026-10-23", &cat)
	e.a.DB.Exec("UPDATE transactions SET review_state='approved' WHERE id=?", missing)
	check := func(uid int, query string, want int) map[string]any {
		t.Helper()
		v := filterResult(t, e, uid, "/api/transactions?"+query)
		if v["total"] != float64(want) {
			t.Fatalf("%s: %+v", query, v)
		}
		return v
	}
	check(3, "category=uncategorized", 2) // any missing split, excluding transfers/private data
	check(1, "category=uncategorized", 3)
	check(3, "category=uncategorized&pending=1", 1)
	check(3, fmt.Sprintf("category=1&spending_group=%d&direction=out", gid), 1)
	check(3, "category=uncategorized&spending_group=unassigned", 1)
	check(3, "direction=in", 1)
	check(3, "category=01&direction=in", 1)
	check(2, "category=uncategorized", 0)
	check(3, "category=uncategorized&account=2", 0)
	page := check(3, "category=1&direction=out", 106)
	if len(page["items"].([]any)) != 100 {
		t.Fatal("filters paged in the client")
	}
	next := check(3, "category=1&direction=out&offset=100&list_version="+url.QueryEscape(page["list_version"].(string)), 106)
	if len(next["items"].([]any)) != 6 {
		t.Fatal("wrong filtered second page")
	}
	check(3, "q=%25", 0) // literal percent does not mean match everything
	for _, q := range []string{"category=-1", "category=bad", "spending_group=0", "direction=expense"} {
		status(t, e.req(t, 1, "/api/transactions?"+q, "GET", nil), 400)
	}
}
func TestImportClassificationFiltersUseCurrentRulesAndLedger(t *testing.T) {
	e := setup(t)
	p := e.stage(t, "matching.ofx", ofx(ofxRow("filter-match", "20261021", "-2.00", "Synthetic merchant")))
	result := filterResult(t, e, 1, "/api/imports?page=0&category=uncategorized")
	if result["total"] != float64(1) {
		t.Fatal(result)
	}
	createTestRule(t, e, ruleBody(1, 1, "Synthetic merchant", "debit", nil, 0, true, 0))
	if filterResult(t, e, 1, "/api/imports?page=0&category=uncategorized")["total"] != float64(0) {
		t.Fatal("staged filter used old rule suggestions")
	}
	if filterResult(t, e, 1, "/api/imports?page=0&category=1&direction=out&q=merchant")["total"] != float64(1) {
		t.Fatal("current staged rule did not match")
	}
	nested := fmt.Sprintf("/api/imports?page=0&activity=file:%d&category=1", p.ID)
	if filterResult(t, e, 1, nested)["total"] != float64(1) {
		t.Fatal("account expansion ignored filters")
	}
	// A filter read never rewrites the import or bypasses rule-change confirmation.
	status(t, e.commit(t, p.ID, nil, false), 409)
	e.a.DB.Exec("UPDATE rules SET enabled=0")
	status(t, e.commit(t, p.ID, nil, false), 200)
	stamp := filterResult(t, e, 1, "/api/imports?page=0&category=uncategorized")
	id := queryInt(e.a.DB, "SELECT id FROM transactions WHERE import_id=?", p.ID)
	cat := int64(1)
	body := editBody(1, -200, []Allocation{{CategoryID: &cat, Amount: -200, Note: ""}})
	body["date"] = "2026-10-21"
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if filterResult(t, e, 1, "/api/imports?page=0&category=uncategorized")["total"] != float64(0) {
		t.Fatal("committed filter used original import category")
	}
	status(t, e.req(t, 1, "/api/imports?page=1&category=uncategorized&list_version="+url.QueryEscape(stamp["list_version"].(string)), "GET", nil), 409)
	if filterResult(t, e, 1, "/api/imports?page=0&category=1")["total"] != float64(1) {
		t.Fatal("edited ledger category missing")
	}
	if filterResult(t, e, 2, "/api/imports?page=0&category=1")["total"] != float64(0) {
		t.Fatal("private metadata leaked")
	}
	for _, q := range []string{"category=bad", "spending_group=-1", "direction=bad", "account=bad"} {
		status(t, e.req(t, 1, "/api/imports?page=0&"+q, "GET", nil), 400)
	}
}
