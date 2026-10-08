package app

import (
	"database/sql"
	"net/http"
	"time"
)

// browserActor checks the initiating credential and refreshes roles in the same
// serialized transaction as the mutation. GET consent binding needs no CSRF.
func browserActor(q queryer, r *http.Request) (User, error) {
	var u User
	cookie, err := r.Cookie("finance_session")
	if err != nil {
		return u, fail(401, "Please sign in again")
	}
	var csrf string
	err = q.QueryRow(`SELECT u.id,u.username,u.admin,u.budget_member,s.csrf FROM users u JOIN sessions s ON s.user_id=u.id WHERE u.id=? AND u.disabled=0 AND u.deleted_at IS NULL AND s.token=? AND s.expires_at>?`, Current(r).ID, hash(cookie.Value), time.Now().Unix()).Scan(&u.ID, &u.Username, &u.Admin, &u.Member, &csrf)
	if err != nil {
		return u, fail(401, "Please sign in again")
	}
	if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("X-CSRF-Token") != csrf {
		return u, fail(403, "Invalid security token")
	}
	return u, nil
}

func (a *App) browserWrite(r *http.Request, fn func(*sql.Tx, User) error) error {
	return a.write(func(tx *sql.Tx) error {
		u, err := browserActor(tx, r)
		if err != nil {
			return err
		}
		return fn(tx, u)
	})
}
