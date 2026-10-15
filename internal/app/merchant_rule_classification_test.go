package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func merchantClassificationFixture(t *testing.T) *testEnv {
	e := setup(t)
	migrationExec(t, e.a.DB, "INSERT INTO merchants(id,name,category_id,spending_group_id) VALUES(1,'FNB',1,1)")
	return e
}
func saveClassifyingRule(t *testing.T, e *testEnv, pattern string, category, group int64, direction string) int64 {
	t.Helper()
	var id int64
	err := e.a.write(func(tx *sql.Tx) error {
		return e.a.writeMerchantRule(tx, User{ID: 1, Admin: true, Member: true}, &id, merchantRuleInput{Merchant: 1, Pattern: pattern, Direction: direction, Enabled: true, CategoryID: &category, SpendingGroupID: &group})
	})
	if err != nil {
		t.Fatal(err)
	}
	return id
}
func TestMerchantIndependentRuleClassifications(t *testing.T) {
	e := merchantClassificationFixture(t)
	fee := saveClassifyingRule(t, e, "FNB account fee", 1, 7, "debit")
	interest := saveClassifyingRule(t, e, "FNB interest paid", 2, 5, "credit")
	for _, test := range []struct {
		description             string
		amount, category, group int64
	}{{"FNB account fee", -500, 1, 7}, {"FNB interest paid", 250, 2, 5}} {
		m, err := matchMerchantDetails(e.a.DB, 1, test.description, test.amount)
		if err != nil || m == nil || m.ID != 1 || m.CategoryID == nil || *m.CategoryID != test.category || m.SpendingGroupID == nil || *m.SpendingGroupID != test.group {
			t.Fatal(m, err)
		}
	}
	if queryInt(e.a.DB, "SELECT category_id FROM merchants WHERE id=1") != 1 || queryInt(e.a.DB, "SELECT spending_group_id FROM merchants WHERE id=1") != 1 {
		t.Fatal("rule edited merchant defaults")
	}
	status(t, e.req(t, 1, "/api/merchant-rules/"+merchantTestID(fee), "PUT", map[string]any{"merchant_id": 1, "pattern": "FNB account fee", "direction": "debit", "enabled": true, "version": 1, "category_id": 2, "spending_group_id": 3}), 200)
	if queryInt(e.a.DB, "SELECT category_id FROM merchant_rules WHERE id=?", interest) != 2 || queryInt(e.a.DB, "SELECT spending_group_id FROM merchant_rules WHERE id=?", interest) != 5 {
		t.Fatal("edited sibling")
	}
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", map[string]any{"merchant_id": 1, "pattern": "FNB legacy default", "direction": "any", "enabled": true}), 200)
	if queryInt(e.a.DB, "SELECT category_id FROM merchant_rules WHERE pattern='FNB legacy default'") != 1 {
		t.Fatal("legacy creation default lost")
	}
	status(t, e.req(t, 1, "/api/categories/2", "PUT", map[string]any{"name": "Salary", "archived": true, "version": 1}), 409)
}
func merchantTestID(id int64) string { return strconv.FormatInt(id, 10) }
func TestMerchantRuleConflictsStayUncategorized(t *testing.T) {
	e := merchantClassificationFixture(t)
	saveClassifyingRule(t, e, "FNB", 1, 7, "any")
	second := saveClassifyingRule(t, e, "FNB interest", 2, 5, "any")
	p := ParsedFile{Rows: []SourceRow{{Description: "FNB interest", Amount: 500}}}
	if err := e.a.annotate(e.a.DB, 1, &p); err != nil {
		t.Fatal(err)
	}
	row := p.Rows[0]
	if row.MerchantID == nil || *row.MerchantID != 1 || row.CategoryID != nil || row.SpendingGroupID != nil || !row.RuleConflict {
		t.Fatal(row)
	}
	// Even an otherwise matching category rule must not override ambiguity.
	status(t, e.req(t, 1, "/api/rules", "POST", map[string]any{"account_id": 1, "pattern": "FNB", "category_id": 1}), 200)
	if err := e.a.annotate(e.a.DB, 1, &p); err != nil {
		t.Fatal(err)
	}
	if p.Rows[0].CategoryID != nil || !p.Rows[0].RuleConflict {
		t.Fatal("fallback hid ambiguity")
	}
	migrationExec(t, e.a.DB, "UPDATE merchant_rules SET priority=1 WHERE id="+merchantTestID(second))
	if err := e.a.annotate(e.a.DB, 1, &p); err != nil {
		t.Fatal(err)
	}
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 2 || p.Rows[0].RuleConflict {
		t.Fatal(p.Rows[0])
	}
}
func TestMerchantRuleValidationPrivacyAndFingerprint(t *testing.T) {
	e := merchantClassificationFixture(t)
	rule := saveClassifyingRule(t, e, "FNB", 1, 7, "any")
	p := ParsedFile{Rows: []SourceRow{{Description: "FNB account fee", Amount: -100}}}
	if err := e.a.annotate(e.a.DB, 1, &p); err != nil {
		t.Fatal(err)
	}
	before := p.ClassificationVersion
	for _, fields := range []map[string]any{{"category_id": 999}, {"spending_group_id": 999}, {"category_id": 0}} {
		body := map[string]any{"merchant_id": 1, "pattern": "bad rule", "direction": "any", "enabled": true}
		for k, v := range fields {
			body[k] = v
		}
		status(t, e.req(t, 1, "/api/merchant-rules", "POST", body), 400)
	}
	status(t, e.req(t, 2, "/api/merchant-rules/"+merchantTestID(rule), "PUT", map[string]any{"merchant_id": 1, "pattern": "FNB", "direction": "any", "enabled": true, "version": 1, "category_id": 2}), 403)
	migrationExec(t, e.a.DB, "UPDATE merchant_rules SET category_id=2,version=version+1 WHERE id="+merchantTestID(rule))
	if err := e.a.annotate(e.a.DB, 1, &p); err != nil {
		t.Fatal(err)
	}
	if p.ClassificationVersion == before {
		t.Fatal("stale merchant preview fingerprint")
	}
	status(t, e.req(t, 1, "/api/merchant-rules/"+merchantTestID(rule), "PUT", map[string]any{"merchant_id": 1, "pattern": "FNB", "direction": "any", "enabled": true, "version": 1, "category_id": 1}), 409)
}
func TestMerchantRulePortableRoundTrip(t *testing.T) {
	e := merchantClassificationFixture(t)
	saveClassifyingRule(t, e, "FNB fees", 1, 7, "debit")
	saveClassifyingRule(t, e, "FNB interest", 2, 5, "credit")
	status(t, e.req(t, 1, "/api/merchant-rules", "POST", map[string]any{"merchant_id": 1, "pattern": "FNB unknown", "direction": "any", "enabled": true, "clear_category": true, "clear_group": true}), 200)
	var out bytes.Buffer
	if err := e.a.ExportRuleset(&out); err != nil {
		t.Fatal(err)
	}
	var cfg RulesetConfig
	if err := json.Unmarshal(out.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	if len(cfg.MerchantRules) != 3 || cfg.MerchantRules[2].Category == nil || *cfg.MerchantRules[2].Category != "" {
		t.Fatal(out.String())
	}
	migrationExec(t, e.a.DB, "DELETE FROM merchant_rules")
	if err := e.a.write(func(tx *sql.Tx) error {
		return e.a.importRulesetTx(tx, User{ID: 1, Admin: true, Member: true}, cfg, &RulesetImportSummary{})
	}); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT category_id FROM merchant_rules WHERE pattern='FNB interest'") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE pattern='FNB unknown' AND category_id IS NULL AND spending_group_id IS NULL") != 1 {
		t.Fatal("roundtrip classification lost")
	}
}
func TestMerchantRuleClassificationMigration(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:len(schemaMigrations)-1], 30); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, "INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic fees','','expense'); INSERT INTO spending_groups(id,name,color) VALUES(1,'Synthetic bank','teal'); INSERT INTO merchants(id,name,category_id,spending_group_id) VALUES(1,'Synthetic FNB',1,1); INSERT INTO merchant_rules(id,merchant_id,pattern,version) VALUES(1,1,'fees',8)")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM merchant_rules WHERE id=1 AND category_id=1 AND spending_group_id=1 AND version=8") != 1 {
		t.Fatal("migration changed rule identity/classification")
	}
	once := migrationSnapshot(t, db, "SELECT * FROM merchant_rules")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if migrationSnapshot(t, db, "SELECT * FROM merchant_rules") != once {
		t.Fatal("migration repeated")
	}
}

