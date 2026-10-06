package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

func autoPeriod(q queryer, account int64, date string) any {
	if queryInt(q, "SELECT household FROM accounts WHERE id=?", account) != 1 {
		return nil
	}
	var id int64
	err := q.QueryRow("SELECT id FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", date, date).Scan(&id)
	if err != nil {
		return nil
	}
	return id
}
func reassign(q queryer) error {
	_, err := q.Exec("UPDATE transactions SET period_id=CASE WHEN (SELECT household FROM accounts WHERE id=account_id)=1 THEN (SELECT id FROM periods WHERE start_date<=transactions.date AND end_date>=transactions.date ORDER BY start_date DESC,id DESC LIMIT 1) ELSE NULL END,version=version+1 WHERE assignment='auto'")
	return err
}

// transactionQuery is the common authorized ledger scope for browser and MCP reads.
func (a *App) transactionQuery(q queryer, r *http.Request, u User) (string, []any, error) {
	seenSQL := transactionSeenSQL(u)
	sqlText := "SELECT " + seenSQL + " seen,t.*,a.name account_name,a.household,a.bank_id,(SELECT name FROM merchants WHERE id=t.merchant_id) merchant_name,(SELECT logo_data FROM merchants WHERE id=t.merchant_id) merchant_logo,g.name spending_group_name,g.color spending_group_color,p.name period_name,CASE WHEN p.id IS NOT NULL AND (t.date<p.start_date OR t.date>p.end_date) THEN 1 ELSE 0 END outside_period FROM transactions t JOIN accounts a ON a.id=t.account_id LEFT JOIN periods p ON p.id=t.period_id LEFT JOIN spending_groups g ON g.id=t.spending_group_id WHERE " + accountAccessSQL(u)
	args := []any{u.Member, u.ID}
	params := r.URL.Query()
	if seen := params.Get("seen"); seen != "" {
		if seen != "0" && seen != "1" {
			return "", nil, fail(400, "Choose seen or unseen transactions")
		}
		sqlText += " AND " + seenSQL + "=?"
		seenValue, _ := strconv.Atoi(seen)
		args = append(args, seenValue)
	}
	if params.Has("dashboard_spending") || params.Has("dashboard_income") {
		income := params.Has("dashboard_income")
		flag := "dashboard_spending"
		if income {
			flag = "dashboard_income"
		}
		if params.Has("dashboard_spending") && income {
			return "", nil, fail(400, "Choose income or spending")
		}
		if params.Get(flag) != "1" || params.Get("period") == "" {
			return "", nil, fail(400, "Choose a budget period for dashboard transactions")
		}
		if err := requireMember(u); err != nil {
			return "", nil, err
		}
		if params.Get("account") == "" {
			sqlText += " AND a.household=1"
		}
		// Match the same allocation scope as dashboard spending, including refunds.
		predicate := "(dc.kind='expense' OR dl.category_id IS NULL AND dl.amount_cents<0)"
		if income {
			predicate = "(dc.kind='income' OR dl.category_id IS NULL AND dl.amount_cents>=0)"
		}
		sqlText += " AND t.is_transfer=0 AND EXISTS(SELECT 1 FROM allocations dl LEFT JOIN categories dc ON dc.id=dl.category_id WHERE dl.transaction_id=t.id AND " + predicate
		if category := params.Get("category"); category != "" {
			if category == "uncategorized" {
				sqlText += " AND dl.category_id IS NULL"
			} else {
				sqlText += " AND dl.category_id=?"
				args = append(args, category)
			}
		}
		sqlText += ")"
	}
	filters, err := readLedgerFilters(r)
	if err != nil {
		return "", nil, err
	}
	filterSQL, filterArgs := filters.sql("t")
	sqlText += " AND " + filterSQL
	args = append(args, filterArgs...)
	if v := params.Get("id"); v != "" {
		sqlText += " AND t.id=?"
		args = append(args, v)
	}
	if v := params.Get("imports"); v != "" {
		ids := strings.Split(v, ",")
		if len(ids) > 100 {
			return "", nil, fail(400, "Select at most 100 imports")
		}
		placeholders := []string{}
		for _, raw := range ids {
			id, err := strconv.ParseInt(raw, 10, 64)
			if err != nil || id <= 0 {
				return "", nil, fail(400, "Invalid import scope")
			}
			placeholders = append(placeholders, "?")
			args = append(args, id)
		}
		sqlText += " AND t.import_id IN (" + strings.Join(placeholders, ",") + ")"
	}
	if v := params.Get("account"); v != "" {
		sqlText += " AND t.account_id=?"
		args = append(args, v)
	}
	if v := params.Get("period"); v != "" {
		if err := requireMember(u); err != nil {
			return "", nil, err
		}
		var start, end string
		if err := q.QueryRow("SELECT start_date,end_date FROM periods WHERE id=?", v).Scan(&start, &end); err != nil {
			return "", nil, fail(404, "Period not found")
		}
		sqlText += " AND (a.household=1 AND t.period_id=? OR a.household=0 AND t.date>=? AND t.date<=?)"
		args = append(args, v, start, end)
	}
	if params.Get("pending") == "1" {
		sqlText += " AND t.review_state='pending_review'"
	}
	if params.Get("excluded") == "1" {
		if err := requireMember(u); err != nil {
			return "", nil, err
		}
		sqlText += " AND a.household=1 AND (t.period_id IS NULL OR t.assignment='outside')"
	}
	if params.Get("unassigned") == "1" {
		if err := requireMember(u); err != nil {
			return "", nil, err
		}
		sqlText += " AND a.household=1 AND t.period_id IS NULL AND t.assignment!='outside'"
	}
	return sqlText, args, nil
}

