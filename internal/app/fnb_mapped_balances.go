package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

// Existing mapped accounts only. Unlike discovery this cannot rename/create an
// account, insert a discovery or create a grant.
func (a *App) applyMappedFNBBalances(u User, targets []fnbTarget, snapshot fnbSnapshot, observed string) error {
	if len(snapshot.Accounts) > 100 || snapshot.Error != "" {
		return fmt.Errorf("invalid balance response")
	}
	balances := map[string]int64{}
	wanted := map[string]bool{}
	scoped := fnbSnapshot{}
	for _, target := range targets {
		wanted[target.BankID] = true
	}
	for _, row := range snapshot.Accounts {
		if !wanted[row.BankID] {
			continue
		}
		scoped.Accounts = append(scoped.Accounts, row)
		if _, exists := balances[row.BankID]; exists || row.Balance == nil {
			return fmt.Errorf("missing or repeated balance")
		}
		amount, err := Cents(*row.Balance)
		if err != nil {
			return err
		}
		balances[row.BankID] = amount
	}
	if len(balances) != len(targets) {
		return fmt.Errorf("missing target balance")
	}
	if err := validateFNBSnapshot(scoped); err != nil {
		return err
	}
	stamp, err := time.Parse(time.RFC3339, observed)
	if err != nil {
		return err
	}
	loc, err := time.LoadLocation("Africa/Johannesburg")
	if err != nil {
		return err
	}
	date := stamp.In(loc).Format("2006-01-02")
	return a.fnbWrite(u, func(tx *sql.Tx, actor User) error {
		var raw string
		if err := tx.QueryRow("SELECT details FROM bank_refresh_jobs WHERE id=?", u.bankRefresh.JobID).Scan(&raw); err != nil {
			return err
		}
		var rows []bankRefreshAccount
		if err := json.Unmarshal([]byte(raw), &rows); err != nil {
			return err
		}
		for _, target := range targets {
			if _, err := tx.Exec("UPDATE accounts SET balance_cents=?,balance_date=?,version=version+1 WHERE id=?", balances[target.BankID], date, target.ID); err != nil {
				return err
			}
			if _, err := tx.Exec("UPDATE fnb_discoveries SET balance_cents=?,balance_date=? WHERE user_id=? AND account_id=? AND bank_id=?", balances[target.BankID], date, u.ID, target.ID, target.BankID); err != nil {
				return err
			}
			for n := range rows {
				if rows[n].AccountID == target.ID {
					rows[n].BalancesUpdated = 1
					rows[n].BalancesComplete = true
				}
			}
		}
		if _, err := tx.Exec("UPDATE fnb_connections SET last_success=? WHERE user_id=?", observed, actor.ID); err != nil {
			return err
		}
		return saveBankRefreshDetails(tx, actor, rows)
	})
}

// Completion reads the CURRENT schedule under the write lock. Even after MCP
// revocation, cleanup may clear this run's refreshing marker, never save output.
func (a *App) finishAgentFNB(u User, code string, snapshot fnbSnapshot, balances bool) error {
	return a.write(func(tx *sql.Tx) error {
		actor, err := fnbActorTx(tx, u)
		if err != nil {
			return err
		}
		authorized := a.authorizeBankRefresh(tx, u.bankRefresh.Identity, u.bankRefresh.Exact) == nil
		var interval int
		var daily, calendarJSON string
		if err := tx.QueryRow("SELECT interval_hours,daily_time,schedule_json FROM fnb_connections WHERE user_id=?", u.ID).Scan(&interval, &daily, &calendarJSON); err != nil {
			return err
		}
		calendar, err := parseFNBCalendar(calendarJSON)
		if err != nil {
			return err
		}
		due := nextFNBDue(time.Now(), interval, daily)
		if calendar != nil {
			due = nextFNBCalendarDue(time.Now(), calendar)
		}
		if !authorized {
			_, err = tx.Exec("UPDATE fnb_connections SET state='ready',next_due=?,version=version+1 WHERE user_id=? AND state='refreshing'", due, u.ID)
			return err
		}
		if code != "" {
			_, err = tx.Exec("UPDATE fnb_connections SET state='action_required',last_error=?,last_diagnostics=?,next_due=NULL,version=version+1 WHERE user_id=?", code, fnbDiagnosticJSON(snapshot.Diagnostics), u.ID)
			return err
		}
		if balances {
			_, err = tx.Exec("UPDATE fnb_connections SET state='ready',last_success=?,next_due=?,last_error='',last_diagnostics=?,version=version+1 WHERE user_id=?", time.Now().UTC().Format(time.RFC3339), due, fnbDiagnosticJSON(snapshot.Diagnostics), u.ID)
		} else {
			_, err = tx.Exec("UPDATE fnb_connections SET state='ready',next_due=?,last_error='',last_diagnostics=?,version=version+1 WHERE user_id=?", due, fnbDiagnosticJSON(snapshot.Diagnostics), u.ID)
		}
		if err != nil {
			return err
		}
		return audit(tx, actor, nil, "mcp", u.bankRefresh.Identity.TokenID, "bank_refresh_connection_completed", map[string]any{"job_id": u.bankRefresh.JobID})
	})
}
