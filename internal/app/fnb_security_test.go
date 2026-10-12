package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

func TestFNBKeyStorageSecurity(t *testing.T) {
	for _, kind := range []string{"valid", "public_file", "writable_directory", "symlink", "oversized", "fifo", "backup_path", "relative_path", "backup_symlink"} {
		t.Run(kind, func(t *testing.T) {
			directory := t.TempDir()
			a := &App{fnbKeyPath: filepath.Join(directory, "fnb.key"), BackupDir: filepath.Join(directory, "backups")}
			key := bytes.Repeat([]byte{42}, 32)
			switch kind {
			case "public_file":
				if err := os.WriteFile(a.fnbKeyPath, key, 0644); err != nil {
					t.Fatal(err)
				}
			case "writable_directory":
				if err := os.Chmod(directory, 0777); err != nil {
					t.Fatal(err)
				}
			case "symlink":
				target := filepath.Join(t.TempDir(), "original.key")
				if err := os.WriteFile(target, key, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, a.fnbKeyPath); err != nil {
					t.Fatal(err)
				}
			case "oversized":
				if err := os.WriteFile(a.fnbKeyPath, key, 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Truncate(a.fnbKeyPath, 1<<30); err != nil {
					t.Fatal(err)
				}
			case "fifo":
				if err := syscall.Mkfifo(a.fnbKeyPath, 0600); err != nil {
					t.Fatal(err)
				}
			case "backup_path":
				a.fnbKeyPath = filepath.Join(a.BackupDir, "fnb.key")
			case "relative_path":
				a.fnbKeyPath = "fnb.key"
			case "backup_symlink":
				if err := os.MkdirAll(a.BackupDir, 0700); err != nil {
					t.Fatal(err)
				}
				linked := filepath.Join(directory, "linked")
				if err := os.Symlink(a.BackupDir, linked); err != nil {
					t.Fatal(err)
				}
				a.fnbKeyPath = filepath.Join(linked, "fnb.key")
			}
			got, err := a.fnbKey(true)
			defer clear(got)
			if kind != "valid" {
				if err == nil || len(got) != 0 {
					t.Fatal("unsafe key storage accepted")
				}
				return
			}
			if err != nil || len(got) != 32 {
				t.Fatal("private key generation failed", err)
			}
			info, err := os.Stat(a.fnbKeyPath)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("generated key permissions")
			}
			restored, err := a.fnbKey(false)
			defer clear(restored)
			if err != nil || !bytes.Equal(got, restored) {
				t.Fatal("generated key not reusable")
			}
		})
	}
}

