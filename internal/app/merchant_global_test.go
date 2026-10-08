package app

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"strings"
	"testing"
)

func syntheticMerchantLogo(t *testing.T, size int) string {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, size, size))); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(b.Bytes())
}
func TestMerchantGlobalRulesPrivacyAndBulk(t *testing.T) {
	e := setup(t)
	logo := syntheticMerchantLogo(t, 8)
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Global Market", "logo_data": logo})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])
	status(t, e.req(t, 3, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Denied"}), 403)
	body := map[string]any{"merchant_id": mid, "pattern": "seed", "direction": "any", "priority": 1, "enabled": true}
	w = e.req(t, 1, "/api/merchant-rules", "POST", body)
	status(t, w, 200)
	rule := num(workflowJSON(t, w.Body.Bytes())["id"])
	status(t, e.req(t, 3, "/api/merchant-rules", "POST", body), 403)
	for _, account := range []int64{1, 2} {
		m, err := matchMerchant(e.a.DB, account, "seed purchase", -100)
		if err != nil || m != mid {
			t.Fatal(m, err)
		}
	}
	shared := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	private := seedTransaction(t, e, 2, -200, "2026-10-21", nil)
	workflowExec(t, e, "UPDATE transactions SET description='seed purchase' WHERE id IN (?,?)", shared, private)
	// A budget member without the private grant cannot preview or apply the private ledger.
	workflowExec(t, e, "UPDATE users SET budget_member=1 WHERE id=3")
	w = e.req(t, 3, fmt.Sprintf("/api/merchant-rules/%d/preview", rule), "POST", nil)
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["total"]) != 1 {
		t.Fatal(w.Body.String())
	}
	req := map[string]any{"operation": "merchant_rule", "rule_id": rule, "items": []map[string]any{{"id": shared, "version": 1}, {"id": private, "version": 1}}}
	status(t, e.req(t, 3, "/api/transactions/bulk/preview", "POST", req), 403)
	w = e.req(t, 1, "/api/transactions/bulk/preview", "POST", req)
	status(t, w, 200)
	status(t, e.req(t, 1, "/api/transactions/bulk/apply", "POST", map[string]any{"token": workflowJSON(t, w.Body.Bytes())["token"]}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE merchant_id=?", mid) != 2 {
		t.Fatal("global rule did not apply across accounts")
	}
	w = e.req(t, 3, "/api/transactions", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), "merchant_logo") || strings.Contains(w.Body.String(), "Private") {
		t.Fatal(w.Body.String())
	}
	safe := mcpSafe(map[string]any{"id": shared, "merchant_logo": logo, "merchant_name": "Global Market", "merchant_id": mid}).(map[string]any)
	if len(safe) != 1 {
		t.Fatal("MCP disclosed merchant metadata")
	}
	// Named transactions are preserved, even when the rule changes.
	body["priority"] = 2
	body["version"] = 1
	status(t, e.req(t, 1, fmt.Sprintf("/api/merchant-rules/%d", rule), "PUT", body), 200)
	w = e.req(t, 1, fmt.Sprintf("/api/merchant-rules/%d/preview", rule), "POST", nil)
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["total"]) != 0 {
		t.Fatal(w.Body.String())
	}
}
func TestMerchantLogoValidationVersionAndScope(t *testing.T) {
	e := setup(t)
	logo := syntheticMerchantLogo(t, 16)
	for _, raw := range []string{"https://example.com/logo.png", "data:image/svg+xml;base64,PHN2Zz4=", syntheticMerchantLogo(t, 513)} {
		status(t, e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Invalid logo", "logo_data": raw}), 400)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 0 {
		t.Fatal("invalid upload created a merchant")
	}
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Logo shop", "logo_data": logo})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])
	version := num(workflowJSON(t, w.Body.Bytes())["logo_version"])
	workflowExec(t, e, "INSERT INTO grants VALUES(2,1,'editor')")
	body := map[string]any{"account_id": 1, "merchant_id": mid, "pattern": "shop", "direction": "any", "enabled": true, "merchant_logo": "", "merchant_version": version}
	status(t, e.req(t, 2, "/api/merchant-rules", "POST", body), 403)
	body["merchant_version"] = version - 1
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules") != 0 {
		t.Fatal("stale logo saved a rule")
	}
	body["merchant_version"] = version
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 200)
	if queryInt(e.a.DB, "SELECT length(logo_data) FROM merchants WHERE id=?", mid) != 0 {
		t.Fatal("logo removal failed")
	}
	workflowExec(t, e, "INSERT INTO merchants(id,account_id,name)VALUES(100,1,'Private scope')")
	body = map[string]any{"merchant_id": 100, "pattern": "private", "direction": "any", "enabled": true}
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 400)
}
func TestMerchantLegacyMigrationPreservesReferences(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	workflowExec(t, e, "PRAGMA foreign_keys=OFF")
	workflowExec(t, e, "DROP TABLE merchant_rules; DROP TABLE merchants; CREATE TABLE merchants(id INTEGER PRIMARY KEY,account_id INTEGER NOT NULL REFERENCES accounts(id),name TEXT NOT NULL COLLATE NOCASE,UNIQUE(account_id,name)); CREATE TABLE merchant_rules(id INTEGER PRIMARY KEY,account_id INTEGER NOT NULL REFERENCES accounts(id),merchant_id INTEGER NOT NULL REFERENCES merchants(id),pattern TEXT NOT NULL,direction TEXT NOT NULL DEFAULT 'any',priority INTEGER NOT NULL DEFAULT 0,enabled INTEGER NOT NULL DEFAULT 1,version INTEGER NOT NULL DEFAULT 1); INSERT INTO merchants VALUES(42,1,'Legacy'); INSERT INTO merchant_rules VALUES(81,1,42,'seed','any',5,1,3)")
	workflowExec(t, e, "UPDATE transactions SET merchant_id=42 WHERE id=?", id)
	workflowExec(t, e, "PRAGMA foreign_keys=ON")
	removePostBaselineFixtureTables(t, e.a.DB)
	workflowExec(t, e, "DELETE FROM migrations WHERE version>=17; INSERT INTO migrations(version) VALUES(16)")
	for i := 0; i < 2; i++ {
		if err := migrate(e.a.DB); err != nil {
			t.Fatal(err)
		}
	}
	if queryInt(e.a.DB, "SELECT merchant_id FROM transactions WHERE id=?", id) != 42 || queryInt(e.a.DB, "SELECT version FROM merchant_rules WHERE id=81 AND account_id=1") != 3 {
		t.Fatal("migration lost IDs or scopes")
	}
	violations, err := data(e.a.DB, "PRAGMA foreign_key_check")
	if err != nil || len(violations) != 0 {
		t.Fatal(violations, err)
	}
	if queryInt(e.a.DB, "PRAGMA foreign_keys") != 1 {
		t.Fatal("migration disabled foreign keys")
	}
	status(t, e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "New global"}), 200)
}

