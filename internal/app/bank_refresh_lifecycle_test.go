package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/url"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPBankRefreshOAuthRevocationCancelsProvider(t *testing.T) {
	e, token := bankRefreshFixture(t)
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	var client string
	if err = e.a.DB.QueryRow("SELECT client_id FROM mcp_tokens WHERE id=?", identity.TokenID).Scan(&client); err != nil {
		t.Fatal(err)
	}
	entered := make(chan struct{})
	e.a.fnbProvider = func(ctx context.Context, _ fnbCredentials, _ bool) (fnbSnapshot, error) {
		close(entered)
		<-ctx.Done()
		return fnbSnapshot{}, ctx.Err()
	}
	id := mcpPrepare(t, e, token, refreshChange("balances", 1))
	mcpApprove(t, e, 1, id)
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(result)
	}
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("provider did not start")
	}
	status(t, oauthPost(e, "/oauth/revoke", url.Values{"client_id": {client}, "token": {token}}), 200)
	if state := waitBankRefresh(t, e, result["job_id"].(string)); state != "cancelled" {
		t.Fatal(state)
	}
	if queryInt(e.a.DB, "SELECT balance_cents FROM accounts WHERE id=1") != 0 {
		t.Fatal("revoked provider persisted output")
	}
}

func TestMCPBankRefreshBusyReplayAndExpiredResults(t *testing.T) {
	e, token := bankRefreshFixture(t)
	var calls atomic.Int32
	release := make(chan struct{})
	e.a.fnbProvider = func(ctx context.Context, _ fnbCredentials, _ bool) (fnbSnapshot, error) {
		calls.Add(1)
		select {
		case <-release:
		case <-ctx.Done():
		}
		balance := "1.00"
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic account", BankID: "12345678901", Balance: &balance}}}, nil
	}
	id := mcpPrepare(t, e, token, refreshChange("balances", 1))
	mcpApprove(t, e, 1, id)
	second := mcpPrepare(t, e, token, refreshChange("balances", 1))
	mcpApprove(t, e, 1, second)
	e.a.fnbMu.Lock()
	_, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	e.a.fnbMu.Unlock()
	if !failed || calls.Load() != 0 {
		t.Fatal("busy admission ran provider")
	}
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(result)
	}
	job := result["job_id"].(string)
	replay, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || replay["job_id"] != job {
		t.Fatal("running replay did not return same job", replay)
	}
	if _, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": second}); !failed {
		t.Fatal("concurrent job admitted")
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 409)
	close(release)
	if got := waitBankRefresh(t, e, job); got != "succeeded" {
		t.Fatal(got)
	}
	if calls.Load() != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_proposals WHERE id=? AND status='approved'", second) != 1 {
		t.Fatal("replay/busy consumed a request")
	}
	if err := e.a.write(func(tx *sql.Tx) error { return pruneBankRefreshJobs(tx, time.Now().AddDate(0, 0, 91)) }); err != nil {
		t.Fatal(err)
	}
	result, failed = mcpCall(t, e, token, "get_bank_refresh_status", map[string]any{"job_id": job})
	if failed || result["status"] != "result_expired" || len(result["accounts"].([]any)) != 0 {
		t.Fatal(result)
	}
	if result["totals"].(map[string]any)["balances_complete"] != false {
		t.Fatal("expired details claimed complete balances")
	}
	replay, failed = mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || replay["job_id"] != job || calls.Load() != 1 {
		t.Fatal("expired result re-executed", replay)
	}
	identity, err := e.a.mcpIdentity(token)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = e.a.DB.Exec("UPDATE mcp_tokens SET token_hash=? WHERE id=?", hash("replacement-numeric-identity"), identity.TokenID); err != nil {
		t.Fatal(err)
	}
	if v, failed := mcpCall(t, e, token, "get_bank_refresh_status", map[string]any{"job_id": job}); !failed {
		t.Fatal("reused numeric connection identity inherited result", v)
	}
}

func TestMCPBankRefreshReadOnlyNonAdminAndLostAccess(t *testing.T) {
	e, token := bankRefreshFixture(t)
	for _, user := range []int{2, 3} {
		other := mcpToken(t, e, user, true)
		setTestMCPPermissions(t, e, other, bankRefreshPermissions())
		if v, failed := mcpCall(t, e, other, "get_bank_refresh_status", map[string]any{}); !failed {
			t.Fatal("non-admin bank reads", v)
		}
		if v, failed := mcpCall(t, e, other, "prepare_change", refreshChange("balances", 1)); !failed {
			t.Fatal("non-admin bank operation", v)
		}
	}
	read := mcpToken(t, e, 1, false)
	p := bankRefreshPermissions()
	p.Capabilities = mcpCapabilities{}
	setTestMCPPermissions(t, e, read, p)
	if v, failed := mcpCall(t, e, read, "get_bank_refresh_status", map[string]any{}); failed {
		t.Fatal(v)
	}
	if _, failed := mcpCall(t, e, read, "prepare_change", refreshChange("balances", 1)); !failed {
		t.Fatal("read-only refresh")
	}
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		balance := "1.00"
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic account", BankID: "12345678901", Balance: &balance}}}, nil
	}
	id := mcpPrepare(t, e, token, refreshChange("balances", 1))
	mcpApprove(t, e, 1, id)
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed {
		t.Fatal(result)
	}
	job := result["job_id"].(string)
	waitBankRefresh(t, e, job)
	if _, err := e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if v, failed := mcpCall(t, e, token, "get_bank_refresh_status", map[string]any{"job_id": job}); !failed {
		t.Fatal("hidden result disclosed", v)
	}
	if _, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("replay ignored lost access")
	}
	settings := e.req(t, 1, "/api/mcp/settings", "GET", nil)
	status(t, settings, 200)
	var view map[string]any
	if err := json.Unmarshal(settings.Body.Bytes(), &view); err != nil {
		t.Fatal(err)
	}
	if len(view["bank_refresh_jobs"].([]any)) != 0 {
		t.Fatal("browser result ignored scope")
	}
	// Proposal approval must recheck connection versions before any bank action.
	if _, err := e.a.DB.Exec("UPDATE accounts SET sync_hidden=0 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	stale := mcpPrepare(t, e, token, refreshChange("balances", 1))
	setTestMCPPermissions(t, e, token, bankRefreshPermissions())
	status(t, e.req(t, 1, fmt.Sprintf("/api/mcp/proposals/%s", stale), "POST", map[string]any{"approve": true}), 409)
}
