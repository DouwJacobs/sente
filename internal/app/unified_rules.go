package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
)

// RuleRef preserves original IDs/domains for configuration and MCP compatibility.
type RuleRef struct {
	Kind    string `json:"kind"`
	ID      int64  `json:"id"`
	Version int64  `json:"version"`
}

func ruleTable(kind string, id int64) (string, int64, error) {
	if kind == "merchant" && id > 0 {
		return "merchant_rules", id, nil
	}
	if kind == "category" {
		if id < 0 {
			return "builtin_rules", -id, nil
		}
		if id > 0 {
			return "rules", id, nil
		}
	}
	return "", 0, fail(400, "Choose an existing rule")
}
func visibleRules(q queryer, u User) ([]map[string]any, error) {
	rows, err := data(q, `SELECT r.id,r.user_id,r.account_id,r.pattern,r.category_id,r.priority,r.direction,r.spending_group_id,r.enabled,r.version,c.name category_name,s.name spending_group_name,a.name account_name,'category' rule_kind,0 builtin,'' merchant_name,'' merchant_logo
 FROM rules r JOIN accounts a ON a.id=r.account_id LEFT JOIN categories c ON c.id=r.category_id LEFT JOIN spending_groups s ON s.id=r.spending_group_id WHERE `+accountAccessSQL(u)+`
 UNION ALL SELECT -r.id,0,0,r.pattern,r.category_id,r.priority,r.direction,r.spending_group_id,r.enabled,r.version,c.name,s.name,'All accounts','category',1,'','' FROM builtin_rules r LEFT JOIN categories c ON c.id=r.category_id LEFT JOIN spending_groups s ON s.id=r.spending_group_id ORDER BY 1`, u.Member, u.ID)
	if err != nil {
		return nil, err
	}
	merchants, err := data(q, "SELECT r.*,m.name merchant_name,m.logo_data merchant_logo,m.version merchant_version,c.name category_name,s.name spending_group_name,CASE WHEN r.account_id IS NULL THEN 'All accounts' ELSE a.name END account_name,'merchant' rule_kind,0 builtin FROM merchant_rules r JOIN merchants m ON m.id=r.merchant_id LEFT JOIN accounts a ON a.id=r.account_id LEFT JOIN categories c ON c.id=r.category_id LEFT JOIN spending_groups s ON s.id=r.spending_group_id WHERE r.account_id IS NULL OR "+accountAccessSQL(u)+" ORDER BY r.id", u.Member, u.ID)
	return append(rows, merchants...), err
}
func (a *App) unifiedRules(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	u := Current(r)
	rows, err := visibleRules(tx, u)
	if err != nil {
		return err
	}
	raw, _ := json.Marshal(rows)
	version := hash(string(raw))
	if expected := r.URL.Query().Get("list_version"); expected != "" && expected != version {
		return fail(409, "This list changed. Start from the first page.")
	}
	params := r.URL.Query()
	for key, allowed := range map[string]map[string]bool{"status": {"": true, "all": true, "active": true, "paused": true}, "kind": {"": true, "all": true, "merchant": true, "category": true}, "direction": {"": true, "all": true, "any": true, "debit": true, "credit": true}} {
		if !allowed[params.Get(key)] {
			return fail(400, "Choose a valid rule filter")
		}
	}
	search := normalize(params.Get("q"))
	groups := map[string]map[string]any{}
	keys := []string{}
	for _, row := range rows {
		if status := params.Get("status"); status != "" && status != "all" && ((status == "active") != (num(row["enabled"]) == 1)) {
			continue
		}
		if direction := params.Get("direction"); direction != "" && direction != "all" && row["direction"] != direction {
			continue
		}
		if kind := params.Get("kind"); kind != "" && kind != "all" && row["rule_kind"] != kind {
			continue
		}
		ref := RuleRef{row["rule_kind"].(string), num(row["id"]), num(row["version"])}
		key := fmt.Sprintf("%v:%v", row["rule_kind"], row["id"])
		if row["rule_kind"] == "category" {
			b, _ := json.Marshal([]any{normalize(row["pattern"].(string)), row["category_id"], row["spending_group_id"], row["direction"], row["priority"], row["enabled"], row["builtin"]})
			key = string(b)
		} else {
			b, _ := json.Marshal([]any{"merchant", normalize(row["pattern"].(string)), row["merchant_id"], row["category_id"], row["spending_group_id"], row["direction"], row["priority"], row["enabled"], num(row["account_id"]) == 0})
			key = string(b)
		}
		group := groups[key]
		if group == nil {
			group = map[string]any{}
			for k, v := range row {
				group[k] = v
			}
			group["refs"] = []RuleRef{}
			group["rows"] = []map[string]any{}
			groups[key] = group
			keys = append(keys, key)
		}
		group["refs"] = append(group["refs"].([]RuleRef), ref)
		group["rows"] = append(group["rows"].([]map[string]any), row)
	}
	items := []map[string]any{}
	for _, key := range keys {
		row := groups[key]
		text := normalize(fmt.Sprint(row["pattern"], " ", row["merchant_name"], " ", row["category_name"], " ", row["spending_group_name"]))
		if search != "" && !strings.Contains(text, search) {
			continue
		}
		if account := params.Get("account"); account != "" {
			id, e := strconv.ParseInt(account, 10, 64)
			if e != nil || id < 1 {
				return fail(400, "Choose an account")
			}
			found := false
			for _, member := range row["rows"].([]map[string]any) {
				found = found || num(member["account_id"]) == 0 || num(member["account_id"]) == id
			}
			if !found {
				continue
			}
		}
		editable := true
		for _, member := range row["rows"].([]map[string]any) {
			scope := num(member["account_id"])
			if member["rule_kind"] == "merchant" {
				editable = editable && merchantScopeAccess(tx, a, u, scope)
			} else if scope == 0 {
				editable = editable && u.Member
			} else {
				editable = editable && ruleAccess(tx, a, u, scope)
			}
		}
		row["editable"] = editable
		items = append(items, row)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if num(items[i]["priority"]) != num(items[j]["priority"]) {
			return num(items[i]["priority"]) > num(items[j]["priority"])
		}
		return fmt.Sprint(items[i]["rule_kind"], items[i]["id"]) < fmt.Sprint(items[j]["rule_kind"], items[j]["id"])
	})
	total := len(items)
	start := min(page*size, total)
	end := min(start+size, total)
	send(w, map[string]any{"items": items[start:end], "total": total, "page": page, "page_size": size, "list_version": version})
	return nil
}