func TestMerchantRegexAndNaturalLanguageMatching(t *testing.T) {
	e := setup(t)
	// Create merchant
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Checkers"})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])

	// Test pipe alternation: "checkers|shoprite"
	body := map[string]any{"merchant_id": mid, "pattern": "checkers|shoprite", "direction": "any", "priority": 1, "enabled": true}
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 200)

	for _, desc := range []string{"CHECKERS HYPER 123", "Shoprite Sea Point", "checkers", "SHOPRITE"} {
		m, err := matchMerchant(e.a.DB, 1, desc, -100)
		if err != nil || m != mid {
			t.Fatalf("expected match for %q, got id=%d err=%v", desc, m, err)
		}
	}
	m, err := matchMerchant(e.a.DB, 1, "Woolworths Food", -100)
	if err != nil || m != 0 {
		t.Fatalf("expected no match, got id=%d", m)
	}

	// Test natural language: "woolworths or woolies"
	w = e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Woolworths"})
	status(t, w, 200)
	wid := num(workflowJSON(t, w.Body.Bytes())["id"])
	body = map[string]any{"merchant_id": wid, "pattern": "woolworths or woolies", "direction": "any", "priority": 1, "enabled": true}
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 200)

	for _, desc := range []string{"WOOLWORTHS SANDTON", "Woolies Food"} {
		m, err = matchMerchant(e.a.DB, 1, desc, -100)
		if err != nil || m != wid {
			t.Fatalf("expected match for %q, got id=%d err=%v", desc, m, err)
		}
	}
}