func transactionSeenSQL(u User) string {
	return fmt.Sprintf("EXISTS(SELECT 1 FROM transaction_seen s WHERE s.transaction_id=t.id AND s.user_id=%d AND s.transaction_version=t.version)", u.ID)
}

func transactionListState(q queryer, sqlText string, args []any, u User) (string, any, error) {
	stateQuery := "SELECT group_concat(id||':'||version||':'||seen) state,COUNT(*) total FROM (SELECT t.id,t.version," + transactionSeenSQL(u) + " seen" + sqlText[strings.Index(sqlText, " FROM transactions"):] + " ORDER BY t.id)"
	state, err := data(q, stateQuery, args...)
	if err != nil {
		return "", nil, err
	}
	return hash(fmt.Sprint(state[0]["state"])), state[0]["total"], nil
}

func (a *App) transactions(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	params := r.URL.Query()
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	sqlText, args, err := a.transactionQuery(tx, r, u)
	if err != nil {
		return err
	}
	listVersion, total, err := transactionListState(tx, sqlText, args, u)
	if err != nil {
		return err
	}
	if expected := params.Get("list_version"); expected != "" && expected != listVersion {
		return fail(409, "This list changed. Start from the first page.")
	}
	limit := 100
	if params.Get("dashboard_spending") == "1" {
		limit = 20
	}
	offset, _ := strconv.Atoi(params.Get("offset"))
	if offset < 0 {
		offset = 0
	}
	sqlText += " ORDER BY t.date DESC,t.id DESC LIMIT ? OFFSET ?"
	args = append(args, limit+1, offset)
	items, err := data(tx, sqlText, args...)
	if err != nil {
		return err
	}
	more := len(items) > limit
	if more {
		items = items[:limit]
	}
	for _, item := range items {
		id := num(item["id"])
		alloc, err := data(tx, "SELECT l.category_id,l.amount_cents,l.note,c.name category_name,c.group_name,c.kind FROM allocations l LEFT JOIN categories c ON c.id=l.category_id WHERE l.transaction_id=? ORDER BY l.id", id)
		if err != nil {
			return err
		}
		if params.Get("dashboard_spending") == "1" || params.Get("dashboard_income") == "1" {
			var matched int64
			category := params.Get("category")
			categoryID, _ := strconv.ParseInt(category, 10, 64)
			for _, allocation := range alloc {
				if category == "uncategorized" && allocation["category_id"] != nil {
					continue
				}
				if category != "" && category != "uncategorized" && num(allocation["category_id"]) != categoryID {
					continue
				}
				amount := num(allocation["amount_cents"])
				if params.Get("dashboard_income") == "1" {
					if allocation["kind"] == "income" || allocation["category_id"] == nil && amount >= 0 {
						matched += amount
					}
				} else if allocation["kind"] == "expense" || allocation["category_id"] == nil && amount < 0 {
					matched -= amount
				}
			}
			if params.Get("dashboard_income") == "1" {
				item["matched_income_cents"] = matched
			} else {
				item["matched_spend_cents"] = matched
			}
		}
		item["allocations"] = alloc
		tags, err := data(tx, "SELECT x.id,x.name FROM transaction_tags tt JOIN tags x ON x.id=tt.tag_id WHERE tt.transaction_id=? ORDER BY x.name,x.id", id)
		if err != nil {
			return err
		}
		item["tags"] = tags
		item["currency"] = "ZAR"
		if !u.Member {
			item["period_id"] = nil
			item["period_name"] = nil
			item["outside_period"] = int64(0)
			item["assignment"] = "auto"
		}
		item["can_edit"] = a.can(tx, u, num(item["account_id"]), true)
		var provenance any
		json.Unmarshal([]byte(item["provenance"].(string)), &provenance)
		item["provenance"] = provenance
		var left, right int64
		err = tx.QueryRow("SELECT left_id,right_id FROM transfer_links WHERE left_id=? OR right_id=?", id, id).Scan(&left, &right)
		if err == nil {
			other := left
			if other == id {
				other = right
			}
			var account int64
			tx.QueryRow("SELECT account_id FROM transactions WHERE id=?", other).Scan(&account)
			if a.can(tx, u, account, false) {
				item["transfer_counterpart_id"] = other
			} else {
				item["transfer_counterpart_hidden"] = true
			}
		}
	}
	send(w, map[string]any{"items": items, "more": more, "offset": offset, "total": total, "list_version": listVersion})
	return nil
}

