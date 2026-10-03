package app

import (
	"crypto/subtle"
	"database/sql"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// Disabled administrators still close onboarding. Recovery must never reopen
// an unauthenticated route that can claim an existing installation.
func (a *App) setupStatus(w http.ResponseWriter, r *http.Request) error {
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE admin=1").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		send(w, map[string]any{"required": false})
		return nil
	}
	send(w, map[string]any{"required": true, "csrf": a.setupToken})
	return nil
}

func (a *App) setupAdmin(w http.ResponseWriter, r *http.Request) error {
	// This endpoint is deliberately usable only by the configured browser origin,
	// with a token obtained from the same-origin status response.
	if r.Header.Get("Origin") != a.PublicURL ||
		subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(a.setupToken)) != 1 {
		return fail(403, "Reload setup from this application's address before continuing")
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &body); err != nil {
		return err
	}
	body.Username = strings.TrimSpace(body.Username)
	if len(body.Username) < 2 || len(body.Username) > 80 || len(body.Password) < 12 || len(body.Password) > 72 {
		return fail(400, "Use a username of 2–80 characters and password of 12–72 bytes")
	}
	var count int
	if err := a.DB.QueryRow("SELECT COUNT(*) FROM users WHERE admin=1").Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return fail(409, "Setup is already complete. Please sign in.")
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	token, csrf := randomToken(), randomToken()
	expires := time.Now().Add(7 * 24 * time.Hour)
	u := User{Username: body.Username, Admin: true, Member: true}
	err = a.write(func(tx *sql.Tx) error {
		var count int
		if err := tx.QueryRow("SELECT COUNT(*) FROM users WHERE admin=1").Scan(&count); err != nil {
			return err
		}
		if count > 0 {
			return fail(409, "Setup is already complete. Please sign in.")
		}
		result, err := tx.Exec("INSERT INTO users(username,password,admin,budget_member) VALUES(?,?,1,1)", body.Username, string(digest))
		if err != nil {
			return fail(409, "Username already exists. Choose another username.")
		}
		u.ID, err = result.LastInsertId()
		if err != nil {
			return err
		}
		if _, err := tx.Exec("INSERT INTO sessions VALUES(?,?,?,?)", hash(token), u.ID, csrf, expires.Unix()); err != nil {
			return err
		}
		return audit(tx, u, nil, "user", u.ID, "setup", map[string]bool{"admin": true, "budget_member": true})
	})
	if err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "finance_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, Expires: expires})
	send(w, map[string]any{"user": u, "csrf": csrf})
	return nil
}
