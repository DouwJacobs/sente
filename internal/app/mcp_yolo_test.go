package app

import (
	"reflect"
	"testing"
)

func TestMCPFutureYOLORequiresExplicitConsentAndResolvesNewGrants(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	token := mcpToken(t, e, 1, true)
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	old := mcpPermissions{SchemaVersion: 1, Preset: "yolo", Capabilities: mcpCapabilities{FinancialEdit: true}, Automatic: mcpCapabilities{FinancialEdit: true}}
	setTestMCPPermissions(t, e, token, old)
	p, err := readMCPPermissions(e.a.DB, identity)
	if err != nil || p.Capabilities.DeleteTransactions || p.IncludeFuturePermissions {
		t.Fatal("legacy YOLO expanded silently", p, err)
	}
	old.IncludeFuturePermissions = true
	setTestMCPPermissions(t, e, token, old)
	var before, after string
	e.a.DB.QueryRow("SELECT permissions FROM mcp_tokens WHERE id=?", identity.TokenID).Scan(&before)
	p, err = readMCPPermissions(e.a.DB, identity)
	if err != nil || p.Capabilities != allMCPCapabilities() || p.Automatic != allMCPCapabilities() {
		t.Fatal("future capability catalogue did not resolve", p, err)
	}
	value, kind := reflect.ValueOf(p), reflect.TypeOf(p)
	for i := 0; i < value.NumField(); i++ {
		tag := kind.Field(i).Tag.Get("json")
		if len(tag) > 5 && tag[:5] == "read_" && !value.Field(i).Bool() {
			t.Fatal("future YOLO omitted read grant", tag)
		}
	}
	e.a.DB.QueryRow("SELECT permissions FROM mcp_tokens WHERE id=?", identity.TokenID).Scan(&after)
	if before != after {
		t.Fatal("reading future consent rewrote the stored snapshot")
	}
	id := mcpPrepare(t, e, token, removalChange(false, 2))
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal("newly supported grant not automatically approved", v)
	}
	id = mcpPrepare(t, e, token, removalChange(false, 3))
	old.IncludeFuturePermissions = false
	setTestMCPPermissions(t, e, token, old)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("future consent revocation ignored", v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=3") != 1 {
		t.Fatal("revoked grant removed transaction")
	}
}

func TestMCPFutureYOLOCannotBypassPresetOrReadOnlyConnection(t *testing.T) {
	e := setup(t)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", IncludeFuturePermissions: true}
	if _, err := e.a.validateMCPPermissions(e.a.DB, e.owner, p); err == nil {
		t.Fatal("future policy enabled outside YOLO")
	}
	token := mcpToken(t, e, 1, false)
	p.Preset = "yolo"
	setTestMCPPermissions(t, e, token, p)
	// A malformed stored policy must not expand a connection without proposal scope.
	if _, err := e.a.mcpIdentity(token); err == nil {
		t.Fatal("read-only token gained future YOLO access")
	}
}
