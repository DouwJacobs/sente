package app

import (
	"fmt"
	"testing"
)

func TestMCPMerchantConsentProposalsAndScope(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	if _, failed := mcpCall(t, e, token, "list_merchants", nil); !failed {
		t.Fatal("merchant reads silently granted")
	}
	change := map[string]any{"operation": "save_merchant", "merchant": map[string]any{"account_id": 0, "name": "Agent Market", "logo_data": syntheticMerchantLogo(t, 8)}}
	if _, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("legacy permissions gained merchant writes")
	}
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadMerchants: true, Capabilities: mcpCapabilities{ManageMerchants: true, ManageMerchantRules: true, AssignMerchants: true}, Automatic: mcpCapabilities{ManageMerchants: true, ManageMerchantRules: true, AssignMerchants: true}}
	setTestMCPPermissions(t, e, token, p)
	apply := func(change map[string]any) map[string]any {
		t.Helper()
		v, failed := mcpCall(t, e, token, "prepare_change", change)
		if failed {
			t.Fatal(v)
		}

		result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": v["proposal_id"]})
		if failed {
			t.Fatal(result)
		}
		return result
	}
	result := apply(change)
	mid := num(result["id"])
	if queryInt(e.a.DB, "SELECT length(logo_data) FROM merchants WHERE id=?", mid) == 0 {
		t.Fatal("logo lost")
	}
	v, failed := mcpCall(t, e, token, "list_merchants", map[string]any{})
	if failed {
		t.Fatal(v)
	}
	item := v["items"].([]any)[0].(map[string]any)
	if _, ok := item["logo_data"]; ok {
		t.Fatal("unsolicited logo")
	}
	v, failed = mcpCall(t, e, token, "list_merchants", map[string]any{"include_logo": true})
	if failed || v["items"].([]any)[0].(map[string]any)["logo_data"] == nil {
		t.Fatal(v)
	}
	result = apply(map[string]any{"operation": "save_merchant_rule", "merchant_rule": map[string]any{"account_id": 0, "merchant_id": mid, "pattern": "seed", "direction": "any", "enabled": true}})
	rule := num(result["id"])
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	workflowExec(t, e, "UPDATE transactions SET description='seed market' WHERE id=?", id)
	if v, failed = mcpCall(t, e, token, "preview_merchant_rule", map[string]any{"id": rule}); failed || num(v["total"]) != 1 {
		t.Fatal(v)
	}
	apply(map[string]any{"operation": "assign_merchants", "merchant_items": []any{map[string]any{"id": id, "version": 1, "merchant_id": mid}}})
	if queryInt(e.a.DB, "SELECT merchant_id FROM transactions WHERE id=?", id) != mid || queryInt(e.a.DB, "SELECT amount_cents FROM transactions WHERE id=?", id) != -100 {
		t.Fatal("assignment lost or money changed")
	}
	change = map[string]any{"operation": "save_merchant", "id": mid, "merchant": map[string]any{"account_id": 0, "name": "Changed", "version": 2, "logo_data": ""}}
	v, failed = mcpCall(t, e, token, "prepare_change", change)
	if failed {
		t.Fatal(v)
	}
	p.Capabilities.ManageMerchants = false
	p.Automatic.ManageMerchants = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": v["proposal_id"]}); !failed {
		t.Fatal("revoked capability applied")
	}
	p.Capabilities.ManageMerchants = true
	p.Constraints.AccountIDs = []int64{1}
	setTestMCPPermissions(t, e, token, p)
	if _, failed = mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("account-restricted connection edited global merchant")
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/merchant-rules/%d", rule), "PUT", map[string]any{"account_id": 0, "merchant_id": mid, "pattern": "seed", "direction": "any", "enabled": true, "version": 1}), 200)
	delChange := map[string]any{"operation": "delete_merchant_rule", "id": rule, "version": 2}
	p.Capabilities.DeleteMerchantRule = false
	p.Constraints.AccountIDs = nil
	setTestMCPPermissions(t, e, token, p)
	if _, failed = mcpCall(t, e, token, "prepare_change", delChange); !failed {
		t.Fatal("delete_merchant_rule allowed without capability")
	}
	p.Capabilities.DeleteMerchantRule = true
	p.Automatic.DeleteMerchantRule = true
	setTestMCPPermissions(t, e, token, p)
	v, failed = mcpCall(t, e, token, "prepare_change", delChange)
	if failed {
		t.Fatal(v)
	}
	if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": v["proposal_id"]}); failed {
		t.Fatal("apply delete_merchant_rule failed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE id=?", rule) != 0 {
		t.Fatal("merchant rule was not deleted")
	}
}
func TestDashboardIncomeAllocationScope(t *testing.T) {
	e := setup(t)
	salary := seedTransaction(t, e, 1, 1000, "2026-10-21", workflowPtr(int64(2)))
	seedTransaction(t, e, 1, 200, "2026-10-22", workflowPtr(int64(1)))
	seedTransaction(t, e, 2, 9000, "2026-10-22", workflowPtr(int64(2)))
	w := e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, w, 200)
	v := workflowJSON(t, w.Body.Bytes())
	if num(v["income_cents"]) != 1000 || len(v["income_categories"].([]any)) != 1 {
		t.Fatal(v)
	}
	w = e.req(t, 1, "/api/transactions?dashboard_income=1&period=1&category=2", "GET", nil)
	status(t, w, 200)
	v = workflowJSON(t, w.Body.Bytes())
	items := v["items"].([]any)
	if len(items) != 1 || num(items[0].(map[string]any)["id"]) != salary || num(items[0].(map[string]any)["matched_income_cents"]) != 1000 {
		t.Fatal(v)
	}
	status(t, e.req(t, 3, "/api/transactions?dashboard_income=1&period=1", "GET", nil), 403)
}
