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
