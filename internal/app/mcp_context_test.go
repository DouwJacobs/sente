package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
)

func shareContextForTest(t *testing.T, e *testEnv, token string, share bool) {
	t.Helper()
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	p, err := readMCPPermissions(e.a.DB, identity)
	if err != nil {
		t.Fatal(err)
	}
	p.ReadContext = share
	version := queryInt(e.a.DB, "SELECT permission_version FROM mcp_tokens WHERE id=?", identity.TokenID)
	status(t, e.req(t, int(identity.User.ID), "/api/mcp/connections/"+strconv.FormatInt(identity.TokenID, 10)+"/permissions", "PUT", map[string]any{"version": version, "permissions": p, "consent": true}), 200)
}
func TestMCPContextOwnershipConsentAndInitialization(t *testing.T) {
	e := setup(t)
	context := "Synthetic owner context: groceries exclude dining; never auto-approve."
	status(t, e.req(t, 1, "/api/mcp/context", "PUT", map[string]any{"context": context, "version": 0}), 200)
	owner := mcpToken(t, e, 1, false)
	other := mcpToken(t, e, 3, false)
	init := map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "synthetic", "version": "1"}}
	initial := mcpRPC(t, e, owner, "initialize", init)
	if strings.Contains(initial["instructions"].(string), context) {
		t.Fatal("context shared without consent")
	}
	v, failed := mcpCall(t, e, owner, "get_session_context", map[string]any{})
	if failed || v["shared"] != false || v["context"] != "" {
		t.Fatal(v)
	}
	shareContextForTest(t, e, owner, true)
	initial = mcpRPC(t, e, owner, "initialize", init)
	if !strings.Contains(initial["instructions"].(string), context) {
		t.Fatal("context absent from initialization", initial)
	}
	v, failed = mcpCall(t, e, owner, "get_session_context", map[string]any{})
	if failed || v["context"] != context || v["shared"] != true {
		t.Fatal(v)
	}
	// Same shared MCP server must never retain another connection's prose.
	shareContextForTest(t, e, other, true)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			token := owner
			expected := true
			if i%2 == 1 {
				token = other
				expected = false
			}
			result := mcpRPC(t, e, token, "initialize", init)
			if strings.Contains(result["instructions"].(string), context) != expected {
				t.Error("cross-connection context leak")
			}
		}(i)
	}
	wg.Wait()
	status(t, e.req(t, 1, "/api/mcp/context", "PUT", map[string]any{"context": "Updated synthetic context", "version": 1}), 200)
	v, failed = mcpCall(t, e, owner, "get_session_context", map[string]any{})
	if failed || v["context"] != "Updated synthetic context" || v["version"] != float64(2) {
		t.Fatal(v)
	}
	shareContextForTest(t, e, owner, false)
	initial = mcpRPC(t, e, owner, "initialize", init)
	if strings.Contains(initial["instructions"].(string), "Updated synthetic context") {
		t.Fatal("revoked context still shared")
	}
	v, failed = mcpCall(t, e, owner, "get_session_context", map[string]any{})
	if failed || v["context"] != "" || v["shared"] != false {
		t.Fatal(v)
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, httptest.NewRequest("GET", "/api/mcp", nil))
	if strings.Contains(w.Body.String(), context) || strings.Contains(w.Body.String(), "Updated synthetic context") {
		t.Fatal("public context leak")
	}
}
func TestMCPContextValidationStalenessDeletionAndAudit(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 3, "/api/mcp/context", "PUT", map[string]any{"context": "Private synthetic context", "version": 0}), 200)
	own := e.req(t, 1, "/api/mcp/context", "GET", nil)
	status(t, own, 200)
	if strings.Contains(own.Body.String(), "Private synthetic") {
		t.Fatal("cross-user browser disclosure")
	}
	status(t, e.req(t, 3, "/api/mcp/context", "PUT", map[string]any{"context": "Overwrite", "version": 0}), 409)
	status(t, e.req(t, 3, "/api/mcp/context", "PUT", map[string]any{"context": strings.Repeat("x", 6001), "version": 1}), 400)
	// Limit is Unicode characters, not bytes.
	status(t, e.req(t, 1, "/api/mcp/context", "PUT", map[string]any{"context": strings.Repeat("£", 6000), "version": 0}), 200)
	auditRows, err := data(e.a.DB, "SELECT details FROM audit WHERE entity='mcp_context'")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(auditRows)
	if strings.Contains(string(raw), "Private synthetic") || strings.Contains(string(raw), "£") {
		t.Fatal("context content entered audit")
	}
	// Audit failure rolls back the new text/version.
	if _, err := e.a.DB.Exec("CREATE TRIGGER fail_context_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END"); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 3, "/api/mcp/context", "PUT", map[string]any{"context": "Must roll back", "version": 1}), 500)
	e.a.DB.Exec("DROP TRIGGER fail_context_audit")
	saved, err := readPersonalMCPContext(e.a.DB, 3)
	if err != nil || saved.Content != "Private synthetic context" || saved.Version != 1 {
		t.Fatal(saved, err)
	}
	status(t, e.req(t, 1, "/api/users/3", "DELETE", map[string]any{"confirm_username": "viewer", "version": 1}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_user_context WHERE user_id=3") != 0 {
		t.Fatal("retired user context retained")
	}
}
func TestMCPContextRequiresBrowserSessionAndCSRF(t *testing.T) {
	e := setup(t)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, httptest.NewRequest("GET", "/api/mcp/context", nil))
	status(t, w, 401)
	r := httptest.NewRequest("PUT", "/api/mcp/context", strings.NewReader("{}"))
	r.Header.Set("Origin", "http://localhost:8080")
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
}

func TestMCPContextUpgradePreservesExistingDataAndConsent(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, false)
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	before, err := data(e.a.DB, "SELECT id,account_id,amount_cents,source_description FROM transactions ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.DB.Exec("DROP TABLE mcp_user_context; DELETE FROM migrations WHERE version>=22; INSERT OR IGNORE INTO migrations VALUES(21)"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	after, err := data(e.a.DB, "SELECT id,account_id,amount_cents,source_description FROM transactions ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(before)
	c, _ := json.Marshal(after)
	if string(b) != string(c) {
		t.Fatal("migration changed financial source data")
	}
	p, err := readMCPPermissions(e.a.DB, identity)
	if err != nil || p.ReadContext {
		t.Fatal("migration expanded consent", p, err)
	}
	if queryInt(e.a.DB, "SELECT MAX(version) FROM migrations") != schemaVersion {
		t.Fatal("schema migration missing")
	}
	saved, err := readPersonalMCPContext(e.a.DB, 1)
	if err != nil || saved.Content != "" || saved.Version != 0 {
		t.Fatal(saved, err)
	}
}
