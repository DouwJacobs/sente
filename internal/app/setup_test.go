package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync"
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func emptySetupApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	a, err := Open(filepath.Join(dir, "finance.sqlite"), "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func setupRequest(a *App, username, password, origin, csrf string) *httptest.ResponseRecorder {
	b, _ := json.Marshal(map[string]string{"username": username, "password": password})
	r := httptest.NewRequest("POST", "/api/setup", bytes.NewReader(b))
	r.Header.Set("Origin", origin)
	r.Header.Set("X-CSRF-Token", csrf)
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Handler("").ServeHTTP(w, r)
	return w
}
func setupState(t *testing.T, a *App) map[string]any {
	t.Helper()
	w := httptest.NewRecorder()
	a.Handler("").ServeHTTP(w, httptest.NewRequest("GET", "/api/setup", nil))
	status(t, w, 200)
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("setup status must not be cached")
	}
	var state map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	return state
}
func TestBrowserSetupSecurityAndSession(t *testing.T) {
	a := emptySetupApp(t)
	state := setupState(t, a)
	if state["required"] != true || state["csrf"] == "" {
		t.Fatal("fresh installation should require setup")
	}
	token := state["csrf"].(string)
	for _, attempt := range []struct {
		origin, csrf, password string
		code                   int
	}{
		{"", token, "synthetic-setup-password", 403},
		{"http://evil.example", token, "synthetic-setup-password", 403},
		{a.PublicURL, "", "synthetic-setup-password", 403},
		{a.PublicURL, "wrong", "synthetic-setup-password", 403},
		{a.PublicURL, token, "short", 400},
	} {
		status(t, setupRequest(a, "owner", attempt.password, attempt.origin, attempt.csrf), attempt.code)
	}
	if queryInt(a.DB, "SELECT COUNT(*) FROM users") != 0 {
		t.Fatal("rejected setup created a user")
	}
	w := setupRequest(a, " owner ", "synthetic-setup-password", a.PublicURL, token)
	status(t, w, 200)
	var response struct {
		User User
		CSRF string
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if !response.User.Admin || !response.User.Member || response.User.Username != "owner" || response.CSRF == "" {
		t.Fatal("incorrect setup user/session")
	}
	cookies := w.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode || cookies[0].Value == "" {
		t.Fatal("missing protected session")
	}
	var digest string
	a.DB.QueryRow("SELECT password FROM users").Scan(&digest)
	if bcrypt.CompareHashAndPassword([]byte(digest), []byte("synthetic-setup-password")) != nil {
		t.Fatal("password not hashed correctly")
	}
	r := httptest.NewRequest("GET", "/api/me", nil)
	r.AddCookie(cookies[0])
	me := httptest.NewRecorder()
	a.Handler("").ServeHTTP(me, r)
	status(t, me, 200)
	if queryInt(a.DB, "SELECT COUNT(*) FROM audit WHERE action='setup'") != 1 {
		t.Fatal("setup audit missing")
	}
	state = setupState(t, a)
	if state["required"] != false || state["csrf"] != nil {
		t.Fatal("completed setup must not issue a bootstrap token")
	}
	status(t, setupRequest(a, "attacker", "synthetic-setup-password", a.PublicURL, token), 409)
	// A disabled administrator must not turn password recovery into public setup.
	a.DB.Exec("UPDATE users SET disabled=1")
	if setupState(t, a)["required"] != false {
		t.Fatal("disabled admin reopened setup")
	}
	status(t, setupRequest(a, "attacker", "synthetic-setup-password", a.PublicURL, token), 409)
}
func TestConcurrentBrowserSetup(t *testing.T) {
	a := emptySetupApp(t)
	token := setupState(t, a)["csrf"].(string)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, name := range []string{"owner-one", "owner-two"} {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			codes <- setupRequest(a, name, "synthetic-setup-password", a.PublicURL, token).Code
		}(name)
	}
	wg.Wait()
	close(codes)
	success, closed := 0, 0
	for code := range codes {
		switch code {
		case 200:
			success++
		case 409:
			closed++
		default:
			t.Fatalf("unexpected setup response %d", code)
		}
	}
	if success != 1 || closed != 1 || queryInt(a.DB, "SELECT COUNT(*) FROM users WHERE admin=1") != 1 || queryInt(a.DB, "SELECT COUNT(*) FROM sessions") != 1 {
		t.Fatal("concurrent setup must create exactly one admin and session")
	}
}
