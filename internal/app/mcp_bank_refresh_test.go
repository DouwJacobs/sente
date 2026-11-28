package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func bankRefreshPermissions() mcpPermissions {
	return mcpPermissions{SchemaVersion: 1, Preset: "custom", ReadBankRefresh: true, Capabilities: mcpCapabilities{RefreshAccounts: true, RefreshTransactions: true}, Constraints: mcpConstraints{SelectedAccounts: true, AccountIDs: []int64{1}}}
}

func refreshChange(mode string, ids ...int64) map[string]any {
	return map[string]any{"operation": "refresh_bank", "refresh": map[string]any{"mode": mode, "account_ids": ids}}
}

func bankRefreshFixture(t *testing.T) (*testEnv, string) {
	t.Helper()
	e := setup(t)
	connectFNBTest(t, e)
	transactionTargets(t, e)
	token := mcpToken(t, e, 1, true)
	setTestMCPPermissions(t, e, token, bankRefreshPermissions())
	return e, token
}

func waitBankRefresh(t *testing.T, e *testEnv, job string) string {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		var state string
		if err := e.a.DB.QueryRow("SELECT status FROM bank_refresh_jobs WHERE id=?", job).Scan(&state); err != nil {
			t.Fatal(err)
		}
		if state != "queued" && state != "running" {
			// Wait for the inherited provider reservation to be released too.
			if e.a.fnbMu.TryLock() {
				e.a.fnbMu.Unlock()
				return state
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("bank refresh did not finish")
	return ""
}

func TestMCPBankRefreshModesPrivacyApprovalAndReplay(t *testing.T) {
	for _, mode := range []string{"balances", "transactions", "balances_and_transactions"} {
		t.Run(mode, func(t *testing.T) {
			e, token := bankRefreshFixture(t)
			var calls atomic.Int32
			balance := "123.45"
			e.a.fnbProvider = func(_ context.Context, c fnbCredentials, _ bool) (fnbSnapshot, error) {
				calls.Add(1)
				b, transactions := refreshModes(mode)
				_ = b
				if transactions && (len(c.TransactionAccounts) != 1 || c.TransactionAccounts[0] != "12345678901") {
					return fnbSnapshot{}, fmt.Errorf("wrong transaction scope")
				}
				if !transactions && len(c.TransactionAccounts) != 0 {
					return fnbSnapshot{}, fmt.Errorf("balance refresh fetched transactions")
				}
				s := fnbSnapshot{Accounts: []fnbAccountSnapshot{{BankID: "12345678901", Name: "Changed bank name", Balance: &balance}, {BankID: "22222222222", Name: "Unselected secret", Balance: &balance}, {BankID: "33333333333", Name: "Must not discover", Balance: &balance}}}
				if transactions {
					s.Reports = []FNBReport{transactionReport("12345678901", c.RunID)}
				}
				return s, nil
			}
			availability, failed := mcpCall(t, e, token, "get_bank_refresh_status", map[string]any{})
			if failed || availability["available"] != true || len(availability["accounts"].([]any)) != 1 {
				t.Fatal(availability)
			}
			proposal := mcpPrepare(t, e, token, refreshChange(mode, 1))
			if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal}); !failed {
				t.Fatal("unapproved bank request ran")
			}
			mcpApprove(t, e, 1, proposal)
			if calls.Load() != 0 {
				t.Fatal("prepare/approve contacted bank")
			}
			settings := e.req(t, 1, "/api/mcp/settings", "GET", nil)
			status(t, settings, 200)
			if strings.Contains(settings.Body.String(), "credential_hash") || strings.Contains(settings.Body.String(), "refresh_evidence") {
				t.Fatal("private intent leaked in browser")
			}
			accepted, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal})
			if failed || accepted["status"] != "queued" {
				t.Fatal(accepted)
			}
			job := accepted["job_id"].(string)
			if got := waitBankRefresh(t, e, job); got != "succeeded" {
				t.Fatal(got)
			}
			result, failed := mcpCall(t, e, token, "get_bank_refresh_status", map[string]any{"job_id": job})
			if failed {
				t.Fatal(result)
			}
			raw, _ := json.Marshal(result)
			for _, secret := range []string{"12345678901", "22222222222", "33333333333", "credential", "bank_id", "Changed bank name", "Unselected secret", "Must not discover", "bank-reference", "provenance"} {
				if strings.Contains(string(raw), secret) {
					t.Fatal("leaked", secret, string(raw))
				}
			}
			b, transactions := refreshModes(mode)
			totals := result["totals"].(map[string]any)
			if b && (totals["balances_updated"] != float64(1) || totals["balances_complete"] != true || queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 12345) {
				t.Fatal(result)
			}
			if transactions && (totals["inserted"] != float64(2) || totals["transactions_complete"] != true || totals["possible_gap"] != true) {
				t.Fatal(result)
			}
			if !transactions && queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
				t.Fatal("balance mode changed ledger")
			}
			if !b && (queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE last_success IS NOT NULL") != 0) {
				t.Fatal("transaction refresh changed balance freshness")
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_discoveries") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE name='Changed bank name'") != 0 || queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=2") != 0 {
				t.Fatal("scope/discovery/name side effect")
			}
			replay, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal})
			if failed || replay["job_id"] != job || calls.Load() != 1 {
				t.Fatal("replay restarted provider", replay)
			}
			other := mcpToken(t, e, 1, true)
			setTestMCPPermissions(t, e, other, bankRefreshPermissions())
			if v, failed := mcpCall(t, e, other, "get_bank_refresh_status", map[string]any{"job_id": job}); !failed {
				t.Fatal("cross-connection read", v)
			}
			second := mcpPrepare(t, e, token, refreshChange(mode, 1))
			mcpApprove(t, e, 1, second)
			if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": second}); !failed {
				t.Fatal("cooldown bypass")
			}
			if calls.Load() != 1 {
				t.Fatal("cooldown contacted bank")
			}
		})
	}
}

