package app

import (
	"database/sql"
	"net/http"
	"strings"

	"finance-tracker/internal/ledger"
)

type Allocation = ledger.Allocation

func validateAllocations(q queryer, amount int64, alloc []Allocation, approve, transfer bool) error {
	return ledger.ValidateAllocations(amount, alloc, approve, transfer, func(id int64) bool {
		return queryInt(q, "SELECT COUNT(*) FROM categories WHERE id=?", id) == 1
	})
}

type transactionInput struct {
	Version         int64        `json:"version"`
	Note            *string      `json:"note,omitempty"`
	MerchantID      *int64       `json:"merchant_id,omitempty"`
	ClearMerchant   bool         `json:"clear_merchant,omitempty"`
	Tags            *[]int64     `json:"tag_ids,omitempty"`
	Approve         bool         `json:"approve"`
	Date            string       `json:"date"`
	Amount          int64        `json:"amount_cents"`
	Description     string       `json:"description"`
	Allocations     []Allocation `json:"allocations"`
	Transfer        bool         `json:"is_transfer"`
	Assignment      string       `json:"assignment"`
	PeriodID        *int64       `json:"period_id"`
	SpendingGroupID *int64       `json:"spending_group_id"`
	Rule            *struct {
		Pattern string `json:"pattern"`
	} `json:"rule"`
}

func (a *App) editTransaction(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	id := parseID(r)
	var b transactionInput
	if err := decode(r, &b); err != nil {
		return err
	}
	application := pendingRuleResult{}
	err := a.write(func(tx *sql.Tx) error {
		var err error
		application, err = a.editTransactionTx(tx, u, id, b)
		return err
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"ok": true, "pending_rule": application})
	return nil
}

func (a *App) editTransactionTx(tx *sql.Tx, u User, id int64, b transactionInput) (pendingRuleResult, error) {
	if !validDate(b.Date) || strings.TrimSpace(b.Description) == "" || len(b.Description) > 1000 || b.Amount > 900000000000000 || b.Amount < -900000000000000 {
		return pendingRuleResult{}, fail(400, "Provide a valid date, amount, and description")
	}
	if b.Assignment != "auto" && b.Assignment != "manual" && b.Assignment != "outside" {
		return pendingRuleResult{}, fail(400, "Invalid period assignment")
	}
	application := pendingRuleResult{}
	var account, oldAmount int64
	var oldPeriod any
	var oldAssignment string
	var oldTransfer bool
	if err := tx.QueryRow("SELECT account_id,amount_cents,is_transfer,period_id,assignment FROM transactions WHERE id=?", id).Scan(&account, &oldAmount, &oldTransfer, &oldPeriod, &oldAssignment); err != nil {
		return application, fail(404, "Transaction not found")
	}
	if !a.can(tx, u, account, true) {
		return application, fail(403, "Account editor access required")
	}
	if err := a.validateMetadata(tx, u, account, b); err != nil {
		return application, err
	}
	if err := validateArchivedAllocations(tx, id, b.Allocations); err != nil {
		return application, err
	}
	if b.SpendingGroupID != nil && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.SpendingGroupID) != 1 {
		return application, fail(400, "Choose an existing spending group")
	}
	linked := queryInt(tx, "SELECT COUNT(*) FROM transfer_links WHERE left_id=? OR right_id=?", id, id) > 0
	if linked && (b.Amount != oldAmount || !b.Transfer) {
		return application, fail(400, "Unlink the transfer before changing its amount or designation")
	}
	if err := validateAllocations(tx, b.Amount, b.Allocations, b.Approve, b.Transfer); err != nil {
		return application, err
	}
	var period any
	if !u.Member {
		period = oldPeriod
		b.Assignment = oldAssignment
	} else if b.Assignment == "auto" {
		period = autoPeriod(tx, account, b.Date)
	} else if b.Assignment == "manual" {
		if err := requireMember(u); err != nil {
			return application, err
		}
		if queryInt(tx, "SELECT household FROM accounts WHERE id=?", account) != 1 {
			return application, fail(400, "Private accounts are outside the household budget")
		}
		if b.PeriodID == nil || queryInt(tx, "SELECT COUNT(*) FROM periods WHERE id=?", *b.PeriodID) != 1 {
			return application, fail(400, "Choose an existing period")
		}
		period = *b.PeriodID
	}
	res, err := tx.Exec("UPDATE transactions SET date=?,amount_cents=?,description=?,is_transfer=?,spending_group_id=?,period_id=?,assignment=?,review_state='pending_review',reviewed_at=NULL,reviewed_by=NULL,version=version+1 WHERE id=? AND version=?", b.Date, b.Amount, strings.TrimSpace(b.Description), b.Transfer, b.SpendingGroupID, period, b.Assignment, id, b.Version)
	if err != nil {
		return application, err
	}
	if err := affected(res); err != nil {
		return application, err
	}
	if _, err := tx.Exec("DELETE FROM allocations WHERE transaction_id=?", id); err != nil {
		return application, err
	}
	for _, l := range b.Allocations {
		if _, err := tx.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents,note) VALUES(?,?,?,?)", id, l.CategoryID, l.Amount, l.Note); err != nil {
			return application, err
		}
	}
	if err := writeTransactionMetadata(tx, id, b); err != nil {
		return application, err
	}
	if b.Rule != nil {
		if b.Transfer || len(b.Allocations) != 1 || b.Allocations[0].CategoryID == nil || b.Amount == 0 {
			return application, fail(400, "Rules need a single categorized transaction that is not a transfer")
		}
		direction := "debit"
		if b.Amount > 0 {
			direction = "credit"
		}
		rule := ruleInput{AccountID: account, Pattern: b.Rule.Pattern, CategoryID: *b.Allocations[0].CategoryID, SpendingGroupID: b.SpendingGroupID, Direction: direction}
		var ruleID int64
		existing, err := data(tx, "SELECT id,pattern,version,priority FROM rules WHERE account_id=? AND direction=?", account, direction)
		if err != nil {
			return application, err
		}
		for _, old := range existing {
			if normalize(old["pattern"].(string)) != normalize(rule.Pattern) {
				continue
			}
			if ruleID != 0 {
				return application, fail(409, "Multiple rules use this description. Edit the rules before saving another.")
			}
			ruleID = num(old["id"])
			rule.Version = num(old["version"])
			rule.Priority = int(num(old["priority"]))
		}
		if err := a.writeRule(tx, u, &ruleID, &rule); err != nil {
			return application, err
		}
		application, err = a.applyRuleToPending(tx, u, account, ruleID, id)
		if err != nil {
			return application, err
		}
	}
	if err := acceptCategorized(tx, u, account, id); err != nil {
		return application, err
	}
	if err := markSeen(tx, u.ID, id, b.Version+1); err != nil {
		return application, err
	}
	if err := audit(tx, u, account, "transaction", id, "edited", b); err != nil {
		return application, err
	}
	if b.Approve {
		if _, err := tx.Exec("UPDATE transactions SET review_state='approved',reviewed_by=?,reviewed_at=CURRENT_TIMESTAMP WHERE id=?", u.ID, id); err != nil {
			return application, err
		}
		return application, audit(tx, u, account, "transaction", id, "approved", map[string]any{})
	}
	return application, nil
}
