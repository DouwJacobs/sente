package app

import (
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrandingPermissionsValidationAndConcurrency(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 2, "/api/branding", "GET", nil), 200)
	for _, id := range []int{2, 3} {
		status(t, e.req(t, id, "/api/branding", "PUT", map[string]any{"display_name": "Other", "version": 1}), 403)
	}
	// Budget members retain their ordinary settings permissions, never branding rights.
	e.a.DB.Exec("UPDATE users SET budget_member=1 WHERE id=2")
	status(t, e.req(t, 2, "/api/settings", "PUT", map[string]any{"start_day": 21}), 200)
	status(t, e.req(t, 2, "/api/branding", "PUT", map[string]any{"display_name": "Other", "version": 1}), 403)
	for _, name := range []string{"", "   ", "x", strings.Repeat("a", 61)} {
		status(t, e.req(t, 1, "/api/branding", "PUT", map[string]any{"display_name": name, "version": 1}), 400)
	}
	name := "<script>Family</script>"
	status(t, e.req(t, 1, "/api/branding", "PUT", map[string]any{"display_name": "  " + name + "  ", "version": 1}), 200)
	status(t, e.req(t, 1, "/api/branding", "PUT", map[string]any{"display_name": "Stale", "version": 1}), 409)
	var got struct {
		Name    string `json:"display_name"`
		Version int    `json:"version"`
	}
	json.Unmarshal(e.req(t, 3, "/api/branding", "GET", nil).Body.Bytes(), &got)
	if got.Name != name || got.Version != 2 {
		t.Fatalf("%+v", got)
	}
	r := httptest.NewRequest("GET", "/api/branding", nil)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 401)
}
func TestBrandingMigrationAndRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "finance.sqlite")
	a, err := Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	groups := queryInt(a.DB, "SELECT COUNT(*) FROM spending_groups")
	// Simulate the previous schema: real tables survive the upgrade.
	a.DB.Exec("DROP TABLE workspace_branding; DELETE FROM migrations WHERE version>=4; INSERT OR IGNORE INTO migrations VALUES(3)")
	a.Close()
	a, err = Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	var name string
	a.DB.QueryRow("SELECT display_name FROM workspace_branding").Scan(&name)
	if name != "Household" {
		t.Fatal(name)
	}
	if queryInt(a.DB, "SELECT COUNT(*) FROM spending_groups") != groups {
		t.Fatal("groups changed")
	}
	a.DB.Exec("UPDATE workspace_branding SET display_name='Family',version=2")
	a.Close()
	a, err = Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.DB.QueryRow("SELECT display_name FROM workspace_branding").Scan(&name)
	if name != "Family" || queryInt(a.DB, "SELECT version FROM workspace_branding") != 2 {
		t.Fatal("branding lost on restart")
	}
}
