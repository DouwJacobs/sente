package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"finance-tracker/internal/buildinfo"
)

func mcpToken(t *testing.T, e *testEnv, user int, write bool) string {
	t.Helper()
	client, request := oauthStart(t, e)
	code := oauthApprove(t, e, user, request, write)
	w := oauthExchange(e, client, code, testVerifier)
	status(t, w, 200)
	return oauthJSON(t, w)["access_token"].(string)
}
func mcpRPC(t *testing.T, e *testEnv, token, method string, params any) map[string]any {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	r := httptest.NewRequest("POST", "/api/mcp", bytes.NewReader(b))
	r.Header.Set("Authorization", "Bearer "+token)
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Accept", "application/json, text/event-stream")
	r.Header.Set("MCP-Protocol-Version", "2025-11-25")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	if w.Code == 403 && strings.Contains(w.Header().Get("WWW-Authenticate"), "insufficient_scope") {
		return map[string]any{"content": []any{map[string]any{"text": `{"error":"Read-only connection"}`}}, "isError": true}
	}
	status(t, w, 200)
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(w.Body.String())
	}
	if result["error"] != nil {
		t.Fatal(result)
	}
	return result["result"].(map[string]any)
}
func mcpCall(t *testing.T, e *testEnv, token, name string, args any) (map[string]any, bool) {
	t.Helper()
	result := mcpRPC(t, e, token, "tools/call", map[string]any{"name": name, "arguments": args})
	content := result["content"].([]any)[0].(map[string]any)["text"].(string)
	var value map[string]any
	if err := json.Unmarshal([]byte(content), &value); err != nil {
		t.Fatal(content)
	}
	isError, _ := result["isError"].(bool)
	return value, isError
}
func mcpPrepare(t *testing.T, e *testEnv, token string, args any) string {
	t.Helper()
	v, err := mcpCall(t, e, token, "prepare_change", args)
	if err {
		t.Fatal(v)
	}
	return v["proposal_id"].(string)
}
func mcpApprove(t *testing.T, e *testEnv, user int, id string) {
	t.Helper()
	status(t, e.req(t, user, "/api/mcp/proposals/"+id, "POST", map[string]any{"approve": true}), 200)
}
func seedMCPTransactions(t *testing.T, e *testEnv) {
	t.Helper()
	for _, q := range []string{
		`INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,fitid,period_id) VALUES(1,1,'2026-10-21',-101,'Market 12345678901 owner@example.test','2026-10-21',-101,'Secret source','{"bank":"12345678901"}','private-ref',1),(2,1,'2026-10-22',-202,'Another merchant','2026-10-22',-202,'Another merchant','{}','other-ref',1),(3,2,'2026-10-23',-303,'Private merchant','2026-10-23',-303,'Private merchant','{}','private-account-ref',NULL)`,
		`INSERT INTO allocations(transaction_id,amount_cents,note) VALUES(1,-101,'secret note'),(2,-202,''),(3,-303,'')`,
	} {
		if _, err := e.a.DB.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
}
func TestMCPPrivacyAndTransport(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 3, false)
	initialized := mcpRPC(t, e, token, "initialize", map[string]any{"protocolVersion": "2025-11-25", "capabilities": map[string]any{}, "clientInfo": map[string]any{"name": "test", "version": "1"}})
	if initialized["serverInfo"].(map[string]any)["version"] != buildinfo.Current().Version {
		t.Fatal("MCP must advertise the application build version")
	}
	if initialized["serverInfo"].(map[string]any)["name"] != "finance-tracker" {
		t.Fatal(initialized)
	}
	list := mcpRPC(t, e, token, "tools/list", map[string]any{})
	if len(list["tools"].([]any)) != 23 {
		t.Fatal(list)
	}
	result, err := mcpCall(t, e, token, "list_transactions", map[string]any{})
	if err {
		t.Fatal(result)
	}
	b, _ := json.Marshal(result)
	for _, secret := range []string{"12345678901", "owner@example.test", "secret note", "Secret source", "Private merchant", "fitid", "provenance", "source_description", "bank_id"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("MCP leaked %s: %s", secret, b)
		}
	}
	if len(result["items"].([]any)) != 2 {
		t.Fatal(result)
	}
	if result["items"].([]any)[0].(map[string]any)["date"] != "2026-10-22" || len(result["list_version"].(string)) != 64 {
		t.Fatal("redaction changed dates or paging version", result)
	}
	if !strings.Contains(string(b), "Market") || !strings.Contains(string(b), "[redacted]") {
		t.Fatal(result)
	}
	accounts, err := mcpCall(t, e, token, "list_accounts", map[string]any{})
	if err || strings.Contains(fmt.Sprint(accounts), "Shared") {
		t.Fatal(accounts)
	}
	if _, err := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "create_category", "category": map[string]any{"name": "New", "kind": "expense"}}); !err {
		t.Fatal("read-only prepared a write")
	}
	for _, tc := range []struct {
		origin, token string
		cookie        bool
		code          int
	}{{"", "", true, 401}, {"http://evil.example", token, false, 403}, {"", "wrong", false, 401}} {
		r := httptest.NewRequest("GET", "/api/mcp", nil)
		r.Header.Set("Origin", tc.origin)
		r.Header.Set("Accept", "text/event-stream")
		if tc.token != "" {
			r.Header.Set("Authorization", "Bearer "+tc.token)
		}
		if tc.cookie {
			r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
		}
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		status(t, w, tc.code)
	}
	if _, err := mcpCall(t, e, token, "get_budget_summary", map[string]any{}); !err {
		t.Fatal("non-member accessed budget")
	}
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=3")
	result, err = mcpCall(t, e, token, "list_transactions", map[string]any{})
	if err || len(result["items"].([]any)) != 0 {
		t.Fatal("revoked grant persisted", result)
	}
}

