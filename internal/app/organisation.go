package app

import (
	"database/sql"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

func (a *App) validateMetadata(tx *sql.Tx, u User, account int64, b transactionInput) error {
	if b.Note != nil && utf8.RuneCountInString(*b.Note) > 2000 {
		return fail(400, "Transaction note must be at most 2000 characters")
	}
	if b.ClearMerchant && b.MerchantID != nil {
		return fail(400, "Choose or clear a merchant")
	}
	if b.MerchantID != nil && queryInt(tx, "SELECT COUNT(*) FROM merchants WHERE id=? AND (account_id=? OR account_id IS NULL)", *b.MerchantID, account) != 1 {
		return fail(400, "Choose a merchant in this account")
	}
	if b.Tags != nil {
		if len(*b.Tags) > 20 {
			return fail(400, "Choose at most 20 tags")
		}
		seen := map[int64]bool{}
		for _, id := range *b.Tags {
			if seen[id] || queryInt(tx, "SELECT COUNT(*) FROM tags WHERE id=? AND account_id=?", id, account) != 1 {
				return fail(400, "Choose distinct tags in this account")
			}
			seen[id] = true
		}
	}
	return nil
}
func writeTransactionMetadata(tx *sql.Tx, id int64, b transactionInput) error {
	if b.Note != nil {
		if _, e := tx.Exec("UPDATE transactions SET note=? WHERE id=?", *b.Note, id); e != nil {
			return e
		}
	}
	if b.ClearMerchant {
		if _, e := tx.Exec("UPDATE transactions SET merchant_id=NULL WHERE id=?", id); e != nil {
			return e
		}
	} else if b.MerchantID != nil {
		if _, e := tx.Exec("UPDATE transactions SET merchant_id=? WHERE id=?", *b.MerchantID, id); e != nil {
			return e
		}
	}
	if b.Tags != nil {
		if _, e := tx.Exec("DELETE FROM transaction_tags WHERE transaction_id=?", id); e != nil {
			return e
		}
		for _, tag := range *b.Tags {
			if _, e := tx.Exec("INSERT INTO transaction_tags VALUES(?,?)", id, tag); e != nil {
				return e
			}
		}
	}
	return nil
}
func validateArchivedAllocations(tx *sql.Tx, id int64, alloc []Allocation) error {
	assigned := map[int64]int64{}
	for _, l := range alloc {
		if l.CategoryID != nil && queryInt(tx, "SELECT archived FROM categories WHERE id=?", *l.CategoryID) == 1 {
			assigned[*l.CategoryID]++
		}
	}
	for cat, count := range assigned {
		if count > queryInt(tx, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id=?", id, cat) {
			return fail(400, "Archived categories cannot receive new assignments")
		}
	}
	return nil
}

func labelID(tx *sql.Tx, u User, a *App, account int64, kind, name string, create bool) (int64, error) {
	if account == 0 && kind == "merchant" {
		if err := requireMember(u); err != nil {
			return 0, err
		}
	} else if !a.can(tx, u, account, create) || queryInt(tx, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", account) != 1 {
		return 0, fail(403, "Enabled account access required")
	}
	table, max, min := "tags", 40, 1
	if kind == "merchant" {
		table, max, min = "merchants", 100, 2
	} else if kind != "tag" {
		return 0, fail(400, "Choose merchant or tag")
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < min || utf8.RuneCountInString(name) > max {
		return 0, fail(400, "Use a valid "+kind+" name")
	}
	var id int64
	e := tx.QueryRow("SELECT id FROM "+table+" WHERE COALESCE(account_id,0)=? AND finance_normalize(name)=finance_normalize(?)", account, name).Scan(&id)
	if e == nil {
		return id, nil
	}
	if e != sql.ErrNoRows {
		return 0, e
	}
	if !create {
		return 0, nil
	}
	res, e := tx.Exec("INSERT INTO "+table+"(account_id,name) VALUES(?,?)", nullableAccount(account), name)
	if e != nil {
		return 0, e
	}
	id, e = res.LastInsertId()
	if e == nil {
		e = audit(tx, u, nullableAccount(account), kind, id, "created", map[string]any{"name": name})
	}
	return id, e
}
func (a *App) labels(w http.ResponseWriter, r *http.Request) error {
	table := "tags"
	if r.URL.Query().Get("kind") == "merchant" {
		table = "merchants"
	} else if r.URL.Query().Get("kind") != "tag" {
		return fail(400, "Choose merchant or tag")
	}
	u := Current(r)
	text := "SELECT x.id,x.name,x.account_id,a.name account_name,'' logo_data,0 logo_version FROM tags x JOIN accounts a ON a.id=x.account_id WHERE " + accountAccessSQL(u)
	args := []any{u.Member, u.ID}
	if table == "merchants" {
		text = "SELECT x.id,x.name,x.account_id,CASE WHEN x.account_id IS NULL THEN 'All accounts' ELSE a.name END account_name,x.logo_data,x.version logo_version,x.category_id,c.name category_name,x.spending_group_id,s.name spending_group_name,s.color spending_group_color FROM merchants x LEFT JOIN accounts a ON a.id=x.account_id LEFT JOIN categories c ON c.id=x.category_id LEFT JOIN spending_groups s ON s.id=x.spending_group_id WHERE (x.account_id IS NULL OR " + accountAccessSQL(u) + ")"
	}
	if r.URL.Query().Get("scope") == "global" && table == "merchants" {
		text += " AND x.account_id IS NULL"
	}
	if account := r.URL.Query().Get("account"); account != "" {
		if table == "merchants" {
			text += " AND (x.account_id IS NULL OR a.id=?)"
		} else {
			text += " AND a.id=?"
		}
		args = append(args, account)
	}
	return a.metadataPage(w, r, text, args, "name", "name,id")
}
func (a *App) createLabel(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Account         int64   `json:"account_id"`
		Kind            string  `json:"kind"`
		Name            string  `json:"name"`
		Logo            *string `json:"logo_data"`
		CategoryID      *int64  `json:"category_id"`
		SpendingGroupID *int64  `json:"spending_group_id"`
	}
	if e := decode(r, &b); e != nil {
		return e
	}
	var id, version int64
	e := a.write(func(tx *sql.Tx) error {
		var e error
		id, e = labelID(tx, Current(r), a, b.Account, b.Kind, b.Name, true)
		if e != nil {
			return e
		}
		if b.Kind == "merchant" {
			version = queryInt(tx, "SELECT version FROM merchants WHERE id=?", id)
			if b.CategoryID != nil || b.SpendingGroupID != nil {
				var catID, groupID any
				if b.CategoryID != nil {
					catID = *b.CategoryID
				}
				if b.SpendingGroupID != nil {
					groupID = *b.SpendingGroupID
				}
				if _, err := tx.Exec("UPDATE merchants SET category_id=?, spending_group_id=? WHERE id=?", catID, groupID, id); err != nil {
					return err
				}
			}
			if b.Logo != nil {
				if e = updateMerchantLogo(tx, a, Current(r), id, version, *b.Logo); e != nil {
					return e
				}
				version++
			}
		} else if b.Logo != nil {
			return fail(400, "Only merchants can have logos")
		}
		return nil
	})
	if e != nil {
		return e
	}
	send(w, map[string]any{"id": id, "logo_version": version})
	return nil
}
func (a *App) updateLabel(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	var b struct {
		Kind            string  `json:"kind"`
		Name            *string `json:"name"`
		CategoryID      *int64  `json:"category_id"`
		ClearCategory   bool    `json:"clear_category"`
		SpendingGroupID *int64  `json:"spending_group_id"`
		ClearGroup      bool    `json:"clear_group"`
		Logo            *string `json:"logo_data"`
		Version         int64   `json:"version"`
	}
	if e := decode(r, &b); e != nil {
		return e
	}
	u := Current(r)
	e := a.write(func(tx *sql.Tx) error {
		if b.Kind != "merchant" {
			return fail(400, "Only merchants can be updated")
		}
		var account sql.NullInt64
		var oldVersion int64
		if tx.QueryRow("SELECT account_id, version FROM merchants WHERE id=?", id).Scan(&account, &oldVersion) != nil {
			return fail(404, "Merchant not found")
		}
		if !merchantScopeAccess(tx, a, u, account.Int64) {
			return fail(403, "Merchant editor access required")
		}
		if b.Logo != nil {
			if e := updateMerchantLogo(tx, a, u, id, oldVersion, *b.Logo); e != nil {
				return e
			}
			oldVersion++
		}
		setClauses := []string{}
		args := []any{}
		if b.Name != nil {
			name := strings.TrimSpace(*b.Name)
			if len(name) < 2 || len(name) > 100 {
				return fail(400, "Use a valid merchant name")
			}
			setClauses = append(setClauses, "name=?")
			args = append(args, name)
		}
		if b.ClearCategory {
			setClauses = append(setClauses, "category_id=NULL")
		} else if b.CategoryID != nil {
			if queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=?", *b.CategoryID) == 0 {
				return fail(400, "Choose an existing category")
			}
			setClauses = append(setClauses, "category_id=?")
			args = append(args, *b.CategoryID)
		}
		if b.ClearGroup {
			setClauses = append(setClauses, "spending_group_id=NULL")
		} else if b.SpendingGroupID != nil {
			if queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.SpendingGroupID) == 0 {
				return fail(400, "Choose an existing spending group")
			}
			setClauses = append(setClauses, "spending_group_id=?")
			args = append(args, *b.SpendingGroupID)
		}
		if len(setClauses) > 0 {
			setClauses = append(setClauses, "version=version+1")
			args = append(args, id)
			if _, e := tx.Exec("UPDATE merchants SET "+strings.Join(setClauses, ", ")+" WHERE id=?", args...); e != nil {
				return e
			}
		}
		return audit(tx, u, nullableAccount(account.Int64), "merchant", id, "updated", b)
	})
	if e != nil {
		return e
	}
	success(w)
	return nil
}
func categoryDeps(q queryer, id int64) (map[string]any, error) {
	rules, e := data(q, "SELECT COUNT(*) n FROM rules WHERE category_id=? AND enabled=1", id)
	if e != nil {
		return nil, e
	}
	built, e := data(q, "SELECT COUNT(*) n FROM builtin_rules WHERE category_id=? AND enabled=1", id)
	if e != nil {
		return nil, e
	}
	budgets, e := data(q, "SELECT p.id,p.name,COUNT(*) entries FROM group_targets g JOIN periods p ON p.id=g.period_id WHERE g.category_id=? AND g.included=1 AND g.carry_forward=1 GROUP BY p.id ORDER BY p.start_date,p.id", id)
	if e != nil {
		return nil, e
	}
	return map[string]any{"active_rules": num(rules[0]["n"]) + num(built[0]["n"]), "carry_forward_periods": budgets}, nil
}
func (a *App) categoryDependencies(w http.ResponseWriter, r *http.Request) error {
	if e := requireMember(Current(r)); e != nil {
		return e
	}
	id := parseID(r)
	if queryInt(a.DB, "SELECT COUNT(*) FROM categories WHERE id=?", id) != 1 {
		return fail(404, "Category not found")
	}
	// Restricted dependencies are reported without private account counts or content.
	v, e := categoryDeps(a.DB, id)
	if e != nil {
		return e
	}
	visible := queryInt(a.DB, "SELECT COUNT(*) FROM rules x JOIN accounts a ON a.id=x.account_id WHERE x.category_id=? AND x.enabled=1 AND "+accountAccessSQL(Current(r)), id, Current(r).Member, Current(r).ID) + queryInt(a.DB, "SELECT COUNT(*) FROM builtin_rules WHERE category_id=? AND enabled=1", id)
	v["restricted_rules"] = num(v["active_rules"]) > visible
	if v["restricted_rules"] == true {
		v["active_rules"] = visible
		v["blocked"] = true
	}
	send(w, v)
	return nil
}
func (a *App) updateCategory(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if e := requireMember(u); e != nil {
		return e
	}
	var b struct {
		Name               string `json:"name"`
		Archived           bool   `json:"archived"`
		SpendingGroupID    *int64 `json:"spending_group_id,omitempty"`
		ClearSpendingGroup bool   `json:"clear_spending_group,omitempty"`
		Version            int64  `json:"version,omitempty"`
	}
	if e := decode(r, &b); e != nil {
		return e
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 1 || utf8.RuneCountInString(b.Name) > 100 {
		return fail(400, "Use a category name of 1–100 characters")
	}
	id := parseID(r)
	e := a.write(func(tx *sql.Tx) error {
		var oldName string
		var oldArchived int
		var oldGroup sql.NullInt64
		var version int64
		if tx.QueryRow("SELECT name,archived,spending_group_id,version FROM categories WHERE id=?", id).Scan(&oldName, &oldArchived, &oldGroup, &version) != nil {
			return fail(404, "Category not found")
		}
		if version != b.Version {
			return fail(409, "Category changed; reload it")
		}
		if queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id!=? AND finance_normalize(name)=finance_normalize(?)", id, b.Name) > 0 {
			return fail(409, "Category name already exists")
		}
		if b.Archived && oldArchived == 0 {
			deps, e := categoryDeps(tx, id)
			if e != nil {
				return e
			}
			if deps["active_rules"].(int64) > 0 || len(deps["carry_forward_periods"].([]map[string]any)) > 0 {
				return fail(409, "Replace or pause active rules and stop budget carry-forward before archiving")
			}
		}
		var newGroup any
		if b.ClearSpendingGroup {
			newGroup = nil
		} else if b.SpendingGroupID != nil {
			if *b.SpendingGroupID > 0 {
				if queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.SpendingGroupID) != 1 {
					return fail(400, "Choose a valid spending group")
				}
				newGroup = *b.SpendingGroupID
			} else {
				newGroup = nil
			}
		} else if oldGroup.Valid {
			newGroup = oldGroup.Int64
		}
		res, e := tx.Exec("UPDATE categories SET name=?,archived=?,spending_group_id=?,version=version+1 WHERE id=? AND version=?", b.Name, b.Archived, newGroup, id, b.Version)
		if e != nil {
			return e
		}
		if e = affected(res); e != nil {
			return e
		}
		return audit(tx, u, nil, "category", id, "updated", map[string]any{"before": map[string]any{"name": oldName, "archived": oldArchived, "spending_group_id": oldGroup.Int64}, "after": b})
	})
	if e != nil {
		return e
	}
	success(w)
	return nil
}

