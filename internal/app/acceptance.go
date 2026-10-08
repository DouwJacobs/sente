package app

import (
	"database/sql"
	"net/http"
)

const acceptanceSQL = `(EXISTS(SELECT 1 FROM allocations WHERE transaction_id=transactions.id)
 AND NOT EXISTS(SELECT 1 FROM allocations WHERE transaction_id=transactions.id AND
 ((category_id IS NULL AND transactions.is_transfer=0) OR (transactions.amount_cents<0 AND allocations.amount_cents>0)
 OR (transactions.amount_cents>0 AND allocations.amount_cents<0) OR (transactions.amount_cents=0 AND allocations.amount_cents!=0)))
 AND (SELECT SUM(amount_cents) FROM allocations WHERE transaction_id=transactions.id)=amount_cents)`

// Classification determines acceptance; seeing a transaction is a personal action.
func acceptCategorized(tx *sql.Tx, u User, account, id int64) error {
	ready := queryInt(tx, "SELECT COUNT(*) FROM transactions WHERE id=? AND "+acceptanceSQL, id) == 1
	state := "pending_review"
	if ready {
		state = "approved"
	}
	old := ""
	if err := tx.QueryRow("SELECT review_state FROM transactions WHERE id=?", id).Scan(&old); err != nil {
		return err
	}
	if _, err := tx.Exec("UPDATE transactions SET review_state=? WHERE id=?", state, id); err != nil {
		return err
	}
	if ready && old != state {
		return audit(tx, u, account, "transaction", id, "automatically_accepted", map[string]any{"reason": "categorized_or_transfer"})
	}
	return nil
}

func markSeen(tx *sql.Tx, user, id, version int64) error {
	_, err := tx.Exec(`INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) VALUES(?,?,?)
 ON CONFLICT(user_id,transaction_id) DO UPDATE SET transaction_version=excluded.transaction_version,seen_at=CURRENT_TIMESTAMP`, user, id, version)
	return err
}

func (a *App) transactionSeen(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Seen  *bool `json:"seen"`
		Items []struct {
			ID      int64 `json:"id"`
			Version int64 `json:"version"`
		} `json:"items"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Seen == nil || len(b.Items) == 0 || len(b.Items) > 100 {
		return fail(400, "Choose seen or unseen and select 1–100 transactions")
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		for _, item := range b.Items {
			var account, version int64
			if err := tx.QueryRow("SELECT account_id,version FROM transactions WHERE id=?", item.ID).Scan(&account, &version); err != nil {
				return fail(404, "Transaction not found")
			}
			if !a.can(tx, u, account, false) || queryInt(tx, "SELECT sync_hidden FROM accounts WHERE id=?", account) != 0 {
				return fail(403, "Account access required")
			}
			if version != item.Version {
				return fail(409, "Transaction changed; reload before marking it seen or unseen")
			}
			if *b.Seen {
				if err := markSeen(tx, u.ID, item.ID, version); err != nil {
					return err
				}
			} else {
				if _, err := tx.Exec("DELETE FROM transaction_seen WHERE user_id=? AND transaction_id=?", u.ID, item.ID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