func TestMCPRedactsTextWithoutChangingMetadata(t *testing.T) {
	value := mcpSafe(map[string]any{"date": "2026-10-21", "list_version": "abc123456789abcdef", "description": "Merchant 12345678901 card ****1234 +27 82 123 4567 name@example.test https://example.test", "notes": "hidden"}).(map[string]any)
	if value["date"] != "2026-10-21" || value["list_version"] != "abc123456789abcdef" || value["notes"] != nil {
		t.Fatal(value)
	}
	if strings.Contains(value["description"].(string), "1234") || strings.Contains(value["description"].(string), "example.test") {
		t.Fatal(value)
	}
}

func TestMCPSetupLinkKeepsFinancialRequestsAuthenticated(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	for _, accept := range []string{"", "*/*", "text/html,application/xhtml+xml", "text/plain"} {
		r := httptest.NewRequest("GET", "/api/mcp?note=untrusted-query", nil)
		r.Header.Set("Accept", accept)
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		status(t, w, 200)
		for _, required := range []string{e.a.mcpResource(), "OAuth browser sign-in with S256 PKCE", "/.well-known/oauth-protected-resource/api/mcp", "finance:read", "finance:propose", "prepare_change", "Settings → MCP"} {
			if !strings.Contains(w.Body.String(), required) {
				t.Fatal("setup instructions missing", required)
			}
		}
		for _, private := range []string{"untrusted-query", "12345678901", "Secret source", "secret note", "Private merchant", "owner@example.test", "token1"} {
			if strings.Contains(w.Body.String(), private) {
				t.Fatal("public setup leaked data", private)
			}
		}
		if w.Header().Get("Content-Type") != "text/plain; charset=utf-8" || w.Header().Get("Cache-Control") != "no-store" || !strings.Contains(w.Header().Get("Vary"), "Accept") {
			t.Fatal(w.Header())
		}
	}
	r := httptest.NewRequest("HEAD", "/api/mcp", nil)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 200)
	if w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatal("HEAD returned a body or omitted length")
	}
	for _, tc := range []struct{ method, accept, protocol, authorization string }{
		{"POST", "text/plain", "", ""}, {"POST", "application/json, text/event-stream", "", ""},
		{"GET", "text/event-stream", "", ""}, {"GET", "application/json", "", ""},
		{"GET", "*/*", "2025-11-25", ""}, {"GET", "text/plain", "", "Bearer invalid"},
	} {
		r := httptest.NewRequest(tc.method, "/api/mcp", nil)
		r.Header.Set("Accept", tc.accept)
		r.Header.Set("MCP-Protocol-Version", tc.protocol)
		r.Header.Set("Authorization", tc.authorization)
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		status(t, w, 401)
		if !strings.Contains(w.Header().Get("WWW-Authenticate"), e.a.PublicURL+"/.well-known/oauth-protected-resource/api/mcp") {
			t.Fatal("OAuth challenge missing", w.Header())
		}
	}
	r = httptest.NewRequest("GET", "/api/mcp", nil)
	r.Header.Set("Origin", "https://foreign.example")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
}
func TestMCPProposalResultSchemaUpgrade(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "create_category", "category": map[string]any{"name": "Upgrade category", "kind": "expense"}})
	mcpApprove(t, e, 1, id)
	seedMCPTransactions(t, e)
	batchID := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}, map[string]any{"id": 2, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, batchID)
	removePostBaselineFixtureTables(t, e.a.DB)
	if _, err := e.a.DB.Exec("ALTER TABLE mcp_proposals DROP COLUMN result; DELETE FROM migrations WHERE version>=23; INSERT OR IGNORE INTO migrations VALUES(22)"); err != nil {
		t.Fatal(err)
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || result["id"] == nil {
		t.Fatal(result)
	}
	replayed, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || replayed["id"] != result["id"] || queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE name='Upgrade category'") != 1 {
		t.Fatal("upgraded proposal replay was not idempotent", replayed)
	}
	batch, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": batchID})
	if failed || batch["updated"] != float64(2) {
		t.Fatal(batch)
	}
	replayed, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": batchID})
	if failed || replayed["updated"] != batch["updated"] || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id IN (1,2) AND version=2 AND review_state='approved'") != 2 {
		t.Fatal("upgraded batch replay repeated writes", replayed)
	}
}

