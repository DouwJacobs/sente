package app

import (
	"database/sql"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Recheck the initiating session and role within the serialized write, after hashing.
func userSecurityActorTx(tx *sql.Tx, r *http.Request, admin bool) error {
	cookie, err := r.Cookie("finance_session")
	if err != nil {
		return fail(401, "Please sign in again")
	}
	var isAdmin bool
	err = tx.QueryRow(`SELECT u.admin FROM users u JOIN sessions s ON s.user_id=u.id WHERE u.id=? AND u.disabled=0 AND u.deleted_at IS NULL AND s.token=? AND s.csrf=? AND s.expires_at>?`, Current(r).ID, hash(cookie.Value), r.Header.Get("X-CSRF-Token"), time.Now().Unix()).Scan(&isAdmin)
	if err != nil {
		return fail(401, "Please sign in again")
	}
	if admin && !isAdmin {
		return fail(403, "Administrator access required")
	}
	return nil
}

func passwordDigest(password string) (string, error) {
	if len(password) < 12 || len(password) > 72 {
		return "", fail(400, "New password must be 12–72 bytes")
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(digest), err
}

// MCP connection deletion cascades through proposals, codes and access/refresh tokens.
// Authorization requests must also be cleared so consent cannot outlive a reset.
func revokeUserAgentsTx(tx *sql.Tx, id int64) error {
	if _, err := tx.Exec("DELETE FROM mcp_oauth_requests WHERE user_id=?", id); err != nil {
		return err
	}
	_, err := tx.Exec("DELETE FROM mcp_tokens WHERE user_id=?", id)
	return err
}

func setUserPasswordTx(tx *sql.Tx, id int64, digest, keepSession string) error {
	res, err := tx.Exec("UPDATE users SET password=?,version=version+1 WHERE id=? AND deleted_at IS NULL", digest, id)
	if err != nil {
		return err
	}
	if err := affected(res); err != nil {
		return err
	}
	if err := revokeUserAgentsTx(tx, id); err != nil {
		return err
	}
	_, err = tx.Exec("DELETE FROM sessions WHERE user_id=? AND token!=?", id, keepSession)
	return err
}

func (a *App) changePassword(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	var b struct {
		Old string `json:"old_password"`
		New string `json:"new_password"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	digest, err := passwordDigest(b.New)
	if err != nil {
		return err
	}
	cookie, err := r.Cookie("finance_session")
	if err != nil {
		return fail(401, "Please sign in again")
	}
	err = a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, false); err != nil {
			return err
		}
		var old string
		if err := tx.QueryRow("SELECT password FROM users WHERE id=?", u.ID).Scan(&old); err != nil {
			return err
		}
		if bcrypt.CompareHashAndPassword([]byte(old), []byte(b.Old)) != nil {
			return fail(400, "Current password is incorrect")
		}
		if err := setUserPasswordTx(tx, u.ID, digest, hash(cookie.Value)); err != nil {
			return err
		}
		return audit(tx, u, nil, "user", u.ID, "password_changed", map[string]any{"current_session_retained": true, "agents_revoked": true})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

func (a *App) adminPassword(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	id := parseID(r)
	if id == u.ID {
		return fail(400, "Use Change your password in Security")
	}
	var b struct {
		New     string `json:"new_password"`
		Version int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	digest, err := passwordDigest(b.New)
	if err != nil {
		return err
	}
	err = a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, true); err != nil {
			return err
		}
		var version int64
		if err := tx.QueryRow("SELECT version FROM users WHERE id=? AND deleted_at IS NULL", id).Scan(&version); err != nil {
			return fail(404, "User not found")
		}
		if version != b.Version {
			return fail(409, "User changed; reload before continuing")
		}
		if err := setUserPasswordTx(tx, id, digest, ""); err != nil {
			return err
		}
		return audit(tx, u, nil, "user", id, "password_reset", map[string]any{"all_sessions_revoked": true, "agents_revoked": true})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}

func (a *App) deleteUser(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	id := parseID(r)
	var b struct {
		Username string `json:"confirm_username"`
		Version  int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, true); err != nil {
			return err
		}
		var username string
		var version int64
		var admin bool
		if err := tx.QueryRow("SELECT username,version,admin FROM users WHERE id=? AND deleted_at IS NULL", id).Scan(&username, &version, &admin); err != nil {
			return fail(404, "User not found")
		}
		if version != b.Version {
			return fail(409, "User changed; reload before continuing")
		}
		if admin && queryInt(tx, "SELECT COUNT(*) FROM users WHERE admin=1 AND disabled=0 AND deleted_at IS NULL AND id!=?", id) == 0 {
			return fail(400, "Keep at least one enabled administrator")
		}
		if id == u.ID {
			return fail(400, "Ask another administrator to delete your user")
		}
		if b.Username != username {
			return fail(400, "Type the username exactly to confirm deletion")
		}
		// Preserve the identity row referenced by immutable ledger provenance and audits.
		if _, err := tx.Exec("UPDATE users SET password='',admin=0,budget_member=0,disabled=1,deleted_at=CURRENT_TIMESTAMP,version=version+1 WHERE id=?", id); err != nil {
			return err
		}
		if err := revokeUserAgentsTx(tx, id); err != nil {
			return err
		}
		for _, table := range []string{"notification_receipts", "notification_preferences", "mcp_user_context", "sessions", "grants", "transaction_seen", "fnb_connections", "fnb_discoveries"} {
			if _, err := tx.Exec("DELETE FROM "+table+" WHERE user_id=?", id); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "user", id, "deleted", map[string]any{"historical_identity_retained": true, "access_revoked": true})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
