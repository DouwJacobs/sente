package app

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestMCPRuleImpactUsesPrecedenceAndAuthorizedCounts(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	// Shared descriptions intentionally match a private row too.
	if _, err := e.a.DB.Exec("UPDATE transactions SET description='Synthetic Market 12345678901 owner@example.test'; INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction) VALUES(1,1,'Synthetic',2,0,'any')"); err != nil {
		t.Fatal(err)
	}
	token := mcpToken(t, e, 3, false)
	args := map[string]any{"rule": map[string]any{"account_id": 1, "pattern": "Market", "category_id": 1, "direction": "debit", "enabled": true}, "sample_limit": 1}
	v, failed := mcpCall(t, e, token, "preview_categorization_rule", args)
	if failed || v["description_match_count"] != float64(2) || v["future_rule_match_count"] != float64(2) || v["conflict_count"] != float64(2) || v["future_classified_count"] != float64(0) || v["existing_eligible_count"] != float64(0) || len(v["samples"].([]any)) != 1 {
		t.Fatal(v)
	}
	b, _ := json.Marshal(v)
	for _, secret := range []string{"12345678901", "owner@example.test", "secret note", "provenance", "source_description", "bank_id"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("preview leak", secret, string(b))
		}
	}
	if v["standalone_rule_sweeps_existing"] != false {
		t.Fatal(v)
	}
	if _, err := e.a.DB.Exec("DELETE FROM rules"); err != nil {
		t.Fatal(err)
	}
	editor := mcpToken(t, e, 1, false)
	v, failed = mcpCall(t, e, editor, "preview_categorization_rule", args)
	if failed || v["existing_eligible_count"] != float64(2) || v["future_classified_count"] != float64(2) {
		t.Fatal(v)
	}
	// A split with a missing category is uncategorized, but not eligible for a retrospective unsplit rule fill.
	if _, err := e.a.DB.Exec("UPDATE allocations SET amount_cents=-51 WHERE transaction_id=1; INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-50)"); err != nil {
		t.Fatal(err)
	}
	v, failed = mcpCall(t, e, editor, "preview_categorization_rule", args)
	if failed || v["uncategorized_count"] != float64(2) || v["existing_eligible_count"] != float64(1) {
		t.Fatal(v)
	}
	args["account_ids"] = []int64{1, 2}
	if v, failed := mcpCall(t, e, token, "preview_categorization_rule", args); !failed {
		t.Fatal("private scope accepted", v)
	}
	args["account_ids"] = []int64{1, 1}
	if v, failed := mcpCall(t, e, token, "preview_categorization_rule", args); !failed {
		t.Fatal("duplicate accounts accepted", v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 || queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 1 {
		t.Fatal("preview wrote ledger or rules")
	}
}

func TestMCPNarrowCategoryAssignmentsPreserveSplitsAndReplay(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	if _, err := e.a.DB.Exec("UPDATE allocations SET amount_cents=-51 WHERE transaction_id=1; INSERT INTO allocations(transaction_id,amount_cents,note) VALUES(1,-50,'second hidden note')"); err != nil {
		t.Fatal(err)
	}
	first := queryInt(e.a.DB, "SELECT MIN(id) FROM allocations WHERE transaction_id=1")
	second := queryInt(e.a.DB, "SELECT MAX(id) FROM allocations WHERE transaction_id=1")
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 1, "version": 1, "category_id": 1, "allocation_id": first}, map[string]any{"id": 1, "version": 1, "category_id": 2, "allocation_id": second}, map[string]any{"id": 2, "version": 1, "category_id": 1}}})
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NOT NULL") != 0 {
		t.Fatal("prepared categories changed ledger")
	}
	mcpApprove(t, e, 1, id)
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || result["updated"] != float64(2) || result["assigned_allocations"] != float64(3) {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=1 AND id IN (?,?)", first, second) != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=1 AND note IN ('secret note','second hidden note')") != 2 || queryInt(e.a.DB, "SELECT amount_cents FROM transactions WHERE id=1") != -101 {
		t.Fatal("assignment replaced allocations or financial content")
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=1 AND transaction_id IN (1,2) AND transaction_version=2") != 2 {
		t.Fatal("version/seen mismatch")
	}
	result, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || result["updated"] != float64(2) || queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 {
		t.Fatal("replay changed categories", result)
	}
	if result, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 1, "version": 2, "category_id": 1, "allocation_id": first}}}); !failed {
		t.Fatal("recategorization accepted", result)
	}
}

