package app

func evaluateDescription(q queryer, account int64, description string, amount int64, rules []classificationRule) (SourceRow, *MatchedMerchant, error) {
	row := SourceRow{Description: description, Amount: amount}
	mm, err := matchMerchantDetails(q, account, description, amount)
	if err != nil {
		return row, nil, err
	}
	if mm != nil {
		row.RuleConflict = mm.ClassificationConflict
		if mm.ID > 0 {
			row.MerchantID = &mm.ID
		}
		row.MerchantName = mm.Name
		if mm.CategoryID != nil {
			row.CategoryID = mm.CategoryID
			row.SpendingGroupID = mm.SpendingGroupID
			row.Suggestion = "Merchant: " + mm.Name
		}
	}
	if row.RuleConflict {
		row.Suggestion = "Conflicting merchant rules: needs review"
	}
	if row.CategoryID == nil && !row.RuleConflict {
		classify(&row, rules)
	}
	// Import commit also fills a missing group from the recognized merchant.
	// Reflect that same final effect in both import previews and historical previews.
	if mm != nil && !row.RuleConflict && row.SpendingGroupID == nil {
		row.SpendingGroupID = mm.SpendingGroupID
	}
	return row, mm, nil
}