type unifiedRuleInput struct {
	Kind     string            `json:"kind"`
	Refs     []RuleRef         `json:"refs"`
	Scope    string            `json:"scope"`
	Rule     ruleInput         `json:"rule"`
	Merchant merchantRuleInput `json:"merchant_rule"`
	Delete   bool              `json:"delete"`
}

func (a *App) saveUnifiedRule(w http.ResponseWriter, r *http.Request) error {
	var b unifiedRuleInput
	if err := decode(r, &b); err != nil {
		return err
	}
	refs := []RuleRef{}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error { var err error; refs, err = a.writeUnifiedRulesTx(tx, u, b); return err })
	if err != nil {
		return err
	}
	send(w, map[string]any{"refs": refs})
	return nil
}
func (a *App) writeUnifiedRulesTx(tx *sql.Tx, u User, b unifiedRuleInput) ([]RuleRef, error) {
	refs := []RuleRef{}

	if b.Kind != "category" && b.Kind != "merchant" {
		return nil, fail(400, "Choose a rule type")
	}
	if len(b.Refs) > 10000 {
		return nil, fail(400, "Too many rules")
	}
	targets := b.Refs
	if len(targets) == 0 {
		if b.Delete {
			return nil, fail(400, "Choose a saved rule")
		}
		if b.Scope == "current" {
			accounts, err := data(tx, "SELECT a.id FROM accounts a WHERE "+accountAccessSQL(u)+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) ORDER BY a.id", u.Member, u.ID, u.Member, u.ID)
			if err != nil {
				return nil, err
			}
			for _, acc := range accounts {
				targets = append(targets, RuleRef{Kind: b.Kind, Version: num(acc["id"])})
			}
		} else {
			targets = []RuleRef{{Kind: b.Kind}}
		}
		if len(targets) == 0 {
			return nil, fail(400, "No accounts with editor access")
		}
	}
	seen := map[string]bool{}
	merchantLogoWritten := false
	for _, ref := range targets {
		if ref.Kind != b.Kind {
			return nil, fail(400, "A rule cannot change its matching type")
		}
		key := fmt.Sprintf("%s:%d", ref.Kind, ref.ID)
		if ref.ID != 0 && seen[key] {
			return nil, fail(400, "Choose each rule once")
		}
		seen[key] = true
		id := ref.ID
		if b.Kind == "merchant" {
			input := b.Merchant
			if merchantLogoWritten {
				input.Logo = nil
			}
			input.Version = ref.Version
			if id > 0 {
				input.Account = queryInt(tx, "SELECT COALESCE(account_id,0) FROM merchant_rules WHERE id=?", id)
			}
			if id == 0 && b.Scope == "current" {
				input.Account = ref.Version
				input.Version = 0
			}
			if b.Delete {
				if err := a.deleteMerchantRuleTx(tx, u, id, ref.Version); err != nil {
					return nil, err
				}
				continue
			}
			if err := a.writeMerchantRule(tx, u, &id, input); err != nil {
				return nil, err
			}
			if input.Logo != nil {
				merchantLogoWritten = true
			}
		} else {
			input := b.Rule
			input.Version = ref.Version
			if b.Delete {
				if id < 0 {
					if err := a.deleteBuiltinRule(tx, u, id, ref.Version); err != nil {
						return nil, err
					}
				} else {
					account := queryInt(tx, "SELECT account_id FROM rules WHERE id=?", id)
					if !ruleAccess(tx, a, u, account) {
						return nil, fail(403, "Account editor access required")
					}
					res, err := tx.Exec("DELETE FROM rules WHERE id=? AND version=?", id, ref.Version)
					if err != nil {
						return nil, err
					}
					if err = affected(res); err != nil {
						return nil, err
					}
					if err = audit(tx, u, account, "rule", id, "deleted", nil); err != nil {
						return nil, err
					}
				}
				continue
			}
			if id > 0 {
				input.AccountID = queryInt(tx, "SELECT account_id FROM rules WHERE id=?", id)
			}
			if id == 0 && b.Scope == "current" {
				input.AccountID = ref.Version
				input.Version = 0
			}
			if id == 0 && b.Scope == "global" {
				if err := a.createBuiltinRule(tx, u, &id, input); err != nil {
					return nil, err
				}
				id = -id
			} else {
				if err := a.writeRule(tx, u, &id, &input); err != nil {
					return nil, err
				}
			}
		}
		table, lookup, err := ruleTable(b.Kind, id)
		if err != nil {
			return nil, err
		}
		refs = append(refs, RuleRef{b.Kind, id, queryInt(tx, "SELECT version FROM "+table+" WHERE id=?", lookup)})
	}
	return refs, nil
}

