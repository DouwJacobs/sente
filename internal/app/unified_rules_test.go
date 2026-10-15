package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func backfillPreview(t *testing.T, e *testEnv, user int, refs []RuleRef, period int64) map[string]any {
	t.Helper()
	w := e.req(t, user, "/api/rule-backfill/preview", "POST", ruleBackfillInput{refs, period})
	status(t, w, 200)
	return workflowJSON(t, w.Body.Bytes())
}
func unifiedCreate(t *testing.T, e *testEnv, b map[string]any) []RuleRef {
	t.Helper()
	w := e.req(t, 1, "/api/unified-rules", "POST", b)
	status(t, w, 200)
	var v struct {
		Refs []RuleRef `json:"refs"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	return v.Refs
}
func TestUnifiedRulesSearchGroupingAndFutureOnly(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	b := map[string]any{"kind": "category", "scope": "current", "rule": map[string]any{"pattern": "EFT Example", "category_id": 1, "direction": "debit", "enabled": true, "spending_group_id": group}}
	refs := unifiedCreate(t, e, b)
	if len(refs) != 2 {
		t.Fatal(refs)
	}
	for i := 0; i < 24; i++ {
		createTestRule(t, e, ruleBody(1, 1, fmt.Sprintf("Other %02d", i), "any", nil, 0, true, 0))
	}
	w := e.req(t, 1, "/api/unified-rules?page=0&q=EFT", "GET", nil)
	status(t, w, 200)
	result := workflowJSON(t, w.Body.Bytes())
	items := result["items"].([]any)
	if num(result["total"]) != 1 || len(items[0].(map[string]any)["refs"].([]any)) != 2 {
		t.Fatal(w.Body.String())
	}
	w = e.req(t, 1, "/api/unified-rules?page=0&q=Recurring&account=1", "GET", nil)
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["total"]) != 1 {
		t.Fatal(w.Body.String())
	}
	// Account filtering must never truncate a grouped write's account membership.
	if len(workflowJSON(t, w.Body.Bytes())["items"].([]any)[0].(map[string]any)["refs"].([]any)) != 2 {
		t.Fatal("incomplete group")
	}
	version := result["list_version"].(string)
	workflowExec(t, e, "UPDATE rules SET version=version+1 WHERE id=?", refs[0].ID)
	status(t, e.req(t, 1, "/api/unified-rules?page=1&list_version="+version, "GET", nil), 409)
	// A viewer cannot mutate a rule or inspect private account copies.
	status(t, e.req(t, 3, "/api/unified-rules", "POST", b), 400)
	w = e.req(t, 3, "/api/unified-rules?page=0&q=EFT", "GET", nil)
	status(t, w, 200)
	for _, item := range workflowJSON(t, w.Body.Bytes())["items"].([]any) {
		if item.(map[string]any)["editable"] != false {
			t.Fatal("viewer rule editable")
		}
	}
}
func TestUnifiedBackfillMissingOnlyPeriodPrecedenceAndReplay(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	refs := unifiedCreate(t, e, map[string]any{"kind": "category", "scope": "1", "rule": map[string]any{"account_id": 1, "pattern": "Synthetic merchant", "category_id": 1, "spending_group_id": group, "direction": "debit", "enabled": true}})
	targets := []int64{}
	for i := 0; i < 105; i++ {
		targets = append(targets, merchantTransaction(t, e, 1, -100, nil))
	}
	workflowExec(t, e, "UPDATE transactions SET period_id=1 WHERE account_id=1")
	otherGroup := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Exceptions'")
	workflowExec(t, e, "UPDATE transactions SET spending_group_id=? WHERE id=?", otherGroup, targets[0])
	workflowExec(t, e, "UPDATE allocations SET note='keep note' WHERE transaction_id=?", targets[0])
	category := int64(2)
	categorized := merchantTransaction(t, e, 1, -100, &category)
	workflowExec(t, e, "UPDATE transactions SET period_id=1,spending_group_id=? WHERE id=?", otherGroup, categorized)
	outside := merchantTransaction(t, e, 1, -100, nil)
	workflowExec(t, e, "UPDATE transactions SET period_id=NULL,assignment='outside' WHERE id=?", outside)
	private := merchantTransaction(t, e, 2, -100, nil)
	split := merchantTransaction(t, e, 1, -100, nil)
	workflowExec(t, e, "UPDATE transactions SET period_id=1 WHERE id=?", split)
	workflowExec(t, e, "UPDATE allocations SET amount_cents=-40 WHERE transaction_id=?", split)
	workflowExec(t, e, "INSERT INTO allocations(transaction_id,amount_cents) VALUES(?,-60)", split)
	transfer := merchantTransaction(t, e, 1, -100, nil)
	workflowExec(t, e, "UPDATE transactions SET period_id=1,is_transfer=1 WHERE id=?", transfer)
	preview := backfillPreview(t, e, 1, refs, 1)
	if len(preview["items"].([]any)) != 105 {
		t.Fatalf("unexpected preview: %d changes, %v conflicts", len(preview["items"].([]any)), preview["conflicts"])
	}
	before := queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions")
	w := e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]})
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["applied"]) != 105 {
		t.Fatal(w.Body.String())
	}
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 409)
	for _, id := range targets {
		if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 2 || queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", id) != 1 {
			t.Fatal(id)
		}
	}
	if queryInt(e.a.DB, "SELECT spending_group_id FROM transactions WHERE id=?", targets[0]) != otherGroup {
		t.Fatal("overwrote group")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND note='keep note'", targets[0]) != 1 {
		t.Fatal("overwrote note")
	}
	for _, id := range []int64{outside, private, split, transfer, categorized} {
		if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 1 {
			t.Fatalf("changed protected %d", id)
		}
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != before {
		t.Fatal("money changed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='rule_backfilled'") != 105 {
		t.Fatal("audit missing")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved' AND version=2") != 105 {
		t.Fatal("acceptance missing")
	}
	// A winning merchant prevents a category-only rule from filling classification.
	merchant := workflowJSON(t, e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Priority merchant"}).Body.Bytes())
	merchantRefs := unifiedCreate(t, e, map[string]any{"kind": "merchant", "scope": "1", "merchant_rule": map[string]any{"account_id": 1, "merchant_id": merchant["id"], "pattern": "Synthetic merchant", "category_id": 2, "direction": "debit", "enabled": true}})
	preview = backfillPreview(t, e, 1, refs, 0)
	if len(preview["items"].([]any)) != 0 {
		t.Fatal("category fallback bypassed merchant")
	}
	preview = backfillPreview(t, e, 1, merchantRefs, 0)
	// Previously classified values are kept while missing merchant/group fields may be filled.
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 200)
	if queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", outside) != 2 || queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", targets[0]) != 1 {
		t.Fatal("merchant precedence/preservation")
	}
}
func TestUnifiedBackfillStalePermissionConflictAndRollback(t *testing.T) {
	e := setup(t)
	refs := unifiedCreate(t, e, map[string]any{"kind": "category", "scope": "1", "rule": map[string]any{"account_id": 1, "pattern": "Synthetic merchant", "category_id": 1, "direction": "debit", "enabled": true}})
	id := merchantTransaction(t, e, 1, -100, nil)
	preview := backfillPreview(t, e, 1, refs, 0)
	status(t, e.req(t, 3, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 409)
	workflowExec(t, e, "UPDATE transactions SET version=version+1 WHERE id=?", id)
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 409)
	preview = backfillPreview(t, e, 1, refs, 0)
	workflowExec(t, e, "CREATE TRIGGER reject_rule_backfill BEFORE INSERT ON audit WHEN NEW.action='rule_backfilled' BEGIN SELECT RAISE(ABORT,'synthetic'); END")
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 500)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id IS NULL", id) != 1 {
		t.Fatal("non-atomic backfill")
	}
	workflowExec(t, e, "DROP TRIGGER reject_rule_backfill")
	createTestRule(t, e, ruleBody(1, 2, "merchant", "debit", nil, -100, true, 0))
	// Legacy category rules conflict across priorities; do not change that behavior.
	preview = backfillPreview(t, e, 1, refs, 0)
	if num(preview["conflicts"]) != 1 || len(preview["items"].([]any)) != 0 {
		t.Fatalf("unexpected preview: %d changes, %v conflicts", len(preview["items"].([]any)), preview["conflicts"])
	}
	workflowExec(t, e, "UPDATE rules SET enabled=0,version=version+1 WHERE id=?", refs[0].ID)
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 409)
}
func TestUnifiedGroupOnlyConfigurationAndExampleRollback(t *testing.T) {
	e := setup(t)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	b := map[string]any{"kind": "category", "scope": "1", "rule": map[string]any{"account_id": 1, "pattern": "Only group", "category_id": 0, "spending_group_id": group, "direction": "any", "enabled": true}}
	count := queryInt(e.a.DB, "SELECT COUNT(*) FROM rules")
	audits := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	w := e.req(t, 1, "/api/unified-rules/test", "POST", map[string]any{"draft": b, "description": "Only group EFT", "direction": "debit"})
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), "Recurring") {
		t.Fatal(w.Body.String())
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != count || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != audits {
		t.Fatal("example mutated state")
	}
	refs := unifiedCreate(t, e, b)
	rules, err := loadRules(e.a.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := evaluateDescription(e.a.DB, 1, "Only group EFT", -100, rules)
	if err != nil || result.CategoryID != nil || result.SpendingGroupID == nil || *result.SpendingGroupID != group {
		t.Fatal(result, err)
	}
	var exported bytes.Buffer
	if err = e.a.exportRulesetFor(&exported, &e.owner); err != nil {
		t.Fatal(err)
	}
	// export uses the explicitly configured offline operator.
	before := queryInt(e.a.DB, "SELECT version FROM rules WHERE id=?", refs[0].ID)
	var cfg RulesetConfig
	if err = json.Unmarshal(exported.Bytes(), &cfg); err != nil {
		t.Fatal(err)
	}
	err = e.a.write(func(tx *sql.Tx) error { return e.a.importRulesetTx(tx, e.owner, cfg, &RulesetImportSummary{}) })
	if err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT version FROM rules WHERE id=?", refs[0].ID) != before {
		t.Fatal("unchanged roundtrip rewrote rule")
	}
}
func TestOptionalRuleMigrationPreservesAndRollsBack(t *testing.T) {
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "upgrade.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	steps := schemaMigrations[:31-migrationBaseline+1]
	if err = runSchemaMigrations(db, steps, 31); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, "INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused'); INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Synthetic','synthetic',1); INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Synthetic','','expense'); INSERT INTO rules(id,user_id,account_id,pattern,category_id,version) VALUES(41,1,1,'Synthetic',1,7)")
	broken := append([]schemaMigration{}, schemaMigrations...)
	original := broken[len(broken)-1].apply
	broken[len(broken)-1].apply = func(tx *sql.Tx, o migrationOrigin) error {
		if err := original(tx, o); err != nil {
			return err
		}
		return errors.New("synthetic rollback")
	}
	if runSchemaMigrations(db, broken, 32) == nil {
		t.Fatal("expected failed upgrade")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 31 || queryInt(db, "SELECT version FROM rules WHERE id=41") != 7 {
		t.Fatal("rollback lost state")
	}
	if err = runSchemaMigrations(db, schemaMigrations, 32); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT category_id FROM rules WHERE id=41") != 1 || queryInt(db, "SELECT version FROM rules WHERE id=41") != 7 {
		t.Fatal("upgrade changed rule")
	}
	if err = runSchemaMigrations(db, schemaMigrations, 32); err != nil {
		t.Fatal(err)
	}
}

func TestUnifiedMerchantGroupedLogoAndGroupOnlyBackfill(t *testing.T) {
	e := setup(t)
	merchant := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "name": "Grouped synthetic"})
	status(t, merchant, 200)
	mid := num(workflowJSON(t, merchant.Body.Bytes())["id"])
	logo := syntheticMerchantLogo(t, 8)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	body := map[string]any{"kind": "merchant", "scope": "current", "merchant_rule": map[string]any{"merchant_id": mid, "pattern": "Grouped synthetic", "direction": "any", "enabled": true, "merchant_logo": logo, "merchant_version": 1, "spending_group_id": group, "clear_category": true}}
	refs := unifiedCreate(t, e, body)
	if len(refs) != 2 || queryInt(e.a.DB, "SELECT version FROM merchants WHERE id=?", mid) != 2 {
		t.Fatal("logo repeated or accounts lost")
	}
	w := e.req(t, 1, "/api/unified-rules?page=0&q=Grouped%20synthetic", "GET", nil)
	status(t, w, 200)
	items := workflowJSON(t, w.Body.Bytes())["items"].([]any)
	if len(items) != 1 || len(items[0].(map[string]any)["refs"].([]any)) != 2 {
		t.Fatal("merchant account copies not grouped")
	}
	target := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	workflowExec(t, e, "UPDATE transactions SET description='Grouped synthetic payment' WHERE id=?", target)
	preview := backfillPreview(t, e, 1, refs, 0)
	if len(preview["items"].([]any)) != 1 {
		t.Fatal(preview)
	}
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 200)
	if queryInt(e.a.DB, "SELECT spending_group_id FROM transactions WHERE id=?", target) != group || queryInt(e.a.DB, "SELECT merchant_id FROM transactions WHERE id=?", target) != mid || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id IS NULL", target) != 1 {
		t.Fatal("group-only merchant effects")
	}
	rules, err := loadRules(e.a.DB, 1)
	if err != nil {
		t.Fatal(err)
	}
	result, _, err := evaluateDescription(e.a.DB, 1, "Grouped synthetic payment", -100, rules)
	if err != nil || result.SpendingGroupID == nil || *result.SpendingGroupID != group || result.CategoryID != nil {
		t.Fatal("import preview differs from final import effects")
	}
	// A grouped pause retains each existing account and updates every copy once.
	body = map[string]any{"kind": "merchant", "refs": refs, "merchant_rule": map[string]any{"account_id": 0, "merchant_id": mid, "pattern": "Grouped synthetic", "direction": "any", "enabled": false, "spending_group_id": group, "clear_category": true}}
	unifiedCreate(t, e, body)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE account_id IS NOT NULL AND enabled=0 AND version=2") != 2 {
		t.Fatal("grouped edit changed scope")
	}
}

func TestTransactionMerchantRuleIsFutureOnly(t *testing.T) {
	e := setup(t)
	merchant := e.req(t, 1, "/api/labels", "POST", map[string]any{"kind": "merchant", "account_id": 1, "name": "Synthetic transaction merchant"})
	status(t, merchant, 200)
	mid := num(workflowJSON(t, merchant.Body.Bytes())["id"])
	source := merchantTransaction(t, e, 1, -100, nil)
	target := merchantTransaction(t, e, 1, -100, nil)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	body := pendingRuleBody(1, group)
	body["merchant_id"] = mid
	body["rule"] = map[string]any{"pattern": "Synthetic merchant", "merchant_id": mid}
	result := pendingRuleCounts(t, e, source, body)
	if result.RuleKind != "merchant" || result.RuleID == 0 || result.Applied != 0 {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM rules") != 0 {
		t.Fatal("rule changed history or created category duplicate")
	}
	preview := backfillPreview(t, e, 1, []RuleRef{{"merchant", result.RuleID, result.RuleVersion}}, 0)
	status(t, e.req(t, 1, "/api/rule-backfill/apply", "POST", map[string]any{"token": preview["token"]}), 200)
	if queryInt(e.a.DB, "SELECT merchant_id FROM transactions WHERE id=?", target) != mid || queryInt(e.a.DB, "SELECT category_id FROM allocations WHERE transaction_id=?", target) != 1 {
		t.Fatal("explicit historical application missing")
	}
}

func TestUnifiedGroupOnlyMCPConsentAndFutureOnly(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, true)
	permissions := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{CreateRule: true}, Constraints: mcpConstraints{AccountIDs: []int64{1}}}
	setTestMCPPermissions(t, e, token, permissions)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	target := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	workflowExec(t, e, "UPDATE transactions SET description='MCP group-only EFT' WHERE id=?", target)
	proposal := mcpPrepare(t, e, token, map[string]any{"operation": "save_rule", "rule": map[string]any{"account_id": 1, "pattern": "MCP group-only", "category_id": 0, "spending_group_id": group}})
	mcpApprove(t, e, 1, proposal)
	if result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); failed {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE category_id IS NULL AND spending_group_id=?", group) != 1 || queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", target) != 1 {
		t.Fatal("MCP group-only effect or historical mutation")
	}
	if result, failed := mcpCall(t, e, token, "prepare_change", map[string]any{"operation": "save_merchant_rule", "merchant_rule": map[string]any{"account_id": 1, "merchant_id": 1, "pattern": "MCP group-only", "direction": "any", "enabled": true}}); !failed {
		t.Fatal("category rule grant gained merchant permission", result)
	}
	permissions.Capabilities.CreateRule = false
	setTestMCPPermissions(t, e, token, permissions)
	if result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); !failed {
		t.Fatal("revoked grant replay", result)
	}
}

func TestMCPRuleSearchAndFutureOnlyDescription(t *testing.T) {
	e := setup(t)
	token := mcpToken(t, e, 1, false)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Recurring'")
	createTestRule(t, e, ruleBody(1, 1, "Unique EFT reference", "debit", &group, 0, true, 0))
	createTestRule(t, e, ruleBody(2, 1, "Unique EFT reference", "debit", &group, 0, true, 0))
	createTestRule(t, e, ruleBody(1, 2, "Unrelated salary", "credit", nil, 0, true, 0))
	for _, query := range []string{"Unique EFT", "Recurring", "Groceries"} {
		result, failed := mcpCall(t, e, token, "list_rules", map[string]any{"filters": map[string]string{"q": query, "page_size": "1"}})
		if failed || num(result["total"]) != 1 || len(result["items"].([]any)) != 2 {
			t.Fatal(query, result)
		}
		for _, item := range result["items"].([]any) {
			if strings.Contains(fmt.Sprint(item), "merchant_name") {
				t.Fatal("merchant consent expanded")
			}
		}
	}
	result, failed := mcpCall(t, e, token, "list_rules", map[string]any{"filters": map[string]string{"q": "no matching synthetic rule"}})
	if failed || num(result["total"]) != 0 {
		t.Fatal(result)
	}
	result, failed = mcpCall(t, e, token, "list_rules", map[string]any{"filters": map[string]string{"q": "Recurring", "page_size": "1"}})
	if failed {
		t.Fatal(result)
	}
	listVersion := result["list_version"].(string)
	workflowExec(t, e, "UPDATE spending_groups SET name='Changed recurring',version=version+1 WHERE id=?", group)
	if result, failed = mcpCall(t, e, token, "list_rules", map[string]any{"filters": map[string]string{"page": "1", "list_version": listVersion}}); !failed {
		t.Fatal("renamed search assignment did not invalidate paging", result)
	}
}