func TestMCPProposalLookupDatabaseFailure(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "create_category", "category": map[string]any{"name": "Lookup failure category", "kind": "expense"}})
	mcpApprove(t, e, 1, id)
	removePostBaselineFixtureTables(t, e.a.DB)
	if _, err := e.a.DB.Exec("ALTER TABLE mcp_proposals DROP COLUMN result; DELETE FROM migrations WHERE version>=23; INSERT OR IGNORE INTO migrations VALUES(22)"); err != nil {
		t.Fatal(err)
	}
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if !failed || result["error"] != "Proposal could not be loaded; tracker database maintenance may be required" {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE name='Lookup failure category'") != 0 {
		t.Fatal("lookup failure wrote finances")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='approved'", id) != 1 {
		t.Fatal("lookup failure consumed approval")
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": "missing"}); !failed || result["error"] != "Proposal not found" {
		t.Fatal(result)
	}
	if _, err := e.a.DB.Exec("ALTER TABLE mcp_proposals RENAME TO unavailable_proposals"); err != nil {
		t.Fatal(err)
	}
	result, failed = mcpCall(t, e, token, "get_change_status", map[string]any{"proposal_id": id})
	if !failed || result["error"] != "Proposal could not be loaded; tracker database maintenance may be required" {
		t.Fatal(result)
	}
}

