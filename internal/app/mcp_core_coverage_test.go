package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPCoreReadCoverage(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	if _, err := e.a.DB.Exec(`UPDATE categories SET archived=1 WHERE id=1; INSERT INTO group_targets(period_id,spending_group_id,category_id,amount_cents,included,carry_forward) VALUES(1,1,1,12345,1,1); INSERT INTO budget_groups(period_id,spending_group_id) VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	token := mcpToken(t, e, 1, false)
	categories, failed := mcpCall(t, e, token, "list_categories", map[string]any{"filters": map[string]string{"id": "1"}})
	if failed || categories["items"].([]any)[0].(map[string]any)["archived"] != float64(1) {
		t.Fatal(categories)
	}
	limits, failed := mcpCall(t, e, token, "get_budget_limits", map[string]any{"period_id": 1, "filters": map[string]string{"group": "1", "budget_only": "1"}})
	if failed {
		t.Fatal(limits)
	}
	row := limits["items"].([]any)[0].(map[string]any)
	if row["amount_cents"] != float64(12345) || row["carry_forward"] != float64(1) {
		t.Fatal(limits)
	}
	summary, failed := mcpCall(t, e, token, "get_budget_summary", map[string]any{"filters": map[string]string{"period": "1"}})
	if failed || summary["spending_groups"] == nil || summary["group_total"] == nil || summary["spending_breakdown"] != nil {
		t.Fatal(summary)
	}
	trends, failed := mcpCall(t, e, token, "get_budget_trends", map[string]any{"filters": map[string]string{"period": "1", "count": "1"}})
	if failed || trends["periods"] == nil || trends["entries"] == nil {
		t.Fatal(trends)
	}
	health, failed := mcpCall(t, e, token, "get_account_import_health", map[string]any{})
	if failed {
		t.Fatal(health)
	}
	body, _ := json.Marshal(health)
	if strings.Contains(string(body), "bank_id") || strings.Contains(string(body), "Private merchant") {
		t.Fatal(string(body))
	}
	for _, item := range health["items"].([]any) {
		r := item.(map[string]any)
		if !strings.HasPrefix(r["name"].(string), "Account ") || r["possible_gap"] != true {
			t.Fatal(r)
		}
	}
	restricted := mcpToken(t, e, 3, false)
	if _, failed = mcpCall(t, e, restricted, "get_budget_trends", map[string]any{}); !failed {
		t.Fatal("non-member read report")
	}
}

func TestMCPCoreMetadataPreservedAndPrivate(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	if _, err := e.a.DB.Exec(`INSERT INTO merchants(id,account_id,name) VALUES(1,1,'Private label'); INSERT INTO tags(id,account_id,name) VALUES(1,1,'Private tag'); UPDATE transactions SET note='Private transaction note',merchant_id=1 WHERE id=1; INSERT INTO transaction_tags VALUES(1,1)`); err != nil {
		t.Fatal(err)
	}
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, id)
	if result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(result)
	}
	var note string
	var merchant int
	if err := e.a.DB.QueryRow("SELECT note,merchant_id FROM transactions WHERE id=1").Scan(&note, &merchant); err != nil || note != "Private transaction note" || merchant != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_tags WHERE transaction_id=1 AND tag_id=1") != 1 {
		t.Fatal(note, merchant, err)
	}
	result, failed := mcpCall(t, e, token, "list_transactions", map[string]any{"filters": map[string]string{"id": "1"}})
	body, _ := json.Marshal(result)
	if failed || strings.Contains(string(body), "Private label") || strings.Contains(string(body), "Private tag") || strings.Contains(string(body), "Private transaction note") {
		t.Fatal(string(body))
	}
}

func TestMCPCoreReadsRespectSelectedAccounts(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, false)
	setTestMCPPermissions(t, e, token, mcpPermissions{SchemaVersion: 1, Preset: "review", Constraints: mcpConstraints{SelectedAccounts: true, AccountIDs: []int64{1}}})
	health, failed := mcpCall(t, e, token, "get_account_import_health", map[string]any{})
	if failed || len(health["items"].([]any)) != 1 || health["items"].([]any)[0].(map[string]any)["id"] != float64(1) {
		t.Fatal(health)
	}
	if result, failed := mcpCall(t, e, token, "get_budget_trends", map[string]any{"filters": map[string]string{"period": "1", "account": "2"}}); !failed {
		t.Fatal("out-of-scope account report", result)
	}
	if _, err := e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	health, failed = mcpCall(t, e, token, "get_account_import_health", map[string]any{})
	if failed || len(health["items"].([]any)) != 0 {
		t.Fatal(health)
	}
}