type merchantRuleInput struct {
	Account         int64   `json:"account_id"`
	Merchant        int64   `json:"merchant_id"`
	Pattern         string  `json:"pattern"`
	Direction       string  `json:"direction"`
	Priority        int     `json:"priority,omitempty"`
	Enabled         bool    `json:"enabled"`
	Version         int64   `json:"version,omitempty"`
	Logo            *string `json:"merchant_logo,omitempty"`
	LogoVersion     int64   `json:"merchant_version,omitempty"`
	CategoryID      *int64  `json:"category_id,omitempty"`
	ClearCategory   bool    `json:"clear_category,omitempty"`
	SpendingGroupID *int64  `json:"spending_group_id,omitempty"`
	ClearGroup      bool    `json:"clear_group,omitempty"`
}

func (a *App) merchantRules(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	text := "SELECT x.*,m.name merchant_name,m.logo_data merchant_logo,m.version merchant_version,m.category_id,c.name category_name,m.spending_group_id,s.name spending_group_name,s.color spending_group_color,CASE WHEN x.account_id IS NULL THEN 'All accounts' ELSE a.name END account_name FROM merchant_rules x JOIN merchants m ON m.id=x.merchant_id LEFT JOIN categories c ON c.id=m.category_id LEFT JOIN spending_groups s ON s.id=m.spending_group_id LEFT JOIN accounts a ON a.id=x.account_id WHERE (x.account_id IS NULL OR " + accountAccessSQL(u) + ")"
	args := []any{u.Member, u.ID}
	if acc := r.URL.Query().Get("account"); acc != "" {
		text += " AND (x.account_id IS NULL OR a.id=?)"
		args = append(args, acc)
	}
	return a.metadataPage(w, r, text, args, "pattern||' '||merchant_name", "id")
}
func (a *App) saveMerchantRule(w http.ResponseWriter, r *http.Request) error {
	var b merchantRuleInput
	if err := decode(r, &b); err != nil {
		return err
	}
	id := parseID(r)
	if err := a.write(func(tx *sql.Tx) error { return a.writeMerchantRule(tx, Current(r), &id, b) }); err != nil {
		return err
	}
	send(w, map[string]any{"id": id})
	return nil
}
func (a *App) writeMerchantRule(tx *sql.Tx, u User, id *int64, b merchantRuleInput) error {
	b.Pattern = strings.TrimSpace(b.Pattern)
	if len(b.Pattern) < 2 || len(b.Pattern) > 200 || b.Direction != "any" && b.Direction != "debit" && b.Direction != "credit" || b.Priority > 1000000 || b.Priority < -1000000 {
		return fail(400, "Use a description match, valid direction and priority")
	}
	localID := *id
	if localID != 0 {
		var old sql.NullInt64
		if tx.QueryRow("SELECT account_id FROM merchant_rules WHERE id=?", localID).Scan(&old) != nil {
			return fail(404, "Merchant rule not found")
		}
		if !merchantScopeAccess(tx, a, u, old.Int64) {
			return fail(403, "Merchant rule access required")
		}
	}
	if !merchantScopeAccess(tx, a, u, b.Account) {
		return fail(403, "Merchant rule access required")
	}
	if queryInt(tx, "SELECT COUNT(*) FROM merchants WHERE id=? AND (account_id IS NULL OR account_id=?)", b.Merchant, b.Account) != 1 {
		return fail(400, "Choose a merchant in this scope")
	}
	if b.Logo != nil {
		if err := updateMerchantLogo(tx, a, u, b.Merchant, b.LogoVersion, *b.Logo); err != nil {
			return err
		}
	}
	if b.CategoryID != nil || b.ClearCategory || b.SpendingGroupID != nil || b.ClearGroup {
		var catID, groupID any
		if b.CategoryID != nil && !b.ClearCategory {
			if queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=?", *b.CategoryID) == 0 {
				return fail(400, "Choose an existing category")
			}
			catID = *b.CategoryID
		}
		if b.SpendingGroupID != nil && !b.ClearGroup {
			if queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *b.SpendingGroupID) == 0 {
				return fail(400, "Choose an existing spending group")
			}
			groupID = *b.SpendingGroupID
		}
		setClauses := []string{}
		args := []any{}
		if b.CategoryID != nil || b.ClearCategory {
			setClauses = append(setClauses, "category_id=?")
			args = append(args, catID)
		}
		if b.SpendingGroupID != nil || b.ClearGroup {
			setClauses = append(setClauses, "spending_group_id=?")
			args = append(args, groupID)
		}
		if len(setClauses) > 0 {
			setClauses = append(setClauses, "version=version+1")
			args = append(args, b.Merchant)
			if _, e := tx.Exec("UPDATE merchants SET "+strings.Join(setClauses, ", ")+" WHERE id=?", args...); e != nil {
				return e
			}
		}
	}
	if localID == 0 {
		res, e := tx.Exec("INSERT INTO merchant_rules(account_id,merchant_id,pattern,direction,priority,enabled) VALUES(?,?,?,?,?,?)", nullableAccount(b.Account), b.Merchant, b.Pattern, b.Direction, b.Priority, b.Enabled)
		if e != nil {
			return e
		}
		localID, _ = res.LastInsertId()
	} else {
		res, e := tx.Exec("UPDATE merchant_rules SET account_id=?,merchant_id=?,pattern=?,direction=?,priority=?,enabled=?,version=version+1 WHERE id=? AND version=?", nullableAccount(b.Account), b.Merchant, b.Pattern, b.Direction, b.Priority, b.Enabled, localID, b.Version)
		if e != nil {
			return e
		}
		if e = affected(res); e != nil {
			return e
		}
	}
	*id = localID
	b.Logo = nil // Image data is not duplicated in the audit log.
	return audit(tx, u, nullableAccount(b.Account), "merchant_rule", localID, "saved", b)
}