func TestMCPBulkApprovalAtomicityAndPreservation(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	changes := map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}, map[string]any{"id": 2, "version": 1, "category_id": 1}}}
	id := mcpPrepare(t, e, token, changes)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NOT NULL") != 0 {
		t.Fatal("preview wrote data")
	}
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("unapproved applied")
	}
	status(t, e.req(t, 2, "/api/mcp/proposals/"+id, "POST", map[string]any{"approve": true}), 409)
	mcpApprove(t, e, 1, id)
	e.a.DB.Exec("UPDATE transactions SET version=version+1 WHERE id=2")
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("stale bulk applied")
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE category_id IS NOT NULL") != 0 {
		t.Fatal("partial bulk commit")
	}
	changes["edits"].([]any)[1].(map[string]any)["version"] = 2
	id = mcpPrepare(t, e, token, changes)
	mcpApprove(t, e, 1, id)
	result, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if err || result["updated"] != float64(2) {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 2 {
		t.Fatal("categorized not accepted")
	}
	var description, note, source string
	e.a.DB.QueryRow("SELECT description,source_description FROM transactions WHERE id=1").Scan(&description, &source)
	e.a.DB.QueryRow("SELECT note FROM allocations WHERE transaction_id=1").Scan(&note)
	if description != "Market 12345678901 owner@example.test" || source != "Secret source" || note != "secret note" {
		t.Fatal("classification corrupted hidden fields")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=1") != 2 {
		t.Fatal("editing user seen missing")
	}
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); err {
		t.Fatal("replay failed")
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=1") != 2 {
		t.Fatal("replay duplicated writes")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='mcp' AND action='applied'") != 1 {
		t.Fatal("missing agent audit")
	}
}
func TestMCPRulesCategoriesBudgetsAndRevocation(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	for _, change := range []map[string]any{
		{"operation": "create_category", "category": map[string]any{"name": "MCP category", "kind": "expense"}},
		{"operation": "save_rule", "rule": map[string]any{"account_id": 1, "pattern": "Merchant", "category_id": 1, "direction": "debit"}},
		{"operation": "update_budget", "id": 1, "budget": map[string]any{"version": 1, "merge": true, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 12345}}}},
	} {
		id := mcpPrepare(t, e, token, change)
		mcpApprove(t, e, 1, id)
		if result, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); err {
			t.Fatal(result)
		}
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=1 AND category_id=1") != 12345 {
		t.Fatal("limits not saved")
	}
	summary, err := mcpCall(t, e, token, "get_budget_summary", map[string]any{"filters": map[string]string{"period": "1"}})
	if err || summary["budget_cents"] != float64(12345) {
		t.Fatal(summary)
	}
	ruleID := queryInt(e.a.DB, "SELECT id FROM rules LIMIT 1")
	id := mcpPrepare(t, e, token, map[string]any{"operation": "save_rule", "id": ruleID, "rule": map[string]any{"account_id": 1, "pattern": "Merchant", "category_id": 1, "version": 1, "enabled": false}})
	mcpApprove(t, e, 1, id)
	mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if queryInt(e.a.DB, "SELECT enabled FROM rules WHERE id=?", ruleID) != 0 {
		t.Fatal("rule not paused")
	}
	id = mcpPrepare(t, e, token, map[string]any{"operation": "delete_rule", "id": ruleID, "version": 2})
	mcpApprove(t, e, 1, id)
	mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 {
		t.Fatal("rule not deleted")
	}
	tokenID := queryInt(e.a.DB, "SELECT connection_id FROM mcp_access_tokens WHERE token_hash=?", hash(token))
	status(t, e.req(t, 2, fmt.Sprintf("/api/mcp/tokens/%d", tokenID), "DELETE", nil), 409)
	status(t, e.req(t, 1, fmt.Sprintf("/api/mcp/tokens/%d", tokenID), "DELETE", nil), 200)
	if _, err := e.a.mcpIdentity(token); err == nil {
		t.Fatal("revoked token valid")
	}
	token = mcpToken(t, e, 1, true)
	e.a.DB.Exec("UPDATE mcp_tokens SET expires_at=?", time.Now().Unix()-1)
	if _, err := e.a.mcpIdentity(token); err == nil {
		t.Fatal("expired token valid")
	}
}
func TestMCPRejectInvalidSplitsAndRevokedEditor(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 3, true)
	if _, err := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "category_id": 1}}}); !err {
		t.Fatal("viewer can edit")
	}
	token = mcpToken(t, e, 1, true)
	if _, err := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "allocations": []any{map[string]any{"category_id": 1, "amount_cents": -100}}}}}); !err {
		t.Fatal("invalid split allowed")
	}
	id := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 3, "version": 1, "category_id": 1}}})
	mcpApprove(t, e, 1, id)
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2")
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("revoked editor applied")
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=3") != 1 {
		t.Fatal("revoked write committed")
	}
}

