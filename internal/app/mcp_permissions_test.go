package app

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func setTestMCPPermissions(t *testing.T, e *testEnv, token string, p mcpPermissions) {
	t.Helper()
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.a.DB.Exec("UPDATE mcp_tokens SET permissions=?,permission_version=permission_version+1 WHERE id=?", string(raw), identity.TokenID); err != nil {
		t.Fatal(err)
	}
}
func TestMCPRestrictedCapabilitiesBlockLegacyBypassesAndReplay(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	permissions := mcpPermissions{SchemaVersion: 1, Preset: "categorization", Capabilities: mcpCapabilities{AssignMissing: true, CreateRule: true}, Constraints: mcpConstraints{MissingOnly: true, ConstrainedRules: true, AccountIDs: []int64{1}}}
	setTestMCPPermissions(t, e, token, permissions)
	for _, change := range []map[string]any{
		{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1}}},
		{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "amount_cents": -102, "category_id": 1}}},
		{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "description": "Edited", "category_id": 1}}},
		{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "allocations": []any{map[string]any{"category_id": 1, "amount_cents": -101, "note": "changed"}}}}},
		{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 3, "version": 1, "category_id": 1}}},
		{"operation": "create_category", "category": map[string]any{"name": "Denied category", "kind": "expense"}},
		{"operation": "delete_rule", "id": 1, "version": 1},
		{"operation": "update_budget", "id": 1, "budget": map[string]any{"version": 1, "targets": []any{}, "merge": true}},
		{"operation": "save_rule", "id": -1, "rule": map[string]any{"account_id": 0, "pattern": "Merchant", "category_id": 1}},
		{"operation": "save_rule", "rule": map[string]any{"account_id": 2, "pattern": "Merchant", "category_id": 1}},
	} {
		if v, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
			t.Fatal("restricted bypass", change, v)
		}
	}
	// The generic legacy tool may fill a missing category, but must obey the same constraints.
	proposal := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, proposal)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); failed {
		t.Fatal(v)
	}
	if v, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 2, "category_id": 2}}}); !failed {
		t.Fatal("recategorization bypass", v)
	}
	permissions.Capabilities.AssignMissing = false
	setTestMCPPermissions(t, e, token, permissions)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); !failed {
		t.Fatal("replay bypassed reduced capabilities", v)
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 {
		t.Fatal("replay changed financial data")
	}
}
func TestMCPGrantChangesBlockAppliedReplay(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "assign_categories", "categorizations": []any{map[string]any{"id": 3, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, id)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if _, err := e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2"); err != nil {
		t.Fatal(err)
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("applied replay ignored grant revocation", v)
	}
}
func TestMCPPermissionsMigrationPreservesLegacyGrants(t *testing.T) {
	e := setup(t)
	read := mcpToken(t, e, 1, false)
	write := mcpToken(t, e, 1, true)
	removePostBaselineFixtureTables(t, e.a.DB)
	if _, err := e.a.DB.Exec("ALTER TABLE mcp_tokens DROP COLUMN permission_version; ALTER TABLE mcp_tokens DROP COLUMN permissions; DELETE FROM migrations WHERE version>=13; INSERT OR IGNORE INTO migrations VALUES(12)"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		token string
		write bool
	}{{read, false}, {write, true}} {
		identity, err := e.a.mcpIdentity(test.token)
		if err != nil {
			t.Fatal(err)
		}
		p, err := readMCPPermissions(e.a.DB, identity)
		if err != nil {
			t.Fatal(err)
		}
		expected := legacyMCPPermissions(test.write)
		actualRaw, _ := json.Marshal(p)
		expectedRaw, _ := json.Marshal(expected)
		if string(actualRaw) != string(expectedRaw) || p.Capabilities.ChangeSeen {
			t.Fatal("migration changed grant", p)
		}
	}
}

