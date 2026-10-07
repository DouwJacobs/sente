package app

import (
	"database/sql"
	"net/http"
	"strconv"

	"finance-tracker/internal/classification"
)

type MatchedMerchant struct {
	ID              int64
	Name            string
	CategoryID      *int64
	SpendingGroupID *int64
}

func matchMerchantDetails(q queryer, account int64, description string, amount int64) (*MatchedMerchant, error) {
	rules, e := data(q, "SELECT r.merchant_id,r.pattern,r.priority,m.name merchant_name,m.category_id,m.spending_group_id FROM merchant_rules r JOIN merchants m ON m.id=r.merchant_id WHERE (r.account_id=? OR r.account_id IS NULL) AND r.enabled=1 AND (r.direction='any' OR r.direction='debit' AND ?<0 OR r.direction='credit' AND ?>0) ORDER BY r.priority DESC,r.id", account, amount, amount)
	if e != nil {
		return nil, e
	}
	matched := false
	var priority int64
	var result *MatchedMerchant
	conflict := false
	for _, rule := range rules {
		if !classification.MatchPattern(description, rule["pattern"].(string)) {
			continue
		}
		p := num(rule["priority"])
		if matched && p < priority {
			break
		}
		mid := num(rule["merchant_id"])
		if !matched {
			matched = true
			priority = p
			var catID, groupID *int64
			if rule["category_id"] != nil {
				c := num(rule["category_id"])
				catID = &c
			}
			if rule["spending_group_id"] != nil {
				g := num(rule["spending_group_id"])
				groupID = &g
			}
			result = &MatchedMerchant{
				ID:              mid,
				Name:            rule["merchant_name"].(string),
				CategoryID:      catID,
				SpendingGroupID: groupID,
			}
		} else if mid != result.ID {
			conflict = true
		}
	}
	if conflict || !matched {
		return nil, nil
	}
	return result, nil
}

func matchMerchant(q queryer, account int64, description string, amount int64) (int64, error) {
	m, err := matchMerchantDetails(q, account, description, amount)
	if err != nil || m == nil {
		return 0, err
	}
	return m.ID, nil
}
func (a *App) merchantRulePreview(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	u := Current(r)
	tx, e := a.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var scope sql.NullInt64
	var mid, version int64
	if tx.QueryRow("SELECT account_id,merchant_id,version FROM merchant_rules WHERE id=? AND enabled=1", id).Scan(&scope, &mid, &version) != nil {
		return fail(404, "Active rule not found")
	}
	account := scope.Int64
	if !merchantScopeAccess(tx, a, u, account) {
		return fail(403, "Account editor access required")
	}
	rows, e := data(tx, "SELECT t.id,t.version,t.description,t.amount_cents,t.date,t.account_id FROM transactions t JOIN accounts a ON a.id=t.account_id WHERE ( ?=0 OR a.id=? ) AND a.sync_hidden=0 AND "+accountAccessSQL(u)+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) AND t.merchant_id IS NULL ORDER BY t.date DESC,t.id DESC LIMIT 10001", account, account, u.Member, u.ID, u.Member, u.ID)
	if e != nil {
		return e
	}
	if len(rows) > 10000 {
		return fail(400, "Too many unnamed transactions; narrow them using transaction filters")
	}
	matches := []map[string]any{}
	for _, row := range rows {
		m, e := matchMerchant(tx, num(row["account_id"]), row["description"].(string), num(row["amount_cents"]))
		if e != nil {
			return e
		}
		if m == mid {
			matches = append(matches, row)
		}
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 0 {
		return fail(400, "Choose a valid page")
	}
	total := len(matches)
	start := page * 100
	if start > total {
		start = total
	}
	end := start + 100
	if end > total {
		end = total
	}
	send(w, map[string]any{"items": matches[start:end], "rule_version": version, "total": total})
	return nil
}
func (a *App) importMerchant(tx *sql.Tx, id, account int64, description string, amount int64) error {
	mm, e := matchMerchantDetails(tx, account, description, amount)
	if e != nil || mm == nil {
		return e
	}
	if _, e = tx.Exec("UPDATE transactions SET merchant_id=? WHERE id=? AND merchant_id IS NULL", mm.ID, id); e != nil {
		return e
	}
	if mm.CategoryID != nil {
		if _, e = tx.Exec("UPDATE allocations SET category_id=? WHERE transaction_id=? AND category_id IS NULL", *mm.CategoryID, id); e != nil {
			return e
		}
	}
	if mm.SpendingGroupID != nil {
		if _, e = tx.Exec("UPDATE transactions SET spending_group_id=? WHERE id=? AND spending_group_id IS NULL", *mm.SpendingGroupID, id); e != nil {
			return e
		}
	}
	return nil
}