func TestMerchantCategoryPrecedenceAndFallback(t *testing.T) {
	e := setup(t)
	// Create merchant with category and spending group
	catID := int64(1)
	groupID := int64(1)
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{
		"kind":              "merchant",
		"name":              "Supermarket",
		"category_id":       catID,
		"spending_group_id": groupID,
	})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])

	// Add rule for Supermarket
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", map[string]any{
		"merchant_id": mid,
		"pattern":     "supermarket|grocery",
		"direction":   "any",
		"priority":    10,
		"enabled":     true,
	}), 200)

	// Add generic category rule matching "market" with different category
	otherCatID := int64(2)
	status(t, e.req(t, 1, "/api/rules", "POST", map[string]any{
		"account_id":  1,
		"pattern":     "market",
		"category_id": otherCatID,
		"priority":    1,
	}), 200)

	// Test import with matching merchant: should get merchant's category (catID)
	p := &ParsedFile{
		AccountID: "1",
		Rows: []SourceRow{
			{Row: 1, Date: "2026-10-21", Amount: -1500, Description: "SUPERMARKET PURCHASE"},
			{Row: 2, Date: "2026-10-21", Amount: -800, Description: "FARMERS MARKET PURCHASE"},
		},
	}
	if err := e.a.annotate(e.a.DB, 1, p); err != nil {
		t.Fatal(err)
	}
	// Row 1 matches merchant -> should have merchant catID and MerchantID
	if p.Rows[0].MerchantID == nil || *p.Rows[0].MerchantID != mid {
		t.Fatalf("expected merchant %d, got %v", mid, p.Rows[0].MerchantID)
	}
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != catID {
		t.Fatalf("expected category %d from merchant, got %v", catID, p.Rows[0].CategoryID)
	}

	// Row 2 does NOT match merchant -> should fall back to generic rule (otherCatID)
	if p.Rows[1].MerchantID != nil {
		t.Fatalf("expected no merchant for row 2, got %v", p.Rows[1].MerchantID)
	}
	if p.Rows[1].CategoryID == nil || *p.Rows[1].CategoryID != otherCatID {
		t.Fatalf("expected fallback category %d, got %v", otherCatID, p.Rows[1].CategoryID)
	}
}

func TestMerchantDuplicateRuleMigration(t *testing.T) {
	e := setup(t)
	// Create a merchant
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Shell"})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])

	// Insert duplicate rules directly
	workflowExec(t, e, "INSERT INTO merchant_rules(account_id,merchant_id,pattern,direction,priority,enabled,version) VALUES(NULL,?, 'shell oil', 'any', 5, 1, 1)", mid)
	workflowExec(t, e, "INSERT INTO merchant_rules(account_id,merchant_id,pattern,direction,priority,enabled,version) VALUES(NULL,?, 'shell garage', 'any', 2, 1, 1)", mid)

	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE merchant_id=?", mid) != 2 {
		t.Fatal("expected 2 rules before migration")
	}

	// Trigger migration
	removePostBaselineFixtureTables(t, e.a.DB)
	workflowExec(t, e, "DELETE FROM migrations WHERE version>=18; INSERT OR IGNORE INTO migrations(version) VALUES(17)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}

	// Should now be 1 rule with merged pattern
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE merchant_id=?", mid) != 1 {
		t.Fatal("rules were not merged")
	}
	var pattern string
	if err := e.a.DB.QueryRow("SELECT pattern FROM merchant_rules WHERE merchant_id=?", mid).Scan(&pattern); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(pattern, "shell oil") || !strings.Contains(pattern, "shell garage") || !strings.Contains(pattern, "|") {
		t.Fatalf("unexpected merged pattern: %q", pattern)
	}
}
