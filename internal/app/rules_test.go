package app

import (
	"encoding/json"
	"strconv"
	"testing"
)

func ruleBody(account, category int64, pattern, direction string, group any, priority int, enabled bool, version int64) map[string]any {
	return map[string]any{"account_id": account, "category_id": category, "pattern": pattern, "direction": direction, "spending_group_id": group, "priority": priority, "enabled": enabled, "version": version}
}
func createTestRule(t *testing.T, e *testEnv, body map[string]any) int64 {
	t.Helper()
	w := e.req(t, 1, "/api/rules", "POST", body)
	status(t, w, 200)
	var v map[string]int64
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v["id"]
}
func TestRuleDirectionGroupConflictAndReview(t *testing.T) {
	e := setup(t)
	gid := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Day-to-day'")
	id := createTestRule(t, e, ruleBody(1, 1, "Market", "debit", gid, 5, true, 0))
	p := e.stage(t, "rules.ofx", ofx(ofxRow("rule-out", "20261027", "-42.12", "  MARKET   purchase "), ofxRow("rule-refund", "20261028", "42.12", "Market refund")))
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 1 || p.Rows[0].SpendingGroupID == nil || *p.Rows[0].SpendingGroupID != gid || p.Rows[1].CategoryID != nil {
		t.Fatalf("direction/group mismatch: %+v", p.Rows)
	}
	status(t, e.commit(t, p.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review' AND is_transfer=0") != 1 {
		t.Fatal("categorized entry not accepted or missing entry accepted")
	}
	if queryInt(e.a.DB, "SELECT spending_group_id FROM transactions WHERE fitid='rule-out'") != gid {
		t.Fatal("group not committed")
	}
	// A group named Transfer is presentation only, never a ledger designation.
	transferGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Transfer'")
	createTestRule(t, e, ruleBody(1, 1, "Internal candidate", "debit", transferGroup, 0, true, 0))
	labelOnly := e.stage(t, "label-only.ofx", ofx(ofxRow("label-only", "20261028", "-6.00", "Internal candidate")))
	status(t, e.commit(t, labelOnly.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT is_transfer FROM transactions WHERE fitid='label-only'") != 0 || queryInt(e.a.DB, "SELECT spending_group_id FROM transactions WHERE fitid='label-only'") != transferGroup {
		t.Fatal("group changed ledger semantics")
	}
	// Refunds are credits but can deliberately retain an expense category.
	createTestRule(t, e, ruleBody(1, 1, "Market", "credit", gid, 5, true, 0))
	p = e.stage(t, "refund.ofx", ofx(ofxRow("rule-refund-2", "20261029", "5.00", "Market refund")))
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 1 {
		t.Fatal("refund category lost")
	}
	// Conflicting outputs are withheld even at different priorities.
	createTestRule(t, e, ruleBody(1, 2, "purchase", "debit", nil, 50, true, 0))
	p = e.stage(t, "conflict.ofx", ofx(ofxRow("conflict-rule", "20261030", "-3.00", "Market purchase")))
	if !p.Rows[0].RuleConflict || p.Rows[0].CategoryID != nil || p.Rows[0].SpendingGroupID != nil || len(p.Rows[0].RuleMatches) != 2 {
		t.Fatalf("conflict silently classified: %+v", p.Rows[0])
	}
	if p.Rows[0].RuleMatches[0].CategoryID != 2 {
		t.Fatal("priority not visible")
	}
	// Pausing applies to future staging; it never rewrites existing ledger rows.
	status(t, e.req(t, 1, "/api/rules/"+ruleID(id), "PUT", ruleBody(1, 1, "Market", "debit", gid, 5, false, 1)), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations l JOIN transactions t ON t.id=l.transaction_id WHERE t.fitid='rule-out' AND l.category_id=1") != 1 {
		t.Fatal("existing transaction changed")
	}
}
func ruleID(v int64) string { return strconv.FormatInt(v, 10) }

func TestRulePermissionsVersionsAndPreview(t *testing.T) {
	e := setup(t)
	body := ruleBody(1, 1, "Market", "any", nil, 0, true, 0)
	for _, uid := range []int{2, 3} {
		status(t, e.req(t, uid, "/api/rules", "POST", body), 403)
	}
	id := createTestRule(t, e, body)
	status(t, e.req(t, 1, "/api/rules/"+ruleID(id), "PUT", ruleBody(1, 1, "Shop", "debit", nil, 2, true, 1)), 200)
	status(t, e.req(t, 1, "/api/rules/"+ruleID(id), "PUT", body), 409)
	status(t, e.req(t, 3, "/api/rules/"+ruleID(id), "DELETE", map[string]int{"version": 2}), 403)
	status(t, e.req(t, 1, "/api/rules/"+ruleID(id), "DELETE", map[string]int{"version": 1}), 409)
	before := queryInt(e.a.DB, "SELECT COUNT(*) FROM rules")
	w := e.req(t, 1, "/api/rules/preview", "POST", map[string]any{"account_id": 1, "description": "Shop purchase", "direction": "debit", "draft": ruleBody(1, 2, "Shop", "debit", nil, 10, true, 0)})
	status(t, w, 200)
	var row SourceRow
	json.Unmarshal(w.Body.Bytes(), &row)
	if !row.RuleConflict || row.CategoryID != nil || len(row.RuleMatches) != 2 {
		t.Fatalf("preview missed conflict %+v", row)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != before || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
		t.Fatal("preview wrote records")
	}
	status(t, e.req(t, 3, "/api/rules/preview", "POST", map[string]any{"account_id": 1, "description": "Shop", "direction": "debit"}), 403)
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1")
	status(t, e.req(t, 1, "/api/rules", "POST", body), 403)
	status(t, e.req(t, 1, "/api/rules/preview", "POST", map[string]any{"account_id": 1, "description": "Shop", "direction": "debit"}), 403)
	status(t, e.upload(t, 1, 1, "hidden.csv", []byte(csvFile("2026/10/27,-1.00,1.00,Shop"))), 403)
}
func TestRuleStaleStageRequiresNewPreview(t *testing.T) {
	e := setup(t)
	createTestRule(t, e, ruleBody(1, 1, "Market", "debit", nil, 0, true, 0))
	p := e.stage(t, "stale.ofx", ofx(ofxRow("stale", "20261027", "-4.00", "Market purchase")))
	createTestRule(t, e, ruleBody(1, 2, "purchase", "debit", nil, 1, true, 0))
	status(t, e.commit(t, p.ID, nil, false), 409)
	w := e.req(t, 1, "/api/imports", "GET", nil)
	status(t, w, 200)
	var items []struct {
		Preview ParsedFile `json:"preview"`
	}
	json.Unmarshal(w.Body.Bytes(), &items)
	if len(items) != 1 || !items[0].Preview.Rows[0].RuleConflict {
		t.Fatal("current preview not reclassified")
	}
	status(t, e.req(t, 1, "/api/imports/"+ruleID(p.ID)+"/commit", "POST", map[string]any{"classification_version": items[0].Preview.ClassificationVersion, "decisions": map[string]string{}}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NULL") != 1 {
		t.Fatal("stale category committed")
	}
}
func TestRuleMigrationPreservesLegacyDefaults(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("DROP TABLE rules")
	_, err := e.a.DB.Exec("CREATE TABLE rules(id INTEGER PRIMARY KEY,user_id INTEGER NOT NULL REFERENCES users(id),account_id INTEGER NOT NULL REFERENCES accounts(id),pattern TEXT NOT NULL,category_id INTEGER NOT NULL REFERENCES categories(id),priority INTEGER NOT NULL DEFAULT 0); INSERT INTO rules VALUES(7,1,1,'Market',1,9); DELETE FROM migrations WHERE version>=8; INSERT OR IGNORE INTO migrations VALUES(7)")
	if err != nil {
		t.Fatal(err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	var direction string
	var enabled, version, category, priority int64
	var group any
	err = e.a.DB.QueryRow("SELECT direction,enabled,version,category_id,priority,spending_group_id FROM rules WHERE id=7").Scan(&direction, &enabled, &version, &category, &priority, &group)
	if err != nil || direction != "any" || enabled != 1 || version != 1 || category != 1 || priority != 9 || group != nil {
		t.Fatalf("legacy migration changed behavior %s %d %d %v", direction, enabled, version, err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 1 {
		t.Fatal("repeat migration lost rules")
	}
}

func TestRuleBatchAtomicScopeAndConcurrency(t *testing.T) {
	e := setup(t)
	body := map[string]any{"rules": []any{
		map[string]any{"id": 0, "rule": ruleBody(1, 1, "Supplier", "debit", nil, 0, true, 0)},
		map[string]any{"id": 0, "rule": ruleBody(2, 1, "Supplier", "debit", nil, 0, true, 0)},
	}}
	status(t, e.req(t, 3, "/api/rules/batch", "POST", body), 403)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 {
		t.Fatal("partial unauthorized batch")
	}
	status(t, e.req(t, 1, "/api/rules/batch", "POST", body), 200)
	rows, _ := data(e.a.DB, "SELECT * FROM rules ORDER BY id")
	first, second := num(rows[0]["id"]), num(rows[1]["id"])
	// A stale second account rolls back the first edit too.
	updates := map[string]any{"rules": []any{
		map[string]any{"id": first, "rule": ruleBody(1, 2, "Supplier", "debit", nil, 0, true, 1)},
		map[string]any{"id": second, "rule": ruleBody(2, 2, "Supplier", "debit", nil, 0, true, 0)},
	}}
	status(t, e.req(t, 1, "/api/rules/batch", "POST", updates), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE category_id=1 AND version=1") != 2 {
		t.Fatal("stale batch partially applied")
	}
	// Hiding or revoking one account prevents all edits and deletes.
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=2")
	updates["rules"] = []any{
		map[string]any{"id": first, "rule": ruleBody(1, 1, "Supplier", "debit", nil, 0, false, 1)},
		map[string]any{"id": second, "rule": ruleBody(2, 1, "Supplier", "debit", nil, 0, false, 1)},
	}
	status(t, e.req(t, 1, "/api/rules/batch", "POST", updates), 403)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE enabled=1") != 2 {
		t.Fatal("hidden batch partly paused")
	}
	deleted := map[string]any{"rules": []any{map[string]any{"id": first, "version": 1}, map[string]any{"id": second, "version": 1}}}
	status(t, e.req(t, 1, "/api/rules/batch", "DELETE", deleted), 403)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 2 {
		t.Fatal("hidden batch partly deleted")
	}
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=0 WHERE id=2")
	status(t, e.req(t, 1, "/api/rules/batch", "POST", updates), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE enabled=0 AND version=2") != 2 {
		t.Fatal("batch pause failed")
	}
	status(t, e.req(t, 1, "/api/rules/batch", "DELETE", deleted), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 2 {
		t.Fatal("stale delete partly applied")
	}
	deleted["rules"] = []any{map[string]any{"id": first, "version": 2}, map[string]any{"id": second, "version": 2}}
	status(t, e.req(t, 1, "/api/rules/batch", "DELETE", deleted), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 {
		t.Fatal("group delete failed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
		t.Fatal("rule batch created transactions")
	}
}
