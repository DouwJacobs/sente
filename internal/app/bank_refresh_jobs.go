package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Lock order: reserve fnbMu with TryLock BEFORE a.write/a.mu. Never wait for
// fnbMu in a SQL transaction. The worker inherits that reservation after commit.
func (a *App) applyBankRefresh(i mcpIdentity, proposal string) (*mcp.CallToolResult, any, error) {
	reserved := a.fnbMu.TryLock()
	launched := false
	defer func() {
		if reserved && !launched {
			a.fnbMu.Unlock()
		}
	}()
	var result map[string]any
	var actor bankRefreshActor
	err := a.write(func(tx *sql.Tx) error {
		select {
		case <-a.stop:
			return fail(503, "Tracker is stopping")
		default:
		}
		if !i.Write || !a.mcpTokenActive(tx, i) {
			return fail(403, "MCP write access revoked")
		}
		var raw, status, saved string
		var expires int64
		if tx.QueryRow("SELECT payload,status,result,expires_at FROM mcp_proposals WHERE id=? AND token_id=? AND user_id=? AND operation='refresh_bank'", proposal, i.TokenID, i.User.ID).Scan(&raw, &status, &saved, &expires) != nil {
			return fail(404, "Proposal not found")
		}
		var exact mcpExactChange
		if err := json.Unmarshal([]byte(raw), &exact); err != nil {
			return err
		}
		if err := a.authorizeBankRefresh(tx, i, exact); err != nil {
			return err
		}
		if status == "applied" {
			return json.Unmarshal([]byte(saved), &result)
		}
		if expires <= time.Now().Unix() {
			return fail(409, "Proposal expired; prepare a new refresh")
		}
		if status != "approved" {
			return fail(403, "Approve the refresh in Settings → MCP first")
		}
		if !reserved {
			return fail(409, "Bank refresh is busy; retry this approved proposal later")
		}
		var state string
		if tx.QueryRow("SELECT state FROM fnb_connections WHERE user_id=?", i.User.ID).Scan(&state) != nil || state != "ready" {
			return fail(409, "Bank connection needs attention; open Accounts & banking")
		}
		key, err := a.fnbKey(false)
		if err != nil {
			return fail(503, "Bank connection needs attention; open Accounts & banking")
		}
		clear(key)
		if !a.bankRefreshRuntimeReady() {
			return fail(503, "Bank refresh runtime unavailable; open Accounts & banking")
		}
		now := time.Now().Unix()
		if queryInt(tx, "SELECT last_accepted FROM bank_refresh_admissions WHERE user_id=?", i.User.ID)+60 > now {
			return fail(429, "Wait before requesting another bank refresh")
		}
		id := randomToken()
		ids, _ := json.Marshal(exact.Change.Refresh.AccountIDs)
		accounts := make([]bankRefreshAccount, 0, len(exact.Change.Refresh.AccountIDs))
		for _, id := range exact.Change.Refresh.AccountIDs {
			accounts = append(accounts, newBankRefreshAccount(id))
		}
		details, _ := json.Marshal(accounts)
		if _, err = tx.Exec("INSERT INTO bank_refresh_jobs(id,user_id,connection_id,connection_key,proposal_id,intent,account_ids,mode,status,created_at,details) VALUES(?,?,?,?,?,?,?,?,'queued',?,?)", id, i.User.ID, i.TokenID, exact.Refresh.ConnectionKey, proposal, raw, string(ids), exact.Change.Refresh.Mode, now, string(details)); err != nil {
			return err
		}
		if _, err = tx.Exec("INSERT INTO bank_refresh_admissions(user_id,last_accepted) VALUES(?,?) ON CONFLICT(user_id) DO UPDATE SET last_accepted=excluded.last_accepted", i.User.ID, now); err != nil {
			return err
		}
		result = map[string]any{"operation": "refresh_bank", "job_id": id, "status": "queued", "poll_after_seconds": 5}
		encoded, _ := json.Marshal(result)
		if _, err = tx.Exec("UPDATE mcp_proposals SET status='applied',result=? WHERE id=?", string(encoded), proposal); err != nil {
			return err
		}
		if err = audit(tx, i.User, nil, "mcp", i.TokenID, "bank_refresh_accepted", map[string]any{"proposal_id": proposal, "job_id": id, "mode": exact.Change.Refresh.Mode, "account_ids": exact.Change.Refresh.AccountIDs}); err != nil {
			return err
		}
		actor = bankRefreshActor{Identity: i, Exact: exact, JobID: id}
		return nil
	})
	if err != nil {
		return mcpFailure(err)
	}
	if actor.JobID != "" {
		launched = true
		go func() { defer a.fnbMu.Unlock(); a.executeBankRefresh(actor) }()
	}
	return mcpResult(result), nil, nil
}

func newBankRefreshAccount(id int64) bankRefreshAccount {
	return bankRefreshAccount{AccountID: id, Label: genericBankAccount(id)}
}

