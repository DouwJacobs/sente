package app

import (
	"encoding/json"
	"testing"
)

func TestMCPBatchProposalApprovalAtomicIsolation(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	ids := []string{}
	for _, name := range []string{"Batch one", "Batch two"} {
		ids = append(ids, mcpPrepare(t, e, token, map[string]any{"operation": "create_category", "category": map[string]any{"name": name, "kind": "expense"}}))
	}
	path := "/api/mcp/proposals/batch"
	status(t, e.req(t, 2, path, "POST", map[string]any{"approve": true, "proposal_ids": ids}), 409)
	status(t, e.req(t, 1, path, "POST", map[string]any{"approve": true, "proposal_ids": []string{ids[0], ids[0]}}), 400)
	e.a.DB.Exec("UPDATE mcp_proposals SET expires_at=0 WHERE id=?", ids[1])
	status(t, e.req(t, 1, path, "POST", map[string]any{"approve": true, "proposal_ids": ids}), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE status='approved'") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='approved'") != 0 {
		t.Fatal("partial batch approval")
	}
	e.a.DB.Exec("UPDATE mcp_proposals SET expires_at=9999999999 WHERE id=?", ids[1])
	status(t, e.req(t, 1, path, "POST", map[string]any{"approve": true, "proposal_ids": ids}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE status='approved'") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE name LIKE 'Batch %'") != 0 {
		t.Fatal("approval applied financial effects")
	}
}
func TestMCPAutomaticApprovalTypesMixedEffectsAndRevocation(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	p := legacyMCPPermissions(true)
	p.Preset = "custom"
	p.Automatic.AssignMissing = true
	p.Automatic.CreateRule = true
	setTestMCPPermissions(t, e, token, p)
	v, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}}})
	if failed || v["status"] != "approved" {
		t.Fatal(v)
	}
	id := v["proposal_id"].(string)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NOT NULL") != 0 {
		t.Fatal("auto approval wrote finances")
	}
	mixed, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 2, "version": 1, "category_id": 1, "description": "Edited merchant"}}})
	if failed || mixed["status"] != "pending" {
		t.Fatal("mixed bypass", mixed)
	}
	rule, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "save_rule", "rule": map[string]any{"account_id": 1, "pattern": "Shop", "category_id": 1}})
	if failed || rule["status"] != "approved" {
		t.Fatal(rule)
	}
	p.Automatic.AssignMissing = false
	setTestMCPPermissions(t, e, token, p)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("removed auto approval ignored", v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NOT NULL") != 0 {
		t.Fatal("revoked auto approval changed finances")
	}
	p.Automatic.AssignMissing = true
	setTestMCPPermissions(t, e, token, p)
	for i := 0; i < 2; i++ {
		if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
			t.Fatal(v)
		}
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 {
		t.Fatal("replay changed data")
	}
	var raw string
	e.a.DB.QueryRow("SELECT payload FROM mcp_proposals WHERE id=?", id).Scan(&raw)
	var exact mcpExactChange
	json.Unmarshal([]byte(raw), &exact)
	if !exact.AutomaticApproval {
		t.Fatal("approval basis not retained")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='automatically_approved'") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='applied'") != 1 {
		t.Fatal("missing durable auto audit")
	}
}
func TestMCPAutomaticApprovalRequiresExplicitMatchingGrants(t *testing.T) {
	e := setup(t)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Automatic: mcpCapabilities{DeleteRule: true}}
	if _, err := e.a.validateMCPPermissions(e.a.DB, e.owner, p); err == nil {
		t.Fatal("automatic grant without capability")
	}
	p = legacyMCPPermissions(true)
	if p.Automatic != (mcpCapabilities{}) {
		t.Fatal("legacy automatically approved")
	}
}
