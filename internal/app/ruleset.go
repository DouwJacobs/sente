package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const maxRulesetBytes = 32 << 20

type RulesetConfig struct {
	Version        int                   `json:"version"`
	ExportedAt     string                `json:"exported_at,omitempty"`
	SpendingGroups []SpendingGroupExport `json:"spending_groups"`
	Categories     []CategoryExport      `json:"categories"`
	Merchants      []MerchantExport      `json:"merchants"`
	MerchantRules  []MerchantRuleExport  `json:"merchant_rules"`
	Rules          []RuleExport          `json:"rules"`
}
type SpendingGroupExport struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}
type CategoryExport struct {
	Name          string `json:"name"`
	GroupName     string `json:"group_name"` // Historical identity only, never a category's spending group.
	Kind          string `json:"kind"`
	SpendingGroup string `json:"spending_group,omitempty"` // Version 1 compatibility; no ownership is assigned.
	Archived      bool   `json:"archived,omitempty"`
}
type MerchantExport struct {
	Name              string  `json:"name"`
	AccountName       string  `json:"account_name,omitempty"`
	LogoData          string  `json:"logo_data,omitempty"`
	Category          string  `json:"category,omitempty"`
	CategoryGroupName *string `json:"category_group_name,omitempty"`
	SpendingGroup     string  `json:"spending_group,omitempty"`
}
type MerchantRuleExport struct {
	Merchant            string `json:"merchant"`
	MerchantAccountName string `json:"merchant_account_name,omitempty"`
	AccountName         string `json:"account_name,omitempty"`
	Pattern             string `json:"pattern"`
	Direction           string `json:"direction"`
	Priority            int    `json:"priority"`
	Enabled             bool   `json:"enabled"`
}
type RuleExport struct {
	Pattern           string  `json:"pattern"`
	Category          string  `json:"category"`
	CategoryGroupName *string `json:"category_group_name,omitempty"`
	SpendingGroup     string  `json:"spending_group,omitempty"`
	AccountName       string  `json:"account_name,omitempty"`
	Builtin           bool    `json:"builtin,omitempty"`
	Direction         string  `json:"direction"`
	Priority          int     `json:"priority"`
	Enabled           bool    `json:"enabled"`
}

func rulesetUser(q queryer) (User, error) {
	username := os.Getenv("FINANCE_RULESET_USER")
	rows, err := data(q, "SELECT id,username,admin,budget_member FROM users WHERE disabled=0 AND admin=1 AND budget_member=1 AND (?='' OR username=?)", username, username)
	if err != nil {
		return User{}, err
	}
	if len(rows) != 1 {
		return User{}, fmt.Errorf("set FINANCE_RULESET_USER to one existing administrator with budget membership")
	}
	row := rows[0]
	return User{ID: num(row["id"]), Username: row["username"].(string), Admin: true, Member: true}, nil
}
func exportString(row map[string]any, key string) string { value, _ := row[key].(string); return value }
func exportPtr(row map[string]any, key string) *string {
	if row[key] == nil {
		return nil
	}
	value := exportString(row, key)
	return &value
}

// ExportRuleset reads one consistent snapshot, bounded by the selected operator's account access.
// It contains classification configuration and optional logos, never ledger data or credentials.
func (a *App) ExportRuleset(w io.Writer) error {
	return a.exportRulesetFor(w, nil)
}

func (a *App) exportRulesetFor(w io.Writer, actor *User) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	u, err := rulesetActor(tx, actor)
	if err != nil {
		return err
	}
	return exportRulesetTx(tx, u, w)
}

