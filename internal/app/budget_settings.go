package app

import (
	"database/sql"
	"net/http"
)

func (a *App) settings(w http.ResponseWriter, r *http.Request) error {
	if err := requireMember(Current(r)); err != nil {
		return err
	}
	send(w, map[string]any{"start_day": queryInt(a.DB, "SELECT start_day FROM settings WHERE id=1"), "currency": "ZAR", "timezone": "Africa/Johannesburg"})
	return nil
}
func (a *App) updateSettings(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireMember(u); err != nil {
		return err
	}
	var b struct {
		Day int `json:"start_day"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Day < 1 || b.Day > 31 {
		return fail(400, "Start day must be 1–31")
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireMember(u); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE settings SET start_day=? WHERE id=1", b.Day); err != nil {
			return err
		}
		return audit(tx, u, nil, "settings", 1, "updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
