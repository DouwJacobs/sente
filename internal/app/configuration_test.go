package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"testing"
)

func configurationFile(t *testing.T, e *testEnv, uid int, raw []byte, source int64) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	file, err := writer.CreateFormFile("file", "synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	file.Write(raw)
	if source != 0 {
		writer.WriteField("source_id", fmt.Sprint(source))
	}
	writer.Close()
	r := httptest.NewRequest("POST", "/api/configuration/preview", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: fmt.Sprint("token", uid)})
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}
func portableFixture() RulesetConfig {
	return RulesetConfig{Version: 2,
		SpendingGroups: []SpendingGroupExport{{Name: "Synthetic trips", Color: "teal"}},
		Categories:     []CategoryExport{{Name: "Synthetic supplies", Kind: "expense"}},
		Merchants:      []MerchantExport{{Name: "Synthetic shop", Category: "Synthetic supplies", SpendingGroup: "Synthetic trips"}},
		MerchantRules:  []MerchantRuleExport{{Merchant: "Synthetic shop", Pattern: "SYNTHETIC SHOP", Direction: "debit", Priority: 0, Enabled: true}},
		Rules:          []RuleExport{{Pattern: "SYNTHETIC SHOP", Category: "Synthetic supplies", SpendingGroup: "Synthetic trips", Builtin: true, Direction: "debit", Priority: -100, Enabled: true}}}
}
func configBytes(t *testing.T, cfg RulesetConfig) []byte {
	t.Helper()
	raw, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func previewID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	status(t, w, 200)
	return workflowJSON(t, w.Body.Bytes())["id"].(string)
}
func TestConfigurationFreshInstallAndUpgrade(t *testing.T) {
	a := emptySetupApp(t)
	for _, table := range []string{"categories", "spending_groups", "rules", "builtin_rules", "merchants", "merchant_rules"} {
		if queryInt(a.DB, "SELECT COUNT(*) FROM "+table) != 0 {
			t.Fatalf("fresh install seeded %s", table)
		}
	}
	e := setup(t)
	seedTestDefaults(t, e)
	before := queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules")
	e.a.DB.Exec("DELETE FROM migrations WHERE version>=20; INSERT OR IGNORE INTO migrations VALUES(19)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != before {
		t.Fatal("upgrade erased existing rules")
	}
}
func TestConfigurationPreviewAtomicityPermissionsAndExport(t *testing.T) {
	e := setup(t)
	raw := configBytes(t, portableFixture())
	status(t, e.req(t, 3, "/api/configuration", "GET", nil), 403)
	status(t, e.req(t, 3, "/api/configuration/export", "GET", nil), 403)
	status(t, configurationFile(t, e, 3, raw, 0), 403)
	auditBefore := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	id := previewID(t, configurationFile(t, e, 1, raw, 0))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != auditBefore {
		t.Fatal("preview persisted writes")
	}
	status(t, e.req(t, 3, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 403)
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM configuration_sources") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 1 {
		t.Fatal("import incomplete")
	}
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 409)
	versions := queryInt(e.a.DB, "SELECT SUM(version) FROM merchant_rules")
	id = previewID(t, configurationFile(t, e, 1, raw, 1))
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT SUM(version) FROM merchant_rules") != versions {
		t.Fatal("repeat import rewrote rules")
	}
	status(t, e.req(t, 1, "/api/configuration/export", "GET", nil), 200)
	// Forgetting the source must never delete imported configuration.
	status(t, e.req(t, 1, "/api/configuration/sources/1", "DELETE", map[string]any{"version": 2}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 1 {
		t.Fatal("forget erased configuration")
	}
}
func TestConfigurationStalePreviewAndExplicitReplacements(t *testing.T) {
	e := setup(t)
	cfg := portableFixture()
	raw := configBytes(t, cfg)
	id := previewID(t, configurationFile(t, e, 1, raw, 0))
	e.a.DB.Exec("UPDATE categories SET version=version+1 WHERE id=1")
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM configuration_sources") != 0 {
		t.Fatal("stale preview saved source")
	}
	id = previewID(t, configurationFile(t, e, 1, raw, 0))
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	// Local changes must be previewed and explicitly replaced even on a manual pull.
	e.a.DB.Exec("UPDATE builtin_rules SET priority=42,version=version+1")
	id = previewID(t, configurationFile(t, e, 1, raw, 1))
	auditBefore := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 409)
	if queryInt(e.a.DB, "SELECT priority FROM builtin_rules") != 42 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != auditBefore {
		t.Fatal("unconfirmed import overwrote local edit")
	}
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id, "allow_updates": true}), 200)
	if queryInt(e.a.DB, "SELECT priority FROM builtin_rules") != -100 {
		t.Fatal("confirmed replacement missing")
	}
	// Different sources coexist and omitted entries are retained.
	cfg.Rules[0].Pattern = "A DIFFERENT SHOP"
	id = previewID(t, configurationFile(t, e, 1, configBytes(t, cfg), 0))
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM configuration_sources") != 2 {
		t.Fatal("sources did not coexist")
	}
}
func TestConfigurationRejectsInvalidRulesAndExpiredPreview(t *testing.T) {
	e := setup(t)
	cfg := portableFixture()
	cfg.Rules = append(cfg.Rules, cfg.Rules[0])
	cfg.Rules[1].Pattern = " synthetic shop "
	status(t, configurationFile(t, e, 1, configBytes(t, cfg), 0), 400)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM merchants") != 0 {
		t.Fatal("invalid import partially committed")
	}
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2")
	cfg = portableFixture()
	cfg.Rules[0].Builtin = false
	cfg.Rules[0].AccountName = "Private"
	status(t, configurationFile(t, e, 1, configBytes(t, cfg), 0), 403)
	cfg = portableFixture()
	cfg.MerchantRules[0].Merchant = "Missing merchant"
	status(t, configurationFile(t, e, 1, configBytes(t, cfg), 0), 400)
	status(t, configurationFile(t, e, 1, []byte(`{"version":2,"unknown":true}`), 0), 400)
	id := previewID(t, configurationFile(t, e, 1, configBytes(t, portableFixture()), 0))
	e.a.DB.Exec("UPDATE configuration_previews SET expires_at=0")
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 409)
}
func TestConfigurationRepositoryInputSafety(t *testing.T) {
	for _, value := range []configurationSource{
		{RepositoryURL: "file:///etc", Path: "sente.json"},
		{RepositoryURL: "https://user:secret@example.com/repo", Path: "sente.json"},
		{RepositoryURL: "https://example.com/repo", Path: "../secrets.json"},
		{RepositoryURL: "https://example.com/repo", Path: "sente.json", Ref: "--upload-pack=bad"},
		{RepositoryURL: "https://example.com/repo", Path: "sente.json", Ref: "main:other"},
	} {
		if _, err := validateConfigurationRepository(value); err == nil {
			t.Fatalf("accepted unsafe input %+v", value)
		}
	}
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.169.254", "::1", "::ffff:192.168.1.1", "100.64.0.1"} {
		if publicConfigurationAddress(netip.MustParseAddr(value)) {
			t.Fatalf("accepted private host %s", value)
		}
	}
	var buffer limitedConfigurationBuffer
	if _, err := buffer.Write(make([]byte, maxRulesetBytes+1)); err == nil {
		t.Fatal("unbounded repository output")
	}
	if _, err := decodeRuleset(bytes.NewReader(starterConfiguration)); err != nil {
		t.Fatal(err)
	}
}