func TestMCPBankRefreshConsentAndBatchBoundaries(t *testing.T) {
	e, token := bankRefreshFixture(t)
	for _, p := range []mcpPermissions{legacyMCPPermissions(true), legacyMCPPermissions(false), {SchemaVersion: 1, Preset: "yolo", Capabilities: legacyMCPPermissions(true).Capabilities}} {
		setTestMCPPermissions(t, e, token, p)
		if v, failed := mcpCall(t, e, token, "prepare_change", refreshChange("balances", 1)); !failed {
			t.Fatal("legacy consent expanded", v)
		}
	}
	p := bankRefreshPermissions()
	setTestMCPPermissions(t, e, token, p)
	for _, change := range []map[string]any{refreshChange("bad", 1), refreshChange("balances"), refreshChange("balances", 1, 1), refreshChange("balances", 0), refreshChange("balances", 1, 2), refreshChange("balances", 999), {"operation": "refresh_bank", "refresh": map[string]any{"mode": "balances", "account_ids": []int{1}}, "id": 1}} {
		if v, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
			t.Fatal("invalid batch accepted", change, v)
		}
	}
	p.Capabilities.RefreshAccounts = false
	setTestMCPPermissions(t, e, token, p)
	if _, failed := mcpCall(t, e, token, "prepare_change", refreshChange("balances_and_transactions", 1)); !failed {
		t.Fatal("combined lacked both capabilities")
	}
	p = bankRefreshPermissions()
	p.ReadBankRefresh = false
	if _, err := e.a.validateMCPPermissions(e.a.DB, e.owner, p); err == nil {
		t.Fatal("read dependency omitted")
	}
	p = bankRefreshPermissions()
	p.Automatic.RefreshAccounts = true
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, refreshChange("balances", 1))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='approved'", id) != 1 {
		t.Fatal("automatic balance permission ignored")
	}
	id = mcpPrepare(t, e, token, refreshChange("balances_and_transactions", 1))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='pending'", id) != 1 {
		t.Fatal("combined auto approval bypass")
	}
	p.Preset = "yolo"
	p.IncludeFuturePermissions = true
	p.Capabilities = mcpCapabilities{}
	p.Automatic = mcpCapabilities{}
	p.ReadBankRefresh = false
	setTestMCPPermissions(t, e, token, p)
	i, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := readMCPPermissions(e.a.DB, i)
	if err != nil || !resolved.ReadBankRefresh || !resolved.Capabilities.RefreshAccounts || !resolved.Automatic.RefreshTransactions {
		t.Fatal("future YOLO did not resolve", resolved, err)
	}
	newYOLO, err := e.a.validateMCPPermissions(e.a.DB, e.owner, mcpPermissions{SchemaVersion: 1, Preset: "yolo"})
	if err != nil || !newYOLO.ReadBankRefresh || !newYOLO.Capabilities.RefreshTransactions {
		t.Fatal(newYOLO, err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM bank_refresh_jobs") != 0 {
		t.Fatal("preparation created jobs")
	}
}