func TestMCPNarrowCategoriesRecheckAtomicPreconditions(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}, map[string]any{"id": 2, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, id)
	if _, err := e.a.DB.Exec("UPDATE transactions SET is_transfer=1 WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if !failed || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=1 AND category_id IS NOT NULL") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='approved'", id) != 1 {
		t.Fatal("failed batch was not atomic", v)
	}
	for _, assignment := range []map[string]any{{"id": 1, "version": 1, "category_id": 1, "allocation_id": 999}, {"id": 2, "version": 1, "category_id": 1}} {
		if v, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "assign_categories", "categorizations": []any{assignment}}); !failed {
			t.Fatal("invalid category proposal", v)
		}
	}
}

func TestMCPCategorizationContextHistoryIsolationAndParentCounts(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	if _, err := e.a.DB.Exec("UPDATE transactions SET description='Synthetic Merchant'; UPDATE categories SET name='Groceries 12345678901' WHERE id=1; INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction) VALUES(1,1,'merchant',1,0,'debit')"); err != nil {
		t.Fatal(err)
	}
	cat := int64(1)
	history := seedTransaction(t, e, 1, -100, "2026-10-20", &cat)
	privateCategory := int64(2)
	private := seedTransaction(t, e, 2, -200, "2026-10-20", &privateCategory)
	if _, err := e.a.DB.Exec("UPDATE transactions SET description='  SYNTHETIC MERCHANT  ' WHERE id IN (?,?); UPDATE allocations SET amount_cents=-50 WHERE transaction_id=?; INSERT INTO allocations(transaction_id,category_id,amount_cents,note) VALUES(?,1,-50,'hidden history note')", history, private, history, history); err != nil {
		t.Fatal(err)
	}
	token := mcpToken(t, e, 3, false)
	v, failed := mcpCall(t, e, token, "get_categorization_context", map[string]any{"transaction_ids": []int64{1, 2}})
	if failed || v["similarity"] != "same_normalized_description" {
		t.Fatal(v)
	}
	items := v["items"].([]any)
	if len(items) != 2 {
		t.Fatal(v)
	}
	for _, entry := range items {
		item := entry.(map[string]any)
		if item["historical_transaction_count"] != float64(1) || item["historical_category_total"] != float64(1) || item["matching_rule_count"] != float64(1) || item["conflict"] != false || item["suggested_category_id"] != float64(1) {
			t.Fatal(item)
		}
		categories := item["historical_categories"].([]any)
		if len(categories) != 1 || categories[0].(map[string]any)["transaction_count"] != float64(1) {
			t.Fatal("split double-counted", item)
		}
	}
	b, _ := json.Marshal(v)
	for _, secret := range []string{"12345678901", "hidden history note", "secret note", "source_description", "provenance"} {
		if strings.Contains(string(b), secret) {
			t.Fatal("context disclosure", secret, string(b))
		}
	}
	owner := mcpToken(t, e, 1, false)
	v, failed = mcpCall(t, e, owner, "get_categorization_context", map[string]any{"transaction_ids": []int64{1, 2}})
	if failed || v["items"].([]any)[0].(map[string]any)["historical_transaction_count"] != float64(3) {
		t.Fatal("authorized private history missing", v)
	}
	for _, ids := range [][]int64{{1, 1}, {1, 3}, {0}, {}} {
		if v, failed := mcpCall(t, e, token, "get_categorization_context", map[string]any{"transaction_ids": ids}); !failed {
			t.Fatal("invalid/inaccessible context", v)
		}
	}
	if _, err := e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2"); err != nil {
		t.Fatal(err)
	}
	v, failed = mcpCall(t, e, owner, "get_categorization_context", map[string]any{"transaction_ids": []int64{1, 2}})
	if failed || v["items"].([]any)[0].(map[string]any)["historical_transaction_count"] != float64(1) {
		t.Fatal("revoked history leaked", v)
	}
}