func TestMerchantRuleMigrationRollbackRetry(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:len(schemaMigrations)-1], 30); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, "INSERT INTO merchants(id,name) VALUES(1,'Synthetic'); INSERT INTO merchant_rules(merchant_id,pattern) VALUES(1,'synthetic'); CREATE TRIGGER stop_rule_upgrade BEFORE UPDATE ON merchant_rules BEGIN SELECT RAISE(ABORT,'synthetic migration failure'); END")
	if err := migrate(db); err == nil {
		t.Fatal("expected upgrade failure")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 30 {
		t.Fatal("recorded failed migration")
	}
	columns, err := data(db, "PRAGMA table_info(merchant_rules)")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range columns {
		if c["name"] == "category_id" {
			t.Fatal("schema partially persisted")
		}
	}
	migrationExec(t, db, "DROP TRIGGER stop_rule_upgrade")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM merchant_rules") != 1 {
		t.Fatal("rule lost on retry")
	}
}
func TestMCPMerchantRuleIndependentAssignments(t *testing.T) {
	e := merchantClassificationFixture(t)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadMerchants: true, Capabilities: mcpCapabilities{ManageMerchantRules: true}, Automatic: mcpCapabilities{ManageMerchantRules: true}}
	setTestMCPPermissions(t, e, token, p)
	first := saveClassifyingRule(t, e, "FNB fees", 1, 7, "debit")
	change := map[string]any{"operation": "save_merchant_rule", "merchant_rule": map[string]any{"account_id": 0, "merchant_id": 1, "pattern": "FNB interest", "direction": "credit", "enabled": true, "category_id": 2, "spending_group_id": 5}}
	id := mcpPrepare(t, e, token, change)
	for i := 0; i < 2; i++ {
		if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
			t.Fatal(v)
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules") != 2 || queryInt(e.a.DB, "SELECT spending_group_id FROM merchant_rules WHERE id=?", first) != 7 || queryInt(e.a.DB, "SELECT category_id FROM merchants WHERE id=1") != 1 {
		t.Fatal("MCP changed sibling/defaults or replayed")
	}
	v, failed := mcpCall(t, e, token, "list_merchant_rules", map[string]any{})
	if failed || !strings.Contains(jsonString(v), "category_id") {
		t.Fatal(v)
	}
	tid := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	migrationExec(t, e.a.DB, "UPDATE transactions SET description='FNB fees' WHERE id="+merchantTestID(tid))
	preview, failed := mcpCall(t, e, token, "preview_merchant_rule", map[string]any{"id": first})
	if failed {
		t.Fatal(preview)
	}
	items := preview["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["classification_state"] != "classified" {
		t.Fatal(preview)
	}
	if strings.Contains(jsonString(preview), "Future imports") {
		t.Fatal("browser explanation leaked")
	}
	p.Capabilities.ManageMerchantRules = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("revoked replay bypass")
	}
}
func jsonString(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestMCPOldMerchantRuleProposalRequiresNewPreview(t *testing.T) {
	e := merchantClassificationFixture(t)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadMerchants: true, Capabilities: mcpCapabilities{ManageMerchantRules: true}, Automatic: mcpCapabilities{ManageMerchantRules: true}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "save_merchant_rule", "merchant_rule": map[string]any{"account_id": 0, "merchant_id": 1, "pattern": "FNB fee", "direction": "debit", "enabled": true, "category_id": 1}})
	migrationExec(t, e.a.DB, "UPDATE mcp_proposals SET payload=json_remove(payload,'$.merchant_rule_classification_version') WHERE id='"+id+"'")
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("old proposal applied changed semantics")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules") != 0 {
		t.Fatal("partial write")
	}
}

