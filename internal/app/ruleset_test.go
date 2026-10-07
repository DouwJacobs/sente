package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestRulesetExportImportRoundTrip(t *testing.T) {
	e := setup(t)

	// Create test spending group
	status(t, e.req(t, 1, "/api/spending-groups", "POST", map[string]any{
		"name":  "Groceries & Food",
		"color": "teal",
	}), 200)

	// Create merchant with logo and rule
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{
		"kind":      "merchant",
		"name":      "TestMart",
		"logo_data": syntheticMerchantLogo(t, 8),
	})
	status(t, w, 200)
	mid := num(workflowJSON(t, w.Body.Bytes())["id"])

	status(t, e.req(t, 1, "/api/merchant-rules", "POST", map[string]any{
		"merchant_id": mid,
		"pattern":     "TESTMART",
		"direction":   "debit",
		"priority":    150,
		"enabled":     true,
	}), 200)

	// Export to buffer
	var buf bytes.Buffer
	if err := e.a.ExportRuleset(&buf); err != nil {
		t.Fatalf("export failed: %v", err)
	}

	exported := buf.String()
	var config RulesetConfig
	if err := json.Unmarshal(buf.Bytes(), &config); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range config.SpendingGroups {
		found = found || g.Name == "Groceries & Food"
	}
	if !found || len(config.Merchants) != 1 || config.Merchants[0].Name != "TestMart" {
		t.Fatalf("missing exported entities")
	}

	// Create clean database to test import
	cleanDir := t.TempDir()
	cleanPath := filepath.Join(cleanDir, "imported.sqlite")
	cleanApp, err := Open(cleanPath, "http://localhost:8080", filepath.Join(cleanDir, "backups"))
	if err != nil {
		t.Fatalf("open clean db failed: %v", err)
	}
	defer cleanApp.Close()
	if err := cleanApp.Initialize(); err != nil {
		t.Fatalf("init clean db failed: %v", err)
	}

	if _, err := cleanApp.CreateUser("operator", "synthetic-import-password", true, true); err != nil {
		t.Fatal(err)
	}
	// Import into clean database
	summary, err := cleanApp.ImportRuleset(strings.NewReader(exported))
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}

	if summary.MerchantsAdded == 0 || summary.MerchantRulesAdded == 0 {
		t.Fatalf("summary indicates nothing imported: %+v", summary)
	}

	// Verify imported data in clean DB
	var logoData string
	err = cleanApp.DB.QueryRow("SELECT logo_data FROM merchants WHERE name='TestMart'").Scan(&logoData)
	if err != nil || logoData == "" {
		t.Fatalf("merchant logo not restored properly: %v, logo=%s", err, logoData)
	}

	var pattern string
	err = cleanApp.DB.QueryRow("SELECT pattern FROM merchant_rules mr JOIN merchants m ON m.id=mr.merchant_id WHERE m.name='TestMart'").Scan(&pattern)
	if err != nil || pattern != "TESTMART" {
		t.Fatalf("merchant rule not restored properly: %v, pattern=%s", err, pattern)
	}
}

