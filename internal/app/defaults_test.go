package app

import (
	"encoding/json"
	"testing"
)

func seedTestDefaults(t *testing.T, e *testEnv) {
	t.Helper()
	if err := e.a.write(seedClassification); err != nil {
		t.Fatal(err)
	}
}
func TestDefaultsSeedOncePreserveHistoryAndCustomOverride(t *testing.T) {
	e := setup(t)
	seedTestDefaults(t, e)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != int64(len(defaultRules)) || queryInt(e.a.DB, "SELECT COUNT(*) FROM categories") != int64(len(defaultCategories)) {
		t.Fatal("starter definitions missing")
	}
	if queryInt(e.a.DB, "SELECT id FROM categories WHERE name='Groceries'") != 1 || queryInt(e.a.DB, "SELECT id FROM categories WHERE name='Salary'") != 2 {
		t.Fatal("existing category ids replaced")
	}
	builtin := queryInt(e.a.DB, "SELECT id FROM builtin_rules WHERE pattern='Checkers'")
	status(t, e.req(t, 1, "/api/rules/"+ruleID(-builtin), "PUT", ruleBody(0, 1, "Checkers", "debit", nil, -1000, false, 1)), 200)
	status(t, e.req(t, 1, "/api/rules/"+ruleID(-builtin), "PUT", ruleBody(0, 1, "Checkers", "debit", nil, -1000, true, 1)), 409)
	status(t, e.req(t, 3, "/api/rules/"+ruleID(-builtin), "PUT", ruleBody(0, 1, "Checkers", "debit", nil, 0, true, 2)), 403)
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT enabled FROM builtin_rules WHERE id=?", builtin) != 0 {
		t.Fatal("restart reset paused default")
	}
	w := e.req(t, 1, "/api/rules", "GET", nil)
	status(t, w, 200)
	var listed []map[string]any
	json.Unmarshal(w.Body.Bytes(), &listed)
	if len(listed) != len(defaultRules) {
		t.Fatal("builtins not listed")
	}
	status(t, e.req(t, 1, "/api/rules/"+ruleID(-builtin), "DELETE", map[string]any{"version": 2}), 200)
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE id=?", builtin) != 0 {
		t.Fatal("deleted default returned")
	}
	p := e.stage(t, "defaults.ofx", ofx(ofxRow("default-salary", "20261027", "500.00", "Synthetic salary"), ofxRow("default-fee", "20261028", "-5.00", "Monthly Account Fee")))
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 2 || p.Rows[1].CategoryID == nil || !p.Rows[1].RuleMatches[0].Builtin {
		t.Fatal("default classification failed")
	}
	// A custom fee classification suppresses the general fallback, even at lower priority.
	createTestRule(t, e, ruleBody(1, 1, "Monthly Account Fee", "debit", nil, -2000, true, 0))
	p = e.stage(t, "override.ofx", ofx(ofxRow("default-override", "20261029", "-1.00", "Monthly Account Fee")))
	if p.Rows[0].RuleConflict || p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 1 || p.Rows[0].RuleMatches[0].Builtin {
		t.Fatalf("custom override failed %+v", p.Rows[0])
	}
	status(t, e.commit(t, p.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved' AND is_transfer=0") != 1 {
		t.Fatal("categorized defaults were not accepted")
	}
}
func TestDefaultMigrationPreservesConflictingKindsAndNewAccounts(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("INSERT INTO categories(name,group_name,kind) VALUES('Bank charges','','income')")
	e.a.DB.Exec("INSERT INTO categories(name,group_name,kind) VALUES('Eating out','One','expense'),('Eating out','Two','expense')")
	removePostBaselineFixtureTables(t, e.a.DB)
	e.a.DB.Exec("DELETE FROM migrations WHERE version>=9; INSERT OR IGNORE INTO migrations VALUES(8)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE pattern='Monthly Account Fee'") != 0 {
		t.Fatal("default used conflicting category kind")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE pattern='Mcd '") != 0 {
		t.Fatal("default silently chose duplicate category")
	}
	e.a.DB.Exec("INSERT INTO accounts(id,name,bank_id,household) VALUES(8,'New account','88888888888',1)")
	w := e.req(t, 1, "/api/rules/preview", "POST", map[string]any{"account_id": 8, "description": "Engen synthetic purchase", "direction": "debit"})
	status(t, w, 200)
	var row SourceRow
	json.Unmarshal(w.Body.Bytes(), &row)
	if row.CategoryID == nil || !row.RuleMatches[0].Builtin {
		t.Fatal("new account lacks default fallback")
	}
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=8")
	status(t, e.req(t, 1, "/api/rules/preview", "POST", map[string]any{"account_id": 8, "description": "Engen synthetic purchase", "direction": "debit"}), 403)
}
func TestTransactionRuleAtomicityDirectionAndGroup(t *testing.T) {
	e := setup(t)
	gid := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Day-to-day'")
	p := e.stage(t, "transaction-rule.ofx", ofx(ofxRow("transaction-rule", "20261027", "-25.12", "Synthetic retailer *1234")))
	status(t, e.commit(t, p.ID, nil, false), 200)
	tid := queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='transaction-rule'")
	body := map[string]any{"version": 1, "date": "2026-10-27", "amount_cents": -2512, "description": "Synthetic retailer *1234", "allocations": []any{map[string]any{"category_id": 1, "amount_cents": -2512, "note": ""}}, "is_transfer": false, "spending_group_id": gid, "assignment": "auto", "period_id": nil, "rule": map[string]any{"pattern": " "}}
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", tid) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 {
		t.Fatal("invalid rule partially saved transaction")
	}
	status(t, e.req(t, 3, "/api/transactions/"+ruleID(tid), "PUT", body), 403)
	body["rule"] = map[string]any{"pattern": "Synthetic retailer"}
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE account_id=1 AND category_id=1 AND spending_group_id=? AND direction='debit'", gid) != 1 {
		t.Fatal("rule lost account/category/group/direction")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=? AND version=2 AND review_state='approved'", tid) != 1 {
		t.Fatal("categorized rule edit was not accepted")
	}
	// Stale edits roll back the rule too; transfers/splits cannot create one.
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 1 {
		t.Fatal("stale edit added rule")
	}
	body["version"] = 2
	body["is_transfer"] = true
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", tid) != 2 {
		t.Fatal("rejected transfer-rule edit committed")
	}
	body["is_transfer"] = false
	body["allocations"] = []any{map[string]any{"category_id": 1, "amount_cents": -1500, "note": ""}, map[string]any{"category_id": 1, "amount_cents": -1012, "note": ""}}
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 400)

	// Editing the same pattern updates its category rather than creating conflicting duplicates.
	body["allocations"] = []any{map[string]any{"category_id": 2, "amount_cents": -2512, "note": ""}}
	body["rule"] = map[string]any{"pattern": " synthetic RETAILER "}
	status(t, e.req(t, 1, "/api/transactions/"+ruleID(tid), "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 1 || queryInt(e.a.DB, "SELECT category_id FROM rules") != 2 {
		t.Fatal("repeat transaction edit duplicated the rule")
	}

}
func TestFreshDefaultCatalog(t *testing.T) {
	e := setup(t)
	// Simulate an old installation upgrading exactly once with financial records preserved.
	e.a.DB.Exec("INSERT INTO targets(period_id,category_id,amount_cents) VALUES(1,1,10000)")
	removePostBaselineFixtureTables(t, e.a.DB)
	e.a.DB.Exec("DELETE FROM migrations WHERE version>=9; INSERT OR IGNORE INTO migrations VALUES(8)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE category_id=1") != 10000 {
		t.Fatal("migration rewrote budget")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != 24 {
		t.Fatal("wrong starter rule count")
	}
	// Invalid category creation still rejects duplicate names and non-members.
	status(t, e.req(t, 1, "/api/categories", "POST", map[string]any{"name": " groceries ", "kind": "expense"}), 409)
	status(t, e.req(t, 3, "/api/categories", "POST", map[string]any{"name": "Private project", "kind": "expense"}), 403)
}
