package app

import "database/sql"

type pendingRuleResult struct {
	Applied   int `json:"applied"`
	Conflicts int `json:"conflicts"`
}

// A transaction-created rule fills only intact, single uncategorized allocations.
// All writes share the edit/rule transaction; complete classification automatically accepts the result.
func (a *App) applyRuleToPending(tx *sql.Tx, u User, account, ruleID, editedID int64) (pendingRuleResult, error) {
	result := pendingRuleResult{}
	if !ruleAccess(tx, a, u, account) {
		return result, fail(403, "Account editor access required")
	}
	rules, err := loadRules(tx, account)
	if err != nil {
		return result, err
	}
	var saved *classificationRule
	for i := range rules {
		if rules[i].ID == ruleID {
			saved = &rules[i]
			break
		}
	}
	if saved == nil {
		return result, nil
	}
	candidates, err := data(tx, `SELECT t.id,t.version,t.description,t.amount_cents,t.spending_group_id,l.id allocation_id
 FROM transactions t JOIN allocations l ON l.transaction_id=t.id
 WHERE t.account_id=? AND t.id!=? AND t.review_state='pending_review' AND t.is_transfer=0
 AND l.category_id IS NULL AND l.amount_cents=t.amount_cents
 AND l.transaction_id IN (SELECT transaction_id FROM allocations GROUP BY transaction_id HAVING COUNT(*)=1)
 AND instr(finance_normalize(t.description),finance_normalize(?))>0
 ORDER BY t.id`, account, editedID, saved.Pattern)
	if err != nil {
		return result, err
	}
	for _, candidate := range candidates {
		row := SourceRow{Description: candidate["description"].(string), Amount: num(candidate["amount_cents"])}
		classify(&row, rules)
		participates := false
		for _, match := range row.RuleMatches {
			if match.ID == ruleID {
				participates = true
				break
			}
		}
		if !participates {
			continue
		}
		if row.RuleConflict {
			result.Conflicts++
			continue
		}
		if row.CategoryID == nil {
			continue
		}
		res, err := tx.Exec("UPDATE transactions SET spending_group_id=COALESCE(spending_group_id,?),reviewed_at=NULL,reviewed_by=NULL,version=version+1 WHERE id=? AND version=? AND account_id=? AND review_state='pending_review' AND is_transfer=0", row.SpendingGroupID, candidate["id"], candidate["version"], account)
		if err != nil {
			return result, err
		}
		if err := affected(res); err != nil {
			return result, err
		}
		res, err = tx.Exec("UPDATE allocations SET category_id=? WHERE id=? AND transaction_id=? AND category_id IS NULL", row.CategoryID, candidate["allocation_id"], candidate["id"])
		if err != nil {
			return result, err
		}
		if err := affected(res); err != nil {
			return result, err
		}
		if err := acceptCategorized(tx, u, account, num(candidate["id"])); err != nil {
			return result, err
		}
		group := candidate["spending_group_id"]
		if group == nil {
			group = row.SpendingGroupID
		}
		if err := audit(tx, u, account, "transaction", num(candidate["id"]), "rule_applied", map[string]any{"rule_id": ruleID, "category_id": row.CategoryID, "spending_group_id": group}); err != nil {
			return result, err
		}
		result.Applied++
	}
	return result, nil
}