func TestFNBCredentialEndpointSecurityAndSQLInjection(t *testing.T) {
	e := setup(t)
	e.a.fnbKeyPath = filepath.Join(t.TempDir(), "fnb.key")
	username := "synthetic-audit-user'; DROP TABLE users; --"
	password := "synthetic-audit-secret' OR 1=1; --"
	credentials := map[string]any{"username": username, "password": password, "version": 0}
	endpoints := []struct{ method, path string }{
		{"GET", "/api/fnb"}, {"PUT", "/api/fnb"}, {"DELETE", "/api/fnb"},
		{"PUT", "/api/fnb/debug"}, {"PUT", "/api/fnb/schedule"},
		{"PUT", "/api/fnb/accounts"}, {"POST", "/api/fnb/refresh"},
		{"POST", "/api/fnb/transactions"},
	}
	for _, endpoint := range endpoints {
		for _, boundary := range []string{"anonymous", "non_admin", "missing_csrf", "foreign_origin", "bearer_only", "expired_session"} {
			if endpoint.method == "GET" && (boundary == "missing_csrf" || boundary == "foreign_origin") {
				continue
			}
			t.Run(endpoint.method+endpoint.path+"/"+boundary, func(t *testing.T) {
				raw, _ := json.Marshal(credentials)
				r := httptest.NewRequest(endpoint.method, endpoint.path, bytes.NewReader(raw))
				r.Header.Set("Origin", "http://localhost:8080")
				r.Header.Set("X-CSRF-Token", "csrf")
				expected := http.StatusForbidden
				switch boundary {
				case "anonymous", "bearer_only":
					expected = http.StatusUnauthorized
					if boundary == "bearer_only" {
						r.Header.Set("Authorization", "Bearer "+mcpToken(t, e, 1, false))
					}
				case "non_admin":
					r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token2"})
				case "expired_session":
					expected = http.StatusUnauthorized
					r.AddCookie(&http.Cookie{Name: "finance_session", Value: "expired"})
					if _, err := e.a.DB.Exec("INSERT OR REPLACE INTO sessions VALUES(?,?,?,?)", hash("expired"), 1, "csrf", 1); err != nil {
						t.Fatal(err)
					}
				default:
					r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
					if boundary == "missing_csrf" {
						r.Header.Del("X-CSRF-Token")
					}
					if boundary == "foreign_origin" {
						r.Header.Set("Origin", "https://attacker.invalid")
					}
				}
				w := httptest.NewRecorder()
				e.h.ServeHTTP(w, r)
				status(t, w, expected)
				if strings.Contains(w.Body.String(), username) || strings.Contains(w.Body.String(), password) {
					t.Fatal("credential reflected")
				}
				if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections") != 0 {
					t.Fatal("rejected request saved credentials")
				}
			})
		}
	}
	status(t, e.req(t, 1, "/api/fnb", "PUT", credentials), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users") != 3 {
		t.Fatal("SQL payload changed users")
	}
	var ciphertext []byte
	if err := e.a.DB.QueryRow("SELECT secret FROM fnb_connections WHERE user_id=1").Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte(username)) || bytes.Contains(ciphertext, []byte(password)) {
		t.Fatal("plaintext credentials stored")
	}
	key, err := e.a.fnbKey(false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	opened, err := fnbUnseal(key, ciphertext, 1)
	if err != nil || opened.Username != username || opened.Password != password {
		t.Fatal("SQL payload not stored as literal credential data")
	}
	for _, path := range []string{"/api/fnb", "/api/fnb?user_id=1%27%20OR%201=1%20--", "/api/users", "/api/backups", "/api/settings", "/api/mcp/settings"} {
		w := e.req(t, 1, path, "GET", nil)
		status(t, w, 200)
		if strings.Contains(w.Body.String(), username) || strings.Contains(w.Body.String(), password) || strings.Contains(w.Body.String(), string(key)) {
			t.Fatal("credential or key disclosed in API")
		}
		if w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("sensitive API cacheable")
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE instr(details,?)>0 OR instr(details,?)>0", username, password) != 0 {
		t.Fatal("credentials in audit history")
	}
	if _, err := e.a.DB.Exec("UPDATE users SET admin=1 WHERE id=2"); err != nil {
		t.Fatal(err)
	}
	w := e.req(t, 2, "/api/fnb?user_id=1", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"connection":null`) {
		t.Fatal("other owner connection disclosed")
	}
	for _, payload := range []map[string]any{
		{"username": username, "password": password, "version": 1, "user_id": 2},
		{"username": username, "password": password, "version": "1 OR 1=1"},
		{"username": username, "password": strings.Repeat("x", 513), "version": 1},
	} {
		status(t, e.req(t, 1, "/api/fnb", "PUT", payload), 400)
	}
	status(t, e.req(t, 1, "/api/fnb", "PUT", credentials), 409)
}

func TestFNBKeyNotServedAsStaticFile(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	key, err := e.a.fnbKey(false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	for _, path := range []string{"/config/fnb/fnb.key", "/fnb.key", "/fnb.key.backup", "/%2e%2e/config/fnb/fnb.key", "/assets/%2e%2e/%2e%2e/config/fnb/fnb.key"} {
		r := httptest.NewRequest("GET", path, nil)
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		if w.Code == 200 || bytes.Contains(w.Body.Bytes(), key) {
			t.Fatal("key exposed via static path", path)
		}
	}
}

func TestFNBRejectsCredentialsOverRemoteHTTP(t *testing.T) {
	e := setup(t)
	e.a.PublicURL = "http://finance.example.test"
	e.a.Secure = false
	e.a.fnbKeyPath = filepath.Join(t.TempDir(), "fnb.key")
	r := httptest.NewRequest("PUT", "/api/fnb", strings.NewReader(`{"username":"synthetic","password":"synthetic","version":0}`))
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("Origin", e.a.PublicURL)
	r.Header.Set("X-CSRF-Token", "csrf")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 400)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections") != 0 {
		t.Fatal("remote HTTP saved bank credentials")
	}
	if _, err := os.Stat(e.a.fnbKeyPath); !os.IsNotExist(err) {
		t.Fatal("remote HTTP created key")
	}
}