func TestMCPBankRefreshRevocationWhileFetching(t *testing.T) {
	for _, kind := range []string{"consent", "automatic", "scope", "hidden", "grant", "credential", "disabled", "revoked", "identity"} {
		t.Run(kind, func(t *testing.T) {
			e, token := bankRefreshFixture(t)
			p := bankRefreshPermissions()
			// Exercise private-account grants independently of household membership.
			if _, err := e.a.DB.Exec("UPDATE accounts SET household=0 WHERE id=1; INSERT OR IGNORE INTO grants(user_id,account_id,role) VALUES(1,1,'editor')"); err != nil {
				t.Fatal(err)
			}
			p.Automatic.RefreshAccounts = true
			setTestMCPPermissions(t, e, token, p)
			entered, release := make(chan struct{}), make(chan struct{})
			e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, _ bool) (fnbSnapshot, error) {
				close(entered)
				select {
				case <-release:
				case <-ctx.Done():
				}
				balance := "777.77"
				return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic account", BankID: "12345678901", Balance: &balance}}}, nil
			}
			proposal := mcpPrepare(t, e, token, refreshChange("balances", 1))
			accepted, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": proposal})
			if failed {
				t.Fatal(accepted)
			}
			select {
			case <-entered:
			case <-time.After(10 * time.Second):
				t.Fatal("provider not started")
			}
			i, err := e.a.mcpIdentity(token)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "consent":
				p.Capabilities.RefreshAccounts = false
				setTestMCPPermissions(t, e, token, p)
			case "automatic":
				p.Automatic.RefreshAccounts = false
				setTestMCPPermissions(t, e, token, p)
			case "scope":
				p.Constraints.AccountIDs = []int64{2}
				setTestMCPPermissions(t, e, token, p)
			case "hidden":
				e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1")
			case "grant":
				e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=1")
			case "credential":
				e.a.DB.Exec("UPDATE fnb_connections SET secret=x'00' WHERE user_id=1")
			case "disabled":
				e.a.DB.Exec("UPDATE users SET disabled=1 WHERE id=1")
			case "revoked":
				status(t, e.req(t, 1, fmt.Sprintf("/api/mcp/connections/%d", i.TokenID), "DELETE", nil), 200)
			case "identity":
				e.a.DB.Exec("UPDATE mcp_tokens SET token_hash=? WHERE id=?", hash("replacement-connection"), i.TokenID)
			}
			close(release)
			job := accepted["job_id"].(string)
			if state := waitBankRefresh(t, e, job); state != "cancelled" {
				t.Fatal("revoked job outcome", state)
			}
			if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
				t.Fatal("revoked output persisted")
			}
		})
	}
}

func TestMCPBankRefreshAdmissionAndStageAuditRollback(t *testing.T) {
	for _, stage := range []string{"bank_refresh_accepted", "bank_refresh_stage"} {
		t.Run(stage, func(t *testing.T) {
			e, token := bankRefreshFixture(t)
			var calls atomic.Int32
			e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
				calls.Add(1)
				balance := "1.01"
				return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic account", BankID: "12345678901", Balance: &balance}}}, nil
			}
			id := mcpPrepare(t, e, token, refreshChange("balances", 1))
			mcpApprove(t, e, 1, id)
			_, err := e.a.DB.Exec(fmt.Sprintf("CREATE TRIGGER fail_bank_audit BEFORE INSERT ON audit WHEN NEW.action='%s' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END", stage))
			if err != nil {
				t.Fatal(err)
			}
			result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
			if stage == "bank_refresh_accepted" {
				if !failed || calls.Load() != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM bank_refresh_jobs") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM bank_refresh_admissions") != 0 {
					t.Fatal("admission did not roll back", result)
				}
				if !e.a.fnbMu.TryLock() {
					t.Fatal("reservation leaked")
				}
				e.a.fnbMu.Unlock()
			} else {
				if failed {
					t.Fatal(result)
				}
				if got := waitBankRefresh(t, e, result["job_id"].(string)); got != "failed" {
					t.Fatal(got)
				}
				if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 0 {
					t.Fatal("stage financial rollback failed")
				}
			}
		})
	}
}

