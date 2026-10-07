package classification

// Result keeps conflicting suggestions separate from accepted classification.
type Result struct {
	CategoryID      *int64
	SpendingGroupID *int64
	Suggestion      string
	RuleMatches     []Rule
	RuleConflict    bool
}

func Classify(description string, amount int64, rules []Rule) (result Result) {
	row := &result
	row.CategoryID = nil
	row.SpendingGroupID = nil
	row.Suggestion = ""
	row.RuleMatches = nil
	row.RuleConflict = false
	matches := MatchingRules(rules, description, amount)
	if len(matches) == 0 {
		return result
	}
	row.RuleMatches = matches
	for _, m := range matches[1:] {
		if DifferentOutputs(matches[0], m) {
			row.RuleConflict = true
		}
	}
	if row.RuleConflict {
		row.Suggestion = "Conflicting rule suggestions; choose classification during review"
		return result
	}
	id := matches[0].CategoryID
	row.CategoryID = &id
	row.SpendingGroupID = matches[0].SpendingGroupID
	row.Suggestion = "Description contains: " + matches[0].Pattern
	return result
}
