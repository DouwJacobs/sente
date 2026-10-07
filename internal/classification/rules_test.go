package classification

import "testing"

func TestClassificationPrecedenceDirectionAndConflicts(t *testing.T) {
	group := int64(7)
	rules := []Rule{
		{ID: -1, Builtin: true, Pattern: "market", CategoryID: 3, Direction: "any"},
		{ID: 1, Pattern: "market", CategoryID: 1, SpendingGroupID: &group, Direction: "debit"},
	}
	debit := Classify("  MARKET   purchase ", -29, rules)
	if debit.CategoryID == nil || *debit.CategoryID != 1 || debit.SpendingGroupID == nil || *debit.SpendingGroupID != group || len(debit.RuleMatches) != 1 {
		t.Fatal("custom debit rule did not override builtin", debit)
	}
	credit := Classify("Market refund", 29, rules)
	if credit.CategoryID == nil || *credit.CategoryID != 3 {
		t.Fatal("debit rule classified a credit", credit)
	}
	rules = append(rules, Rule{ID: 2, Pattern: "purchase", CategoryID: 1, Priority: 100, Direction: "debit"})
	conflict := Classify("Market purchase", -29, rules)
	if !conflict.RuleConflict || conflict.CategoryID != nil || conflict.SpendingGroupID != nil || len(conflict.RuleMatches) != 2 {
		t.Fatal("different group outputs silently classified", conflict)
	}
	unmatched := Classify("Other shop", -29, rules)
	if unmatched.CategoryID != nil || unmatched.RuleMatches != nil || unmatched.RuleConflict || unmatched.Suggestion != "" {
		t.Fatal("unmatched source gained classification", unmatched)
	}
}
func TestMerchantPatterns(t *testing.T) {
	for _, pattern := range []string{"market", "market or fuel", "market|fuel", "market,fuel", "^market", "m.rket"} {
		if !MatchPattern("MARKET purchase", pattern) {
			t.Fatalf("merchant pattern %q lost matching behavior", pattern)
		}
	}
	if MatchPattern("other shop", "market") || MatchPattern("market", " ") {
		t.Fatal("unrelated or empty merchant pattern matched")
	}
}
