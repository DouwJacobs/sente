package app

import (
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
