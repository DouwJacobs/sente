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
