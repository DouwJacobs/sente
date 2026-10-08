package app

import (
	"database/sql"
	"net/http"
	"strings"
	"unicode/utf8"
)

func (a *App) branding(w http.ResponseWriter, r *http.Request) error {
	rows, err := data(a.DB, "SELECT display_name,workspace_branding.version,logo FROM workspace_branding JOIN app_identity ON app_identity.id=workspace_branding.id WHERE workspace_branding.id=1")
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
		Name    string  `json:"display_name"`
		Version int64   `json:"version"`
		Logo    *string `json:"logo"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	b.Name = strings.TrimSpace(b.Name)
	if utf8.RuneCountInString(b.Name) < 2 || utf8.RuneCountInString(b.Name) > 60 {
		return fail(400, "Use a workspace name of 2–60 characters")
	}
	if b.Logo != nil {
		normalized, err := normalizeIcon(*b.Logo)
		if err != nil {
			return err
		}
		b.Logo = &normalized
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		result, err := tx.Exec("UPDATE workspace_branding SET display_name=?,version=version+1 WHERE id=1 AND version=?", b.Name, b.Version)
		if err != nil {
			return err
		}
		if err := affected(result); err != nil {
			return err
		}
		if b.Logo != nil {
			if _, err := tx.Exec("UPDATE app_identity SET logo=? WHERE id=1", *b.Logo); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "workspace_branding", 1, "updated", map[string]any{"display_name": b.Name, "version": b.Version, "logo_changed": b.Logo != nil})
	})
	if err != nil {
		return err
	}
	return a.branding(w, r)
}