func TestMCPRulePreviewBoundAndDisabledDraft(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, false)
	args := map[string]any{"rule": map[string]any{"account_id": 1, "pattern": "Market", "category_id": 1, "enabled": false}}
	v, failed := mcpCall(t, e, token, "preview_categorization_rule", args)
	if failed || v["description_match_count"] != float64(1) || v["future_rule_match_count"] != float64(0) || v["existing_eligible_count"] != float64(0) {
		t.Fatal(v)
	}
	if _, err := e.a.DB.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<10001) INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) SELECT 1,'2026-10-20',-100,'Bounded merchant','2026-10-20',-100,'Synthetic','{}' FROM n`); err != nil {
		t.Fatal(err)
	}
	args = map[string]any{"rule": map[string]any{"account_id": 1, "pattern": "Bounded", "category_id": 1}}
	if v, failed := mcpCall(t, e, token, "preview_categorization_rule", args); !failed || !strings.Contains(v["error"].(string), "10000") {
		t.Fatal(v)
	}
}

func TestMCPRuleDeletionAuditAndToolAnnotations(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "save_rule", "rule": map[string]any{"account_id": 1, "pattern": "Audit merchant", "category_id": 1, "direction": "debit"}})
	mcpApprove(t, e, 1, id)
	v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(v)
	}
	ruleID := int64(v["id"].(float64))
	proposal := mcpPrepare(t, e, token, map[string]any{"operation": "delete_rule", "id": ruleID, "version": 1})
	mcpApprove(t, e, 1, proposal)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); failed {
		t.Fatal(v)
	}
	var raw string
	if err := e.a.DB.QueryRow("SELECT details FROM audit WHERE entity='mcp' AND action='applied' AND json_extract(details,'$.proposal_id')=?", proposal).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var evidence map[string]any
	if err := json.Unmarshal([]byte(raw), &evidence); err != nil {
		t.Fatal(err)
	}
	entities := evidence["entities"].([]any)
	deleted := entities[0].(map[string]any)
	before := deleted["before"].([]any)[0].(map[string]any)
	if len(entities) != 1 || deleted["entity"] != "rule" || deleted["id"] != float64(ruleID) || deleted["after"] != nil || before["pattern"] != "Audit merchant" || before["account_id"] != float64(1) || before["version"] != float64(1) || evidence["connection_id"] == nil {
		t.Fatal(evidence)
	}
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); failed {
		t.Fatal("deletion replay failed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='mcp' AND action='applied' AND json_extract(details,'$.proposal_id')=?", proposal) != 1 {
		t.Fatal("deletion replay duplicated audit")
	}
	tools := mcpRPC(t, e, token, "tools/list", map[string]any{})["tools"].([]any)
	for _, entry := range tools {
		tool := entry.(map[string]any)
		annotations := tool["annotations"].(map[string]any)
		if annotations["openWorldHint"] != false {
			t.Fatal(tool)
		}
		switch tool["name"] {
		case "prepare_change":
			if annotations["readOnlyHint"] != false || annotations["destructiveHint"] != false || annotations["idempotentHint"] != false {
				t.Fatal(tool)
			}
		case "apply_change":
			if annotations["readOnlyHint"] != false || annotations["destructiveHint"] != true || annotations["idempotentHint"] != true {
				t.Fatal(tool)
			}
		default:
			if annotations["readOnlyHint"] != true {
				t.Fatal(tool)
			}
		}
	}
}
