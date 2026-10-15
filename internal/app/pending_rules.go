package app

// Legacy response name is retained; rule saving no longer applies historical changes.
type pendingRuleResult struct {
	RuleKind    string `json:"rule_kind,omitempty"`
	RuleVersion int64  `json:"rule_version,omitempty"`
	RuleID      int64  `json:"rule_id,omitempty"`
	Applied     int    `json:"applied"`
	Conflicts   int    `json:"conflicts"`
}
