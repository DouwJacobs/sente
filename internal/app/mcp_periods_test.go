package app

import (
	"encoding/json"
	"testing"
)

func periodChange(name, start, end string) map[string]any {
	return map[string]any{"operation": "create_period", "period": map[string]any{"name": name, "start_date": start, "end_date": end}}
}
func periodPermissions(t *testing.T, e *testEnv) mcpPermissions {
	t.Helper()
	p, err := e.a.validateMCPPermissions(e.a.DB, User{ID: 1, Admin: true, Member: true}, mcpPermissions{SchemaVersion: 1, Preset: "yolo"})
	if err != nil {
		t.Fatal(err)
	}
	if !p.ReadContext || !p.ReadMerchants || !p.ReadBudgetAlerts || p.Capabilities != allMCPCapabilities() || p.Automatic != p.Capabilities {
		t.Fatal(p)
	}
	return p
}
func TestMCPPeriodCreationReplayAndConsent(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	change := periodChange("November", "2026-11-20", "2026-12-19")
	if _, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("legacy grant expanded")
	}
	p := periodPermissions(t, e)
	setTestMCPPermissions(t, e, token, p)
	migrationExec(t, e.a.DB, "INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward,included) VALUES(1,1,12345,1,1)")
	v, failed := mcpCall(t, e, token, "prepare_change", change)
	if failed || v["status"] != "approved" {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM periods") != 1 {
		t.Fatal("preview persisted")
	}
	id := v["proposal_id"].(string)
	var raw string
	e.a.DB.QueryRow("SELECT payload FROM mcp_proposals WHERE id=?", id).Scan(&raw)
	var exact mcpExactChange
	if err := json.Unmarshal([]byte(raw), &exact); err != nil || exact.PeriodEffects == nil || exact.PeriodAfter == nil {
		t.Fatal(raw, err)
	}
	for i := 0; i < 2; i++ {
		result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
		if failed || num(result["id"]) != 2 || result["start_date"] != "2026-11-20" {
			t.Fatal(result)
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM periods") != 2 || queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=2 AND category_id=1") != 12345 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='mcp' AND action='applied'") != 1 {
		t.Fatal("wrong copy/replay/audit")
	}
	if v, failed := mcpCall(t, e, token, "list_budget_periods", map[string]any{}); failed {
		t.Fatal(v)
	}
	p.Capabilities.CreatePeriod = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("revoked replay")
	}
}
func TestMCPPeriodValidationStalenessAndRollback(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	p := periodPermissions(t, e)
	setTestMCPPermissions(t, e, token, p)
	for _, change := range []map[string]any{periodChange("bad", "2026-02-30", "2026-03-20"), periodChange("bad", "2026-12-20", "2026-11-20"), periodChange("", "2026-11-20", "2026-12-19"), periodChange("duplicate", "2026-10-20", "2026-11-19"), {"operation": "create_period"}} {
		if _, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
			t.Fatal(change)
		}
	}
	change := periodChange("November", "2026-11-20", "2026-12-19")
	id := mcpPrepare(t, e, token, change)
	migrationExec(t, e.a.DB, "UPDATE periods SET version=version+1 WHERE id=1")
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("stale preview", v)
	}
	id = mcpPrepare(t, e, token, change)
	migrationExec(t, e.a.DB, "CREATE TRIGGER reject_period_audit BEFORE INSERT ON audit WHEN NEW.entity='mcp' AND NEW.action='applied' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM periods") != 1 {
		t.Fatal("partial write")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_period_audit")
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
}
func TestMCPPeriodRevocationAndYoloDisable(t *testing.T) {
	for _, revoke := range []string{"automatic", "permission", "member", "scope", "connection"} {
		t.Run(revoke, func(t *testing.T) {
			e := setup(t)
			token := mcpToken(t, e, 1, true)
			p := periodPermissions(t, e)
			setTestMCPPermissions(t, e, token, p)
			id := mcpPrepare(t, e, token, periodChange("Overlap", "2026-11-01", "2026-11-30"))
			switch revoke {
			case "automatic":
				p.Preset = "custom"
				p.Automatic = mcpCapabilities{}
			case "permission":
				p.Capabilities.CreatePeriod = false
			case "member":
				migrationExec(t, e.a.DB, "UPDATE users SET budget_member=0 WHERE id=1")
			case "scope":
				p.Constraints.AccountIDs = []int64{1}
			case "connection":
				migrationExec(t, e.a.DB, "DELETE FROM mcp_tokens")
			}
			if revoke != "connection" {
				setTestMCPPermissions(t, e, token, p)
			}
			if revoke == "connection" {
				if _, err := e.a.mcpIdentity(token); err == nil {
					t.Fatal("revoked connection authenticated")
				}
			} else if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
				t.Fatal("revocation bypass")
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM periods") != 1 {
				t.Fatal("partial write")
			}
		})
	}
	e := setup(t)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{CreatePeriod: true}, Constraints: mcpConstraints{AccountIDs: []int64{1}}}
	if _, err := e.a.validateMCPPermissions(e.a.DB, User{ID: 1, Admin: true, Member: true}, p); err == nil {
		t.Fatal("account scope accepted")
	}
}
