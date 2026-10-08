package app

import (
	"context"
	"database/sql"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func securityExec(t *testing.T, e *testEnv, sql string, args ...any) {
	t.Helper()
	if _, err := e.a.DB.Exec(sql, args...); err != nil {
		t.Fatal(err)
	}
}
func seedSecurityAgents(t *testing.T, e *testEnv, id int) {
	t.Helper()
	securityExec(t, e, "INSERT INTO mcp_tokens(id,user_id,name,token_hash,expires_at) VALUES(?,?,'Synthetic agent',?,?)", id, id, fmt.Sprint("agent", id), time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_proposals(id,token_id,user_id,operation,payload,expires_at) VALUES(?, ?, ?, 'transaction.edit','{}',?)", fmt.Sprint("proposal", id), id, id, time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_oauth_codes VALUES(?,?,'client','https://example.test/callback','challenge','resource',?)", fmt.Sprint("code", id), id, time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_access_tokens VALUES(?,?,'resource',?)", fmt.Sprint("access", id), id, time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_refresh_tokens VALUES(?,?,'client','resource',?,0)", fmt.Sprint("refresh", id), id, time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_oauth_clients VALUES('client','Synthetic','[]',?)", time.Now().Add(time.Hour).Unix())
	securityExec(t, e, "INSERT INTO mcp_oauth_requests VALUES('request','client','https://example.test/callback','state','challenge','read',?,?,'session')", time.Now().Add(time.Hour).Unix(), id)
}
func assertSecurityAgentsGone(t *testing.T, e *testEnv, id int) {
	t.Helper()
	for _, table := range []string{"mcp_tokens", "mcp_proposals", "mcp_oauth_codes", "mcp_access_tokens", "mcp_refresh_tokens", "mcp_oauth_requests"} {
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM "+table) != 0 {
			t.Fatalf("%s retained credentials/consent", table)
		}
	}
}
func TestSelfPasswordSecurity(t *testing.T) {
	e := setup(t)
	digest, _ := bcrypt.GenerateFromPassword([]byte("synthetic-old-password"), bcrypt.DefaultCost)
	securityExec(t, e, "UPDATE users SET password=? WHERE id=2", string(digest))
	securityExec(t, e, "INSERT INTO sessions VALUES('other-session',2,'csrf',?)", time.Now().Add(time.Hour).Unix())
	seedSecurityAgents(t, e, 2)
	for _, b := range []map[string]string{
		{"old_password": "wrong", "new_password": "synthetic-new-password"},
		{"old_password": "synthetic-old-password", "new_password": "short"},
		{"old_password": "synthetic-old-password", "new_password": strings.Repeat("é", 37)},
	} {
		status(t, e.req(t, 2, "/api/password", "POST", b), 400)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions WHERE user_id=2") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_tokens WHERE user_id=2") != 1 {
		t.Fatal("failed change revoked access")
	}
	status(t, e.req(t, 2, "/api/password", "POST", map[string]string{"old_password": "synthetic-old-password", "new_password": "synthetic-new-password"}), 200)
	status(t, e.req(t, 2, "/api/me", "GET", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions WHERE user_id=2") != 1 {
		t.Fatal("other sessions retained")
	}
	assertSecurityAgentsGone(t, e, 2)
	status(t, e.req(t, 2, "/api/login", "POST", map[string]string{"username": "other", "password": "synthetic-old-password"}), 401)
	status(t, e.req(t, 2, "/api/login", "POST", map[string]string{"username": "other", "password": "synthetic-new-password"}), 200)
	var details string
	if err := e.a.DB.QueryRow("SELECT details FROM audit WHERE action='password_changed'").Scan(&details); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(details, "synthetic-") {
		t.Fatal("password material in audit")
	}
}
func TestAdminPasswordSecurity(t *testing.T) {
	e := setup(t)
	seedSecurityAgents(t, e, 2)
	b := map[string]any{"new_password": "synthetic-reset-password", "version": 1}
	status(t, e.req(t, 3, "/api/users/2/password", "POST", b), 403)
	status(t, e.req(t, 1, "/api/users/1/password", "POST", b), 400)
	status(t, e.req(t, 1, "/api/users/999/password", "POST", b), 404)
	b["version"] = 2
	status(t, e.req(t, 1, "/api/users/2/password", "POST", b), 409)
	b["version"] = 1
	b["new_password"] = "short"
	status(t, e.req(t, 1, "/api/users/2/password", "POST", b), 400)
	b["new_password"] = "synthetic-reset-password"
	status(t, e.req(t, 1, "/api/users/2/password", "POST", b), 200)
	status(t, e.req(t, 2, "/api/me", "GET", nil), 401)
	status(t, e.req(t, 1, "/api/me", "GET", nil), 200)
	assertSecurityAgentsGone(t, e, 2)
	status(t, e.req(t, 1, "/api/users/2/password", "POST", b), 409)
	status(t, e.req(t, 1, "/api/login", "POST", map[string]string{"username": "other", "password": "synthetic-reset-password"}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE user_id=1 AND entity_id=2 AND action='password_reset'") != 1 {
		t.Fatal("reset attribution missing")
	}
}
func TestUserDeletionRetainsHistoryAndRevokesAccess(t *testing.T) {
	e := setup(t)
	seedSecurityAgents(t, e, 2)
	for _, s := range []string{
		"INSERT INTO grants VALUES(2,2,'editor')",
		"INSERT INTO imports(id,user_id,account_id,name,hash,format,data,status) VALUES(1,2,2,'Synthetic','hash','csv','{}','committed')",
		"INSERT INTO rules(user_id,account_id,pattern,category_id) VALUES(2,2,'Synthetic',1)",
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,import_id,reviewed_by) VALUES(1,2,'2026-10-07',-100,'Synthetic','2026-10-07',-100,'Synthetic','{}',1,2)",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-100)",
		"INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) VALUES(2,1,1)",
		"INSERT INTO audit(user_id,account_id,entity,entity_id,action,details) VALUES(2,2,'transaction',1,'historical','{}')",
		"INSERT INTO fnb_connections(user_id,secret,interval_hours) VALUES(2,x'00',24)",
		"INSERT INTO fnb_discoveries(user_id,bank_id,name,account_id) VALUES(2,'22222222222','Synthetic',2)",
	} {
		securityExec(t, e, s)
	}
	b := map[string]any{"confirm_username": "other", "version": 1}
	status(t, e.req(t, 3, "/api/users/2", "DELETE", b), 403)
	b["confirm_username"] = "wrong"
	status(t, e.req(t, 1, "/api/users/2", "DELETE", b), 400)
	b["confirm_username"] = "other"
	b["version"] = 99
	status(t, e.req(t, 1, "/api/users/2", "DELETE", b), 409)
	b["version"] = 1
	status(t, e.req(t, 1, "/api/users/2", "DELETE", b), 200)
	for _, table := range []string{"grants", "sessions", "transaction_seen", "fnb_connections", "fnb_discoveries"} {
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM "+table+" WHERE user_id=2") != 0 {
			t.Fatalf("%s retained", table)
		}
	}
	assertSecurityAgentsGone(t, e, 2)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE id=2 AND deleted_at IS NOT NULL AND password='' AND disabled=1 AND admin=0 AND budget_member=0") != 1 {
		t.Fatal("identity not retired")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=1 AND amount_cents=-100 AND reviewed_by=2 AND import_id=1") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE user_id=2") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE user_id=2 AND action='historical'") != 1 {
		t.Fatal("history changed")
	}
	if rows, err := data(e.a.DB, "PRAGMA foreign_key_check"); err != nil || len(rows) != 0 {
		t.Fatalf("foreign keys: %v %v", rows, err)
	}
	for _, path := range []string{"/api/users", "/api/users?page=0&page_size=20"} {
		w := e.req(t, 1, path, "GET", nil)
		status(t, w, 200)
		if strings.Contains(w.Body.String(), "other") {
			t.Fatal("deleted user visible")
		}
	}
	status(t, e.req(t, 2, "/api/me", "GET", nil), 401)
	status(t, e.req(t, 1, "/api/users/2", "DELETE", b), 404)
	status(t, e.req(t, 1, "/api/users/2/password", "POST", map[string]any{"new_password": "synthetic-reset-password", "version": 2}), 404)
	status(t, e.req(t, 1, "/api/users/2", "PUT", map[string]any{"username": "other", "version": 2}), 404)
	status(t, e.req(t, 1, "/api/grants", "PUT", map[string]any{"user_id": 2, "account_id": 2, "role": "editor"}), 404)
	if err := e.a.ResetPassword("other", "synthetic-recovery-password"); err == nil {
		t.Fatal("recovered deleted user")
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 1, "/api/setup", "GET", nil), 200)
	if _, err := e.a.CreateUser("other", "synthetic-new-password", false, false); err == nil {
		t.Fatal("reused historical identity")
	}
}
func TestUserDeletionAdministratorAndAtomicity(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 1, "/api/users/1", "DELETE", map[string]any{"confirm_username": "owner", "version": 1}), 400)
	securityExec(t, e, "UPDATE users SET admin=1 WHERE id=2")
	status(t, e.req(t, 1, "/api/users/1", "DELETE", map[string]any{"confirm_username": "owner", "version": 1}), 400)
	status(t, e.req(t, 2, "/api/users/1", "DELETE", map[string]any{"confirm_username": "owner", "version": 1}), 200)
	status(t, e.req(t, 2, "/api/users/2", "DELETE", map[string]any{"confirm_username": "other", "version": 1}), 400)
	status(t, e.req(t, 2, "/api/setup", "GET", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE admin=1 AND disabled=0") != 1 {
		t.Fatal("last admin lost")
	}
}
func TestUserSecurityAuditFailureRollsBack(t *testing.T) {
	for _, action := range []string{"delete", "reset", "self"} {
		t.Run(action, func(t *testing.T) {
			e := setup(t)
			seedSecurityAgents(t, e, 2)
			digest, _ := bcrypt.GenerateFromPassword([]byte("synthetic-old-password"), bcrypt.DefaultCost)
			securityExec(t, e, "UPDATE users SET password=? WHERE id=2", string(digest))
			securityExec(t, e, "CREATE TRIGGER deny_security_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
			switch action {
			case "delete":
				status(t, e.req(t, 1, "/api/users/2", "DELETE", map[string]any{"confirm_username": "other", "version": 1}), 500)
			case "reset":
				status(t, e.req(t, 1, "/api/users/2/password", "POST", map[string]any{"new_password": "synthetic-new-password", "version": 1}), 500)
			case "self":
				status(t, e.req(t, 2, "/api/password", "POST", map[string]string{"old_password": "synthetic-old-password", "new_password": "synthetic-new-password"}), 500)
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE id=2 AND deleted_at IS NULL AND version=1 AND password=?", string(digest)) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions WHERE user_id=2") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_tokens WHERE user_id=2") != 1 {
				t.Fatal("partial security mutation survived audit failure")
			}
		})
	}
}
func TestUserSecurityConcurrentResets(t *testing.T) {
	e := setup(t)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			codes <- e.req(t, 1, "/api/users/2/password", "POST", map[string]any{"new_password": "synthetic-new-password", "version": 1}).Code
		}()
	}
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for code := range codes {
		counts[code]++
	}
	if counts[200] != 1 || counts[409] != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE action='password_reset'") != 1 {
		t.Fatalf("concurrent resets: %v", counts)
	}
}
func TestUserSecurityRechecksSessionAndAdmin(t *testing.T) {
	e := setup(t)
	r := httptest.NewRequest("POST", "/api/users/2/password", nil)
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r = r.WithContext(context.WithValue(r.Context(), authKey, authContext{User: e.owner, CSRF: "csrf"}))
	securityExec(t, e, "UPDATE users SET admin=0 WHERE id=1")
	err := e.a.write(func(tx *sql.Tx) error { return userSecurityActorTx(tx, r, true) })
	if err == nil {
		t.Fatal("stale admin accepted")
	}
	securityExec(t, e, "DELETE FROM sessions WHERE user_id=1")
	err = e.a.write(func(tx *sql.Tx) error { return userSecurityActorTx(tx, r, false) })
	if err == nil {
		t.Fatal("revoked session accepted")
	}
}

