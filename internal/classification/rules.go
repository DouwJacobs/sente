// Package classification contains transport-independent rule matching.
package classification

import "strings"

func Normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }

type Rule struct {
	Builtin           bool   `json:"builtin"`
	ID                int64  `json:"id"`
	Pattern           string `json:"pattern"`
	CategoryID        int64  `json:"category_id"`
	CategoryName      string `json:"category_name"`
	SpendingGroupID   *int64 `json:"spending_group_id,omitempty"`
	SpendingGroupName string `json:"spending_group_name"`
	Direction         string `json:"direction,omitempty"`
	Priority          int    `json:"priority,omitempty"`
}

func MatchingRules(rules []Rule, description string, amount int64) []Rule {
	result := []Rule{}
	for _, rule := range rules {
		if rule.Direction == "debit" && amount >= 0 || rule.Direction == "credit" && amount <= 0 {
			continue
		}
		if strings.Contains(Normalize(description), Normalize(rule.Pattern)) {
			result = append(result, rule)
		}
	}
	custom := []Rule{}
	for _, rule := range result {
		if !rule.Builtin {
			custom = append(custom, rule)
		}
	}
	if len(custom) > 0 {
		return custom
	}
	return result
}
func DifferentOutputs(left, right Rule) bool {
	if left.CategoryID != right.CategoryID {
		return true
	}
	if left.SpendingGroupID == nil || right.SpendingGroupID == nil {
		return left.SpendingGroupID != right.SpendingGroupID
	}
	return *left.SpendingGroupID != *right.SpendingGroupID
}
