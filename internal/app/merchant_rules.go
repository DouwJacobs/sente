package app

import (
	"database/sql"
	"net/http"
	"strings"
)

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
	text := "SELECT x.*,m.name merchant_name,m.logo_data merchant_logo,m.version merchant_version,c.name category_name,s.name spending_group_name,s.color spending_group_color,CASE WHEN x.account_id IS NULL THEN 'All accounts' ELSE a.name END account_name FROM merchant_rules x JOIN merchants m ON m.id=x.merchant_id LEFT JOIN categories c ON c.id=x.category_id LEFT JOIN spending_groups s ON s.id=x.spending_group_id LEFT JOIN accounts a ON a.id=x.account_id WHERE (x.account_id IS NULL OR " + accountAccessSQL(u) + ")"
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
	if err := a.browserWrite(r, func(tx *sql.Tx, u User) error { return a.writeMerchantRule(tx, u, &id, b) }); err != nil {
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
	// New legacy requests inherit merchant defaults once. Existing rules retain
	// their own values when classification fields are omitted.
	catID, groupID, err := merchantRuleClassification(tx, localID, b.Merchant)
	if err != nil {
		return err
	}
	if b.ClearCategory {
		catID = sql.NullInt64{}
	} else if b.CategoryID != nil {
		catID = sql.NullInt64{Int64: *b.CategoryID, Valid: true}
	}
	if b.ClearGroup {
		groupID = sql.NullInt64{}
	} else if b.SpendingGroupID != nil {
		groupID = sql.NullInt64{Int64: *b.SpendingGroupID, Valid: true}
	}
	if catID.Valid && queryInt(tx, "SELECT COUNT(*) FROM categories WHERE id=? AND archived=0", catID.Int64) != 1 {
		return fail(400, "Choose an active category")
	}
	if groupID.Valid && queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", groupID.Int64) != 1 {
		return fail(400, "Choose an existing spending group")
	}

	if localID == 0 {
		res, e := tx.Exec("INSERT INTO merchant_rules(account_id,merchant_id,pattern,direction,priority,enabled,category_id,spending_group_id) VALUES(?,?,?,?,?,?,?,?)", nullableAccount(b.Account), b.Merchant, b.Pattern, b.Direction, b.Priority, b.Enabled, catID, groupID)
		if e != nil {
			return e
		}
		localID, _ = res.LastInsertId()
	} else {
		res, e := tx.Exec("UPDATE merchant_rules SET account_id=?,merchant_id=?,pattern=?,direction=?,priority=?,enabled=?,category_id=?,spending_group_id=?,version=version+1 WHERE id=? AND version=?", nullableAccount(b.Account), b.Merchant, b.Pattern, b.Direction, b.Priority, b.Enabled, catID, groupID, localID, b.Version)
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

func merchantRuleClassification(tx *sql.Tx, id, merchant int64) (sql.NullInt64, sql.NullInt64, error) {
	var category, group sql.NullInt64
	query, lookup := "SELECT category_id,spending_group_id FROM merchant_rules WHERE id=?", id
	if id == 0 {
		query, lookup = "SELECT category_id,spending_group_id FROM merchants WHERE id=?", merchant
	}
	err := tx.QueryRow(query, lookup).Scan(&category, &group)
	return category, group, err
}

// Bind inherited values to proposal preparation rather than rereading defaults
// when an approved proposal is later applied.
func canonicalMerchantRule(tx *sql.Tx, id int64, b merchantRuleInput) (merchantRuleInput, error) {
	category, group, err := merchantRuleClassification(tx, id, b.Merchant)
	if err != nil {
		return b, err
	}
	if b.CategoryID == nil && !b.ClearCategory {
		if category.Valid {
			value := category.Int64
			b.CategoryID = &value
		} else {
			b.ClearCategory = true
		}
	}
	if b.SpendingGroupID == nil && !b.ClearGroup {
		if group.Valid {
			value := group.Int64
			b.SpendingGroupID = &value
		} else {
			b.ClearGroup = true
		}
	}
	return b, nil
}
