package app

import (
	"database/sql"
	"net/http"
	"reflect"
	"strconv"

	"finance-tracker/internal/classification"
)

type MatchedMerchant struct {
	ClassificationConflict bool
	ID                     int64
	Name                   string
	CategoryID             *int64
	SpendingGroupID        *int64
}

func matchMerchantDetails(q queryer, account int64, description string, amount int64) (*MatchedMerchant, error) {
	rules, e := data(q, "SELECT r.merchant_id,r.pattern,r.priority,m.name merchant_name,r.category_id,r.spending_group_id FROM merchant_rules r JOIN merchants m ON m.id=r.merchant_id WHERE (r.account_id=? OR r.account_id IS NULL) AND r.enabled=1 AND (r.direction='any' OR r.direction='debit' AND ?<0 OR r.direction='credit' AND ?>0) ORDER BY r.priority DESC,r.id", account, amount, amount)
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
		} else {
			var category, group *int64
			if rule["category_id"] != nil {
				id := num(rule["category_id"])
				category = &id
			}
			if rule["spending_group_id"] != nil {
				id := num(rule["spending_group_id"])
				group = &id
			}
			if !reflect.DeepEqual(category, result.CategoryID) || !reflect.DeepEqual(group, result.SpendingGroupID) {
				result.ClassificationConflict = true
			}
		}
	}
	if !matched {
		return nil, nil
	}
	if conflict {
		return &MatchedMerchant{ClassificationConflict: true}, nil
	}
	if result.ClassificationConflict {
		result.CategoryID = nil
		result.SpendingGroupID = nil
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
	var pattern, direction string
	if tx.QueryRow("SELECT account_id,merchant_id,version,pattern,direction FROM merchant_rules WHERE id=? AND enabled=1", id).Scan(&scope, &mid, &version, &pattern, &direction) != nil {
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
		amount := num(row["amount_cents"])
		if !classification.MatchPattern(row["description"].(string), pattern) || direction == "debit" && amount >= 0 || direction == "credit" && amount <= 0 {
			continue
		}
		m, e := matchMerchantDetails(tx, num(row["account_id"]), row["description"].(string), amount)
		if e != nil {
			return e
		}
		row["classification"] = "No winning merchant rule"
		row["classification_state"] = "no_winner"
		if m != nil {
			row["category_id"], row["spending_group_id"] = m.CategoryID, m.SpendingGroupID
			if m.ClassificationConflict {
				row["classification"] = "Conflicting rules: needs review"
				row["classification_state"] = "conflict"
			} else if m.ID == mid {
				row["classification"] = "No category: needs review unless an automatic category rule matches"
				row["classification_state"] = "fallback"
				if m.CategoryID != nil {
					row["classification_state"] = "classified"
					var categoryName, groupName string
					tx.QueryRow("SELECT name FROM categories WHERE id=?", *m.CategoryID).Scan(&categoryName)
					if m.SpendingGroupID != nil {
						tx.QueryRow("SELECT name FROM spending_groups WHERE id=?", *m.SpendingGroupID).Scan(&groupName)
					}
					row["classification"] = "Future imports: " + categoryName
					if groupName != "" {
						row["classification"] = row["classification"].(string) + " · " + groupName
					}
				}
			}
		}
		matches = append(matches, row)
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
	if e != nil || mm == nil || mm.ID == 0 {
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
