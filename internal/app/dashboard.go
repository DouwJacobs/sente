package app

import (
	"net/http"
	"strconv"
	"time"
)

func (a *App) dashboard(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pid, _ := strconv.ParseInt(r.URL.Query().Get("period"), 10, 64)
	if pid == 0 {
		loc, _ := time.LoadLocation("Africa/Johannesburg")
		today := time.Now().In(loc).Format("2006-01-02")
		pid = queryInt(tx, "SELECT id FROM periods WHERE start_date<=? AND end_date>=? ORDER BY start_date DESC,id DESC LIMIT 1", today, today)
		if pid == 0 {
			pid = queryInt(tx, "SELECT id FROM periods ORDER BY start_date DESC,id DESC LIMIT 1")
		}
	}
	periods, err := data(tx, "SELECT * FROM periods WHERE id=?", pid)
	if err != nil {
		return err
	}
	if len(periods) == 0 {
		return fail(404, "Period not found")
	}
	p := periods[0]
	account, _ := strconv.ParseInt(r.URL.Query().Get("account"), 10, 64)
	condition := "a.household=1 AND t.period_id=?"
	args := []any{pid}
	showTargets := true
	if account != 0 {
		if !a.can(tx, u, account, false) {
			return fail(403, "Account access required")
		}
		household := queryInt(tx, "SELECT household FROM accounts WHERE id=?", account) == 1
		condition = "t.account_id=? AND t.date>=? AND t.date<=?"
		args = []any{account, p["start_date"], p["end_date"]}
		if household {
			condition = "t.account_id=? AND t.period_id=?"
			args = []any{account, pid}
		}
		showTargets = false
	}
	rows, err := data(tx, "SELECT t.id,t.spending_group_id,g.name spending_group_name,g.color spending_group_color,t.review_state,t.is_transfer,l.amount_cents,c.id category_id,c.name,c.group_name,c.kind FROM transactions t JOIN accounts a ON a.id=t.account_id JOIN allocations l ON l.transaction_id=t.id LEFT JOIN categories c ON c.id=l.category_id LEFT JOIN spending_groups g ON g.id=t.spending_group_id WHERE a.sync_hidden=0 AND "+condition+accountScopeSQL(u), args...)
	if err != nil {
		return err
	}
	income, spent, pendingSpend := int64(0), int64(0), int64(0)
	pendingCount := map[int64]bool{}
	uncategorized := map[int64]bool{}
	categories := map[int64]map[string]any{}
	catRows, err := data(tx, "SELECT c.*,COALESCE(t.amount_cents,0) target_cents FROM categories c LEFT JOIN targets t ON t.category_id=c.id AND t.period_id=? ORDER BY c.name,c.id", pid)
	if err != nil {
		return err
	}
	for _, c := range catRows {
		if !showTargets {
			c["target_cents"] = int64(0)
		}
		c["spent_cents"] = int64(0)
		c["pending_cents"] = int64(0)
		categories[num(c["id"])] = c
	}
	for _, row := range rows {
		if row["review_state"] == "pending_review" {
			pendingCount[num(row["id"])] = true
		}
		if num(row["is_transfer"]) == 1 {
			continue
		}
		amount := num(row["amount_cents"])
		expense := row["kind"] == "expense" || (row["category_id"] == nil && amount < 0)
		if expense {
			spent -= amount
			if row["review_state"] == "pending_review" {
				pendingSpend -= amount
			}
		} else {
			income += amount
		}
		if row["category_id"] == nil {
			uncategorized[num(row["id"])] = true
		} else if expense {
			c := categories[num(row["category_id"])]
			c["spent_cents"] = num(c["spent_cents"]) - amount
			if row["review_state"] == "pending_review" {
				c["pending_cents"] = num(c["pending_cents"]) - amount
			}
		}
	}
	budgetRows := []map[string]any{}
	if showTargets {
		budgetRows, err = data(tx, "SELECT gt.category_id,gt.amount_cents,gt.spending_group_id,g.name spending_group_name,g.color spending_group_color,c.name FROM group_targets gt LEFT JOIN spending_groups g ON g.id=gt.spending_group_id JOIN categories c ON c.id=gt.category_id WHERE gt.period_id=? AND gt.included=1", pid)
		if err != nil {
			return err
		}
	}
	groups, groupTotal, err := dashboardGroups(r, rows, categories, budgetRows)
	if err != nil {
		return err
	}
	incomeCategories, incomeTotal, err := dashboardIncome(r, rows)
	if err != nil {
		return err
	}
	var limits int64
	for _, c := range catRows {
		limits += num(c["target_cents"])
	}
	unassigned := queryInt(tx, "SELECT COUNT(*) FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE a.sync_hidden=0 AND a.household=1 AND t.period_id IS NULL AND t.assignment!='outside'"+accountScopeSQL(u))
	balanceScope := " AND a.household=1"
	balanceArgs := []any{u.Member, u.ID}
	if account != 0 {
		balanceScope = " AND a.id=?"
		balanceArgs = append(balanceArgs, account)
	}
	balances, err := data(tx, "SELECT a.id,a.name,a.household,a.balance_cents,a.balance_date FROM accounts a WHERE "+accountAccessSQL(u)+balanceScope+" ORDER BY a.name", balanceArgs...)
	if err != nil {
		return err
	}
	categoryTotal, balanceTotal := len(catRows), len(balances)
	if !r.URL.Query().Has("category_page") && len(catRows) > 100 {
		catRows = catRows[:100]
	}
	if !r.URL.Query().Has("balance_page") && len(balances) > 100 {
		balances = balances[:100]
	}
	budgetSort(catRows, r.URL.Query().Get("sort"))
	if r.URL.Query().Has("category_page") {
		page, err := strconv.Atoi(r.URL.Query().Get("category_page"))
		if err != nil || page < 0 || page > 1000000 {
			return fail(400, "Choose a valid category page")
		}
		expense := []map[string]any{}
		for _, c := range catRows {
			if c["kind"] == "expense" && (num(c["target_cents"]) != 0 || num(c["spent_cents"]) != 0) {
				expense = append(expense, c)
			}
		}
		categoryTotal = len(expense)
		start := page * 20
		if start > len(expense) {
			start = len(expense)
		}
		end := start + 20
		if end > len(expense) {
			end = len(expense)
		}
		catRows = expense[start:end]
	}
	if r.URL.Query().Has("balance_page") {
		page, err := strconv.Atoi(r.URL.Query().Get("balance_page"))
		if err != nil || page < 0 || page > 1000000 {
			return fail(400, "Choose a valid balance page")
		}
		start := page * 20
		if start > len(balances) {
			start = len(balances)
		}
		end := start + 20
		if end > len(balances) {
			end = len(balances)
		}
		balances = balances[start:end]
	}
	send(w, map[string]any{"period": p, "income_cents": income, "income_categories": incomeCategories, "income_category_total": incomeTotal, "spent_cents": spent, "pending_spend_cents": pendingSpend, "pending_count": len(pendingCount), "uncategorized_count": len(uncategorized), "budget_cents": limits, "remaining_cents": limits - spent, "has_targets": showTargets, "spending_groups": groups, "group_total": groupTotal, "categories": catRows, "category_total": categoryTotal, "balances": balances, "balance_total": balanceTotal, "unassigned_count": unassigned})
	return nil
}
