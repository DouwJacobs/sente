package app

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func TestNetworkSettingsAuthorizationValidationAndPending(t *testing.T) {
	e := setup(t)
	if err := e.a.ApplyNetworkConfig("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 2, "/api/network", "GET", nil), 403)
	status(t, e.req(t, 2, "/api/network", "PUT", map[string]any{"enabled": true, "version": 1, "public_url": "https://finance.example.com"}), 403)
	body := map[string]any{"enabled": true, "version": 1, "public_url": "https://finance.example.com/", "trusted_proxies": "10.0.0.0/24"}
	response := e.req(t, 1, "/api/network", "PUT", body)
	status(t, response, 200)
	if e.a.Secure || e.a.PublicURL != "http://localhost:8080" {
		t.Fatal("save changed active cookie/origin policy")
	}
	var value struct {
		Restart bool          `json:"restart_required"`
		Saved   networkSaved  `json:"saved"`
		Active  networkValues `json:"active"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	if !value.Restart || value.Saved.Version != 2 || value.Saved.PublicURL != "https://finance.example.com" || value.Active.TrustedProxies != "127.0.0.1" {
		t.Fatal("incorrect pending settings")
	}
	status(t, e.req(t, 1, "/api/network", "PUT", body), 409)
	body["version"] = 2
	for _, invalid := range []string{"ftp://example.com", "https://example.com/path", "https://user:secret@example.com", "https://example.com?x=1", "https://example.com#x"} {
		body["public_url"] = invalid
		status(t, e.req(t, 1, "/api/network", "PUT", body), 400)
	}
	body["public_url"] = "https://finance.example.com"
	body["trusted_proxies"] = "0.0.0.0/0"
	status(t, e.req(t, 1, "/api/network", "PUT", body), 400)
	status(t, e.req(t, 1, "/api/network", "PUT", map[string]any{"enabled": false, "version": 2}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM network_settings WHERE enabled=0 AND version=3") != 1 {
		t.Fatal("reset not saved")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='network_settings'") != 2 {
		t.Fatal("missing audited saves")
	}
}
func TestNetworkRestartAndOfflineRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "network.sqlite")
	var a *App
	reopen := func() {
		t.Helper()
		if a != nil {
			if err := a.Close(); err != nil {
				t.Fatal(err)
			}
		}
		var err error
		a, err = Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
		if err != nil {
			t.Fatal(err)
		}
	}
	reopen()
	defer func() {
		if a != nil {
			a.Close()
		}
	}()
	a.DB.Exec("INSERT INTO users(id,username,password,admin,budget_member) VALUES(1,'owner','unused',1,1)")
	a.DB.Exec("INSERT INTO sessions VALUES(?,?,?,?)", hash("token1"), 1, "csrf", time.Now().Add(time.Hour).Unix())
	if err := a.ApplyNetworkConfig("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	e := &testEnv{a: a, h: a.Handler(t.TempDir())}
	status(t, e.req(t, 1, "/api/network", "PUT", map[string]any{"enabled": true, "version": 1, "public_url": "https://finance.example.com", "trusted_proxies": "10.0.0.1"}), 200)
	reopen()
	if err := a.ApplyNetworkConfig("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if a.PublicURL != "https://finance.example.com" || !a.Secure || a.activeNetworkTrusted != "10.0.0.1" {
		t.Fatal("saved policy did not apply after restart")
	}
	if err := a.ResetNetworkSettings(); err != nil {
		t.Fatal(err)
	}
	if a.PublicURL != "https://finance.example.com" {
		t.Fatal("offline reset changed running policy")
	}
	reopen()
	if err := a.ApplyNetworkConfig("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	if a.PublicURL != "http://localhost:8080" || a.Secure || a.activeNetworkTrusted != "127.0.0.1" {
		t.Fatal("environment recovery failed")
	}
	// Upgrade a schema-6 fixture without modifying existing users or sessions.
	a.DB.Exec("DROP TABLE network_settings")
	a.DB.Exec("DELETE FROM migrations WHERE version>=7")
	a.DB.Exec("INSERT OR IGNORE INTO migrations VALUES(6)")
	reopen()
	if queryInt(a.DB, "SELECT MAX(version) FROM migrations") != schemaVersion || queryInt(a.DB, "SELECT COUNT(*) FROM users WHERE id=1") != 1 {
		t.Fatal("migration lost data")
	}
}

func TestNetworkOriginCanonicalization(t *testing.T) {
	for input, want := range map[string]string{"HTTPS://FINANCE.Example.com:443/": "https://finance.example.com", "http://LOCALHOST:80/": "http://localhost", "https://[::1]:443/": "https://[::1]", "https://example.com:8443/": "https://example.com:8443"} {
		got, err := validateNetworkOrigin(input)
		if err != nil || got != want {
			t.Fatalf("%s: got %s error %v", input, got, err)
		}
	}
	for _, input := range []string{"https://example.com:0", "https://example.com:65536", "https://example.com:"} {
		if _, err := validateNetworkOrigin(input); err == nil {
			t.Fatalf("accepted %s", input)
		}
	}
}

func TestNetworkRestartAuthorizationAndVersion(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 1, "/api/network/restart", "POST", map[string]any{"version": 1}), 503)
	requested := make(chan bool, 2)
	e.a.RequestRestart = func() { requested <- true }
	status(t, e.req(t, 2, "/api/network/restart", "POST", map[string]any{"version": 1}), 403)
	status(t, e.req(t, 1, "/api/network/restart", "POST", map[string]any{"version": 99}), 409)
	select {
	case <-requested:
		t.Fatal("rejected request restarted")
	default:
	}
	status(t, e.req(t, 1, "/api/network/restart", "POST", map[string]any{"version": 1}), 200)
	select {
	case <-requested:
	case <-time.After(2 * time.Second):
		t.Fatal("restart not requested")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='restart_requested'") != 1 {
		t.Fatal("missing audit")
	}
}
