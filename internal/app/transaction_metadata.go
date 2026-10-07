package app

import (
	"database/sql"
	"unicode/utf8"
)

func (a *App) validateMetadata(tx *sql.Tx, u User, account int64, b transactionInput) error {
	if b.Note != nil && utf8.RuneCountInString(*b.Note) > 2000 {
		return fail(400, "Transaction note must be at most 2000 characters")
	}
	if b.ClearMerchant && b.MerchantID != nil {
		return fail(400, "Choose or clear a merchant")
	}
	if b.MerchantID != nil && queryInt(tx, "SELECT COUNT(*) FROM merchants WHERE id=? AND (account_id=? OR account_id IS NULL)", *b.MerchantID, account) != 1 {
		return fail(400, "Choose a merchant in this account")
	}
	if b.Tags != nil {
		if len(*b.Tags) > 20 {
			return fail(400, "Choose at most 20 tags")
		}
		seen := map[int64]bool{}
		for _, id := range *b.Tags {
			if seen[id] || queryInt(tx, "SELECT COUNT(*) FROM tags WHERE id=? AND account_id=?", id, account) != 1 {
				return fail(400, "Choose distinct tags in this account")
			}
			seen[id] = true
		}
	}
	return nil
}
func writeTransactionMetadata(tx *sql.Tx, id int64, b transactionInput) error {
	if b.Note != nil {
		if _, e := tx.Exec("UPDATE transactions SET note=? WHERE id=?", *b.Note, id); e != nil {
			return e
		}
	}
	if b.ClearMerchant {
		if _, e := tx.Exec("UPDATE transactions SET merchant_id=NULL WHERE id=?", id); e != nil {
			return e
		}
	} else if b.MerchantID != nil {
		if _, e := tx.Exec("UPDATE transactions SET merchant_id=? WHERE id=?", *b.MerchantID, id); e != nil {
			return e
		}
	}
	if b.Tags != nil {
		if _, e := tx.Exec("DELETE FROM transaction_tags WHERE transaction_id=?", id); e != nil {
			return e
		}
		for _, tag := range *b.Tags {
			if _, e := tx.Exec("INSERT INTO transaction_tags VALUES(?,?)", id, tag); e != nil {
				return e
			}
		}
	}
	return nil
}
func validateArchivedAllocations(tx *sql.Tx, id int64, alloc []Allocation) error {
	assigned := map[int64]int64{}
	for _, l := range alloc {
		if l.CategoryID != nil && queryInt(tx, "SELECT archived FROM categories WHERE id=?", *l.CategoryID) == 1 {
			assigned[*l.CategoryID]++
		}
	}
	for cat, count := range assigned {
		if count > queryInt(tx, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id=?", id, cat) {
			return fail(400, "Archived categories cannot receive new assignments")
		}
	}
	return nil
}
