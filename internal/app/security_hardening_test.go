package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Revocation commits after middleware admission, before the handler mutation.
func admittedMutation(t *testing.T, e *testEnv, path, method string, body any, revoke string) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, strings.NewReader(string(raw)))
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.a.protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := e.a.DB.Exec(revoke); err != nil {
			t.Fatal(err)
		}
		e.a.routes().ServeHTTP(w, r)
	}))(w, r)
	return w
}
func TestBrowserAdmittedRevocationPreventsAccountCreation(t *testing.T) {
	for _, tc := range []struct {
		name, sql string
		code      int
	}{
		{"disabled", "UPDATE users SET disabled=1 WHERE id=1", 401},
		{"deleted", "UPDATE users SET deleted_at=CURRENT_TIMESTAMP WHERE id=1", 401},
		{"reset", "DELETE FROM sessions WHERE user_id=1", 401},
		{"demoted", "UPDATE users SET admin=0 WHERE id=1", 403},
		{"expired", "UPDATE sessions SET expires_at=0 WHERE user_id=1", 401},
		{"csrf rotated", "UPDATE sessions SET csrf='rotated' WHERE user_id=1", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			before := queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts")
			w := admittedMutation(t, e, "/api/accounts", "POST", map[string]any{"name": "Synthetic", "bank_id": "99990001", "household": true}, tc.sql)
			status(t, w, tc.code)
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts") != before || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != 0 {
				t.Fatal("revoked request had effects")
			}
		})
	}
}
func TestBrowserCurrentRolesGuardManagementAndBudgetWrites(t *testing.T) {
	for _, tc := range []struct {
		name, path, method string
		body               any
		sql                string
	}{
		{"user", "/api/users", "POST", map[string]any{"username": "synthetic", "password": "synthetic-password", "admin": false, "budget_member": false}, "UPDATE users SET admin=0 WHERE id=1"},
		{"backup", "/api/backups", "POST", nil, "UPDATE users SET admin=0 WHERE id=1"},
		{"grant", "/api/grants", "PUT", map[string]any{"user_id": 2, "account_id": 2, "role": "editor"}, "UPDATE users SET admin=0 WHERE id=1"},
		{"branding", "/api/branding", "PUT", map[string]any{"display_name": "Synthetic", "version": 1}, "UPDATE users SET admin=0 WHERE id=1"},
		{"category", "/api/categories", "POST", map[string]any{"name": "Synthetic category", "kind": "expense"}, "UPDATE users SET budget_member=0 WHERE id=1"},
		{"group", "/api/spending-groups", "POST", map[string]any{"name": "Synthetic group", "color": "blue"}, "UPDATE users SET budget_member=0 WHERE id=1"},
		{"budget", "/api/settings", "PUT", map[string]any{"start_day": 21}, "UPDATE users SET budget_member=0 WHERE id=1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			w := admittedMutation(t, e, tc.path, tc.method, tc.body, tc.sql)
			status(t, w, 403)
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != 0 {
				t.Fatal("revoked role wrote audit")
			}
		})
	}
}
func TestBrowserFreshMembershipAndPrivateGrants(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=1")
	r := httptest.NewRequest("PUT", "/synthetic", nil)
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r = r.WithContext(context.WithValue(r.Context(), authKey, authContext{User: e.owner, CSRF: "csrf"}))
	e.a.DB.Exec("UPDATE users SET budget_member=0,username='renamed' WHERE id=1")
	err := e.a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if u.Member || u.Username != "renamed" || e.a.can(tx, u, 1, true) {
			t.Fatal("stale membership or identity")
		}
		if !e.a.can(tx, u, 2, true) {
			t.Fatal("legitimate private grant lost")
		}
		return audit(tx, u, 2, "account", 2, "synthetic", nil)
	})
	if err != nil {
		t.Fatal(err)
	}
}
func TestBrowserUserCreationAuditRollbackAndRetry(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("CREATE TRIGGER reject_creation_audit BEFORE INSERT ON audit WHEN NEW.entity='user' AND NEW.action='created' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
	body := map[string]any{"username": "  synthetic-new  ", "password": "synthetic-password", "admin": false, "budget_member": true}
	status(t, e.req(t, 1, "/api/users", "POST", body), 500)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM users WHERE username='synthetic-new'") != 0 {
		t.Fatal("unaudited identity survived")
	}
	e.a.DB.Exec("DROP TRIGGER reject_creation_audit")
	status(t, e.req(t, 1, "/api/users", "POST", body), 200)
	status(t, e.req(t, 1, "/api/users", "POST", body), 409)
	var details string
	e.a.DB.QueryRow("SELECT details FROM audit WHERE entity='user' AND action='created'").Scan(&details)
	if strings.Contains(details, "password") {
		t.Fatal("password material in audit")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='user' AND action='created'") != 1 {
		t.Fatal("creation audit missing")
	}
}
func TestAuthBucketsBoundedWithoutOAuthTraffic(t *testing.T) {
	a := &App{}
	now := time.Now()
	for i := 0; i < authBucketLimit; i++ {
		if !a.authAttempt(fmt.Sprint(i), 10, now) {
			t.Fatal("premature capacity denial")
		}
	}
	if a.authAttempt("fresh-peer", 10, now) || len(a.attempts) != authBucketLimit {
		t.Fatal("fresh peer bypassed cap")
	}
	for i := 1; i < 10; i++ {
		if !a.authAttempt("0", 10, now) {
			t.Fatal("existing peer capacity changed")
		}
	}
	if a.authAttempt("0", 10, now) {
		t.Fatal("peer limit bypassed")
	}
	if !a.authAttempt("fresh-peer", 10, now.Add(authWindow)) || len(a.attempts) != 1 {
		t.Fatal("expired buckets did not prune")
	}
	if !a.authAttempt("oauth:/oauth/token:peer", 2, now.Add(authWindow)) {
		t.Fatal("independent oauth bucket denied")
	}
}
func TestCredentialRetentionPreservesLiveRefreshReuseEvidence(t *testing.T) {
	e := setup(t)
	now := time.Now()
	future := now.Add(time.Hour).Unix()
	past := now.Unix() - 1
	statements := []struct {
		q    string
		args []any
	}{
		{"INSERT INTO mcp_oauth_clients VALUES('live','Synthetic','[]',?)", []any{future}},
		{"INSERT INTO mcp_oauth_clients VALUES('expired','Synthetic','[]',?)", []any{past}},
		{"INSERT INTO mcp_tokens(id,user_id,name,token_hash,client_id,expires_at) VALUES(10,1,'Synthetic','live','live',?),(11,1,'Expired','expired','expired',?)", []any{future, past}},
		{"INSERT INTO mcp_refresh_tokens VALUES('used-live',10,'live','resource',?,1),('expired-refresh',10,'live','resource',?,1),('cascaded',11,'expired','resource',?,1)", []any{future, past, future}},
		{"INSERT INTO mcp_access_tokens VALUES('access-live',10,'resource',?),('access-expired',10,'resource',?)", []any{future, past}},
		{"INSERT INTO mcp_oauth_codes VALUES('code-live',10,'live','redirect','challenge','resource',?),('code-expired',10,'live','redirect','challenge','resource',?)", []any{future, past}},
		{"INSERT INTO sessions VALUES('expired-session',1,'csrf',?)", []any{past}},
	}
	for _, s := range statements {
		if _, err := e.a.DB.Exec(s.q, s.args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := e.a.write(func(tx *sql.Tx) error { return pruneCredentialsTx(tx, now) }); err != nil {
		t.Fatal(err)
	}
	for _, s := range []struct {
		q    string
		want int64
	}{
		{"SELECT COUNT(*) FROM mcp_refresh_tokens WHERE token_hash='used-live' AND used=1", 1},
		{"SELECT COUNT(*) FROM mcp_refresh_tokens", 1},
		{"SELECT COUNT(*) FROM mcp_access_tokens", 1},
		{"SELECT COUNT(*) FROM mcp_oauth_codes", 1},
		{"SELECT COUNT(*) FROM mcp_tokens", 1},
		{"SELECT COUNT(*) FROM mcp_oauth_clients", 1},
		{"SELECT COUNT(*) FROM sessions WHERE token='expired-session'", 0},
	} {
		if queryInt(e.a.DB, s.q) != s.want {
			t.Fatalf("retention invariant: %s", s.q)
		}
	}
	e.a.StartCredentialMaintenance()
	e.a.StartCredentialMaintenance()
}

func TestManualFNBRereadsSessionBeforeFinancialPersistence(t *testing.T) {
	e := setup(t)
	e.a.fnbKeyPath = filepath.Join(t.TempDir(), "key")
	status(t, e.req(t, 1, "/api/fnb", "PUT", map[string]any{"username": "synthetic", "password": "synthetic", "version": 0}), 200)
	before := queryInt(e.a.DB, "SELECT COUNT(*) FROM audit")
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, visible bool) (fnbSnapshot, error) {
		if _, err := e.a.DB.Exec("DELETE FROM sessions WHERE user_id=1"); err != nil {
			t.Fatal(err)
		}
		value := "123.45"
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic", BankID: "99999111", Balance: &value}}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", map[string]any{}), 502)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE bank_id='99999111'") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != before {
		t.Fatal("revoked manual run persisted financial results")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required'") != 1 {
		t.Fatal("failed run not recoverable")
	}
}

