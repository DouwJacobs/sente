package app

import (
	"database/sql"
	"net/http"
	"strings"
)

func (a *App) users(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT id,username,admin,budget_member,disabled,version FROM users WHERE deleted_at IS NULL", []any{}, "username", "username,id")
	}
	v, err := data(a.DB, "SELECT id,username,admin,budget_member,disabled,version FROM users WHERE deleted_at IS NULL ORDER BY username LIMIT 100")
	if err != nil {
		return err
	}
	send(w, v)
	return nil
}
func (a *App) createUser(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Admin    bool   `json:"admin"`
		Member   bool   `json:"budget_member"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	id, err := a.CreateUser(b.Username, b.Password, b.Admin, b.Member)
	if err != nil {
		return err
	}
	if err := audit(a.DB, u, nil, "user", id, "created", map[string]any{"username": b.Username, "admin": b.Admin, "budget_member": b.Member}); err != nil {
		return err
	}
	send(w, map[string]int64{"id": id})
	return nil
}
func (a *App) updateUser(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	id := parseID(r)
	var b struct {
		Username string `json:"username"`
		Admin    bool   `json:"admin"`
		Member   bool   `json:"budget_member"`
		Disabled bool   `json:"disabled"`
		Version  int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	b.Username = strings.TrimSpace(b.Username)
	if len(b.Username) < 2 || len(b.Username) > 80 {
		return fail(400, "Username must be 2–80 characters")
	}
	err := a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, true); err != nil {
			return err
		}
		var wasAdmin bool
		if err := tx.QueryRow("SELECT admin FROM users WHERE id=? AND deleted_at IS NULL", id).Scan(&wasAdmin); err != nil {
			return fail(404, "User not found")
		}
		if wasAdmin && (!b.Admin || b.Disabled) && queryInt(tx, "SELECT COUNT(*) FROM users WHERE admin=1 AND disabled=0 AND id!=?", id) == 0 {
			return fail(400, "Keep at least one enabled administrator")
		}
		res, err := tx.Exec("UPDATE users SET username=?,admin=?,budget_member=?,disabled=?,version=version+1 WHERE id=? AND version=?", b.Username, b.Admin, b.Member, b.Disabled, id, b.Version)
		if err != nil {
			return fail(409, "Username already exists")
		}
		if err := affected(res); err != nil {
			return err
		}
		if b.Disabled {
			if err := revokeUserAgentsTx(tx, id); err != nil {
				return err
			}
			if _, err := tx.Exec("DELETE FROM sessions WHERE user_id=?", id); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "user", id, "updated", b)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
