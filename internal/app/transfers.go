package app

import (
	"database/sql"
	"net/http"
)

func (a *App) linkTransfer(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Left         int64 `json:"left_id"`
		Right        int64 `json:"right_id"`
		LeftVersion  int64 `json:"left_version"`
		RightVersion int64 `json:"right_version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Left == b.Right {
		return fail(400, "Choose two different transactions")
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		accounts := []int64{}
		amounts := []int64{}
		versions := []int64{b.LeftVersion, b.RightVersion}
		for _, id := range []int64{b.Left, b.Right} {
			var account, amount int64
			if err := tx.QueryRow("SELECT account_id,amount_cents FROM transactions WHERE id=?", id).Scan(&account, &amount); err != nil {
				return fail(404, "Transaction not found")
			}
			if !a.can(tx, u, account, true) {
				return fail(403, "Editor access to both accounts is required")
			}
			accounts = append(accounts, account)
			amounts = append(amounts, amount)
			if queryInt(tx, "SELECT COUNT(*) FROM transfer_links WHERE left_id=? OR right_id=?", id, id) > 0 {
				return fail(409, "Transaction is already linked")
			}
		}
		if accounts[0] == accounts[1] || amounts[0] == 0 || amounts[0] != -amounts[1] {
			return fail(400, "Transfers need different accounts and equal opposite amounts; record fees separately")
		}
		if _, err := tx.Exec("INSERT INTO transfer_links VALUES(?,?)", b.Left, b.Right); err != nil {
			return err
		}
		for i, id := range []int64{b.Left, b.Right} {
			res, err := tx.Exec("UPDATE transactions SET is_transfer=1,review_state='pending_review',reviewed_at=NULL,reviewed_by=NULL,version=version+1 WHERE id=? AND version=?", id, versions[i])
			if err != nil {
				return err
			}
			if err := affected(res); err != nil {
				return err
			}
			if err := acceptCategorized(tx, u, accounts[i], id); err != nil {
				return err
			}
			if err := markSeen(tx, u.ID, id, versions[i]+1); err != nil {
				return err
			}
			if err := audit(tx, u, accounts[i], "transaction", id, "transfer_linked", map[string]any{}); err != nil {
				return err
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
func (a *App) unlinkTransfer(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		var left, right int64
		if err := tx.QueryRow("SELECT left_id,right_id FROM transfer_links WHERE left_id=? OR right_id=?", id, id).Scan(&left, &right); err != nil {
			return fail(404, "Transfer link not found")
		}
		for _, tid := range []int64{left, right} {
			var account int64
			tx.QueryRow("SELECT account_id FROM transactions WHERE id=?", tid).Scan(&account)
			if !a.can(tx, u, account, true) {
				return fail(403, "Editor access to both accounts is required")
			}
			if _, err := tx.Exec("UPDATE transactions SET is_transfer=0,review_state='pending_review',reviewed_at=NULL,reviewed_by=NULL,version=version+1 WHERE id=?", tid); err != nil {
				return err
			}
			if err := acceptCategorized(tx, u, account, tid); err != nil {
				return err
			}
			version := queryInt(tx, "SELECT version FROM transactions WHERE id=?", tid)
			if err := markSeen(tx, u.ID, tid, version); err != nil {
				return err
			}
			if err := audit(tx, u, account, "transaction", tid, "transfer_unlinked", map[string]any{}); err != nil {
				return err
			}
		}
		_, err := tx.Exec("DELETE FROM transfer_links WHERE left_id=?", left)
		return err
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