func TestMCPScopedReadsAndAllAggregates(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	p := legacyMCPPermissions(true)
	p.Preset = "custom"
	p.Constraints.AccountIDs = []int64{2}
	setTestMCPPermissions(t, e, token, p)
	for _, name := range []string{"list_accounts", "list_transactions", "get_transaction_review_queue"} {
		args := map[string]any{}
		if name == "get_transaction_review_queue" {
			args["selector"] = "needs_category"
		}
		v, failed := mcpCall(t, e, token, name, args)
		if failed {
			t.Fatal(name, v)
		}
		rows := v["items"].([]any)
		if len(rows) != 1 {
			t.Fatal(name, v)
		}
		id := num(rows[0].(map[string]any)["id"])
		if name == "list_accounts" && id != 2 || name != "list_accounts" && id != 3 {
			t.Fatal(name, v)
		}
	}
	if v, failed := mcpCall(t, e, token, "get_categorization_context", map[string]any{"transaction_ids": []int64{1, 3}}); !failed {
		t.Fatal("partial context leak", v)
	}
	if v, failed := mcpCall(t, e, token, "preview_categorization_rule", map[string]any{"rule": ruleInput{AccountID: 1, Pattern: "Market", CategoryID: 1}}); !failed {
		t.Fatal("rule scope bypass", v)
	}
	if _, err := e.a.DB.Exec("UPDATE allocations SET category_id=1"); err != nil {
		t.Fatal(err)
	}
	v, failed := mcpCall(t, e, token, "list_categories", map[string]any{})
	if failed {
		t.Fatal(v)
	}
	for _, item := range v["items"].([]any) {
		row := item.(map[string]any)
		if num(row["id"]) == 1 && num(row["usage_count"]) != 1 {
			t.Fatal("category count leak", v)
		}
	}
	v, failed = mcpCall(t, e, token, "get_budget_summary", map[string]any{"filters": map[string]string{"period": "1"}})
	if failed {
		t.Fatal(v)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "Market") || num(v["spent_cents"]) != 0 || len(v["balances"].([]any)) != 0 {
		t.Fatal("shared aggregate leak", v)
	}
	if v, failed := mcpCall(t, e, token, "get_budget_summary", map[string]any{"filters": map[string]string{"period": "1", "account": "1"}}); !failed {
		t.Fatal("explicit account bypass", v)
	}
}
func TestMCPBrowserPermissionConsentAndScopeExpansion(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	path := "/api/mcp/connections/" + strconvID(identity.TokenID) + "/permissions"
	p := mcpPermissions{SchemaVersion: 1, Preset: "categorization", Constraints: mcpConstraints{AccountIDs: []int64{1}}}
	status(t, e.req(t, 1, path, "PUT", map[string]any{"version": 1, "permissions": p}), 400)
	status(t, e.req(t, 2, path, "PUT", map[string]any{"version": 1, "permissions": p, "consent": true}), 404)
	status(t, e.req(t, 1, path, "PUT", map[string]any{"version": 1, "permissions": p, "consent": true}), 200)
	actual, err := readMCPPermissions(e.a.DB, identity)
	if err != nil || !actual.Capabilities.AssignMissing || actual.Capabilities.FinancialEdit || !actual.Constraints.ConstrainedRules {
		t.Fatal(actual, err)
	}
	p.Preset = "finance"
	p.Constraints.AccountIDs = nil
	status(t, e.req(t, 1, path, "PUT", map[string]any{"version": 1, "permissions": p, "consent": true}), 409)
	status(t, e.req(t, 1, path, "PUT", map[string]any{"version": 2, "permissions": p, "consent": true}), 200)
	actual, err = readMCPPermissions(e.a.DB, identity)
	if err != nil || !actual.Capabilities.FinancialEdit || actual.Capabilities.ChangeSeen {
		t.Fatal(actual, err)
	}
	read := mcpToken(t, e, 1, false)
	rid, _ := e.a.mcpIdentity(read)
	status(t, e.req(t, 1, "/api/mcp/connections/"+strconvID(rid.TokenID)+"/permissions", "PUT", map[string]any{"version": 1, "permissions": p, "consent": true}), 403)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='mcp' AND action='permissions_changed'") != 2 {
		t.Fatal("permission audit")
	}
}
func TestMCPOAuthGranularConsentAndValidation(t *testing.T) {
	e := setup(t)
	client, request := oauthStart(t, e)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "GET", nil), 200)
	p := mcpPermissions{SchemaVersion: 1, Preset: "categorization"}
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true, "permissions": p}), 400)
	p.Constraints.AccountIDs = []int64{1}
	w := e.req(t, 1, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true, "permissions": p})
	status(t, w, 200)
	uri, _ := url.Parse(oauthJSON(t, w)["redirect"].(string))
	w = oauthExchange(e, client, uri.Query().Get("code"), testVerifier)
	status(t, w, 200)
	identity, err := e.a.mcpIdentity(oauthJSON(t, w)["access_token"].(string))
	if err != nil {
		t.Fatal(err)
	}
	actual, err := readMCPPermissions(e.a.DB, identity)
	if err != nil || actual.Preset != "categorization" || !actual.Capabilities.CreateRule || actual.Capabilities.DeleteRule {
		t.Fatal(actual, err)
	}
	for _, invalid := range []mcpPermissions{
		{SchemaVersion: 1, Preset: "custom", Constraints: mcpConstraints{SelectedAccounts: true}},
		{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{FinancialEdit: true}, Constraints: mcpConstraints{MissingOnly: true}},
		{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{UpdateRule: true}, Constraints: mcpConstraints{ConstrainedRules: true}},
		{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{UpdateBudget: true}, Constraints: mcpConstraints{AccountIDs: []int64{1}}},
	} {
		if _, err := e.a.validateMCPPermissions(e.a.DB, e.owner, invalid); err == nil {
			t.Fatal("accepted invalid constraints", invalid)
		}
	}
}
func strconvID(id int64) string { return fmt.Sprint(id) }
func TestMCPSeenCustomCapabilityAtomicApprovalAndReplay(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 3, true)
	change := map[string]any{"operation": "set_seen", "seen": true, "seen_items": []any{map[string]any{"id": 1, "version": 1}, map[string]any{"id": 2, "version": 1}}}
	if v, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("legacy gained seen grant", v)
	}
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{ChangeSeen: true}, Constraints: mcpConstraints{AccountIDs: []int64{1}}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, change)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen") != 0 {
		t.Fatal("prepare changed seen")
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("approval bypass", v)
	}
	mcpApprove(t, e, 3, id)
	for i := 0; i < 2; i++ {
		if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
			t.Fatal(v)
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=3") != 2 || queryInt(e.a.DB, "SELECT SUM(version) FROM transactions") != 3 {
		t.Fatal("seen changed financial version")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='applied'") != 1 {
		t.Fatal("replayed audit")
	}
	change["seen"] = false
	id = mcpPrepare(t, e, token, change)
	mcpApprove(t, e, 3, id)
	if _, err := e.a.DB.Exec("DELETE FROM transaction_seen WHERE user_id=3 AND transaction_id=2"); err != nil {
		t.Fatal(err)
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("stale seen proposal", v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=3 AND transaction_id=1") != 1 {
		t.Fatal("partial seen write")
	}
}

func TestMCPLegacyFinancialEditKeepsExistingNoopSaveBehavior(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1}}})
	mcpApprove(t, e, 1, id)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=1 AND transaction_id=1 AND transaction_version=2") != 1 {
		t.Fatal("legacy editing save changed behavior")
	}
}
