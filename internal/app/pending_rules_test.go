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
func TestTransactionRuleSavingIsFutureOnly(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	source := merchantTransaction(t, e, 1, -100, nil)
	target := merchantTransaction(t, e, 1, -100, nil)
	body := pendingRuleBody(1, group)
	counts := pendingRuleCounts(t, e, source, body)
	if counts.Applied != 0 || counts.Conflicts != 0 || counts.RuleID == 0 || counts.RuleVersion != 1 {
		t.Fatal(counts)
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id IS NULL", target) != 1 {
		t.Fatal("saving a rule changed history")
	}
	if queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", source) != 1 {
		t.Fatal("source edit was lost")
	}
}
