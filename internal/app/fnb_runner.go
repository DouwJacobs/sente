package app

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"time"
)

func (a *App) refreshFNB(owner int64, manual bool) error {
	transactions := false
	if !manual {
		var u User
		u.ID = owner
		if err := a.DB.QueryRow("SELECT admin,budget_member FROM users WHERE id=?", owner).Scan(&u.Admin, &u.Member); err != nil {
			return err
		}
		targets, err := a.fnbTargets(a.DB, u)
		if err != nil {
			return err
		}
		transactions = len(targets) > 0
	}
	_, err := a.runFNB(owner, manual, transactions)
	return err
}
func (a *App) runFNB(owner int64, manual, transactions bool, scope ...int64) ([]ParsedFile, error) {
	return a.runFNBRequest(owner, manual, transactions, nil, scope...)
}
func (a *App) runFNBRequest(owner int64, manual, transactions bool, request *http.Request, scope ...int64) ([]ParsedFile, error) {
	opts := fnbRunOptions{Trigger: "scheduler", Transactions: transactions, Balances: !manual || !transactions, Discovery: true, Request: request}
	if manual {
		opts.Trigger = "browser"
	}
	if len(scope) > 0 {
		opts.Selected = scope[0]
	}
	return a.runFNBOptions(owner, opts)
}

type fnbRunOptions struct {
	Trigger      string
	Transactions bool
	Balances     bool
	Discovery    bool
	Request      *http.Request
	Selected     int64
	Reserved     bool
	Actor        *bankRefreshActor
}