var reOrWord = regexp.MustCompile(`(?i)\s+or\s+`)

func buildPatternRegex(pattern string) *regexp.Regexp {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return nil
	}
	if strings.HasPrefix(pattern, "^") || strings.HasSuffix(pattern, "$") || (strings.HasPrefix(pattern, "(") && strings.HasSuffix(pattern, ")")) {
		if re, err := regexp.Compile("(?i)" + pattern); err == nil {
			return re
		}
	}
	var tokens []string
	if strings.Contains(pattern, "|") {
		tokens = strings.Split(pattern, "|")
	} else if reOrWord.MatchString(pattern) {
		tokens = reOrWord.Split(pattern, -1)
	} else if strings.Contains(pattern, ",") {
		tokens = strings.Split(pattern, ",")
	}
	if len(tokens) > 1 {
		var parts []string
		for _, tok := range tokens {
			tok = strings.TrimSpace(tok)
			if tok != "" {
				parts = append(parts, regexp.QuoteMeta(tok))
			}
		}
		if len(parts) > 0 {
			if re, err := regexp.Compile("(?i)(?:" + strings.Join(parts, "|") + ")"); err == nil {
				return re
			}
		}
	}
	if strings.ContainsAny(pattern, `.*+?^$[](){}\`) {
		if re, err := regexp.Compile("(?i)" + pattern); err == nil {
			return re
		}
	}
	return nil
}

func matchRulePattern(description, pattern string) bool {
	normDesc := normalize(description)
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	if re := buildPatternRegex(pattern); re != nil {
		if re.MatchString(description) || re.MatchString(normDesc) {
			return true
		}
	}
	return strings.Contains(normDesc, normalize(pattern))
}

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
		if !matchRulePattern(description, rule["pattern"].(string)) {
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