type Allocation struct {
	CategoryID *int64 `json:"category_id"`
	Amount     int64  `json:"amount_cents"`
	Note       string `json:"note,omitempty"`
}

func validateAllocations(q queryer, amount int64, alloc []Allocation, approve, transfer bool) error {
	if len(alloc) == 0 || len(alloc) > 100 {
		return fail(400, "Provide between 1 and 100 allocations")
	}
	var sum int64
	for _, v := range alloc {
		if (amount < 0 && v.Amount > 0) || (amount > 0 && v.Amount < 0) || (amount == 0 && v.Amount != 0) {
			return fail(400, "All allocations must have the same sign as the transaction")
		}
		if len(v.Note) > 500 {
			return fail(400, "Allocation note is too long")
		}
		if v.Amount > 900000000000000 || v.Amount < -900000000000000 {
			return fail(400, "Allocation is too large")
		}
		sum += v.Amount
		if v.CategoryID != nil && queryInt(q, "SELECT COUNT(*) FROM categories WHERE id=?", *v.CategoryID) != 1 {
			return fail(400, "Unknown category")
		}
		if approve && !transfer && v.CategoryID == nil {
			return fail(400, "Categorize every allocation before approval")
		}
	}
	if sum != amount {
		return fail(400, "Split allocations must equal the transaction amount exactly")
	}
	return nil
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
func (a *App) linkTransfer(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
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
	err := a.write(func(tx *sql.Tx) error {
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
	u := Current(r)
	id := parseID(r)
	err := a.write(func(tx *sql.Tx) error {
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
func (a *App) transactionAudit(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	id := parseID(r)
	var account int64
	if err := a.DB.QueryRow("SELECT account_id FROM transactions WHERE id=?", id).Scan(&account); err != nil {
		return fail(404, "Transaction not found")
	}
	if !a.can(a.DB, u, account, false) {
		return fail(403, "Account access required")
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT audit.id,audit.action,audit.details,audit.created_at,u.username FROM audit LEFT JOIN users u ON u.id=audit.user_id WHERE entity='transaction' AND entity_id=?", []any{id}, "action", "id")
	}
	rows, err := data(a.DB, "SELECT audit.id,audit.action,audit.details,audit.created_at,u.username FROM audit LEFT JOIN users u ON u.id=audit.user_id WHERE entity='transaction' AND entity_id=? ORDER BY audit.id LIMIT 100", id)
	if err != nil {
		return err
	}
	send(w, rows)
	return nil
}
