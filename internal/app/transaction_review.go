package app

import (
	"database/sql"
	"net/http"
)

func (a *App) review(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	var b struct {
		Items []struct {
			ID      int64 `json:"id"`
			Version int64 `json:"version"`
		} `json:"items"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.Items) == 0 || len(b.Items) > 100 {
		return fail(400, "Select 1–100 transactions")
	}
	err := a.write(func(tx *sql.Tx) error {
		for _, item := range b.Items {
			var account, amount int64
			var transfer bool
			if err := tx.QueryRow("SELECT account_id,amount_cents,is_transfer FROM transactions WHERE id=?", item.ID).Scan(&account, &amount, &transfer); err != nil {
				return fail(404, "Transaction not found")
			}
			if !a.can(tx, u, account, true) {
				return fail(403, "Account editor access required")
			}
			rows, err := data(tx, "SELECT category_id,amount_cents,note FROM allocations WHERE transaction_id=?", item.ID)
			if err != nil {
				return err
			}
			alloc := []Allocation{}
			for _, row := range rows {
				v := Allocation{Amount: num(row["amount_cents"]), Note: row["note"].(string)}
				if row["category_id"] != nil {
					id := num(row["category_id"])
					v.CategoryID = &id
				}
				alloc = append(alloc, v)
			}
			if err := validateAllocations(tx, amount, alloc, true, transfer); err != nil {
				return err
			}
			res, err := tx.Exec("UPDATE transactions SET review_state='approved',reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP,version=version+1 WHERE id=? AND version=? AND review_state='pending_review'", u.ID, item.ID, item.Version)
			if err != nil {
				return err
			}
			if err := affected(res); err != nil {
				return err
			}
			if err := markSeen(tx, u.ID, item.ID, item.Version+1); err != nil {
				return err
			}
			if err := audit(tx, u, account, "transaction", item.ID, "approved", map[string]any{}); err != nil {
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
