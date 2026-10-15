package app

import "database/sql"

// Saving a transaction-created rule never applies it to historical targets.
func (a *App) saveTransactionRuleTx(tx *sql.Tx, u User, account, transaction int64, b transactionInput) (pendingRuleResult, error) {
	result := pendingRuleResult{RuleKind: "category"}
	if b.Transfer || len(b.Allocations) != 1 || b.Allocations[0].CategoryID == nil || b.Amount == 0 {
		return result, fail(400, "Rules need a single categorized transaction that is not a transfer")
	}
	direction := "debit"
	if b.Amount > 0 {
		direction = "credit"
	}
	table := "rules"
	if b.Rule.MerchantID != nil {
		if *b.Rule.MerchantID <= 0 || queryInt(tx, "SELECT COALESCE(merchant_id,0) FROM transactions WHERE id=?", transaction) != *b.Rule.MerchantID {
			return result, fail(400, "Use the transaction's selected merchant for this rule")
		}
		result.RuleKind = "merchant"
		table = "merchant_rules"
	}
	pattern := normalize(b.Rule.Pattern)
	existing, err := data(tx, "SELECT id,pattern,version,priority FROM "+table+" WHERE account_id=? AND direction=?", account, direction)
	if err != nil {
		return result, err
	}
	var id, version int64
	priority := 0
	for _, old := range existing {
		if normalize(old["pattern"].(string)) != pattern {
			continue
		}
		if id != 0 {
			return result, fail(409, "Multiple rules use this description. Edit the rules before saving another")
		}
		id = num(old["id"])
		version = num(old["version"])
		priority = int(num(old["priority"]))
	}
	if result.RuleKind == "merchant" {
		input := merchantRuleInput{Account: account, Merchant: *b.Rule.MerchantID, Pattern: b.Rule.Pattern, CategoryID: b.Allocations[0].CategoryID, SpendingGroupID: b.SpendingGroupID, ClearGroup: b.SpendingGroupID == nil, Direction: direction, Priority: priority, Enabled: true, Version: version}
		if err = a.writeMerchantRule(tx, u, &id, input); err != nil {
			return result, err
		}
	} else {
		input := ruleInput{AccountID: account, Pattern: b.Rule.Pattern, CategoryID: *b.Allocations[0].CategoryID, SpendingGroupID: b.SpendingGroupID, Direction: direction, Priority: priority, Version: version}
		if err = a.writeRule(tx, u, &id, &input); err != nil {
			return result, err
		}
	}
	result.RuleID = id
	result.RuleVersion = queryInt(tx, "SELECT version FROM "+table+" WHERE id=?", id)
	return result, nil
}