func (a *App) runFNBOptions(owner int64, opts fnbRunOptions) ([]ParsedFile, error) {
	if (opts.Trigger == "mcp") != (opts.Actor != nil) || (opts.Actor != nil && opts.Discovery) {
		return nil, fail(403, "Bank refresh actor does not permit discovery")
	}
	manual, transactions, request := opts.Trigger != "scheduler", opts.Transactions, opts.Request
	select {
	case <-a.stop:
		return nil, fail(503, "Tracker is stopping")
	default:
	}
	if !opts.Reserved {
		if !a.fnbMu.TryLock() {
			return nil, fail(409, "An FNB refresh is already running")
		}
		defer a.fnbMu.Unlock()
	}
	selected := opts.Selected
	a.fnbRefreshTransactions.Store(transactions)
	defer a.fnbRefreshTransactions.Store(false)
	a.fnbRefreshAccount.Store(selected)
	defer a.fnbRefreshAccount.Store(0)
	var u User
	var disabled bool
	if err := a.DB.QueryRow("SELECT id,username,admin,budget_member,disabled FROM users WHERE id=?", owner).Scan(&u.ID, &u.Username, &u.Admin, &u.Member, &disabled); err != nil || disabled || !u.Admin {
		return nil, fail(403, "Connection owner must be an enabled administrator")
	}
	if request != nil {
		cookie, err := request.Cookie("finance_session")
		if err != nil {
			return nil, fail(401, "Please sign in again")
		}
		u.browserSession = hash(cookie.Value)
		u.browserCSRF = request.Header.Get("X-CSRF-Token")
	}
	var secret []byte
	var interval int
	var debug bool
	var state, dailyTime, calendarJSON string
	if err := a.DB.QueryRow("SELECT secret,interval_hours,state,debug_browser,daily_time,schedule_json FROM fnb_connections WHERE user_id=?", owner).Scan(&secret, &interval, &state, &debug, &dailyTime, &calendarJSON); err != nil {
		return nil, fail(404, "Connect FNB first")
	}
	calendar, calendarErr := parseFNBCalendar(calendarJSON)
	if calendarErr != nil {
		return nil, fail(503, "Saved refresh schedule is invalid")
	}
	u.fnbCredentialHash = hash(string(secret))
	if opts.Actor != nil {
		u.bankRefresh = opts.Actor
		p, err := readMCPPermissions(a.DB, opts.Actor.Identity)
		if err != nil {
			return nil, err
		}
		u.MCPAccounts = p.Constraints.AccountIDs
		if state != "ready" {
			return nil, fail(409, "Bank connection needs attention")
		}
	}
	if !manual && state != "ready" {
		return nil, nil
	}
	var targets []fnbTarget
	if opts.Actor != nil {
		var err error
		targets, err = a.bankRefreshTargets(a.DB, u, opts.Actor.Exact.Change.Refresh.AccountIDs)
		if err != nil {
			return nil, err
		}
	} else if transactions {
		var err error
		targets, err = a.fnbTargets(a.DB, u)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, fail(400, "No enabled FNB accounts with editor access. Refresh accounts first.")
		}
	}
	var selectedBank string
	if selected > 0 {
		if !ruleAccess(a.DB, a, u, selected) {
			return nil, fail(403, "Account editor access required")
		}
		if err := a.DB.QueryRow("SELECT a.bank_id FROM accounts a JOIN fnb_discoveries d ON d.account_id=a.id AND d.bank_id=a.bank_id WHERE a.id=? AND d.user_id=? AND d.hidden=0", selected, owner).Scan(&selectedBank); err != nil {
			return nil, fail(400, "Account is not connected to your FNB profile")
		}
	}
	key, err := a.fnbKey(false)
	var credentials fnbCredentials
	if err == nil {
		credentials, err = fnbUnseal(key, secret, owner)
	}
	clear(key)
	if err != nil {
		if opts.Actor != nil {
			_ = a.finishAgentFNB(u, "KEY_UNAVAILABLE", fnbSnapshot{}, false)
			return nil, fail(503, "FNB encryption key unavailable")
		}
		a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error='KEY_UNAVAILABLE',next_due=NULL,version=version+1 WHERE user_id=?", owner)
		return nil, fail(503, "FNB encryption key unavailable")
	}
	hiddenRows, err := data(a.DB, "SELECT d.bank_id FROM fnb_discoveries d LEFT JOIN accounts a ON a.id=d.account_id WHERE d.user_id=? AND (d.hidden=1 OR a.sync_hidden=1)", owner)
	if err != nil {
		return nil, err
	}
	if selected > 0 {
		others, e := data(a.DB, "SELECT bank_id FROM fnb_discoveries WHERE user_id=? AND bank_id!=?", owner, selectedBank)
		if e != nil {
			return nil, e
		}
		hiddenRows = append(hiddenRows, others...)
	}
	if opts.Actor != nil {
		allowed := map[string]bool{}
		for _, target := range targets {
			allowed[target.BankID] = true
		}
		others, err := data(a.DB, "SELECT bank_id FROM fnb_discoveries WHERE user_id=?", owner)
		if err != nil {
			return nil, err
		}
		for _, row := range others {
			if !allowed[row["bank_id"].(string)] {
				hiddenRows = append(hiddenRows, row)
			}
		}
	}
	for _, row := range hiddenRows {
		credentials.Hidden = append(credentials.Hidden, row["bank_id"].(string))
	}
	if transactions {
		credentials.RunID = randomToken()
		for _, target := range targets {
			credentials.TransactionAccounts = append(credentials.TransactionAccounts, target.BankID)
		}
	}
	runID := credentials.RunID
	now := time.Now().UTC().Format(time.RFC3339)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	a.fnbCancelMu.Lock()
	a.fnbCancelOwner, a.fnbCancel = owner, cancel
	if opts.Actor != nil {
		a.fnbCancelConnection = opts.Actor.Identity.TokenID
	}
	a.fnbCancelMu.Unlock()
	defer func() {
		cancel()
		a.fnbCancelMu.Lock()
		a.fnbCancelOwner, a.fnbCancel = 0, nil
		a.fnbCancelConnection = 0
		a.fnbCancelMu.Unlock()
	}()
	done := make(chan struct{})
	defer close(done)
	go func() {
		var ticks <-chan time.Time
		if opts.Actor != nil {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			ticks = ticker.C
		}
		for {
			select {
			case <-a.stop:
				cancel()
				return
			case <-done:
				return
			case <-ticks:
				// Covers OAuth revocation/expiry and access changes that have no
				// browser callback. Financial writes still recheck independently.
				tx, err := a.DB.Begin()
				if err == nil {
					err = a.authorizeBankRefresh(tx, opts.Actor.Identity, opts.Actor.Exact)
					tx.Rollback()
				}
				if err != nil {
					cancel()
					return
				}
			}
		}
	}()
	if err := a.fnbWrite(u, func(tx *sql.Tx, actor User) error {
		if opts.Actor != nil {
			if _, err := tx.Exec("UPDATE bank_refresh_jobs SET status='running',started_at=? WHERE id=? AND status='queued'", time.Now().Unix(), opts.Actor.JobID); err != nil {
				return err
			}
		}
		if selected > 0 && !ruleAccess(tx, a, actor, selected) {
			return fail(403, "Account editor access required")
		}
		for _, target := range targets {
			if !ruleAccess(tx, a, actor, target.ID) {
				return fail(403, "Account access changed")
			}
		}
		_, err := tx.Exec("UPDATE fnb_connections SET state='refreshing',last_attempt=?,last_error='',last_diagnostics='{}',version=version+1 WHERE user_id=?", now, owner)
		return err
	}); err != nil {
		return nil, err
	}
	provider := a.fnbProvider
	if provider == nil {
		provider = runFNBProvider
	}
	var previews []ParsedFile
	snapshot, err := provider(ctx, credentials, debug)
	credentials = fnbCredentials{}
	if ctx.Err() != nil {
		snapshot = fnbSnapshot{Error: "CONNECTOR_TIMEOUT", Diagnostics: map[string]int{"refresh_timed_out": 1}}
		err = nil
	}
	code := "REFRESH_FAILED"
	if err == nil && snapshot.Error != "" {
		code = snapshot.Error
		err = fmt.Errorf("provider failure")
	}
	switch code {
	case "CONNECTOR_START_FAILED", "CONNECTOR_RESPONSE_INVALID", "CONNECTOR_TIMEOUT", "SESSION_CONFLICT", "LOGIN_LAYOUT_CHANGED", "APPROVAL_REQUIRED", "ACCOUNT_LAYOUT_CHANGED", "BROWSER_NOT_FOUND", "KEY_UNAVAILABLE", "BALANCE_LAYOUT_CHANGED", "LOGOUT_REQUIRED", "TRANSACTION_LAYOUT_CHANGED", "TRANSACTION_ACCOUNT_MISMATCH", "TRANSACTION_FEE_REVIEW_REQUIRED", "TRANSACTION_ACCOUNT_UNSUPPORTED":
	default:
		code = "REFRESH_FAILED"
	}
	if err == nil {
		now = time.Now().UTC().Format(time.RFC3339)
		if opts.Actor != nil {
			if opts.Balances {
				err = a.applyMappedFNBBalances(u, targets, snapshot, now)
			}
			if err == nil && transactions {
				previews, err = a.stageFNBTransactions(u, targets, snapshot, runID)
				if err == nil {
					err = a.autoCommitFNB(u, previews)
				}
			}
		} else if transactions {
			if opts.Balances {
				err = a.applyFNBSnapshot(u, snapshot, now)
			}
			if err == nil {
				previews, err = a.stageFNBTransactions(u, targets, snapshot, runID)
				if err == nil {
					err = a.autoCommitFNB(u, previews)
				}
			}
		} else {
			if selected > 0 {
				filtered := snapshot.Accounts[:0]
				for _, account := range snapshot.Accounts {
					if account.BankID == selectedBank {
						filtered = append(filtered, account)
					}
				}
				snapshot.Accounts = filtered
				if !ruleAccess(a.DB, a, u, selected) {
					err = fail(403, "Account editor access required")
				} else {
					err = a.applyFNBSnapshot(u, snapshot, now)
				}
			} else {
				err = a.applyFNBSnapshot(u, snapshot, now)
			}
		}
		if err != nil {
			code = "ACCOUNT_LAYOUT_CHANGED"
			if transactions {
				code = "TRANSACTION_LAYOUT_CHANGED"
			}
		}
	}
	if err != nil {
		if opts.Actor != nil {
			_ = a.finishAgentFNB(u, code, snapshot, opts.Balances)
			return nil, bankRefreshRunError{Code: code, Cause: err}
		}
		a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error=?,last_diagnostics=?,next_due=NULL,version=version+1 WHERE user_id=?", code, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
		return nil, fail(502, "FNB refresh needs attention: "+code)
	}
	if opts.Actor != nil {
		return previews, a.finishAgentFNB(u, "", snapshot, opts.Balances)
	}
	due := nextFNBDue(time.Now(), interval, dailyTime)
	if calendar != nil {
		due = nextFNBCalendarDue(time.Now(), calendar)
	}
	if transactions && manual {
		// Account balance dates and last successful balance refresh remain accurate.
		_, err = a.DB.Exec("UPDATE fnb_connections SET state='ready',next_due=?,last_error='',last_diagnostics=?,version=version+1 WHERE user_id=?", due, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
		return previews, err
	}
	_, err = a.DB.Exec("UPDATE fnb_connections SET state='ready',last_success=?,next_due=?,last_error='',last_skipped=?,last_diagnostics=?,version=version+1 WHERE user_id=?", now, due, snapshot.Skipped, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
	return previews, err
}
