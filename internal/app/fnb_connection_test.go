package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func connectFNBTest(t *testing.T, e *testEnv) {
	t.Helper()
	e.a.fnbKeyPath = filepath.Join(t.TempDir(), "fnb.key")
	status(t, e.req(t, 1, "/api/fnb", "PUT", map[string]any{"username": "synthetic-user", "password": "synthetic-bank-secret", "version": 0}), 200)
}
func fnbDecimal(s string) *string { return &s }
func TestFNBSecretsPermissionsAndSchedule(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	var secret []byte
	e.a.DB.QueryRow("SELECT secret FROM fnb_connections WHERE user_id=1").Scan(&secret)
	if strings.Contains(string(secret), "synthetic") {
		t.Fatal("plaintext secret")
	}
	body := e.req(t, 1, "/api/fnb", "GET", nil)
	status(t, body, 200)
	if strings.Contains(body.Body.String(), "secret") || strings.Contains(body.Body.String(), "synthetic-user") {
		t.Fatal("credential disclosure")
	}
	for _, endpoint := range []string{"/api/fnb", "/api/fnb/refresh", "/api/fnb/schedule", "/api/fnb/accounts"} {
		method := "PUT"
		if endpoint == "/api/fnb/refresh" {
			method = "POST"
		}
		status(t, e.req(t, 2, endpoint, method, map[string]any{}), 403)
	}
	status(t, e.req(t, 2, "/api/fnb", "GET", nil), 403)
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"interval_hours": 24, "version": 1}), 200)
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"interval_hours": 6, "version": 1}), 409)
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"interval_hours": 1, "version": 2}), 400)
	e.a.DB.Exec("UPDATE users SET admin=1 WHERE id=2")
	response := e.req(t, 2, "/api/fnb", "GET", nil)
	status(t, response, 200)
	if !strings.Contains(response.Body.String(), `"connection":null`) {
		t.Fatal("other owner connection exposed")
	}
	key, err := e.a.fnbKey(false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	if _, err = fnbUnseal(key, secret, 2); err == nil {
		t.Fatal("owner not bound")
	}
	modified := append([]byte(nil), secret...)
	modified[len(modified)-1] ^= 1
	if _, err = fnbUnseal(key, modified, 1); err == nil {
		t.Fatal("tamper accepted")
	}
	status(t, e.req(t, 1, "/api/fnb", "DELETE", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections") != 0 {
		t.Fatal("secret remains")
	}
}
func TestFNBRefreshExactBalancesPrivateCreationHideAndRestore(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	snapshot := fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Renamed shared", BankID: "12345678901", Balance: fnbDecimal("-0.01")}, {Name: "New private", BankID: "00123456", Balance: fnbDecimal("1234.56")}}}
	called := 0
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, manual bool) (fnbSnapshot, error) {
		called++
		if c.Username != "synthetic-user" || c.Password != "synthetic-bank-secret" {
			t.Fatal("credentials missing")
		}
		return snapshot, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != -1 || queryInt(e.a.DB, "SELECT household FROM accounts WHERE id=1") != 1 {
		t.Fatal("balance or sharing changed incorrectly")
	}
	id := queryInt(e.a.DB, "SELECT id FROM accounts WHERE bank_id='00123456'")
	if id == 0 || queryInt(e.a.DB, "SELECT household FROM accounts WHERE id=?", id) != 0 || !e.a.can(e.a.DB, e.owner, id, true) {
		t.Fatal("new account is not private/authorized")
	}
	status(t, e.req(t, 1, "/api/fnb/accounts", "PUT", map[string]any{"bank_id": "12345678901", "hidden": true}), 200)
	snapshot.Accounts[0].Balance = fnbDecimal("99.99")
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, manual bool) (fnbSnapshot, error) {
		if len(c.Hidden) != 1 || c.Hidden[0] != "12345678901" {
			t.Fatal("hidden identity not sent to exclusion contract")
		}
		return snapshot, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != -1 {
		t.Fatal("hidden balance updated")
	}
	var rows []map[string]any
	json.Unmarshal(e.req(t, 1, "/api/accounts", "GET", nil).Body.Bytes(), &rows)
	for _, row := range rows {
		if num(row["id"]) == 1 {
			t.Fatal("hidden account shown")
		}
	}
	status(t, e.req(t, 1, "/api/fnb/accounts", "PUT", map[string]any{"bank_id": "12345678901", "hidden": false}), 200)
	if queryInt(e.a.DB, "SELECT household FROM accounts WHERE id=1") != 1 {
		t.Fatal("sharing changed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 0 {
		t.Fatal("ledger imported")
	}
	if called != 1 {
		t.Fatal("unexpected retry")
	}
	// Null bank balance preserves the last dated tracker snapshot.
	snapshot.Accounts[0].Balance = nil
	if err := e.a.applyFNBSnapshot(e.owner, snapshot, "2026-10-02T23:00:00Z"); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != -1 {
		t.Fatal("missing balance erased snapshot")
	}
}
func TestFNBFailureRevocationAtomicityAndMissingKey(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{}, errors.New("synthetic-bank-secret raw provider HTML")
	}
	response := e.req(t, 1, "/api/fnb/refresh", "POST", nil)
	status(t, response, 502)
	if strings.Contains(response.Body.String(), "secret") {
		t.Fatal("raw error leaked")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required' AND next_due IS NULL") != 1 {
		t.Fatal("failed connection still scheduled")
	}
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2")
	snapshot := fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Do not create", BankID: "33333333333", Balance: fnbDecimal("1.00")}, {Name: "Forbidden", BankID: "22222222222", Balance: fnbDecimal("2.00")}}}
	if err := e.a.applyFNBSnapshot(e.owner, snapshot, time.Now().UTC().Format(time.RFC3339)); err == nil {
		t.Fatal("unauthorized overwrite")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE bank_id='33333333333'") != 0 {
		t.Fatal("partial write survived")
	}
	if err := os.Remove(e.a.fnbKeyPath); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 503)
	status(t, e.req(t, 1, "/api/fnb", "PUT", map[string]any{"username": "new synthetic", "password": "new synthetic", "version": 3}), 503)
	if _, err := os.Stat(e.a.fnbKeyPath); !os.IsNotExist(err) {
		t.Fatal("lost key regenerated")
	}
}
func TestFNBHiddenLedgerViewsAndPreservation(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	snapshot := fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Shared", BankID: "12345678901", Balance: fnbDecimal("1.00")}}}
	if err := e.a.applyFNBSnapshot(e.owner, snapshot, time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatal(err)
	}
	e.a.DB.Exec("INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id) VALUES(1,1,'2026-10-21',-100,'Synthetic','2026-10-21',-100,'Synthetic','{}',1)")
	e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-100)")
	status(t, e.req(t, 1, "/api/fnb/accounts", "PUT", map[string]any{"bank_id": "12345678901", "hidden": true}), 200)
	var result map[string]any
	json.Unmarshal(e.req(t, 1, "/api/dashboard?period=1", "GET", nil).Body.Bytes(), &result)
	if num(result["spent_cents"]) != 0 {
		t.Fatal("hidden spending visible")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=1") != 1 {
		t.Fatal("history deleted")
	}
	status(t, e.req(t, 1, "/api/fnb", "DELETE", nil), 200)
	// Discovery preferences survive disconnect and remain available for unhide.
	status(t, e.req(t, 1, "/api/fnb/accounts", "PUT", map[string]any{"bank_id": "12345678901", "hidden": false}), 200)
	json.Unmarshal(e.req(t, 1, "/api/dashboard?period=1", "GET", nil).Body.Bytes(), &result)
	if num(result["spent_cents"]) != 100 {
		t.Fatal("history not restored")
	}
}