func TestConfigurationBundledStarterIsOptional(t *testing.T) {
	e := setup(t)
	id := previewID(t, e.req(t, 1, "/api/configuration/preview", "POST", map[string]any{"bundled": true}))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != 0 {
		t.Fatal("starter preview seeded rules")
	}
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != 4 {
		t.Fatal("starter import incomplete")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE pattern LIKE '%EFT%'") != 0 {
		t.Fatal("starter includes EFT rules")
	}
}
func TestConfigurationPublicStarterPull(t *testing.T) {
	if os.Getenv("SENTE_TEST_REPOSITORY_PULL") != "1" {
		t.Skip("explicit opt-in public repository smoke test")
	}
	e := setup(t)
	w := e.req(t, 1, "/api/configuration/preview", "POST", map[string]any{"starter": true})
	id := previewID(t, w)
	var preview struct {
		Source  configurationSource   `json:"source"`
		Changes []configurationChange `json:"changes"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Source.Revision) != 40 {
		t.Fatalf("unexpected repository revision %q", preview.Source.Revision)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") != 0 {
		t.Fatal("repository preview persisted rules")
	}
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM builtin_rules") == 0 {
		t.Fatal("repository import incomplete")
	}
	w = e.req(t, 1, "/api/configuration/preview", "POST", map[string]any{"source_id": 1})
	id = previewID(t, w)
	if err := json.Unmarshal(w.Body.Bytes(), &preview); err != nil {
		t.Fatal(err)
	}
	if len(preview.Changes) != 0 {
		t.Fatal("unchanged manual repository pull generated changes")
	}
	status(t, e.req(t, 1, "/api/configuration/apply", "POST", map[string]any{"preview_id": id}), 200)
	if queryInt(e.a.DB, "SELECT version FROM configuration_sources WHERE id=1") != 2 {
		t.Fatal("manual pull did not record the applied revision")
	}
}
