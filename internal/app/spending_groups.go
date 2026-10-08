package app

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (a *App) spendingGroups(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT * FROM spending_groups", []any{}, "name", "id")
	}
	rows, err := data(a.DB, "SELECT * FROM spending_groups ORDER BY id LIMIT 100")
	if err != nil {
		return err
	}
	send(w, rows)
	return nil
}

func (a *App) saveSpendingGroup(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b spendingGroupInput
	if err := decode(r, &b); err != nil {
		return err
	}
	id := parseID(r)
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireMember(u); err != nil {
			return err
		}
		return saveSpendingGroupTx(tx, u, &id, b)
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"id": id})
	return nil
}

type spendingGroupInput struct {
	Name    string `json:"name"`
	Color   string `json:"color"`
	Version int64  `json:"version"`
}

func saveSpendingGroupTx(tx *sql.Tx, u User, id *int64, b spendingGroupInput) error {
	if err := requireMember(u); err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 2 || utf8.RuneCountInString(b.Name) > 80 {
		return fail(400, "Use a spending group name of 2–80 characters")
	}
	switch b.Color {
	case "blue", "amber", "purple", "orange", "teal", "slate", "rose":
	default:
		return fail(400, "Choose an available group color")
	}
	if queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE name=? COLLATE NOCASE AND id!=?", b.Name, *id) > 0 {
		return fail(409, "Spending group already exists")
	}
	if *id == 0 {
		result, err := tx.Exec("INSERT INTO spending_groups(name,color) VALUES(?,?)", b.Name, b.Color)
		if err != nil {
			return err
		}
		*id, _ = result.LastInsertId()
	} else {
		if queryInt(tx, "SELECT COUNT(*) FROM spending_groups WHERE id=?", *id) == 0 {
			return fail(404, "Spending group not found")
		}
		result, err := tx.Exec("UPDATE spending_groups SET name=?,color=?,version=version+1 WHERE id=? AND version=?", b.Name, b.Color, *id, b.Version)
		if err != nil {
			return err
		}
		if err := affected(result); err != nil {
			return err
		}
	}
	return audit(tx, u, nil, "spending_group", *id, "saved", b)
}
