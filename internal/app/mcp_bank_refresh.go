package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type bankRefreshInput struct {
	Mode       string  `json:"mode"`
	AccountIDs []int64 `json:"account_ids"`
}

func genericBankAccount(id int64) string { return fmt.Sprintf("Account #%d", id) }

// Numeric SQLite IDs can be reused after deletion. Bind retained jobs to the
// immutable connection credential record, independently of rotating access tokens.
func bankRefreshConnectionKey(q queryer, i mcpIdentity) string {
	var tokenHash string
	if q.QueryRow("SELECT token_hash FROM mcp_tokens WHERE id=? AND user_id=?", i.TokenID, i.User.ID).Scan(&tokenHash) != nil {
		return ""
	}
	return hash(tokenHash)
}

// This evidence is private storage, never a browser or agent output DTO.
type bankRefreshIntent struct {
	ConnectionKey     string   `json:"connection_key"`
	PermissionVersion int64    `json:"permission_version"`
	CredentialHash    string   `json:"credential_hash"`
	Mappings          []string `json:"mappings"`
}

type bankRefreshActor struct {
	Identity mcpIdentity
	Exact    mcpExactChange
	JobID    string
}

func refreshModes(mode string) (balances, transactions bool) {
	return mode == "balances" || mode == "balances_and_transactions", mode == "transactions" || mode == "balances_and_transactions"
}

func validateBankRefreshInput(in mcpChange) error {
	if in.Refresh == nil {
		return fail(400, "Provide refresh mode and explicit account_ids")
	}
	b, t := refreshModes(in.Refresh.Mode)
	if !b && !t {
		return fail(400, "Choose balances, transactions or balances_and_transactions")
	}
	if len(in.Refresh.AccountIDs) < 1 || len(in.Refresh.AccountIDs) > 100 {
		return fail(400, "Select 1–100 accounts")
	}
	if err := distinctPositiveIDs(in.Refresh.AccountIDs, "accounts"); err != nil {
		return err
	}
	only := mcpChange{Operation: "refresh_bank", Refresh: in.Refresh}
	if !reflect.DeepEqual(only, in) {
		return fail(400, "Refresh cannot include unrelated changes")
	}
	return nil
}

func (a *App) bankRefreshReadAccess(q queryer, i mcpIdentity) (User, mcpPermissions, error) {
	var u User
	if err := q.QueryRow("SELECT id,username,admin,budget_member FROM users WHERE id=? AND disabled=0 AND deleted_at IS NULL", i.User.ID).Scan(&u.ID, &u.Username, &u.Admin, &u.Member); err != nil || !u.Admin {
		return u, mcpPermissions{}, fail(403, "Bank refresh requires the connection owner's administrator access")
	}
	p, err := readMCPPermissions(q, mcpIdentity{User: u, TokenID: i.TokenID})
	if err != nil {
		return u, p, err
	}
	if !p.ReadBankRefresh {
		return u, p, fail(403, "Enable bank refresh reads for this connection")
	}
	u.MCPAccounts = p.Constraints.AccountIDs
	return u, p, nil
}

