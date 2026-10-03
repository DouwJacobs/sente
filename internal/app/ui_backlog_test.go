package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func pageBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	status(t, w, 200)
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func TestRuleDefinitionPagesPreserveGroupsAndPermissions(t *testing.T) {
	e := setup(t)
	for i := 0; i < 25; i++ {
		createTestRule(t, e, ruleBody(1, 1, fmt.Sprintf("Page rule %02d", i), "any", nil, 1, true, 0))
		createTestRule(t, e, ruleBody(2, 1, fmt.Sprintf("Page rule %02d", i), "any", nil, 1, true, 0))
	}
	createTestRule(t, e, ruleBody(1, 1, "CAFÉ\u00a0\u00a0supplier", "any", nil, -50, true, 0))
	createTestRule(t, e, ruleBody(2, 1, "café supplier", "any", nil, -50, true, 0))
	unicode := pageBody(t, e.req(t, 1, "/api/rules?page=0&page_size=100", "GET", nil))
	unicodeRows := 0
	for _, item := range unicode["items"].([]any) {
		row := item.(map[string]any)
		if row["normalized_pattern"] == "café supplier" {
			unicodeRows++
		}
	}
	if unicodeRows != 2 || unicode["total"].(float64) != 26 {
		t.Fatal("Unicode group paging differs from classification")
	}
	first := pageBody(t, e.req(t, 1, "/api/rules?page=0&page_size=10", "GET", nil))
	items := first["items"].([]any)
	if len(items) != 20 {
		t.Fatalf("group page split or lost: %d", len(items))
	}
	seen := map[string]bool{}
	for _, item := range items {
		row := item.(map[string]any)
		seen[row["pattern"].(string)] = true
	}
	if len(seen) != 10 {
		t.Fatal("wrong definition count")
	}
	second := pageBody(t, e.req(t, 1, "/api/rules?page=1&page_size=10", "GET", nil))
	for _, item := range second["items"].([]any) {
		if seen[item.(map[string]any)["pattern"].(string)] {
			t.Fatal("duplicate definition across pages")
		}
	}
	limited := pageBody(t, e.req(t, 3, "/api/rules?page=0&page_size=100", "GET", nil))
	for _, item := range limited["items"].([]any) {
		row := item.(map[string]any)
		if row["builtin"].(float64) == 0 && row["account_id"].(float64) == 2 {
			t.Fatal("private rules exposed")
		}
	}
	status(t, e.req(t, 1, "/api/rules?page=-1", "GET", nil), 400)
	status(t, e.req(t, 1, "/api/rules?page=0&page_size=101", "GET", nil), 400)
}
func TestScopedAccountRefreshAndVisibilityVersions(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic first", BankID: "12345678901", Balance: fnbDecimal("1.00")}, {Name: "Synthetic second", BankID: "98765432109", Balance: fnbDecimal("2.00")}}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 200)
	first := queryInt(e.a.DB, "SELECT id FROM accounts WHERE bank_id='12345678901'")
	second := queryInt(e.a.DB, "SELECT id FROM accounts WHERE bank_id='98765432109'")
	e.a.fnbProvider = func(_ context.Context, c fnbCredentials, _ bool) (fnbSnapshot, error) {
		if len(c.Hidden) == 0 {
			t.Fatal("other accounts not excluded")
		}
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "First updated", BankID: "12345678901", Balance: fnbDecimal("3.00")}, {Name: "Second unexpected", BankID: "98765432109", Balance: fnbDecimal("9.00")}, {Name: "Unrequested discovery", BankID: "11111111111", Balance: fnbDecimal("9.00")}}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", map[string]any{"account_id": first}), 200)
	if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=?", first) != 300 || queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=?", second) != 200 || queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE bank_id='11111111111'") != 0 {
		t.Fatal("single-account scope widened")
	}
	version := queryInt(e.a.DB, "SELECT version FROM accounts WHERE id=?", first)
	body := map[string]any{"hidden": true, "version": version}
	status(t, e.req(t, 2, fmt.Sprintf("/api/accounts/%d/visibility", first), "PUT", body), 403)
	status(t, e.req(t, 1, fmt.Sprintf("/api/accounts/%d/visibility", first), "PUT", body), 200)
	status(t, e.req(t, 1, fmt.Sprintf("/api/accounts/%d/visibility", first), "PUT", body), 409)
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", map[string]any{"account_id": first}), 403)
	body = map[string]any{"hidden": false, "version": version + 1}
	status(t, e.req(t, 1, fmt.Sprintf("/api/accounts/%d/visibility", first), "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT hidden FROM fnb_discoveries WHERE account_id=?", first) != 0 {
		t.Fatal("restore did not persist")
	}
}
func TestImportPagesAreBoundedAndRetainFullCommit(t *testing.T) {
	e := setup(t)
	rows := ""
	for i := 0; i < 125; i++ {
		rows += ofxRow(fmt.Sprintf("paged-%d", i), "20261027", "-1.00", fmt.Sprintf("Synthetic purchase %d", i))
	}
	p := e.stage(t, "pages.ofx", ofx(rows))
	history := pageBody(t, e.req(t, 1, "/api/imports?page=0&page_size=1", "GET", nil))
	item := history["items"].([]any)[0].(map[string]any)
	if item["preview"] != nil || item["data"] != nil || item["row_count"].(float64) != 125 {
		t.Fatal("history contains full row data")
	}
	page := pageBody(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d?page=1&page_size=50", p.ID), "GET", nil))
	if page["total"].(float64) != 125 || len(page["rows"].([]any)) != 50 {
		t.Fatal("rows not bounded")
	}
	status(t, e.req(t, 2, fmt.Sprintf("/api/imports/%d?page=0", p.ID), "GET", nil), 404)
	status(t, e.commit(t, p.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review'") != 125 {
		t.Fatal("commit truncated to visible page")
	}
}

func TestMetadataPagesSearchSnapshotsAndPartialLimits(t *testing.T) {
	e := setup(t)
	err := e.a.write(func(tx *sql.Tx) error {
		for i := 0; i < 125; i++ {
			if _, err := tx.Exec("INSERT INTO categories(name,group_name,kind) VALUES(?,'','expense')", fmt.Sprintf("Page category %03d", i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	first := pageBody(t, e.req(t, 1, "/api/categories?page=0&page_size=20&q=Page%20category", "GET", nil))
	if first["total"].(float64) != 125 || len(first["items"].([]any)) != 20 {
		t.Fatal("search or page bound wrong")
	}
	seen := map[float64]bool{}
	for _, item := range first["items"].([]any) {
		seen[item.(map[string]any)["id"].(float64)] = true
	}
	second := pageBody(t, e.req(t, 1, "/api/categories?page=1&page_size=20&q=Page%20category&list_version="+first["list_version"].(string), "GET", nil))
	for _, item := range second["items"].([]any) {
		if seen[item.(map[string]any)["id"].(float64)] {
			t.Fatal("duplicate category across pages")
		}
	}
	e.a.DB.Exec("INSERT INTO categories(name,group_name,kind) VALUES('Page category new','','expense')")
	status(t, e.req(t, 1, "/api/categories?page=1&q=Page%20category&list_version="+first["list_version"].(string), "GET", nil), 409)
	e.a.DB.Exec("INSERT OR REPLACE INTO targets VALUES(1,1,10000),(1,2,20000)")
	version := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{"version": version, "merge": true, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 5000}}}), 200)
	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1 AND category_id=2") != 20000 {
		t.Fatal("unloaded target lost")
	}
	targets := pageBody(t, e.req(t, 1, "/api/periods/1/targets?page=0&page_size=10", "GET", nil))
	if len(targets["items"].([]any)) != 10 {
		t.Fatal("targets unbounded")
	}
	for _, item := range targets["items"].([]any) {
		if item.(map[string]any)["kind"] != "expense" {
			t.Fatal("income in expense limits")
		}
	}
	accounts := pageBody(t, e.req(t, 3, "/api/accounts?page=0&page_size=1", "GET", nil))
	if accounts["total"].(float64) != 1 {
		t.Fatal("private account count leaked")
	}
	for _, endpoint := range []string{"/api/users?page=0&page_size=1", "/api/grants?page=0&page_size=1", "/api/accounts/manage?page=0&page_size=1", "/api/spending-groups?page=0&page_size=1", "/api/periods?page=0&page_size=1"} {
		v := pageBody(t, e.req(t, 1, endpoint, "GET", nil))
		if len(v["items"].([]any)) > 1 {
			t.Fatal("unbounded endpoint", endpoint)
		}
		if endpoint != "/api/spending-groups?page=0&page_size=1" {
			status(t, e.req(t, 3, endpoint, "GET", nil), 403)
		}
	}
}
func TestPagedAggregatesAndTransactionsKeepFullScope(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	err := e.a.write(func(tx *sql.Tx) error {
		for i := 0; i < 35; i++ {
			res, err := tx.Exec("INSERT INTO categories(name,group_name,kind) VALUES(?,'','expense')", fmt.Sprintf("Aggregate category %02d", i))
			if err != nil {
				return err
			}
			id, _ := res.LastInsertId()
			if _, err = tx.Exec("INSERT INTO targets VALUES(1,?,1000)", id); err != nil {
				return err
			}
			if _, err = tx.Exec("INSERT INTO accounts(name,bank_id,household) VALUES(?,?,1)", fmt.Sprintf("Balance %02d", i), fmt.Sprintf("900000%05d", i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	seedTransaction(t, e, 1, -1234, "2026-10-21", &cat)
	dashboard := pageBody(t, e.req(t, 1, "/api/dashboard?period=1&category_page=1&balance_page=1", "GET", nil))
	if len(dashboard["categories"].([]any)) > 20 || len(dashboard["balances"].([]any)) > 20 {
		t.Fatal("dashboard display not bounded")
	}
	if dashboard["budget_cents"].(float64) != float64(queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM targets WHERE period_id=1")) || dashboard["spent_cents"].(float64) != 1234 {
		t.Fatal("aggregate counts only a page")
	}
	first := pageBody(t, e.req(t, 1, "/api/transactions", "GET", nil))
	seedTransaction(t, e, 1, -55, "2026-10-22", &cat)
	status(t, e.req(t, 1, "/api/transactions?offset=100&list_version="+first["list_version"].(string), "GET", nil), 409)
}
func TestAllCurrentRuleScopeIncludesUnloadedAccounts(t *testing.T) {
	e := setup(t)
	err := e.a.write(func(tx *sql.Tx) error {
		for i := 0; i < 105; i++ {
			if _, err := tx.Exec("INSERT INTO accounts(name,bank_id,household) VALUES(?,?,1)", fmt.Sprintf("Scope account %03d", i), fmt.Sprintf("800000%05d", i)); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	rule := ruleBody(0, 1, "All scope supplier", "debit", nil, 0, true, 0)
	status(t, e.req(t, 1, "/api/rules/batch", "POST", map[string]any{"all_current": true, "rules": []any{map[string]any{"id": 0, "rule": rule}}}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE pattern='All scope supplier'") != 107 {
		t.Fatal("bulk scope truncated to lookup page")
	}
	preview := pageBody(t, e.req(t, 1, "/api/rules/preview", "POST", map[string]any{"all_current": true, "description": "All scope supplier example", "direction": "debit", "draft": rule}))
	if preview["accounts"].(float64) != 107 || preview["matches"].(float64) != 107 {
		t.Fatal("preview truncated to lookup page")
	}
}
func TestImportPreviewVersionsProtectCrossPageDecisions(t *testing.T) {
	e := setup(t)
	p := e.stage(t, "preview-version.ofx", ofx(ofxRow("preview-token", "20261021", "-1.00", "Synthetic version purchase")))
	page := pageBody(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d?page=0", p.ID), "GET", nil))
	cat := int64(1)
	seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	// A rules change invalidates the version even when this particular description still has no match.
	createTestRule(t, e, ruleBody(1, 1, "Unrelated supplier", "any", nil, 0, true, 0))
	status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{"preview_version": page["preview_version"]}), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE fitid='preview-token'") != 0 {
		t.Fatal("stale preview committed")
	}
}

func TestEmptyLimitPagesCreateExpenseWithoutSavingLimits(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("DELETE FROM rules")
	e.a.DB.Exec("DELETE FROM targets")
	e.a.DB.Exec("DELETE FROM categories")
	empty := pageBody(t, e.req(t, 1, "/api/periods/1/targets?page=0", "GET", nil))
	if empty["total"].(float64) != 0 {
		t.Fatal("not an empty fixture")
	}
	created := pageBody(t, e.req(t, 1, "/api/categories", "POST", map[string]any{"name": "First synthetic expense", "kind": "expense"}))
	id := int64(created["id"].(float64))
	after := pageBody(t, e.req(t, 1, "/api/periods/1/targets?page=0", "GET", nil))
	item := after["items"].([]any)[0].(map[string]any)
	if item["amount_cents"].(float64) != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM targets") != 0 {
		t.Fatal("category creation saved limits implicitly")
	}
	version := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{"version": version, "merge": true, "targets": []any{map[string]any{"category_id": id, "amount_cents": 8912}}}), 200)
	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1 AND category_id=?", id) != 8912 {
		t.Fatal("explicit limit save failed")
	}
}