func (a *App) executeBankRefresh(actor bankRefreshActor) {
	_, err := a.runFNBOptions(actor.Identity.User.ID, fnbRunOptions{Trigger: "mcp", Transactions: actor.Exact.Change.Refresh.Mode != "balances", Balances: actor.Exact.Change.Refresh.Mode != "transactions", Reserved: true, Actor: &actor})
	status, reason := "succeeded", ""
	if err != nil {
		status, reason = "failed", "refresh_failed"
		var runErr bankRefreshRunError
		if errors.As(err, &runErr) {
			reason = runErr.Code
		}
		var p problem
		if errors.As(err, &p) && (p.Code == 401 || p.Code == 403 || p.Code == 409) {
			status, reason = "cancelled", "authorization_changed"
		}
		select {
		case <-a.stop:
			status, reason = "interrupted", "interrupted"
		default:
		}
	}
	// A terminal receipt contains no bank output. It must remain recordable after
	// revocation, while every financial stage requires current authorization.
	finishErr := a.write(func(tx *sql.Tx) error {
		if err != nil && a.authorizeBankRefresh(tx, actor.Identity, actor.Exact) != nil {
			status, reason = "cancelled", "authorization_changed"
		}
		var raw string
		if err := tx.QueryRow("SELECT details FROM bank_refresh_jobs WHERE id=?", actor.JobID).Scan(&raw); err != nil {
			return err
		}
		var rows []bankRefreshAccount
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return err
		}
		committed, review := false, false
		for _, r := range rows {
			committed = committed || r.BalancesComplete || r.Staged || r.TransactionsComplete
			review = review || r.Exceptions > 0
		}
		if err != nil && committed {
			status = "partial"
		}
		if err == nil && review {
			status, reason = "needs_review", "import_review"
		}
		if _, e := tx.Exec("UPDATE bank_refresh_jobs SET status=?,reason_code=?,finished_at=? WHERE id=?", status, reason, time.Now().Unix(), actor.JobID); e != nil {
			return e
		}
		return audit(tx, actor.Identity.User, nil, "mcp", actor.Identity.TokenID, "bank_refresh_finished", map[string]any{"job_id": actor.JobID, "status": status, "reason_code": reason})
	})
	if finishErr != nil {
		log.Print("Bank refresh terminal receipt failed")
		// Lifecycle-only fallback: no output is persisted or claimed as successful.
		_ = a.write(func(tx *sql.Tx) error {
			_, err := tx.Exec("UPDATE bank_refresh_jobs SET status='interrupted',reason_code='receipt_failed',finished_at=? WHERE id=? AND status IN ('queued','running')", time.Now().Unix(), actor.JobID)
			return err
		})
	}
}

type bankRefreshRunError struct {
	Code  string
	Cause error
}

func (e bankRefreshRunError) Error() string { return e.Code }
func (e bankRefreshRunError) Unwrap() error { return e.Cause }

func (a *App) cancelAgentBankRefresh(owner, connection int64) {
	a.fnbCancelMu.Lock()
	defer a.fnbCancelMu.Unlock()
	if a.fnbCancel != nil && a.fnbCancelOwner == owner && a.fnbCancelConnection == connection {
		a.fnbCancel()
	}
}

// Called in the SAME transaction as staging/auto-import. A failed audit or
// receipt rolls back both financial effects and their counters.
func (a *App) recordBankRefreshImports(tx *sql.Tx, u User, previews []ParsedFile, complete bool) error {
	if u.bankRefresh == nil {
		return nil
	}
	var raw string
	if err := tx.QueryRow("SELECT details FROM bank_refresh_jobs WHERE id=?", u.bankRefresh.JobID).Scan(&raw); err != nil {
		return err
	}
	var rows []bankRefreshAccount
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return err
	}
	for n := range rows {
		for _, p := range previews {
			if hash(p.AccountID) != u.bankRefresh.Exact.Refresh.Mappings[n] {
				continue
			}
			r := &rows[n]
			r.Staged = true
			r.PossibleGap = true
			if complete {
				r.TransactionsComplete = true
				r.Inserted = p.Inserted
				r.Duplicates = p.Skipped
			}
			r.Exceptions = 0
			if !p.AlreadyImported {
				for _, row := range p.Rows {
					if row.Error != "" || row.Duplicate == "possible" || row.Duplicate == "conflict" {
						r.Exceptions++
					}
				}
				if p.Error != "" && r.Exceptions == 0 {
					r.Exceptions = 1
				}
			}
			if complete && p.AlreadyImported && p.Inserted == 0 && p.Skipped == 0 {
				for _, row := range p.Rows {
					if row.Duplicate == "exact_id" || row.Duplicate == "exact_source" {
						r.Duplicates++
					}
				}
			}
		}
	}
	return saveBankRefreshDetails(tx, u, rows)
}

func saveBankRefreshDetails(tx *sql.Tx, u User, rows []bankRefreshAccount) error {
	raw, err := json.Marshal(rows)
	if err != nil {
		return err
	}
	if _, err = tx.Exec("UPDATE bank_refresh_jobs SET details=? WHERE id=? AND status='running'", string(raw), u.bankRefresh.JobID); err != nil {
		return err
	}
	return audit(tx, u, nil, "mcp", u.bankRefresh.Identity.TokenID, "bank_refresh_stage", map[string]any{"job_id": u.bankRefresh.JobID, "accounts": rows})
}

func pruneBankRefreshJobs(tx *sql.Tx, now time.Time) error {
	// Retain a small replay receipt while its originating proposal exists.
	_, err := tx.Exec(`UPDATE bank_refresh_jobs SET status='result_expired',reason_code='result_expired',intent='',details=''
 WHERE status NOT IN ('queued','running','result_expired') AND (created_at<? OR id IN (
 SELECT older.id FROM bank_refresh_jobs older WHERE older.user_id=bank_refresh_jobs.user_id
 ORDER BY older.created_at DESC,older.id DESC LIMIT -1 OFFSET 500))`, now.AddDate(0, 0, -90).Unix())
	if err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM bank_refresh_jobs WHERE status='result_expired' AND NOT EXISTS(SELECT 1 FROM mcp_proposals p WHERE p.id=bank_refresh_jobs.proposal_id)")
	return err
}