func TestMerchantRulesImportMoneyAndAcceptance(t *testing.T) {
	e := merchantClassificationFixture(t)
	saveClassifyingRule(t, e, "FNB fees", 1, 7, "debit")
	saveClassifyingRule(t, e, "FNB interest", 2, 5, "credit")
	saveClassifyingRule(t, e, "FNB ambiguous", 1, 7, "debit")
	saveClassifyingRule(t, e, "FNB ambiguous", 2, 5, "debit")
	p := e.stage(t, "merchant.ofx", ofx(ofxRow("fee-1", "20261021", "-5.00", "FNB fees"), ofxRow("interest-1", "20261022", "2.50", "FNB interest"), ofxRow("ambiguous-1", "20261023", "-3.00", "FNB ambiguous")))
	status(t, e.commit(t, p.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review'") != 1 {
		t.Fatal("ambiguous transaction accepted")
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != -550 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM allocations") != -550 {
		t.Fatal("money changed/double counted")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE merchant_id=1") != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions t JOIN allocations a ON a.transaction_id=t.id WHERE t.fitid='ambiguous-1' AND a.category_id IS NULL AND t.spending_group_id IS NULL") != 1 {
		t.Fatal("ambiguous assignment persisted")
	}
}

func TestMCPMerchantRuleBindsInheritedClassification(t *testing.T) {
	e := merchantClassificationFixture(t)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadMerchants: true, Capabilities: mcpCapabilities{ManageMerchantRules: true}, Automatic: mcpCapabilities{ManageMerchantRules: true}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, map[string]any{"operation": "save_merchant_rule", "merchant_rule": map[string]any{"account_id": 0, "merchant_id": 1, "pattern": "FNB inherited", "direction": "any", "enabled": true}})
	migrationExec(t, e.a.DB, "UPDATE merchants SET category_id=2,spending_group_id=5,version=version+1 WHERE id=1")
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE category_id=1 AND spending_group_id=1") != 1 {
		t.Fatal("inherited preview changed at apply")
	}
}
