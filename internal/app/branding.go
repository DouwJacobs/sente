package app

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (a *App) branding(w http.ResponseWriter, r *http.Request) error {
	rows, err := data(a.DB, "SELECT display_name,version FROM workspace_branding WHERE id=1")
	if err != nil {
		return err
	}
	send(w, rows[0])
	return nil
}

func (a *App) updateBranding(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if !u.Admin {
		return fail(403, "Administrator access required")
	}
	var b struct {
		Name    string `json:"display_name"`
		Version int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 2 || utf8.RuneCountInString(b.Name) > 60 {
		return fail(400, "Use a workspace name of 2–60 characters")
	}
	err := a.write(func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE workspace_branding SET display_name=?,version=version+1 WHERE id=1 AND version=?", b.Name, b.Version)
		if err != nil {
			return err
		}
		if err := affected(result); err != nil {
			return err
		}
		return audit(tx, u, nil, "workspace_branding", 1, "updated", b)
	})
	if err != nil {
		return err
	}
	return a.branding(w, r)
}
