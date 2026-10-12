package app

import (
	"database/sql"
	"net/http"
	"time"
)

// Scheduled runs have no browser credential. Manual runs retain their initiating
// credential and must recheck it after network work, as well as owner/grants.
func fnbActorTx(tx *sql.Tx, u User) (User, error) {
	var actor User
	err := tx.QueryRow("SELECT id,username,admin,budget_member FROM users WHERE id=? AND disabled=0 AND deleted_at IS NULL", u.ID).Scan(&actor.ID, &actor.Username, &actor.Admin, &actor.Member)
	if err != nil || !actor.Admin {
		return actor, fail(403, "Connection access revoked")
	}
	actor.browserSession, actor.browserCSRF = u.browserSession, u.browserCSRF
	actor.fnbCredentialHash = u.fnbCredentialHash
	if u.fnbCredentialHash != "" {
		var secret []byte
		if tx.QueryRow("SELECT secret FROM fnb_connections WHERE user_id=?", u.ID).Scan(&secret) != nil || hash(string(secret)) != u.fnbCredentialHash {
			return actor, fail(403, "Connection access revoked")
		}
	}
	if u.browserSession != "" && queryInt(tx, "SELECT COUNT(*) FROM sessions WHERE token=? AND user_id=? AND csrf=? AND expires_at>?", u.browserSession, u.ID, u.browserCSRF, time.Now().Unix()) != 1 {
		return actor, fail(401, "Please sign in again")
	}
	return actor, nil
}
func (a *App) fnbWrite(u User, fn func(*sql.Tx, User) error) error {
	return a.write(func(tx *sql.Tx) error {
		actor, err := fnbActorTx(tx, u)
		if err != nil {
			return err
		}
		return fn(tx, actor)
	})
}
func (a *App) runFNBBrowser(r *http.Request, transactions bool, scope ...int64) ([]ParsedFile, error) {
	return a.runFNBRequest(Current(r).ID, true, transactions, r, scope...)
}

// Password recovery commits before cancelling. Do not hold the connector lock:
// a running provider must be able to observe cancellation and finish cleanup.
func (a *App) cancelFNBRun(owner int64) {
	a.fnbCancelMu.Lock()
	defer a.fnbCancelMu.Unlock()
	if a.fnbCancel != nil && a.fnbCancelOwner == owner {
		a.fnbCancel()
	}
}
