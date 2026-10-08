package app

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"
)

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
	e := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		var e error
		id, e = labelID(tx, u, a, b.Account, b.Kind, b.Name, true)
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
				if e = updateMerchantLogo(tx, a, u, id, version, *b.Logo); e != nil {
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
	e := a.browserWrite(r, func(tx *sql.Tx, u User) error {
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