func (a *App) deleteMerchantRuleTx(tx *sql.Tx, u User, id, version int64) error {
	var scope sql.NullInt64
	if tx.QueryRow("SELECT account_id FROM merchant_rules WHERE id=?", id).Scan(&scope) != nil {
		return fail(404, "Rule not found")
	}
	if !merchantScopeAccess(tx, a, u, scope.Int64) {
		return fail(403, "Merchant rule access required")
	}
	res, err := tx.Exec("DELETE FROM merchant_rules WHERE id=? AND version=?", id, version)
	if err != nil {
		return err
	}
	if err = affected(res); err != nil {
		return err
	}
	return audit(tx, u, nullableAccount(scope.Int64), "merchant_rule", id, "deleted", nil)
}

func (a *App) testUnifiedRule(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Draft       unifiedRuleInput `json:"draft"`
		Description string           `json:"description"`
		Direction   string           `json:"direction"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if len(b.Description) < 1 || len(b.Description) > 1000 || (b.Direction != "debit" && b.Direction != "credit") || b.Draft.Delete {
		return fail(400, "Provide an example description and direction")
	}
	results := []map[string]any{}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if _, err := tx.Exec("SAVEPOINT rule_example"); err != nil {
			return err
		}
		defer tx.Exec("ROLLBACK TO rule_example")
		refs, err := a.writeUnifiedRulesTx(tx, u, b.Draft)
		if err != nil {
			return err
		}
		accounts, err := data(tx, "SELECT a.id,a.name FROM accounts a WHERE "+accountAccessSQL(u)+" AND (a.household=1 AND ?=1 OR EXISTS(SELECT 1 FROM grants g WHERE g.account_id=a.id AND g.user_id=? AND g.role='editor')) ORDER BY a.id", u.Member, u.ID, u.Member, u.ID)
		if err != nil {
			return err
		}
		amount := int64(-1)
		if b.Direction == "credit" {
			amount = 1
		}
		for _, account := range accounts {
			id := num(account["id"])
			applicable := false
			for _, ref := range refs {
				table, lookup, _ := ruleTable(ref.Kind, ref.ID)
				if table == "builtin_rules" {
					applicable = true
					continue
				}
				scope := queryInt(tx, "SELECT COALESCE(account_id,0) FROM "+table+" WHERE id=?", lookup)
				applicable = applicable || scope == 0 || scope == id
			}
			if !applicable {
				continue
			}
			rules, err := loadRules(tx, id)
			if err != nil {
				return err
			}
			result, _, err := evaluateDescription(tx, id, b.Description, amount, rules)
			if err != nil {
				return err
			}
			category, group := "", ""
			if result.CategoryID != nil {
				if err = tx.QueryRow("SELECT name FROM categories WHERE id=?", result.CategoryID).Scan(&category); err != nil {
					return err
				}
			}
			if result.SpendingGroupID != nil {
				if err = tx.QueryRow("SELECT name FROM spending_groups WHERE id=?", result.SpendingGroupID).Scan(&group); err != nil {
					return err
				}
			}
			results = append(results, map[string]any{"account": account["name"], "merchant": result.MerchantName, "category": category, "group": group, "conflict": result.RuleConflict, "explanation": result.Suggestion})
		}
		return nil
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": results})
	return nil
}