func exportRulesetTx(tx *sql.Tx, u User, w io.Writer) error {
	var err error
	cfg := RulesetConfig{Version: 2, ExportedAt: time.Now().UTC().Format(time.RFC3339),
		SpendingGroups: []SpendingGroupExport{}, Categories: []CategoryExport{}, Merchants: []MerchantExport{}, MerchantRules: []MerchantRuleExport{}, Rules: []RuleExport{}}
	rows, err := data(tx, "SELECT name,color FROM spending_groups ORDER BY id")
	if err != nil {
		return err
	}
	for _, r := range rows {
		cfg.SpendingGroups = append(cfg.SpendingGroups, SpendingGroupExport{exportString(r, "name"), exportString(r, "color")})
	}
	rows, err = data(tx, "SELECT name,group_name,kind,archived FROM categories ORDER BY id")
	if err != nil {
		return err
	}
	for _, r := range rows {
		cfg.Categories = append(cfg.Categories, CategoryExport{Name: exportString(r, "name"), GroupName: exportString(r, "group_name"), Kind: exportString(r, "kind"), Archived: num(r["archived"]) != 0})
	}
	rows, err = data(tx, "SELECT m.name,a.name account_name,m.logo_data,c.name category,c.group_name category_group_name,s.name spending_group FROM merchants m LEFT JOIN accounts a ON a.id=m.account_id LEFT JOIN categories c ON c.id=m.category_id LEFT JOIN spending_groups s ON s.id=m.spending_group_id WHERE m.account_id IS NULL OR "+accountAccessSQL(u)+" ORDER BY m.id", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		cfg.Merchants = append(cfg.Merchants, MerchantExport{Name: exportString(r, "name"), AccountName: exportString(r, "account_name"), LogoData: exportString(r, "logo_data"), Category: exportString(r, "category"), CategoryGroupName: exportPtr(r, "category_group_name"), SpendingGroup: exportString(r, "spending_group")})
	}
	rows, err = data(tx, "SELECT m.name merchant,ma.name merchant_account_name,a.name account_name,x.pattern,x.direction,x.priority,x.enabled FROM merchant_rules x JOIN merchants m ON m.id=x.merchant_id LEFT JOIN accounts ma ON ma.id=m.account_id LEFT JOIN accounts a ON a.id=x.account_id WHERE x.account_id IS NULL OR "+accountAccessSQL(u)+" ORDER BY x.id", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		cfg.MerchantRules = append(cfg.MerchantRules, MerchantRuleExport{Merchant: exportString(r, "merchant"), MerchantAccountName: exportString(r, "merchant_account_name"), AccountName: exportString(r, "account_name"), Pattern: exportString(r, "pattern"), Direction: exportString(r, "direction"), Priority: int(num(r["priority"])), Enabled: num(r["enabled"]) != 0})
	}
	rows, err = data(tx, "SELECT x.pattern,c.name category,c.group_name category_group_name,s.name spending_group,a.name account_name,x.direction,x.priority,x.enabled,0 builtin FROM rules x JOIN accounts a ON a.id=x.account_id JOIN categories c ON c.id=x.category_id LEFT JOIN spending_groups s ON s.id=x.spending_group_id WHERE "+accountAccessSQL(u)+" UNION ALL SELECT x.pattern,c.name,c.group_name,s.name,NULL,x.direction,x.priority,x.enabled,1 FROM builtin_rules x JOIN categories c ON c.id=x.category_id LEFT JOIN spending_groups s ON s.id=x.spending_group_id", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, r := range rows {
		cfg.Rules = append(cfg.Rules, RuleExport{Pattern: exportString(r, "pattern"), Category: exportString(r, "category"), CategoryGroupName: exportPtr(r, "category_group_name"), SpendingGroup: exportString(r, "spending_group"), AccountName: exportString(r, "account_name"), Builtin: num(r["builtin"]) != 0, Direction: exportString(r, "direction"), Priority: int(num(r["priority"])), Enabled: num(r["enabled"]) != 0})
	}
	// Encode first so query/size failures never leave a plausible partial export.
	bytes, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if len(bytes) > maxRulesetBytes {
		return fmt.Errorf("ruleset exceeds 32 MiB")
	}
	_, err = w.Write(append(bytes, '\n'))
	return err
}

func rulesetLookup(q queryer, optional bool, table, name, extra string, args ...any) (int64, error) {
	name = strings.TrimSpace(name)
	if name == "" && optional {
		return 0, nil
	}
	if name == "" {
		return 0, fmt.Errorf("provide an explicit %s name", table)
	}
	values := append([]any{name}, args...)
	rows, err := data(q, "SELECT id FROM "+table+" WHERE finance_normalize(name)=finance_normalize(?)"+extra, values...)
	if err != nil {
		return 0, err
	}
	if len(rows) != 1 {
		return 0, fmt.Errorf("unknown or ambiguous %s name %q", table, name)
	}
	return num(rows[0]["id"]), nil
}
func rulesetCategory(q queryer, name string, legacy *string) (int64, error) {
	extra := ""
	args := []any{}
	if legacy != nil {
		extra = " AND group_name=?"
		args = append(args, *legacy)
	}
	return rulesetLookup(q, true, "categories", name, extra, args...)
}
func rulesetMatch(q queryer, table, where string, args ...any) (int64, int64, error) {
	rows, err := data(q, "SELECT id,version FROM "+table+" WHERE "+where, args...)
	if err != nil {
		return 0, 0, err
	}
	if len(rows) > 1 {
		return 0, 0, fmt.Errorf("ambiguous existing %s match; resolve duplicates before importing", table)
	}
	if len(rows) == 0 {
		return 0, 0, nil
	}
	return num(rows[0]["id"]), num(rows[0]["version"]), nil
}
func rulesetPointer(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

// ImportRuleset is an atomic configuration merge. All references resolve explicitly;
// shared writers enforce permission, validation, versioning and audit contracts.
func (a *App) ImportRuleset(r io.Reader) (*RulesetImportSummary, error) {
	cfg, err := decodeRuleset(r)
	if err != nil {
		return nil, err
	}
	summary := &RulesetImportSummary{}
	err = a.write(func(tx *sql.Tx) error {
		u, err := rulesetActor(tx, nil)
		if err != nil {
			return err
		}
		return a.importRulesetTx(tx, u, cfg, summary)
	})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

func rulesetActor(q queryer, actor *User) (User, error) {
	if actor == nil {
		return rulesetUser(q)
	}
	var u User
	err := q.QueryRow("SELECT id,username,admin,budget_member FROM users WHERE id=? AND disabled=0", actor.ID).Scan(&u.ID, &u.Username, &u.Admin, &u.Member)
	if err != nil || !u.Admin || !u.Member {
		return User{}, fail(403, "Administrator access and budget membership required")
	}
	return u, nil
}

func (a *App) importRulesetTx(tx *sql.Tx, u User, cfg RulesetConfig, summary *RulesetImportSummary) error {
	var err error

	seen := map[string]bool{}
	unique := func(key string) error {
		if seen[key] {
			return fmt.Errorf("duplicate ruleset entry %q", key)
		}
		seen[key] = true
		return nil
	}
	for _, g := range cfg.SpendingGroups {
		name := strings.TrimSpace(g.Name)
		if err = unique("group:" + normalize(name)); err != nil {
			return err
		}
		id, version, e := rulesetMatch(tx, "spending_groups", "finance_normalize(name)=finance_normalize(?)", name)
		if e != nil {
			return e
		}
		if id != 0 {
			var saved string
			if e = tx.QueryRow("SELECT color FROM spending_groups WHERE id=?", id).Scan(&saved); e != nil {
				return e
			}
			if saved == g.Color {
				continue
			}
		}
		if e = saveSpendingGroupTx(tx, u, &id, spendingGroupInput{Name: name, Color: g.Color, Version: version}); e != nil {
			return e
		}
		summary.SpendingGroupsAdded++
	}
	type archiveChange struct {
		id       int64
		name     string
		archived bool
	}
	archives := []archiveChange{}
	for _, c := range cfg.Categories {
		name := strings.TrimSpace(c.Name)
		if err = unique("category:" + c.GroupName + ":" + normalize(name)); err != nil {
			return err
		}
		if c.Kind != "expense" && c.Kind != "income" {
			return fmt.Errorf("invalid category kind for %q", name)
		}
		if c.SpendingGroup != "" {
			if _, e := rulesetLookup(tx, false, "spending_groups", c.SpendingGroup, ""); e != nil {
				return e
			}
		}
		id, _, e := rulesetMatch(tx, "categories", "finance_normalize(name)=finance_normalize(?) AND group_name=?", name, c.GroupName)
		if e != nil {
			return e
		}
		if id == 0 {
			id, e = createCategoryRecordTx(tx, u, categoryInput{Name: name, Kind: c.Kind}, c.GroupName, true)
			if e != nil {
				return e
			}
			summary.CategoriesAdded++
		} else {
			var kind string
			if e = tx.QueryRow("SELECT kind FROM categories WHERE id=?", id).Scan(&kind); e != nil {
				return e
			}
			if kind != c.Kind {
				return fmt.Errorf("category %q kind cannot change", name)
			}
		}
		if !c.Archived && queryInt(tx, "SELECT archived FROM categories WHERE id=?", id) != 0 {
			version := queryInt(tx, "SELECT version FROM categories WHERE id=?", id)
			if e = updateCategoryTx(tx, u, id, categoryUpdateInput{Name: name, Archived: false, Version: version}); e != nil {
				return e
			}
			summary.CategoriesAdded++
		}
		archives = append(archives, archiveChange{id, name, c.Archived})
	}
	account := func(name string, optional bool) (int64, error) {
		id, e := rulesetLookup(tx, optional, "accounts", name, "")
		if e != nil {
			return 0, e
		}
		if id != 0 && !ruleAccess(tx, a, u, id) {
			return 0, fail(403, "Account editor access required")
		}
		return id, nil
	}
	for _, m := range cfg.Merchants {
		scope, e := account(m.AccountName, true)
		if e != nil {
			return e
		}
		name := strings.TrimSpace(m.Name)
		if e = unique(fmt.Sprintf("merchant:%d:%s", scope, normalize(name))); e != nil {
			return e
		}
		cat, e := rulesetCategory(tx, m.Category, m.CategoryGroupName)
		if e != nil {
			return e
		}
		group, e := rulesetLookup(tx, true, "spending_groups", m.SpendingGroup, "")
		if e != nil {
			return e
		}
		id, version, e := rulesetMatch(tx, "merchants", "COALESCE(account_id,0)=? AND finance_normalize(name)=finance_normalize(?)", scope, name)
		if e != nil {
			return e
		}
		if id != 0 {
			var savedLogo string
			var savedCat, savedGroup sql.NullInt64
			if e = tx.QueryRow("SELECT logo_data,category_id,spending_group_id FROM merchants WHERE id=?", id).Scan(&savedLogo, &savedCat, &savedGroup); e != nil {
				return e
			}
			if savedLogo == m.LogoData && savedCat.Int64 == cat && savedGroup.Int64 == group {
				continue
			}
		}
		input := mcpMerchantInput{Account: scope, Name: name, Version: version, Logo: &m.LogoData, CategoryID: rulesetPointer(cat), SpendingGroupID: rulesetPointer(group), ClearCategory: cat == 0, ClearGroup: group == 0}
		if _, e = a.saveMCPMerchant(tx, u, id, &input); e != nil {
			return e
		}
		summary.MerchantsAdded++
	}
	for _, m := range cfg.MerchantRules {
		scope, e := account(m.AccountName, true)
		if e != nil {
			return e
		}
		merchantScope, e := account(m.MerchantAccountName, true)
		if e != nil {
			return e
		}
		mid, e := rulesetLookup(tx, false, "merchants", m.Merchant, " AND COALESCE(account_id,0)=?", merchantScope)
		if e != nil {
			return e
		}
		direction := m.Direction
		if direction == "" {
			direction = "any"
		}
		pattern := strings.TrimSpace(m.Pattern)
		if e = unique(fmt.Sprintf("merchant-rule:%d:%s:%s", scope, normalize(pattern), direction)); e != nil {
			return e
		}
		id, version, e := rulesetMatch(tx, "merchant_rules", "COALESCE(account_id,0)=? AND finance_normalize(pattern)=finance_normalize(?) AND direction=?", scope, pattern, direction)
		if e != nil {
			return e
		}
		if id != 0 && queryInt(tx, "SELECT COUNT(*) FROM merchant_rules WHERE id=? AND merchant_id=? AND priority=? AND enabled=?", id, mid, m.Priority, m.Enabled) == 1 {
			continue
		}
		if e = a.writeMerchantRule(tx, u, &id, merchantRuleInput{Account: scope, Merchant: mid, Pattern: pattern, Direction: direction, Priority: m.Priority, Enabled: m.Enabled, Version: version}); e != nil {
			return e
		}
		summary.MerchantRulesAdded++
	}
	for _, r := range cfg.Rules {
		scope := int64(0)
		if r.Builtin {
			if strings.TrimSpace(r.AccountName) != "" {
				return fmt.Errorf("built-in rule must not specify an account")
			}
		} else {
			scope, err = account(r.AccountName, false)
			if err != nil {
				return err
			}
		}
		cat, e := rulesetCategory(tx, r.Category, r.CategoryGroupName)
		if e != nil {
			return e
		}
		if cat == 0 {
			return fmt.Errorf("provide the category for every rule")
		}
		group, e := rulesetLookup(tx, true, "spending_groups", r.SpendingGroup, "")
		if e != nil {
			return e
		}
		direction := r.Direction
		if direction == "" {
			direction = "any"
		}
		pattern := strings.TrimSpace(r.Pattern)
		if e = unique(fmt.Sprintf("rule:%t:%d:%s:%s", r.Builtin, scope, normalize(pattern), direction)); e != nil {
			return e
		}
		table := "rules"
		where := "account_id=? AND finance_normalize(pattern)=finance_normalize(?) AND direction=?"
		args := []any{scope, pattern, direction}
		if r.Builtin {
			table = "builtin_rules"
			where = "finance_normalize(pattern)=finance_normalize(?) AND direction=?"
			args = []any{pattern, direction}
		}
		id, version, e := rulesetMatch(tx, table, where, args...)
		if e != nil {
			return e
		}
		if id != 0 && queryInt(tx, "SELECT COUNT(*) FROM "+table+" WHERE id=? AND category_id=? AND COALESCE(spending_group_id,0)=? AND priority=? AND enabled=?", id, cat, group, r.Priority, r.Enabled) == 1 {
			continue
		}
		input := ruleInput{AccountID: scope, Pattern: pattern, CategoryID: cat, SpendingGroupID: rulesetPointer(group), Direction: direction, Priority: r.Priority, Enabled: &r.Enabled, Version: version}
		if r.Builtin {
			if id == 0 {
				if e = a.createBuiltinRule(tx, u, &id, input); e != nil {
					return e
				}
			} else {
				if e = a.writeBuiltinRule(tx, u, -id, input); e != nil {
					return e
				}
			}
		} else {
			if e = a.writeRule(tx, u, &id, &input); e != nil {
				return e
			}
		}
		summary.RulesAdded++
	}
	// Apply archive changes last, after imported rules have been paused or replaced.
	for _, c := range archives {
		var archived bool
		var version int64
		if err = tx.QueryRow("SELECT archived,version FROM categories WHERE id=?", c.id).Scan(&archived, &version); err != nil {
			return err
		}
		if archived == c.archived {
			continue
		}
		if err = updateCategoryTx(tx, u, c.id, categoryUpdateInput{Name: c.name, Archived: c.archived, Version: version}); err != nil {
			return err
		}
		summary.CategoriesAdded++
	}
	return nil
}

type RulesetImportSummary struct {
	SpendingGroupsAdded int `json:"spending_groups_added"`
	CategoriesAdded     int `json:"categories_added"`
	MerchantsAdded      int `json:"merchants_added"`
	MerchantRulesAdded  int `json:"merchant_rules_added"`
	RulesAdded          int `json:"rules_added"`
}

func (s *RulesetImportSummary) String() string {
	return fmt.Sprintf("Created or updated: %d spending groups, %d categories, %d merchants, %d merchant rules, %d rules", s.SpendingGroupsAdded, s.CategoriesAdded, s.MerchantsAdded, s.MerchantRulesAdded, s.RulesAdded)
}
func (a *App) ExportRulesetToFile(path string) error {
	var bytes strings.Builder
	if err := a.ExportRuleset(&bytes); err != nil {
		return err
	}
	if path == "-" || path == "" {
		_, err := io.WriteString(os.Stdout, bytes.String())
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".ruleset-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = io.WriteString(file, bytes.String()); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
func (a *App) ImportRulesetFromFile(path string) (*RulesetImportSummary, error) {
	if path == "-" {
		return a.ImportRuleset(os.Stdin)
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return a.ImportRuleset(f)
}

func decodeRuleset(r io.Reader) (RulesetConfig, error) {
	bytes, err := io.ReadAll(io.LimitReader(r, maxRulesetBytes+1))
	if err != nil {
		return RulesetConfig{}, err
	}
	if len(bytes) > maxRulesetBytes {
		return RulesetConfig{}, fmt.Errorf("ruleset exceeds 32 MiB")
	}
	var cfg RulesetConfig
	decoder := json.NewDecoder(strings.NewReader(string(bytes)))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&cfg); err != nil {
		return RulesetConfig{}, fmt.Errorf("invalid ruleset JSON: %w", err)
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return RulesetConfig{}, fmt.Errorf("ruleset must contain exactly one JSON document")
	}
	if cfg.Version != 1 && cfg.Version != 2 {
		return RulesetConfig{}, fmt.Errorf("unsupported ruleset version %d", cfg.Version)
	}
	if len(cfg.SpendingGroups)+len(cfg.Categories)+len(cfg.Merchants)+len(cfg.MerchantRules)+len(cfg.Rules) > 10000 {
		return RulesetConfig{}, fmt.Errorf("ruleset exceeds 10000 entries")
	}

	return cfg, nil
}
