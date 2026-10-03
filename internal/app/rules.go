package app

import (
	"database/sql"
	"net/http"
	"strings"
)

type ruleInput struct {
	AccountID       int64  `json:"account_id"`
	Pattern         string `json:"pattern"`
	CategoryID      int64  `json:"category_id"`
	SpendingGroupID *int64 `json:"spending_group_id"`
	Direction       string `json:"direction"`
	Priority        int    `json:"priority"`
	Enabled         *bool  `json:"enabled"`
	Version         int64  `json:"version"`
}

func (b *ruleInput) validate(q queryer) error {
	b.Pattern = strings.TrimSpace(b.Pattern)
	if b.Direction == "" {
		b.Direction = "any"
	}
	if len(b.Pattern) < 2 || len(b.Pattern) > 200 {
		return fail(400, "Description pattern must be 2-200 characters")
	}
	if b.Direction != "any" && b.Direction != "debit" && b.Direction != "credit" {
		return fail(400, "Choose any, debit or credit direction")
	}
	if queryInt(q, "SELECT COUNT(*) FROM categories WHERE id=?", b.CategoryID) == 0 {
		return fail(400, "Choose a category")
	}
	if b.SpendingGroupID != nil && queryInt(q, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.SpendingGroupID) == 0 {
		return fail(400, "Choose an existing spending group")
	}
	if b.Priority < -1000000 || b.Priority > 1000000 {
		return fail(400, "Priority must be between -1000000 and 1000000")
	}
	return nil
}
func (b ruleInput) active() bool { return b.Enabled == nil || *b.Enabled }
func ruleAccess(q queryer, a *App, u User, account int64) bool {
	return a.can(q, u, account, true) && queryInt(q, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", account) == 1
}
func (a *App) rules(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Query().Has("page") {
		return a.rulePages(w, r)
	}
	u := Current(r)
	v, err := data(a.DB, "SELECT rules.*,c.name category_name,a.name account_name,s.name spending_group_name FROM rules JOIN accounts a ON a.id=rules.account_id JOIN categories c ON c.id=rules.category_id LEFT JOIN spending_groups s ON s.id=rules.spending_group_id WHERE "+accessSQL+" ORDER BY priority DESC,rules.id LIMIT 100", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, row := range v {
		row["builtin"] = int64(0)
	}
	defaults, err := data(a.DB, "SELECT -b.id id,0 account_id,b.pattern,b.category_id,b.spending_group_id,b.direction,b.priority,b.enabled,b.version,1 builtin,c.name category_name,s.name spending_group_name,'All enabled accounts' account_name FROM builtin_rules b JOIN categories c ON c.id=b.category_id LEFT JOIN spending_groups s ON s.id=b.spending_group_id ORDER BY b.priority DESC,b.id LIMIT 100")
	if err != nil {
		return err
	}
	v = append(v, defaults...)
	send(w, v)
	return nil
}
func (a *App) saveRule(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	var b ruleInput
	if err := decode(r, &b); err != nil {
		return err
	}
	id := parseID(r)
	err := a.write(func(tx *sql.Tx) error {
		return a.writeRule(tx, u, &id, &b)
	})
	if err != nil {
		return err
	}
	send(w, map[string]int64{"id": id})
	return nil
}

// Single-account and grouped UI edits share the same authorization/audit boundary.
func (a *App) writeRule(tx *sql.Tx, u User, idRef *int64, input *ruleInput) error {
	id := *idRef
	b := *input
	if id < 0 {
		return a.writeBuiltinRule(tx, u, id, b)
	}

	if id != 0 {
		var account int64
		if err := tx.QueryRow("SELECT account_id FROM rules WHERE id=?", id).Scan(&account); err != nil {
			return fail(404, "Rule not found")
		}
		if !ruleAccess(tx, a, u, account) {
			return fail(403, "Account editor access required")
		}
	}
	if !ruleAccess(tx, a, u, b.AccountID) {
		return fail(403, "Account editor access required")
	}
	if err := b.validate(tx); err != nil {
		return err
	}
	action := "created"
	if id == 0 {
		res, err := tx.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction,spending_group_id,enabled) VALUES(?,?,?,?,?,?,?,?)", u.ID, b.AccountID, b.Pattern, b.CategoryID, b.Priority, b.Direction, b.SpendingGroupID, b.active())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
	} else {
		res, err := tx.Exec("UPDATE rules SET account_id=?,pattern=?,category_id=?,priority=?,direction=?,spending_group_id=?,enabled=?,version=version+1 WHERE id=? AND version=?", b.AccountID, b.Pattern, b.CategoryID, b.Priority, b.Direction, b.SpendingGroupID, b.active(), id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		action = "updated"
	}
	*idRef = id
	return audit(tx, u, b.AccountID, "rule", id, action, b)
}

type ruleBatchItem struct {
	ID      int64     `json:"id"`
	Version int64     `json:"version"`
	Rule    ruleInput `json:"rule"`
}

func (a *App) batchRules(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Rules      []ruleBatchItem `json:"rules"`
		AllCurrent bool            `json:"all_current"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.Rules) == 0 || len(b.Rules) > 10000 {
		return fail(400, "Choose 1–10000 account rules")
	}
	u := Current(r)
	err := a.write(func(tx *sql.Tx) error {
		if b.AllCurrent {
			if r.Method == "DELETE" || len(b.Rules) != 1 || b.Rules[0].ID != 0 {
				return fail(400, "All current accounts applies only to a new rule")
			}
			input := b.Rules[0].Rule
			b.Rules = nil
			accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE "+accessSQL+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) ORDER BY a.id", u.Member, u.ID, u.Member, u.ID)
			if err != nil {
				return err
			}
			if len(accounts) == 0 {
				return fail(400, "No enabled accounts with editor access")
			}
			for _, account := range accounts {
				rule := input
				rule.AccountID = num(account["id"])
				b.Rules = append(b.Rules, ruleBatchItem{Rule: rule})
			}
		}

		seen := map[int64]bool{}
		for _, item := range b.Rules {
			if item.ID != 0 && seen[item.ID] {
				return fail(400, "Each saved rule can appear only once")
			}
			seen[item.ID] = true
			if r.Method != "DELETE" {
				if err := a.writeRule(tx, u, &item.ID, &item.Rule); err != nil {
					return err
				}
				continue
			}
			if item.ID < 0 {
				if err := a.deleteBuiltinRule(tx, u, item.ID, item.Version); err != nil {
					return err
				}
				continue
			}
			var account int64
			if err := tx.QueryRow("SELECT account_id FROM rules WHERE id=?", item.ID).Scan(&account); err != nil {
				return fail(404, "Rule not found")
			}
			if !ruleAccess(tx, a, u, account) {
				return fail(403, "Account editor access required")
			}
			res, err := tx.Exec("DELETE FROM rules WHERE id=? AND version=?", item.ID, item.Version)
			if err != nil {
				return err
			}
			if err := affected(res); err != nil {
				return err
			}
			if err := audit(tx, u, account, "rule", item.ID, "deleted", nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) deleteRule(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	id := parseID(r)
	var b struct {
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		if id < 0 {
			return a.deleteBuiltinRule(tx, u, id, b.Version)
		}
		var account int64
		if err := tx.QueryRow("SELECT account_id FROM rules WHERE id=?", id).Scan(&account); err != nil {
			return fail(404, "Rule not found")
		}
		if !ruleAccess(tx, a, u, account) {
			return fail(403, "Account editor access required")
		}
		res, err := tx.Exec("DELETE FROM rules WHERE id=? AND version=?", id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(res); err != nil {
			return err
		}
		return audit(tx, u, account, "rule", id, "deleted", nil)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

type classificationRule struct {
	Builtin           bool   `json:"builtin"`
	ID                int64  `json:"id"`
	Pattern           string `json:"pattern"`
	CategoryID        int64  `json:"category_id"`
	CategoryName      string `json:"category_name"`
	SpendingGroupID   *int64 `json:"spending_group_id"`
	SpendingGroupName string `json:"spending_group_name"`
	Direction         string `json:"direction"`
	Priority          int    `json:"priority"`
}

func loadRules(q queryer, account int64) ([]classificationRule, error) {
	rows, err := q.Query("SELECT r.id,r.pattern,r.category_id,c.name,r.spending_group_id,COALESCE(s.name,''),r.direction,r.priority,0 builtin FROM rules r JOIN categories c ON c.id=r.category_id LEFT JOIN spending_groups s ON s.id=r.spending_group_id WHERE r.account_id=? AND r.enabled=1 UNION ALL SELECT -r.id,r.pattern,r.category_id,c.name,r.spending_group_id,COALESCE(s.name,''),r.direction,r.priority,1 builtin FROM builtin_rules r JOIN categories c ON c.id=r.category_id LEFT JOIN spending_groups s ON s.id=r.spending_group_id WHERE r.enabled=1 ORDER BY 8 DESC,1", account)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []classificationRule{}
	for rows.Next() {
		var v classificationRule
		if err := rows.Scan(&v.ID, &v.Pattern, &v.CategoryID, &v.CategoryName, &v.SpendingGroupID, &v.SpendingGroupName, &v.Direction, &v.Priority, &v.Builtin); err != nil {
			return nil, err
		}
		result = append(result, v)
	}
	return result, rows.Err()
}
func matchingRules(rules []classificationRule, description string, amount int64) []classificationRule {
	result := []classificationRule{}
	for _, rule := range rules {
		if rule.Direction == "debit" && amount >= 0 || rule.Direction == "credit" && amount <= 0 {
			continue
		}
		if strings.Contains(normalize(description), normalize(rule.Pattern)) {
			result = append(result, rule)
		}
	}
	custom := []classificationRule{}
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
func differentOutputs(left, right classificationRule) bool {
	if left.CategoryID != right.CategoryID {
		return true
	}
	if left.SpendingGroupID == nil || right.SpendingGroupID == nil {
		return left.SpendingGroupID != right.SpendingGroupID
	}
	return *left.SpendingGroupID != *right.SpendingGroupID
}
func classify(row *SourceRow, rules []classificationRule) {
	row.CategoryID = nil
	row.SpendingGroupID = nil
	row.Suggestion = ""
	row.RuleMatches = nil
	row.RuleConflict = false
	matches := matchingRules(rules, row.Description, row.Amount)
	if len(matches) == 0 {
		return
	}
	row.RuleMatches = matches
	for _, m := range matches[1:] {
		if differentOutputs(matches[0], m) {
			row.RuleConflict = true
		}
	}
	if row.RuleConflict {
		row.Suggestion = "Conflicting rule suggestions; choose classification during review"
		return
	}
	id := matches[0].CategoryID
	row.CategoryID = &id
	row.SpendingGroupID = matches[0].SpendingGroupID
	row.Suggestion = "Description contains: " + matches[0].Pattern
}

type rulePreviewInput struct {
	AccountID   int64      `json:"account_id"`
	Description string     `json:"description"`
	Direction   string     `json:"direction"`
	Draft       *ruleInput `json:"draft"`
	ReplaceID   int64      `json:"replace_id"`
	AllCurrent  bool       `json:"all_current"`
}

func (a *App) previewRuleRow(tx *sql.Tx, u User, b rulePreviewInput) (SourceRow, error) {
	var row SourceRow
	if !ruleAccess(tx, a, u, b.AccountID) {
		return SourceRow{}, fail(403, "Account editor access required")
	}
	rules, err := loadRules(tx, b.AccountID)
	if err != nil {
		return SourceRow{}, err
	}
	if b.ReplaceID != 0 {
		if b.ReplaceID < 0 {
			if queryInt(tx, "SELECT COUNT(*) FROM builtin_rules WHERE id=?", -b.ReplaceID) != 1 {
				return SourceRow{}, fail(404, "Rule not found")
			}
		} else if queryInt(tx, "SELECT COUNT(*) FROM rules WHERE id=? AND account_id=?", b.ReplaceID, b.AccountID) != 1 {
			return SourceRow{}, fail(400, "Preview the rule in its saved account")
		}
		filtered := rules[:0]
		for _, v := range rules {
			if v.ID != b.ReplaceID {
				filtered = append(filtered, v)
			}
		}
		rules = filtered
	}
	if b.Draft != nil && b.Draft.active() {
		if b.Draft.AccountID != b.AccountID {
			return SourceRow{}, fail(400, "Draft account does not match")
		}
		if err := b.Draft.validate(tx); err != nil {
			return SourceRow{}, err
		}
		v := classificationRule{Builtin: b.ReplaceID < 0, ID: b.ReplaceID, Pattern: b.Draft.Pattern, CategoryID: b.Draft.CategoryID, SpendingGroupID: b.Draft.SpendingGroupID, Direction: b.Draft.Direction, Priority: b.Draft.Priority}
		tx.QueryRow("SELECT name FROM categories WHERE id=?", v.CategoryID).Scan(&v.CategoryName)
		if v.SpendingGroupID != nil {
			tx.QueryRow("SELECT name FROM spending_groups WHERE id=?", *v.SpendingGroupID).Scan(&v.SpendingGroupName)
		}
		// A new draft follows saved rules at equal priority; edits retain their id.
		at := len(rules)
		for i, saved := range rules {
			if v.Priority > saved.Priority || (v.Priority == saved.Priority && v.ID != 0 && v.ID < saved.ID) {
				at = i
				break
			}
		}
		rules = append(rules, classificationRule{})
		copy(rules[at+1:], rules[at:])
		rules[at] = v
	}
	row = SourceRow{Description: b.Description, Amount: -1}
	if b.Direction == "credit" {
		row.Amount = 1
	}
	classify(&row, rules)
	return row, nil
}
func (a *App) previewRule(w http.ResponseWriter, r *http.Request) error {
	var b rulePreviewInput
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.Description) == 0 || len(b.Description) > 1000 || (b.Direction != "debit" && b.Direction != "credit") {
		return fail(400, "Provide a description and debit/credit direction")
	}
	var result any
	err := a.write(func(tx *sql.Tx) error {
		u := Current(r)
		if !b.AllCurrent {
			row, err := a.previewRuleRow(tx, u, b)
			result = row
			return err
		}
		if b.Draft == nil || b.ReplaceID > 0 {
			return fail(400, "Choose a new or built-in rule to check across accounts")
		}
		accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE "+accessSQL+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) ORDER BY a.id", u.Member, u.ID, u.Member, u.ID)
		if err != nil {
			return err
		}
		matches := 0
		conflict := false
		for _, account := range accounts {
			input := b
			draft := *b.Draft
			draft.AccountID = num(account["id"])
			input.AccountID = draft.AccountID
			input.Draft = &draft
			row, err := a.previewRuleRow(tx, u, input)
			if err != nil {
				return err
			}
			for _, match := range row.RuleMatches {
				if match.ID == b.ReplaceID {
					matches++
					conflict = conflict || row.RuleConflict
					break
				}
			}
		}
		result = map[string]any{"matches": matches, "accounts": len(accounts), "conflict": conflict}
		return nil
	})
	if err != nil {
		return err
	}
	send(w, result)
	return nil
}