func TestFNBScheduleRunsOncePausesOnFailureAndExcludesDisabledOwner(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	transactionTargets(t, e)
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=2")
	e.a.DB.Exec("UPDATE fnb_connections SET interval_hours=24,next_due=1")
	calls := 0
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, manual bool) (fnbSnapshot, error) {
		if manual {
			t.Fatal("scheduled job marked manual")
		}
		calls++
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic", BankID: "12345678901", Balance: fnbDecimal("1.00")}}, Reports: []FNBReport{transactionReport("12345678901", c.RunID)}}, nil
	}
	e.a.refreshDueFNB(time.Now())
	e.a.refreshDueFNB(time.Now())
	if calls != 1 {
		t.Fatal("overlap or repeated completed job")
	}
	if queryInt(e.a.DB, "SELECT next_due FROM fnb_connections") <= time.Now().Unix() {
		t.Fatal("checkpoint not advanced")
	}
	e.a.DB.Exec("UPDATE users SET disabled=1 WHERE id=1;UPDATE fnb_connections SET next_due=1")
	e.a.refreshDueFNB(time.Now())
	if calls != 1 {
		t.Fatal("disabled owner refreshed")
	}
	e.a.DB.Exec("UPDATE users SET disabled=0 WHERE id=1")
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		calls++
		return fnbSnapshot{Error: "APPROVAL_REQUIRED"}, nil
	}
	e.a.refreshDueFNB(time.Now())
	e.a.refreshDueFNB(time.Now())
	if calls != 2 {
		t.Fatal("authentication retried automatically")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required' AND next_due IS NULL") != 1 {
		t.Fatal("failure not paused")
	}
}
func TestFNBConcurrentRefreshAndRevocationDuringProvider(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	entered := make(chan struct{})
	release := make(chan struct{})
	done := make(chan error, 1)
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		close(entered)
		<-release
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Unauthorized", BankID: "12345678901", Balance: fnbDecimal("9.99")}}}, nil
	}
	go func() { done <- e.a.refreshFNB(1, true) }()
	<-entered
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 409)
	e.a.DB.Exec("UPDATE users SET admin=0 WHERE id=1")
	close(release)
	if err := <-done; err == nil {
		t.Fatal("revoked owner updated accounts")
	}
	if queryInt(e.a.DB, "SELECT COALESCE(balance_cents,0) FROM accounts WHERE id=1") != 0 {
		t.Fatal("write after revocation")
	}
}
func TestFNBWindowsStdinTransportSyntheticOnly(t *testing.T) {
	executable := "/mnt/c/Program Files/nodejs/node.exe"
	if _, err := os.Stat(executable); err != nil {
		t.Skip("Windows Node unavailable")
	}
	runner := filepath.Join(t.TempDir(), "synthetic-worker.mjs")
	script := `const {createInterface}=await import('node:readline');const control=createInterface({input:process.stdin});const value=await new Promise(resolve=>control.once('line',line=>resolve(JSON.parse(line))));control.close();process.stdin.destroy();if(value.username!=='synthetic-only'||value.password!=='synthetic-only'||value.hidden[0]!=='123456')process.exit(1);process.stdout.write(JSON.stringify({accounts:[{name:'Synthetic',bank_id:'001234',balance_decimal:'0.01'}],skipped:0}))`
	if err := os.WriteFile(runner, []byte(script), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("FNB_NODE_EXECUTABLE", executable)
	t.Setenv("FNB_RUNNER_PATH", `\\wsl.localhost\Ubuntu`+strings.ReplaceAll(runner, "/", `\`))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	result, err := runFNBProvider(ctx, fnbCredentials{Username: "synthetic-only", Password: "synthetic-only", Hidden: []string{"123456"}}, false)
	if err != nil || len(result.Accounts) != 1 || result.Accounts[0].BankID != "001234" {
		t.Fatalf("synthetic transport failed: %v", err)
	}
}

func TestFNBRestartPreferencesAndInterruptedRecovery(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "finance.sqlite")
	keyPath := filepath.Join(t.TempDir(), "key")
	a, err := Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	a.fnbKeyPath = keyPath
	a.DB.Exec("INSERT INTO users(id,username,password,admin,budget_member) VALUES(1,'owner','unused',1,1)")
	key, err := a.fnbKey(true)
	if err != nil {
		t.Fatal(err)
	}
	plain, _ := json.Marshal(fnbCredentials{Username: "synthetic", Password: "synthetic-secret"})
	secret, err := fnbSeal(key, plain, 1)
	if err != nil {
		t.Fatal(err)
	}
	clear(plain)
	clear(key)
	a.DB.Exec("INSERT INTO fnb_connections(user_id,secret,interval_hours,state,next_due) VALUES(1,?,24,'refreshing',1)", secret)
	a.DB.Exec("INSERT INTO fnb_discoveries(user_id,bank_id,name,hidden) VALUES(1,'123456','Synthetic hidden',1)")
	// Simulate the preceding schema and preserve stored secret/preferences.
	removePostBaselineFixtureTables(t, a.DB)
	if _, err := a.DB.Exec("ALTER TABLE fnb_connections DROP COLUMN debug_browser;ALTER TABLE fnb_connections DROP COLUMN last_diagnostics;DELETE FROM migrations WHERE version>=6;INSERT OR IGNORE INTO migrations VALUES(5)"); err != nil {
		t.Fatal(err)
	}
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	a, err = Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.fnbKeyPath = keyPath
	a.StartFNBScheduler()
	if queryInt(a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required' AND next_due IS NULL AND interval_hours=24") != 1 {
		t.Fatal("interrupted job recovered incorrectly")
	}
	if queryInt(a.DB, "SELECT hidden FROM fnb_discoveries WHERE bank_id='123456'") != 1 {
		t.Fatal("hidden preference lost")
	}
	var saved []byte
	a.DB.QueryRow("SELECT secret FROM fnb_connections").Scan(&saved)
	key, err = a.fnbKey(false)
	if err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	credentials, err := fnbUnseal(key, saved, 1)
	if err != nil || credentials.Username != "synthetic" {
		t.Fatal("secret not persisted")
	}
}

func TestFNBHeadlessDefaultSavedDebugAndSafeDiagnostics(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	var visible []bool
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, debug bool) (fnbSnapshot, error) {
		visible = append(visible, debug)
		return fnbSnapshot{Error: "BALANCE_LAYOUT_CHANGED", Diagnostics: map[string]int{"ledger_nodes": 12, "missing_rows": 1, "password": 999, "other_format": -1}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 502)
	if len(visible) != 1 || visible[0] {
		t.Fatal("manual refresh must default headless")
	}
	status(t, e.req(t, 2, "/api/fnb/debug", "PUT", map[string]any{"debug_browser": true, "version": 3}), 403)
	status(t, e.req(t, 1, "/api/fnb/debug", "PUT", map[string]any{"debug_browser": true, "version": 1}), 409)
	status(t, e.req(t, 1, "/api/fnb/debug", "PUT", map[string]any{"debug_browser": true, "version": 3}), 200)
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 502)
	if !visible[1] {
		t.Fatal("saved debug ignored")
	}
	response := e.req(t, 1, "/api/fnb", "GET", nil)
	status(t, response, 200)
	var result struct {
		Connection struct {
			Debug       int            `json:"debug_browser"`
			Diagnostics map[string]int `json:"last_diagnostics"`
		}
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Connection.Debug != 1 || result.Connection.Diagnostics["missing_rows"] != 1 || len(result.Connection.Diagnostics) != 2 {
		t.Fatal("diagnostic allowlist failed")
	}
	version := queryInt(e.a.DB, "SELECT version FROM fnb_connections WHERE user_id=1")
	status(t, e.req(t, 1, "/api/fnb/debug", "PUT", map[string]any{"debug_browser": false, "version": version}), 200)
	e.a.DB.Exec("UPDATE fnb_connections SET state='ready',interval_hours=24,next_due=1")
	e.a.refreshDueFNB(time.Now())
	if len(visible) != 3 || visible[2] {
		t.Fatal("schedule did not honor headless setting")
	}
}

func TestFNBLogoutFailurePausesWithoutWritingAccounts(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{Error: "LOGOUT_REQUIRED", Diagnostics: map[string]int{"evaluation_failed": 1}, Accounts: []fnbAccountSnapshot{{Name: "Do not save", BankID: "12345678901", Balance: fnbDecimal("9.99")}}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 502)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE last_error='LOGOUT_REQUIRED' AND state='action_required' AND next_due IS NULL") != 1 {
		t.Fatal("logout failure not paused")
	}
	if queryInt(e.a.DB, "SELECT COALESCE(balance_cents,0) FROM accounts WHERE id=1") != 0 {
		t.Fatal("snapshot committed before logout")
	}
}

func TestFNBPreviousSessionPausesWithoutWritingAccounts(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{Error: "SESSION_CONFLICT", Accounts: []fnbAccountSnapshot{{Name: "Do not save", BankID: "12345678901", Balance: fnbDecimal("9.99")}}}, nil
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 502)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE last_error='SESSION_CONFLICT' AND state='action_required' AND next_due IS NULL") != 1 {
		t.Fatal("previous session failure not paused")
	}
	if queryInt(e.a.DB, "SELECT COALESCE(balance_cents,0) FROM accounts WHERE id=1") != 0 {
		t.Fatal("failed session snapshot committed")
	}
}
