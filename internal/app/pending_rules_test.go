package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

func pendingRuleBody(version int64, group int64) map[string]any {
	category := int64(1)
	body := editBody(version, -100, []Allocation{{CategoryID: &category, Amount: -100, Note: "source note"}})
	body["description"] = "Synthetic merchant reference 1234"
	body["spending_group_id"] = group
	body["rule"] = map[string]any{"pattern": "Synthetic merchant"}
	return body
}
func pendingRuleCounts(t *testing.T, e *testEnv, id int64, body map[string]any) pendingRuleResult {
	t.Helper()
	w := e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body)
	status(t, w, 200)
	var result struct {
		PendingRule pendingRuleResult `json:"pending_rule"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result.PendingRule
}
func merchantTransaction(t *testing.T, e *testEnv, account, amount int64, category *int64) int64 {
	t.Helper()
	id := seedTransaction(t, e, account, amount, "2026-10-21", category)
	if _, err := e.a.DB.Exec("UPDATE transactions SET description='Synthetic merchant reference 5678' WHERE id=?", id); err != nil {
		t.Fatal(err)
	}
	return id
}
func TestPendingRuleApplicationPreservesFinancialAndReviewState(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	existingGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Exceptions'")
	source := merchantTransaction(t, e, 1, -100, nil)
	targets := []int64{}
	for i := 0; i < 105; i++ {
		targets = append(targets, merchantTransaction(t, e, 1, -100, nil))
	}
	e.a.DB.Exec("UPDATE transactions SET spending_group_id=? WHERE id=?", existingGroup, targets[0])
	e.a.DB.Exec("UPDATE allocations SET note='keep my note' WHERE transaction_id=?", targets[0])
	category := int64(2)
	categorized := merchantTransaction(t, e, 1, -100, &category)
	approved := merchantTransaction(t, e, 1, -100, nil)
	e.a.DB.Exec("UPDATE transactions SET review_state='approved',reviewed_by=1,reviewed_at=CURRENT_TIMESTAMP WHERE id=?", approved)
	transfer := merchantTransaction(t, e, 1, -100, nil)
	e.a.DB.Exec("UPDATE transactions SET is_transfer=1 WHERE id=?", transfer)
	credit := merchantTransaction(t, e, 1, 100, nil)
	private := merchantTransaction(t, e, 2, -100, nil)
	unrelated := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	split := merchantTransaction(t, e, 1, -100, nil)
	e.a.DB.Exec("UPDATE allocations SET amount_cents=-40 WHERE transaction_id=?", split)
	e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,NULL,-60)", split)
	body := pendingRuleBody(1, group)
	body["approve"] = true
	status(t, e.req(t, 3, fmt.Sprintf("/api/transactions/%d", source), "PUT", body), 403)
	counts := pendingRuleCounts(t, e, source, body)
	if counts.Applied != 105 || counts.Conflicts != 0 {
		t.Fatal(counts)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 107 {
		t.Fatal("rule did not accept categorized matches")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='rule_applied'") != 105 {
		t.Fatal("missing application audits")
	}
	for i, id := range targets {
		wantGroup := group
		if i == 0 {
			wantGroup = existingGroup
		}
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=? AND amount_cents=-100 AND date='2026-10-21' AND source_description='Synthetic' AND source_amount=-100 AND version=2 AND review_state='approved' AND reviewed_at IS NULL AND reviewed_by IS NULL AND spending_group_id=?", id, wantGroup) != 1 {
			t.Fatalf("changed immutable/review fields for %d", id)
		}
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id=1 AND amount_cents=-100", id) != 1 {
			t.Fatal("category/amount not preserved")
		}
	}
	var note string
	e.a.DB.QueryRow("SELECT note FROM allocations WHERE transaction_id=?", targets[0]).Scan(&note)
	if note != "keep my note" {
		t.Fatal("rule erased an allocation note")
	}
	for _, id := range []int64{categorized, approved, transfer, credit, private, unrelated, split} {
		if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 1 {
			t.Fatalf("rule changed protected transaction %d", id)
		}
	}
	if queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", categorized) != category {
		t.Fatal("manual category replaced")
	}
	fresh := merchantTransaction(t, e, 1, -100, nil)
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", source), "PUT", body), 409)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", fresh) != 1 {
		t.Fatal("stale save applied rule")
	}
	// An already-open review selection cannot approve newly classified details unnoticed.
	status(t, e.req(t, 1, "/api/review", "POST", map[string]any{"items": []map[string]int64{{"id": targets[0], "version": 1}}}), 409)
}
func TestPendingRuleConflictsAndAtomicRollback(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	source := merchantTransaction(t, e, 1, -100, nil)
	target := merchantTransaction(t, e, 1, -100, nil)
	createTestRule(t, e, ruleBody(1, 2, "merchant", "debit", nil, 0, true, 0))
	body := pendingRuleBody(1, group)
	counts := pendingRuleCounts(t, e, source, body)
	if counts.Applied != 0 || counts.Conflicts != 1 {
		t.Fatal(counts)
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 1 {
		t.Fatal("conflicting rules assigned category")
	}
	e.a.DB.Exec("DELETE FROM rules WHERE pattern='merchant'")
	// A downstream failure must undo the source edit, rule update, target category and audit.
	_, err := e.a.DB.Exec("CREATE TRIGGER block_application BEFORE INSERT ON audit WHEN NEW.action='rule_applied' BEGIN SELECT RAISE(ABORT,'synthetic application failure'); END")
	if err != nil {
		t.Fatal(err)
	}
	body["version"] = 2
	body["allocations"] = []Allocation{{CategoryID: nil, Amount: -100, Note: ""}}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", source), "PUT", body), 400)
	// Use a valid category with an altered pattern so the failing save tests every write.
	cat := int64(1)
	body["allocations"] = []Allocation{{CategoryID: &cat, Amount: -100, Note: ""}}
	body["description"] = "Synthetic merchant edited reference"
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", source), "PUT", body), 500)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", source) != 2 || queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 1 || queryInt(e.a.DB, "SELECT version FROM rules WHERE pattern='Synthetic merchant'") != 1 {
		t.Fatal("failed application partially committed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id IS NULL", target) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='rule_applied'") != 0 {
		t.Fatal("failure left classification or audit")
	}
	e.a.DB.Exec("DROP TRIGGER block_application")
	counts = pendingRuleCounts(t, e, source, body)
	if counts.Applied != 1 || counts.Conflicts != 0 {
		t.Fatal(counts)
	}
	// Saving/updating the same rule never reclassifies an already categorized match.
	body["version"] = 3
	counts = pendingRuleCounts(t, e, source, body)
	if counts.Applied != 0 || queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 2 {
		t.Fatal("repeat save changed a categorized transaction")
	}
}
