package app

import (
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"

	"finance-tracker/internal/statements"
)

func (a *App) applyFNBSnapshot(u User, snapshot fnbSnapshot, observed string) error {
	if len(snapshot.Accounts) == 0 || len(snapshot.Accounts) > 100 || snapshot.Skipped < 0 || snapshot.Skipped > 100 {
		return fmt.Errorf("invalid discovery")
	}
	seen := map[string]bool{}
	for _, row := range snapshot.Accounts {
		if (!fnbNumber.MatchString(row.BankID) && !(len(row.BankID) <= 64 && row.AccountType == "Credit" && statements.IsMaskedCreditID(row.BankID))) || strings.TrimSpace(row.Name) == "" || len(row.Name) > 100 || seen[row.BankID] {
			return fmt.Errorf("invalid discovery")
		}
		seen[row.BankID] = true
		if row.Balance != nil {
			if _, err := Cents(*row.Balance); err != nil {
				return fmt.Errorf("invalid balance")
			}
		}
	}
	return a.write(func(tx *sql.Tx) error {
		// Recheck permissions after network work; no grant or preference changes occur during sync.
		var active bool
		if err := tx.QueryRow("SELECT admin=1 AND disabled=0,budget_member FROM users WHERE id=?", u.ID).Scan(&active, &u.Member); err != nil || !active {
			return fail(403, "Connection access revoked")
		}
		for _, row := range snapshot.Accounts {
			var balance any = nil
			observedTime, err := time.Parse(time.RFC3339, observed)
			if err != nil {
				return err
			}
			loc, _ := time.LoadLocation("Africa/Johannesburg")
			date := observedTime.In(loc).Format("2006-01-02")
			if row.Balance != nil {
				cents, _ := Cents(*row.Balance)
				balance = cents
			} else {
				date = ""
			}
			var hidden bool
			var linked sql.NullInt64
			err = tx.QueryRow("SELECT hidden,account_id FROM fnb_discoveries WHERE user_id=? AND bank_id=?", u.ID, row.BankID).Scan(&hidden, &linked)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if !hidden {
				if !linked.Valid {
					err := tx.QueryRow("SELECT id FROM accounts WHERE bank_id=?", row.BankID).Scan(&linked)
					if err != nil && err != sql.ErrNoRows {
						return err
					}
				}
				if linked.Valid {
					if !a.can(tx, u, linked.Int64, true) {
						return fail(403, "Account editor access required")
					}
					var accountHidden bool
					tx.QueryRow("SELECT sync_hidden FROM accounts WHERE id=?", linked.Int64).Scan(&accountHidden)
					hidden = accountHidden
				} else {
					res, err := tx.Exec("INSERT INTO accounts(name,bank_id,household) VALUES(?,?,0)", row.Name, row.BankID)
					if err != nil {
						return err
					}
					id, _ := res.LastInsertId()
					linked = sql.NullInt64{Int64: id, Valid: true}
					if _, err = tx.Exec("INSERT INTO grants VALUES(?,?,'editor')", u.ID, id); err != nil {
						return err
					}
				}
				if !hidden {
					if _, err := tx.Exec("UPDATE accounts SET name=?,balance_cents=CASE WHEN ? IS NULL THEN balance_cents ELSE ? END,balance_date=CASE WHEN ? IS NULL THEN balance_date ELSE ? END,version=version+1 WHERE id=?", row.Name, balance, balance, balance, date, linked.Int64); err != nil {
						return err
					}
				}
			}
			if _, err := tx.Exec("INSERT INTO fnb_discoveries(user_id,bank_id,name,balance_cents,balance_date,hidden,account_id) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,bank_id) DO UPDATE SET name=excluded.name,balance_cents=CASE WHEN fnb_discoveries.hidden=1 THEN fnb_discoveries.balance_cents ELSE excluded.balance_cents END,balance_date=CASE WHEN fnb_discoveries.hidden=1 THEN fnb_discoveries.balance_date ELSE excluded.balance_date END,account_id=excluded.account_id,hidden=excluded.hidden", u.ID, row.BankID, row.Name, balance, date, hidden, linked); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "accounts_refreshed", map[string]int{"returned": len(snapshot.Accounts), "skipped_unsupported": snapshot.Skipped})
	})
}

// Secrets travel over stdin, never arguments/environment or error logs. The child
// emits a strict account snapshot or an allowlisted generic failure code.

var fnbNumber = regexp.MustCompile(`^[0-9]{3,64}$`)