func TestUserSecurityCSRFAndMigration(t *testing.T) {
	e := setup(t)
	// Simulate the previous schema and prove additive upgrade preserves identity/session state.
	removePostBaselineFixtureTables(t, e.a.DB)
	securityExec(t, e, "ALTER TABLE users DROP COLUMN deleted_at; DELETE FROM migrations WHERE version>=20; INSERT OR IGNORE INTO migrations VALUES(19)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE deleted_at IS NULL") != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions") != 3 {
		t.Fatal("migration rewrote users/sessions")
	}
	for _, method := range []string{"DELETE", "POST"} {
		path := "/api/users/2"
		if method == "POST" {
			path += "/password"
		}
		r := httptest.NewRequest(method, path, strings.NewReader(`{"confirm_username":"other","new_password":"synthetic-reset-password","version":1}`))
		r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
		r.Header.Set("Origin", e.a.PublicURL)
		r.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		status(t, w, 403)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE id=2 AND version=1 AND deleted_at IS NULL") != 1 {
		t.Fatal("CSRF request mutated user")
	}
}
func TestAdminResetKeepsDisabledStatus(t *testing.T) {
	e := setup(t)
	securityExec(t, e, "UPDATE users SET disabled=1 WHERE id=2")
	status(t, e.req(t, 1, "/api/users/2/password", "POST", map[string]any{"new_password": "synthetic-reset-password", "version": 1}), 200)
	status(t, e.req(t, 1, "/api/login", "POST", map[string]string{"username": "other", "password": "synthetic-reset-password"}), 401)
	if queryInt(e.a.DB, "SELECT disabled FROM users WHERE id=2") != 1 {
		t.Fatal("reset enabled a disabled user")
	}
}