func (a *App) bankRefreshTargets(q queryer, u User, ids []int64) ([]fnbTarget, error) {
	eligible, err := a.fnbTargets(q, u)
	if err != nil {
		return nil, err
	}
	byID := map[int64]fnbTarget{}
	for _, t := range eligible {
		byID[t.ID] = t
	}
	targets := make([]fnbTarget, 0, len(ids))
	for _, id := range ids {
		t, ok := byID[id]
		if !ok {
			return nil, fail(403, "Selected bank account access or mapping is unavailable")
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func (a *App) authorizeBankRefresh(q queryer, i mcpIdentity, exact mcpExactChange) error {
	if err := validateBankRefreshInput(exact.Change); err != nil {
		return err
	}
	u, p, err := a.bankRefreshReadAccess(q, i)
	if err != nil {
		return err
	}
	if !a.mcpTokenActive(q, mcpIdentity{User: u, TokenID: i.TokenID}) {
		return fail(403, "MCP write access revoked")
	}
	b, t := refreshModes(exact.Change.Refresh.Mode)
	if b && !p.Capabilities.RefreshAccounts || t && !p.Capabilities.RefreshTransactions {
		return fail(403, "This connection does not permit this bank refresh")
	}
	if exact.AutomaticApproval && (b && !p.Automatic.RefreshAccounts || t && !p.Automatic.RefreshTransactions) {
		return fail(403, "Automatic bank refresh approval was removed")
	}
	targets, err := a.bankRefreshTargets(q, u, exact.Change.Refresh.AccountIDs)
	if err != nil {
		return err
	}
	if exact.Refresh == nil {
		return fail(409, "Prepare this refresh proposal again")
	}
	if exact.Refresh.ConnectionKey != bankRefreshConnectionKey(q, i) {
		return fail(403, "MCP connection identity changed")
	}
	version := queryInt(q, "SELECT permission_version FROM mcp_tokens WHERE id=? AND user_id=?", i.TokenID, u.ID)
	var secret []byte
	if q.QueryRow("SELECT secret FROM fnb_connections WHERE user_id=?", u.ID).Scan(&secret) != nil || hash(string(secret)) != exact.Refresh.CredentialHash || version != exact.Refresh.PermissionVersion {
		return fail(409, "Bank connection or permissions changed; prepare a new refresh")
	}
	if len(targets) != len(exact.Refresh.Mappings) {
		return fail(409, "Bank mappings changed; prepare a new refresh")
	}
	for n, target := range targets {
		if hash(target.BankID) != exact.Refresh.Mappings[n] {
			return fail(409, "Bank mappings changed; prepare a new refresh")
		}
	}
	return nil
}

func (a *App) prepareBankRefresh(tx *sql.Tx, i mcpIdentity, in mcpChange) (*bankRefreshIntent, error) {
	if err := validateBankRefreshInput(in); err != nil {
		return nil, err
	}
	sort.Slice(in.Refresh.AccountIDs, func(x, y int) bool { return in.Refresh.AccountIDs[x] < in.Refresh.AccountIDs[y] })
	u, _, err := a.bankRefreshReadAccess(tx, i)
	if err != nil {
		return nil, err
	}
	targets, err := a.bankRefreshTargets(tx, u, in.Refresh.AccountIDs)
	if err != nil {
		return nil, err
	}
	var secret []byte
	var state string
	if tx.QueryRow("SELECT secret,state FROM fnb_connections WHERE user_id=?", u.ID).Scan(&secret, &state) != nil {
		return nil, fail(400, "Connect FNB in Accounts & banking first")
	}
	if state != "ready" {
		return nil, fail(409, "Bank connection is busy or needs attention; open Accounts & banking")
	}
	evidence := &bankRefreshIntent{ConnectionKey: bankRefreshConnectionKey(tx, i), PermissionVersion: queryInt(tx, "SELECT permission_version FROM mcp_tokens WHERE id=?", i.TokenID), CredentialHash: hash(string(secret))}
	for _, target := range targets {
		evidence.Mappings = append(evidence.Mappings, hash(target.BankID))
	}
	return evidence, nil
}

type bankRefreshStatusInput struct {
	JobID string `json:"job_id,omitempty"`
}

type bankRefreshAccount struct {
	AccountID            int64  `json:"account_id"`
	Label                string `json:"label"`
	BalancesUpdated      int    `json:"balances_updated"`
	Inserted             int    `json:"inserted"`
	Duplicates           int    `json:"duplicates"`
	Exceptions           int    `json:"exceptions"`
	BalancesComplete     bool   `json:"balances_complete"`
	TransactionsComplete bool   `json:"transactions_complete"`
	Staged               bool   `json:"staged"`
	PossibleGap          bool   `json:"possible_gap"`
}

type bankRefreshStatus struct {
	JobID             string               `json:"job_id,omitempty"`
	Mode              string               `json:"mode,omitempty"`
	Status            string               `json:"status,omitempty"`
	Available         bool                 `json:"available"`
	ReasonCode        string               `json:"reason_code"`
	Recovery          string               `json:"recovery"`
	Accounts          []bankRefreshAccount `json:"accounts"`
	CreatedAt         int64                `json:"created_at,omitempty"`
	StartedAt         int64                `json:"started_at,omitempty"`
	FinishedAt        int64                `json:"finished_at,omitempty"`
	PollAfterSeconds  int                  `json:"poll_after_seconds"`
	RetryAfterSeconds int64                `json:"retry_after_seconds"`
	Totals            bankRefreshAccount   `json:"totals"`
}

func (a *App) readBankRefreshStatus(tx *sql.Tx, i mcpIdentity, jobID string) (bankRefreshStatus, error) {
	out := bankRefreshStatus{Accounts: []bankRefreshAccount{}, PollAfterSeconds: 5}
	u, _, err := a.bankRefreshReadAccess(tx, i)
	if err != nil {
		return out, err
	}
	if jobID != "" {
		var ids, details string
		var started, finished sql.NullInt64
		err = tx.QueryRow("SELECT id,mode,status,reason_code,created_at,started_at,finished_at,account_ids,details FROM bank_refresh_jobs WHERE id=? AND user_id=? AND connection_id=? AND connection_key=?", jobID, u.ID, i.TokenID, bankRefreshConnectionKey(tx, i)).Scan(&out.JobID, &out.Mode, &out.Status, &out.ReasonCode, &out.CreatedAt, &started, &finished, &ids, &details)
		if err != nil {
			return out, fail(404, "Bank refresh result unavailable")
		}
		var targets []int64
		if json.Unmarshal([]byte(ids), &targets) != nil {
			return out, fail(404, "Bank refresh result unavailable")
		}
		if _, err = a.bankRefreshTargets(tx, u, targets); err != nil {
			return bankRefreshStatus{}, fail(404, "Bank refresh result unavailable")
		}
		if details != "" {
			if err = json.Unmarshal([]byte(details), &out.Accounts); err != nil {
				return out, err
			}
		}
		out.StartedAt, out.FinishedAt = started.Int64, finished.Int64
		out.Recovery = bankRefreshRecovery(out.ReasonCode)
		if len(out.Accounts) > 0 {
			out.Totals.BalancesComplete, out.Totals.TransactionsComplete = refreshModes(out.Mode)
		}
		out.Totals.Staged = len(out.Accounts) > 0
		for _, row := range out.Accounts {
			out.Totals.BalancesComplete = out.Totals.BalancesComplete && row.BalancesComplete
			out.Totals.TransactionsComplete = out.Totals.TransactionsComplete && row.TransactionsComplete
			out.Totals.Staged = out.Totals.Staged && row.Staged
			out.Totals.BalancesUpdated += row.BalancesUpdated
			out.Totals.Inserted += row.Inserted
			out.Totals.Duplicates += row.Duplicates
			out.Totals.Exceptions += row.Exceptions
			out.Totals.PossibleGap = out.Totals.PossibleGap || row.PossibleGap
		}
		return out, nil
	}
	var state string
	if tx.QueryRow("SELECT state FROM fnb_connections WHERE user_id=?", u.ID).Scan(&state) != nil {
		out.ReasonCode = "not_connected"
		out.Recovery = "Open Accounts & banking to connect FNB."
		return out, nil
	}
	targets, err := a.fnbTargets(tx, u)
	if err != nil {
		return out, err
	}
	for _, target := range targets {
		if len(out.Accounts) < 100 {
			out.Accounts = append(out.Accounts, bankRefreshAccount{AccountID: target.ID, Label: fmt.Sprintf("Account #%d", target.ID)})
		}
	}
	switch {
	case state == "refreshing":
		out.ReasonCode = "busy"
	case state != "ready":
		out.ReasonCode = "attention_required"
	case !a.bankRefreshRuntimeReady():
		out.ReasonCode = "runtime_unavailable"
	case len(targets) == 0:
		out.ReasonCode = "no_eligible_accounts"
	default:
		if !a.fnbMu.TryLock() {
			out.ReasonCode = "busy"
			break
		}
		a.fnbMu.Unlock()
		key, keyErr := a.fnbKey(false)
		clear(key)
		if keyErr != nil {
			out.ReasonCode = "key_unavailable"
			break
		}
		last := queryInt(tx, "SELECT last_accepted FROM bank_refresh_admissions WHERE user_id=?", u.ID)
		if remain := last + 60 - time.Now().Unix(); remain > 0 {
			out.ReasonCode = "rate_limited"
			out.RetryAfterSeconds = remain
		} else {
			out.Available = true
		}
	}
	out.Recovery = bankRefreshRecovery(out.ReasonCode)
	return out, nil
}

func bankRefreshRecovery(code string) string {
	switch code {
	case "":
		return ""
	case "busy":
		return "Wait for the current bank refresh to finish."
	case "rate_limited":
		return "Wait before requesting another refresh."
	case "interrupted":
		return "Inspect committed results before preparing a new refresh."
	case "authorization_changed":
		return "Review connection permissions and account access before preparing a new refresh."
	case "result_expired":
		return "Detailed results expired; this request will not run again."
	case "import_review":
		return "Open Import activity to review staged exceptions."
	default:
		return "Open Accounts & banking to check the connection and complete any phone approval."
	}
}

func (a *App) mcpBankRefreshStatusTool(ctx context.Context, _ *mcp.CallToolRequest, in bankRefreshStatusInput) (*mcp.CallToolResult, any, error) {
	if len(in.JobID) > 128 {
		return mcpFailure(fail(400, "Invalid bank refresh job ID"))
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return mcpFailure(err)
	}
	defer tx.Rollback()
	out, err := a.readBankRefreshStatus(tx, ctx.Value(mcpKey).(mcpIdentity), in.JobID)
	if err != nil {
		return mcpFailure(err)
	}
	return mcpResult(out), nil, nil
}