func importConfig(t *testing.T, a *App, cfg RulesetConfig) (*RulesetImportSummary, error) {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return a.ImportRuleset(bytes.NewReader(raw))
}
func TestRulesetScopesDirectionsVersionsAndIdempotency(t *testing.T) {
	e := setup(t)
	cfg := RulesetConfig{Version: 2,
		SpendingGroups: []SpendingGroupExport{{Name: "Utilities", Color: "rose"}},
		Merchants:      []MerchantExport{{Name: "Scoped shop", AccountName: "Private", Category: "Groceries", SpendingGroup: "Utilities"}, {Name: "Global shop"}},
		MerchantRules: []MerchantRuleExport{
			{Merchant: "Scoped shop", MerchantAccountName: "Private", AccountName: "Private", Pattern: "same pattern", Direction: "debit", Priority: 0, Enabled: true},
			{Merchant: "Scoped shop", MerchantAccountName: "Private", AccountName: "Private", Pattern: "same pattern", Direction: "credit", Priority: -3, Enabled: false},
			{Merchant: "Global shop", Pattern: "global pattern", Direction: "any", Priority: 0, Enabled: true}},
		Rules: []RuleExport{
			{Pattern: "same pattern", Category: "Groceries", AccountName: "Shared", Direction: "debit", Priority: 0, Enabled: true},
			{Pattern: "same pattern", Category: "Salary", AccountName: "Shared", Direction: "credit", Priority: -9, Enabled: false},
			{Pattern: "builtin pattern", Category: "Groceries", Builtin: true, Direction: "any", Priority: 0, Enabled: true}}}
	summary, err := importConfig(t, e.a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.MerchantRulesAdded != 3 || summary.RulesAdded != 3 {
		t.Fatal(summary)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchant_rules WHERE priority=0") != 2 {
		t.Fatal("priority zero changed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE pattern='same pattern'") != 2 {
		t.Fatal("directional rules collapsed")
	}
	version := queryInt(e.a.DB, "SELECT version FROM spending_groups WHERE name='Utilities'")
	audits := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	summary, err = importConfig(t, e.a, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if *summary != (RulesetImportSummary{}) || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != audits || queryInt(e.a.DB, "SELECT version FROM spending_groups WHERE name='Utilities'") != version {
		t.Fatal("idempotent import mutated config", summary)
	}
	cfg.SpendingGroups[0].Color = "blue"
	cfg.Merchants[0].SpendingGroup = "Exceptions"
	cfg.MerchantRules[0].Priority = 7
	cfg.Rules[0].Priority = 8
	if _, err = importConfig(t, e.a, cfg); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT version FROM spending_groups WHERE name='Utilities'") != version+1 || queryInt(e.a.DB, "SELECT version FROM merchant_rules WHERE direction='debit'") != 2 || queryInt(e.a.DB, "SELECT version FROM rules WHERE direction='debit'") != 2 {
		t.Fatal("updates did not invalidate versions")
	}
	var exported bytes.Buffer
	if err = e.a.ExportRuleset(&exported); err != nil {
		t.Fatal(err)
	}
	var output RulesetConfig
	if err = json.Unmarshal(exported.Bytes(), &output); err != nil {
		t.Fatal(err)
	}
	if output.Version != 2 || len(output.Merchants) != 2 || len(output.MerchantRules) != 3 || len(output.Rules) != 3 {
		t.Fatal("scoped or built-in configuration omitted")
	}
	dest := setup(t)
	if _, err = dest.a.ImportRuleset(bytes.NewReader(exported.Bytes())); err != nil {
		t.Fatal(err)
	}
	var copied bytes.Buffer
	if err = dest.a.ExportRuleset(&copied); err != nil {
		t.Fatal(err)
	}
	var restored RulesetConfig
	if err = json.Unmarshal(copied.Bytes(), &restored); err != nil {
		t.Fatal(err)
	}
	output.ExportedAt = ""
	restored.ExportedAt = ""
	left, _ := json.Marshal(output)
	right, _ := json.Marshal(restored)
	if !bytes.Equal(left, right) {
		t.Fatal("portable round trip changed configuration", string(left), string(right))
	}
	if _, err = e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2"); err != nil {
		t.Fatal(err)
	}
	cfg.SpendingGroups[0].Color = "amber"
	if _, err = importConfig(t, e.a, cfg); err == nil {
		t.Fatal("import ignored revoked private access")
	}
	if queryInt(e.a.DB, "SELECT version FROM spending_groups WHERE name='Utilities'") != version+1 {
		t.Fatal("permission failure partially committed")
	}
	var scoped bytes.Buffer
	if err = e.a.ExportRuleset(&scoped); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(scoped.String(), "Scoped shop") {
		t.Fatal("export disclosed inaccessible scope")
	}
	if _, err = e.a.DB.Exec("UPDATE users SET disabled=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if _, err = importConfig(t, e.a, cfg); err == nil {
		t.Fatal("disabled operator imported")
	}
	var denied bytes.Buffer
	if err = e.a.ExportRuleset(&denied); err == nil || denied.Len() != 0 {
		t.Fatal("disabled operator exported")
	}
}
func TestRulesetRejectsInvalidInputsAtomically(t *testing.T) {
	e := setup(t)
	cases := []string{
		`{"version":99}`, `{"version":2,"unknown":true}`, `{"version":2} {}`,
		`{"version":2,"categories":[{"name":"Groceries","kind":"income","group_name":"Living"}]}`,
		`{"version":2,"spending_groups":[{"name":"Before failure","color":"teal"}],"merchants":[{"name":"Unknown category shop","category":"Missing"}]}`,
		`{"version":2,"rules":[{"pattern":"valid","category":"Groceries","account_name":"Missing","direction":"any","enabled":true}]}`,
		`{"version":2,"rules":[{"pattern":"valid","category":"Groceries","direction":"any","enabled":true}]}`,
		`{"version":2,"merchant_rules":[{"merchant":"Missing","pattern":"valid","direction":"any","enabled":true}]}`,
		`{"version":2,"spending_groups":[{"name":"Bad color","color":"invented"}]}`,
		`{"version":2,"merchants":[{"name":"Invalid logo","logo_data":"data:invalid"}]}`,
		`{"version":2,"merchants":[{"name":"Twice"},{"name":"Twice"}]}`,
	}
	audits := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	for _, raw := range cases {
		if _, err := e.a.ImportRuleset(strings.NewReader(raw)); err == nil {
			t.Errorf("accepted invalid import: %s", raw)
		}
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != audits || queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 0 {
			t.Fatal("invalid import left partial writes")
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM categories WHERE id=1 AND kind='expense'") != 1 {
		t.Fatal("import changed historical category kind")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM spending_groups WHERE name='Before failure'") != 0 {
		t.Fatal("partial group saved")
	}
	if _, err := e.a.ImportRuleset(strings.NewReader(strings.Repeat(" ", maxRulesetBytes+1))); err == nil {
		t.Fatal("unbounded ruleset accepted")
	}
	if _, err := e.a.DB.Exec("UPDATE accounts SET name='Shared' WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	if _, err := importConfig(t, e.a, RulesetConfig{Version: 2, Rules: []RuleExport{{Pattern: "valid", Category: "Groceries", AccountName: "Shared", Direction: "any", Enabled: true}}}); err == nil {
		t.Fatal("ambiguous account accepted")
	}
}
func TestRulesetArchiveDependenciesAndRestore(t *testing.T) {
	e := setup(t)
	enabled := true
	var id int64
	if err := e.a.write(func(tx *sql.Tx) error {
		return e.a.writeRule(tx, e.owner, &id, &ruleInput{AccountID: 1, Pattern: "archive rule", CategoryID: 1, Direction: "any", Enabled: &enabled})
	}); err != nil {
		t.Fatal(err)
	}
	cfg := RulesetConfig{Version: 2, Categories: []CategoryExport{{Name: "Groceries", GroupName: "Living", Kind: "expense", Archived: true}}}
	if _, err := importConfig(t, e.a, cfg); err == nil {
		t.Fatal("archived active rule category")
	}
	cfg.Rules = []RuleExport{{Pattern: "archive rule", Category: "Groceries", AccountName: "Shared", Direction: "any", Enabled: false}}
	if _, err := importConfig(t, e.a, cfg); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT archived FROM categories WHERE id=1") != 1 {
		t.Fatal("archive did not persist")
	}
	cfg.Categories[0].Archived = false
	cfg.Rules[0].Enabled = true
	if summary, err := importConfig(t, e.a, cfg); err != nil {
		t.Fatal("restore and enable atomically", err)
	} else if summary.CategoriesAdded != 1 {
		t.Fatal("restored category missing from import summary")
	}
	if queryInt(e.a.DB, "SELECT archived FROM categories WHERE id=1") != 0 || queryInt(e.a.DB, "SELECT enabled FROM rules WHERE id=?", id) != 1 {
		t.Fatal("restore lost classification")
	}
}