func TestBrowserMembershipRevocationPreventsTransactionAndImportWrites(t *testing.T) {
	for _, operation := range []string{"transaction", "import"} {
		t.Run(operation, func(t *testing.T) {
			e := setup(t)
			for _, q := range []string{
				"DELETE FROM grants WHERE user_id=1 AND account_id=1",
				"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id) VALUES(900,1,'2026-10-25',-100,'Original','2026-10-25',-100,'Original','{}',1)",
				"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(900,1,-100)",
			} {
				if _, err := e.a.DB.Exec(q); err != nil {
					t.Fatal(err)
				}
			}
			if operation == "transaction" {
				body := map[string]any{"version": 1, "date": "2026-10-25", "amount_cents": -100, "description": "Changed", "assignment": "auto", "allocations": []map[string]any{{"category_id": 1, "amount_cents": -100}}}
				status(t, admittedMutation(t, e, "/api/transactions/900", "PUT", body, "UPDATE users SET budget_member=0 WHERE id=1"), 403)
			} else {
				e.h = e.a.protect(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if _, err := e.a.DB.Exec("UPDATE users SET budget_member=0 WHERE id=1"); err != nil {
						t.Fatal(err)
					}
					e.a.routes().ServeHTTP(w, r)
				}))
				status(t, e.upload(t, 1, 1, "synthetic.ofx", []byte(ofx(ofxRow("synthetic-revoked", "20261025", "-1.00", "Synthetic")))), 403)
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=900 AND description='Original' AND version=1") != 1 {
				t.Fatal("revoked membership had write effects")
			}
		})
	}
}