func TestMCPBankRefreshPartialReviewAndProviderFailures(t *testing.T) {
	for _, kind := range []string{"partial", "review", "logout", "mfa"} {
		t.Run(kind, func(t *testing.T) {
			e, token := bankRefreshFixture(t)
			if kind == "review" {
				targets, err := e.a.bankRefreshTargets(e.a.DB, e.owner, []int64{1})
				if err != nil {
					t.Fatal(err)
				}
				previews, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{transactionReport("12345678901", "seed-review")}}, "seed-review")
				if err != nil {
					t.Fatal(err)
				}
				if err = e.a.autoCommitFNB(e.owner, previews); err != nil {
					t.Fatal(err)
				}
			}
			e.a.fnbProvider = func(_ context.Context, c fnbCredentials, _ bool) (fnbSnapshot, error) {
				balance := "10.00"
				s := fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic account", BankID: "12345678901", Balance: &balance}}, Reports: []FNBReport{transactionReport("12345678901", c.RunID)}}
				switch kind {
				case "partial":
					s.Reports[0].RunID = "wrong"
				case "review":
					s.Reports[0].Transactions[0].Reference = "changed-reference"
				case "logout":
					s.Error = "LOGOUT_REQUIRED"
				case "mfa":
					s.Error = "APPROVAL_REQUIRED"
				}
				return s, nil
			}
			id := mcpPrepare(t, e, token, refreshChange("balances_and_transactions", 1))
			mcpApprove(t, e, 1, id)
			result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
			if failed {
				t.Fatal(result)
			}
			got := waitBankRefresh(t, e, result["job_id"].(string))
			if kind == "partial" && got != "partial" {
				t.Fatal(got)
			}
			if kind == "partial" && queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE last_success IS NOT NULL") != 1 {
				t.Fatal("committed balance freshness lost after import failure")
			}
			if kind == "review" && got != "needs_review" {
				t.Fatal(got)
			}
			if (kind == "logout" || kind == "mfa") && (got != "failed" || queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 0) {
				t.Fatal("provider failure persisted", got)
			}
		})
	}
}

func TestBankRefreshMigrationRollbackPreservationAndRetention(t *testing.T) {
	e, token := bankRefreshFixture(t)
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	var before string
	e.a.DB.QueryRow("SELECT permissions FROM mcp_tokens WHERE id=?", identity.TokenID).Scan(&before)
	if _, err = e.a.DB.Exec("DROP TABLE bank_refresh_jobs; DROP TABLE bank_refresh_admissions; DELETE FROM migrations WHERE version=37"); err != nil {
		t.Fatal(err)
	}
	steps := append([]schemaMigration{}, schemaMigrations...)
	steps[len(steps)-1].apply = func(tx *sql.Tx, o migrationOrigin) error {
		if err := migrateBankRefreshJobs(tx, o); err != nil {
			return err
		}
		return fmt.Errorf("injected refresh migration failure")
	}
	if err = runSchemaMigrations(e.a.DB, steps, 37); err == nil || !strings.Contains(err.Error(), "injected") {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM sqlite_master WHERE name='bank_refresh_jobs'") != 0 || queryInt(e.a.DB, "SELECT MAX(version) FROM migrations") != 36 {
		t.Fatal("failed migration not rolled back")
	}
	if err = migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if err = migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	var after string
	e.a.DB.QueryRow("SELECT permissions FROM mcp_tokens WHERE id=?", identity.TokenID).Scan(&after)
	if before != after || queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_discoveries") != 2 {
		t.Fatal("upgrade changed data or consent")
	}
	// A running receipt is interrupted at serving startup, never replayed.
	_, err = e.a.DB.Exec("INSERT INTO bank_refresh_jobs(id,user_id,connection_id,proposal_id,intent,account_ids,mode,status,created_at) VALUES('unfinished',1,?,'gone','{}','[1]','balances','running',?)", identity.TokenID, time.Now().Unix())
	if err != nil {
		t.Fatal(err)
	}
	e.a.StartFNBScheduler()
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM bank_refresh_jobs WHERE id='unfinished' AND status='interrupted'") != 1 {
		t.Fatal("startup resumed job")
	}
	if err = e.a.write(func(tx *sql.Tx) error { return pruneBankRefreshJobs(tx, time.Now().AddDate(0, 0, 91)) }); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM bank_refresh_jobs") != 0 {
		t.Fatal("expired orphan receipt retained")
	}
}