func TestMCPPartialEditPreservesSourceAndHiddenSplitNotes(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, true)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "edit_transactions", "edits": []any{map[string]any{"id": 1, "version": 1, "date": "2026-10-24", "amount_cents": -105, "description": "Updated merchant", "allocations": []any{map[string]any{"category_id": 1, "amount_cents": -55}, map[string]any{"category_id": 1, "amount_cents": -50}}}}})
	mcpApprove(t, e, 1, id)
	if result, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); err {
		t.Fatal(result)
	}
	var date, description, note string
	var amount, sourceAmount int64
	e.a.DB.QueryRow("SELECT date,description,amount_cents,source_amount FROM transactions WHERE id=1").Scan(&date, &description, &amount, &sourceAmount)
	e.a.DB.QueryRow("SELECT note FROM allocations WHERE transaction_id=1 ORDER BY id LIMIT 1").Scan(&note)
	if date != "2026-10-24" || description != "Updated merchant" || amount != -105 || sourceAmount != -101 || note != "secret note" {
		t.Fatal(date, description, amount, sourceAmount, note)
	}
}

func TestMCPProposalRejectionExpiryAndTokenIsolation(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	second := mcpToken(t, e, 1, true)
	change := map[string]any{"operation": "create_category", "category": map[string]any{"name": "Proposed category", "kind": "expense"}}
	id := mcpPrepare(t, e, token, change)
	mcpApprove(t, e, 1, id)
	if _, err := mcpCall(t, e, second, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("another token applied proposal")
	}
	id = mcpPrepare(t, e, token, change)
	status(t, e.req(t, 1, "/api/mcp/proposals/"+id, "POST", map[string]any{"approve": false}), 200)
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("rejected proposal applied")
	}
	id = mcpPrepare(t, e, token, change)
	e.a.DB.Exec("UPDATE mcp_proposals SET expires_at=? WHERE id=?", time.Now().Unix()-1, id)
	status(t, e.req(t, 1, "/api/mcp/proposals/"+id, "POST", map[string]any{"approve": true}), 409)
	if _, err := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !err {
		t.Fatal("expired proposal applied")
	}
	state, err := mcpCall(t, e, token, "get_change_status", map[string]any{"proposal_id": id})
	if err || state["status"] != "expired" {
		t.Fatal(state)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE name='Proposed category'") != 0 {
		t.Fatal("unapplied proposal wrote finances")
	}
}

func TestMCPFlatCategoryExplicitSpendingGroupBudget(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)

	utilGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Utilities'")
	change := map[string]any{
		"operation": "create_category",
		"category": map[string]any{
			"name": "Solar Power",
			"kind": "expense",
		},
	}
	id := mcpPrepare(t, e, token, change)
	mcpApprove(t, e, 1, id)
	res, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(res)
	}

	cid := queryInt(e.a.DB, "SELECT id FROM categories WHERE name='Solar Power'")
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE id=? AND spending_group_id IS NULL", cid) != 1 {
		t.Fatal("MCP category acquired ownership")
	}

	cats, failed := mcpCall(t, e, token, "list_categories", map[string]any{"filters": map[string]string{"id": fmt.Sprint(cid)}})
	if failed {
		t.Fatal(cats)
	}
	items := cats["items"].([]any)
	if len(items) == 0 {
		t.Fatal("category not found via MCP list_categories")
	}
	cItem := items[0].(map[string]any)
	if _, ok := cItem["spending_group_id"]; ok {
		t.Fatal("MCP category exposes spending group ownership")
	}

	pver := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	bChange := map[string]any{
		"operation": "update_budget",
		"id":        1,
		"budget": map[string]any{
			"version": pver,
			"merge":   true,
			"targets": []any{
				map[string]any{"category_id": cid, "amount_cents": 85000, "spending_group_name": "Utilities"},
			},
		},
	}
	bid := mcpPrepare(t, e, token, bChange)
	mcpApprove(t, e, 1, bid)
	bRes, bFailed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": bid})
	if bFailed {
		t.Fatal(bRes)
	}

	gtGroup := queryInt(e.a.DB, "SELECT spending_group_id FROM group_targets WHERE period_id=1 AND category_id=?", cid)
	if gtGroup != utilGroup {
		t.Fatalf("expected group_target to have spending_group_id=%d, got %d", utilGroup, gtGroup)
	}

	summary, sFailed := mcpCall(t, e, token, "get_budget_summary", map[string]any{"filters": map[string]string{"period": "1"}})
	if sFailed {
		t.Fatal(summary)
	}
	groups := summary["spending_groups"].([]any)
	foundUtil := false
	for _, g := range groups {
		gm := g.(map[string]any)
		if gm["name"] == "Utilities" {
			foundUtil = true
			if num(gm["target_cents"]) < 85000 {
				t.Fatalf("expected Utilities target_cents >= 85000, got %v", gm["target_cents"])
			}
		}
	}
	if !foundUtil {
		t.Fatal("Utilities spending group not found in get_budget_summary")
	}
}

