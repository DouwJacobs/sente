package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func mcpAlertPermissions() mcpPermissions {
	return mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadBudgetAlerts: true, Capabilities: mcpCapabilities{ManageBudgetAlerts: true}}
}
func mcpAlertChange(inputs ...any) map[string]any {
	return map[string]any{"operation": "update_budget_alerts", "budget_alerts": inputs}
}
func applyMCPAlert(t *testing.T, e *testEnv, token string, inputs ...any) string {
	t.Helper()
	id := mcpPrepare(t, e, token, mcpAlertChange(inputs...))
	mcpApprove(t, e, 1, id)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	return id
}
func TestMCPBudgetAlertConsentOwnershipAndExactProposal(t *testing.T) {
	e := scopedAlertFixture(t)
	token := mcpToken(t, e, 1, true)
	scope := map[string]any{"category_id": 1, "group_id": 0}
	change := mcpAlertChange(budgetAlertInput(0, 0, false, 70))
	for _, name := range []string{"get_budget_alert_preferences", "prepare_change"} {
		args := scope
		if name == "prepare_change" {
			args = change
		}
		if _, failed := mcpCall(t, e, token, name, args); !failed {
			t.Fatal("legacy consent expanded", name)
		}
	}
	p := mcpAlertPermissions()
	p.Capabilities.ManageBudgetAlerts = false
	setTestMCPPermissions(t, e, token, p)
	if v, failed := mcpCall(t, e, token, "get_budget_alert_preferences", scope); failed || v["enabled"] != true || v["threshold"] != nil || num(v["version"]) != 0 {
		t.Fatal(v)
	}
	if _, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("read consent granted changes")
	}
	p.Capabilities.ManageBudgetAlerts = true
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, change)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences") != 0 {
		t.Fatal("preview wrote preferences")
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("pending applied", v)
	}
	var payload string
	if err := e.a.DB.QueryRow("SELECT payload FROM mcp_proposals WHERE id=?", id).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var exact mcpExactChange
	if err := json.Unmarshal([]byte(payload), &exact); err != nil {
		t.Fatal(err)
	}
	if len(exact.BudgetAlertBefore) != 1 || len(exact.BudgetAlertAfter) != 1 || exact.BudgetAlertBefore[0].Version != 0 || exact.BudgetAlertAfter[0].Version != 1 || exact.BudgetAlertAfter[0].Enabled || *exact.BudgetAlertAfter[0].Threshold != 70 {
		t.Fatal(payload)
	}
	mcpApprove(t, e, 1, id)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT version FROM notification_budget_preferences WHERE user_id=1 AND group_id=0") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='mcp' AND action='applied'") != 1 {
		t.Fatal("replay repeated effects")
	}
	migrationExec(t, e.a.DB, "UPDATE users SET budget_member=1 WHERE id=2")
	other := mcpToken(t, e, 2, true)
	setTestMCPPermissions(t, e, other, p)
	if v, failed := mcpCall(t, e, other, "get_budget_alert_preferences", scope); failed || v["enabled"] != true || num(v["version"]) != 0 {
		t.Fatal("read other user's preference", v)
	}
	if _, failed := mcpCall(t, e, other, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("applied another user's proposal")
	}
	applyMCPAlert(t, e, token, map[string]any{"category_id": 1, "group_id": 0, "version": 1, "reset": true})
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE user_id=1 AND enabled=1 AND threshold IS NULL AND version=2") != 1 {
		t.Fatal("reset did not restore defaults")
	}
	p.ReadBudgetAlerts = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("replay bypassed read consent")
	}
}
func TestMCPBudgetAlertValidationScopesAndRevocation(t *testing.T) {
	e := scopedAlertFixture(t)
	token := mcpToken(t, e, 1, true)
	p := mcpAlertPermissions()
	setTestMCPPermissions(t, e, token, p)
	for _, threshold := range []any{1, 100} {
		if v, failed := mcpCall(t, e, token, "prepare_change", mcpAlertChange(budgetAlertInput(0, 0, true, threshold))); failed {
			t.Fatal(v)
		}
	}
	for _, threshold := range []any{0, 101, 70.5, "70"} {
		if !mcpAlertRejected(t, e, token, mcpAlertChange(budgetAlertInput(0, 0, true, threshold))) {
			t.Fatal("invalid threshold", threshold)
		}
	}
	for _, key := range []string{"group_id", "version", "enabled"} {
		input := budgetAlertInput(0, 0, true, 70)
		delete(input, key)
		if !mcpAlertRejected(t, e, token, mcpAlertChange(input)) {
			t.Fatal("missing field", key)
		}
	}
	for _, input := range []any{budgetAlertInput(-1, 0, true, 70), budgetAlertInput(899, 0, true, 70), map[string]any{"category_id": 2, "group_id": 0, "version": 0, "enabled": true}, map[string]any{"category_id": 1, "group_id": 0, "version": 0, "reset": true, "enabled": false}} {
		if !mcpAlertRejected(t, e, token, mcpAlertChange(input)) {
			t.Fatal("invalid scope/reset", input)
		}
	}
	tooMany := make([]any, 101)
	for i := range tooMany {
		tooMany[i] = budgetAlertInput(0, 0, true, 70)
	}
	if !mcpAlertRejected(t, e, token, mcpAlertChange(tooMany...)) {
		t.Fatal("oversized batch")
	}
	if _, failed := mcpCall(t, e, token, "prepare_change", mcpAlertChange()); !failed {
		t.Fatal("empty batch")
	}
	if _, failed := mcpCall(t, e, token, "prepare_change", mcpAlertChange(budgetAlertInput(0, 0, true, 70), budgetAlertInput(0, 0, true, 80))); !failed {
		t.Fatal("duplicate scope")
	}
	id := mcpPrepare(t, e, token, mcpAlertChange(budgetAlertInput(0, 0, true, 70)))
	for _, consent := range []bool{false, true} {
		p.ReadBudgetAlerts = consent
		setTestMCPPermissions(t, e, token, p)
		code := 403
		if consent {
			code = 200
		}
		status(t, e.req(t, 1, "/api/mcp/proposals/"+id, "POST", map[string]any{"approve": true}), code)
	}
	for _, revoke := range []string{"read", "write", "scope", "member"} {
		revoked := p
		switch revoke {
		case "read":
			revoked.ReadBudgetAlerts = false
		case "write":
			revoked.Capabilities.ManageBudgetAlerts = false
		case "scope":
			revoked.Constraints.AccountIDs = []int64{1}
		case "member":
			migrationExec(t, e.a.DB, "UPDATE users SET budget_member=0 WHERE id=1")
		}
		setTestMCPPermissions(t, e, token, revoked)
		if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
			t.Fatal("revoked access applied", revoke)
		}
		if revoke == "member" {
			migrationExec(t, e.a.DB, "UPDATE users SET budget_member=1 WHERE id=1")
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences") != 0 {
		t.Fatal("revoked apply changed preference")
	}
	setTestMCPPermissions(t, e, token, p)
	nonmember := mcpToken(t, e, 3, true)
	setTestMCPPermissions(t, e, nonmember, p)
	if _, failed := mcpCall(t, e, nonmember, "get_budget_alert_preferences", map[string]any{"category_id": 1, "group_id": 0}); !failed {
		t.Fatal("nonmember read shared scopes")
	}
	for _, preset := range []string{"finance", "categorization", "review"} {
		candidate := mcpPermissions{SchemaVersion: 1, Preset: preset}
		candidate, err := e.a.validateMCPPermissions(e.a.DB, User{ID: 1, Member: true}, candidate)
		if preset == "categorization" && err != nil {
			continue
		}
		if err != nil || candidate.ReadBudgetAlerts || candidate.Capabilities.ManageBudgetAlerts {
			t.Fatal("preset expanded", preset, candidate, err)
		}
	}
	for _, invalid := range []mcpPermissions{
		{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{ManageBudgetAlerts: true}},
		{SchemaVersion: 1, Preset: "custom", ReadBudgetAlerts: true, Constraints: mcpConstraints{AccountIDs: []int64{1}}},
		{SchemaVersion: 1, Preset: "custom", ReadBudgetAlerts: true, Automatic: mcpCapabilities{ManageBudgetAlerts: true}},
	} {
		if _, err := e.a.validateMCPPermissions(e.a.DB, User{ID: 1, Member: true}, invalid); err == nil {
			t.Fatal("invalid consent accepted", invalid)
		}
	}
}
func TestMCPBudgetAlertStaleAtomicityAndAuditRollback(t *testing.T) {
	e := scopedAlertFixture(t)
	token := mcpToken(t, e, 1, true)
	setTestMCPPermissions(t, e, token, mcpAlertPermissions())
	id := mcpPrepare(t, e, token, mcpAlertChange(budgetAlertInput(900, 0, true, 80), budgetAlertInput(0, 0, true, 70)))
	mcpApprove(t, e, 1, id)
	saveBudgetAlerts(t, e, 1, 200, budgetAlertInput(0, 0, false, 90))
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("stale preference applied")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_budget_preferences WHERE group_id=900") != 0 {
		t.Fatal("partially committed stale batch")
	}
	id = mcpPrepare(t, e, token, mcpAlertChange(budgetAlertInput(900, 0, true, 80)))
	mcpApprove(t, e, 1, id)
	migrationExec(t, e.a.DB, "UPDATE spending_groups SET name='Renamed',version=version+1 WHERE id=900")
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("changed preview scope applied")
	}
	id = mcpPrepare(t, e, token, mcpAlertChange(budgetAlertInput(900, 0, true, 80)))
	mcpApprove(t, e, 1, id)
	alertTransaction(t, e, 1, 1, -19000, time.Now().UTC().Format("2006-01-02"))
	migrationExec(t, e.a.DB, "UPDATE transactions SET spending_group_id=900 WHERE id=1")
	before := migrationSnapshot(t, e.a.DB, "SELECT * FROM notification_budget_preferences", "SELECT * FROM notification_conditions", "SELECT * FROM audit")
	migrationExec(t, e.a.DB, "CREATE TRIGGER reject_mcp_alert_audit BEFORE INSERT ON audit WHEN NEW.entity='mcp' AND NEW.action='applied' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("audit failure accepted")
	}
	if before != migrationSnapshot(t, e.a.DB, "SELECT * FROM notification_budget_preferences", "SELECT * FROM notification_conditions", "SELECT * FROM audit") {
		t.Fatal("audit failure retained settings/baselines")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_mcp_alert_audit")
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
}
func TestMCPBudgetAlertSilentBaselineGlobalPrecedenceAndAutomaticConsent(t *testing.T) {
	e := scopedAlertFixture(t)
	token := mcpToken(t, e, 1, true)
	p := mcpAlertPermissions()
	p.Automatic.ManageBudgetAlerts = true
	setTestMCPPermissions(t, e, token, p)
	alertTransaction(t, e, 1, 1, -8000, time.Now().UTC().Format("2006-01-02"))
	before := migrationSnapshot(t, e.a.DB, "SELECT * FROM notification_conditions")
	id := mcpPrepare(t, e, token, mcpAlertChange(budgetAlertInput(0, 0, true, 70)))
	// Automatic approval adds only its approval audit, never preview conditions.
	if before != migrationSnapshot(t, e.a.DB, "SELECT * FROM notification_conditions") {
		t.Fatal("preview retained baselines")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='approved'", id) != 1 {
		t.Fatal("explicit automatic approval missing")
	}
	p.Automatic.ManageBudgetAlerts = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("revoked automatic permission applied")
	}
	id = applyMCPAlert(t, e, token, budgetAlertInput(0, 0, true, 70))
	evaluateAlerts(t, e, time.Now().UTC())
	if alertCount(e, "budget_threshold") != 0 {
		t.Fatal("saved threshold replayed history")
	}
	changeAlertAmount(t, e, -6000)
	evaluateAlerts(t, e, time.Now().UTC())
	changeAlertAmount(t, e, -7000)
	evaluateAlerts(t, e, time.Now().UTC())
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("fresh crossing not delivered")
	}
	evaluateAlerts(t, e, time.Now().UTC().Add(time.Hour))
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("duplicate crossing")
	}
	applyMCPAlert(t, e, token, budgetAlertInput(0, 1, false, 70))
	changeAlertAmount(t, e, -11000)
	evaluateAlerts(t, e, time.Now().UTC())
	if alertCount(e, "budget_overspend") != 0 {
		t.Fatal("muted alerts delivered")
	}
	applyMCPAlert(t, e, token, map[string]any{"category_id": 1, "group_id": 0, "version": 2, "reset": true})
	evaluateAlerts(t, e, time.Now().UTC())
	if alertCount(e, "budget_threshold") != 1 || alertCount(e, "budget_overspend") != 0 {
		t.Fatal("reset replayed history")
	}
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", map[string]any{"type": "budget_threshold", "channel": "in_app", "enabled": false, "version": 0}), 200)
	changeAlertAmount(t, e, -6000)
	evaluateAlerts(t, e, time.Now().UTC())
	changeAlertAmount(t, e, -7500)
	evaluateAlerts(t, e, time.Now().UTC())
	if alertCount(e, "budget_threshold") != 1 {
		t.Fatal("scope overrode global channel")
	}
	var auditDetails string
	if err := e.a.DB.QueryRow("SELECT details FROM audit WHERE entity='mcp' AND action='applied' ORDER BY id DESC LIMIT 1").Scan(&auditDetails); err != nil {
		t.Fatal(err)
	}
	if len(auditDetails) == 0 {
		t.Fatal("missing exact audit", id)
	}
}

// Schema errors reject the RPC before the handler; service errors use isError.
func mcpAlertRejected(t *testing.T, e *testEnv, token string, args any) bool {
	t.Helper()
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": "prepare_change", "arguments": args}})
	req := httptest.NewRequest("POST", "/api/mcp", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("MCP-Protocol-Version", "2025-11-25")
	out := httptest.NewRecorder()
	e.h.ServeHTTP(out, req)
	status(t, out, 200)
	var rpc struct {
		Error  any `json:"error"`
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out.Body.Bytes(), &rpc); err != nil {
		t.Fatal(err)
	}
	return rpc.Error != nil || rpc.Result.IsError
}