func TestMCPBudgetFreezesExactGroupScope(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	// Category/rule defaults are deliberately unrelated to the requested limit.
	// A partial proposal also previews/preserves undistributed legacy limits.
	if _, err := e.a.DB.Exec("INSERT INTO targets VALUES(1,1,500)"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.DB.Exec("UPDATE categories SET spending_group_id=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	version := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	id := mcpPrepare(t, e, token, map[string]any{"operation": "update_budget", "id": 1, "budget": map[string]any{"version": version, "merge": true, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 12345, "spending_group_name": "Exceptions"}}}})
	mcpApprove(t, e, 1, id)
	if _, err := e.a.DB.Exec("UPDATE categories SET spending_group_id=2 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id=4") != 12345 {
		t.Fatal("approved proposal rerouted")
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id IS NULL") != 500 {
		t.Fatal("exact proposal lost legacy limit")
	}
	// Retry cannot repeat the write.
	result, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1") != version+1 {
		t.Fatal("proposal replay rewrote budget")
	}
	// A new group version invalidates an approved preview, even without changing period version.
	id = mcpPrepare(t, e, token, map[string]any{"operation": "update_budget", "id": 1, "budget": map[string]any{"version": version + 1, "merge": true, "group_id": 4, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 999}}}})
	mcpApprove(t, e, 1, id)
	status(t, e.req(t, 1, "/api/spending-groups/4", "PUT", map[string]any{"name": "Exceptions renamed", "color": "orange", "version": 1}), 200)
	if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("changed exact grouped state was applied")
	}
	if queryInt(e.a.DB, "SELECT amount_cents FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id=4") != 12345 {
		t.Fatal("stale proposal changed finances")
	}
	// Older proposals lack the exact effect snapshot and require a fresh preparation.
	legacy := mcpPrepare(t, e, token, map[string]any{"operation": "update_budget", "id": 1, "budget": map[string]any{"version": version + 1, "merge": true, "group_id": 4, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 500}}}})
	mcpApprove(t, e, 1, legacy)
	var payload string
	if err := e.a.DB.QueryRow("SELECT payload FROM mcp_proposals WHERE id=?", legacy).Scan(&payload); err != nil {
		t.Fatal(err)
	}
	var old map[string]any
	if err := json.Unmarshal([]byte(payload), &old); err != nil {
		t.Fatal(err)
	}
	delete(old, "budget_after")
	raw, err := json.Marshal(old)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.a.DB.Exec("UPDATE mcp_proposals SET payload=? WHERE id=?", string(raw), legacy); err != nil {
		t.Fatal(err)
	}
	if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": legacy}); !failed {
		t.Fatal("old unresolved proposal applied")
	}
	if queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1") != version+1 {
		t.Fatal("legacy proposal changed financial state")
	}

}
func TestBudgetExplicitEmptyGroupsAndRecurrence(t *testing.T) {
	e := setup(t)
	if _, err := e.a.DB.Exec("INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward) VALUES(1,1,500,0) ON CONFLICT DO UPDATE SET carry_forward=0"); err != nil {
		t.Fatal(err)
	}
	version := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=1")
	status(t, e.req(t, 1, "/api/targets/1", "PUT", map[string]any{"version": version, "groups": []any{map[string]any{"group_id": 3, "targets": []any{}}, map[string]any{"group_id": 0, "targets": []any{map[string]any{"category_id": 1, "amount_cents": 100}}}}}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM budget_groups WHERE period_id=1 AND spending_group_id=3") != 1 {
		t.Fatal("explicit empty group disappeared")
	}
	if queryInt(e.a.DB, "SELECT carry_forward FROM group_targets WHERE period_id=1 AND category_id=1 AND spending_group_id IS NULL") != 0 {
		t.Fatal("replacement restarted stopped recurrence")
	}
}
