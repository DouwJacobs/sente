package app

import (
	"crypto/elliptic"
	"database/sql"
	"encoding/base64"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func (a *App) notificationPushRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/notifications/push", wrap(a.pushSettings))
	m.HandleFunc("POST /api/notifications/push/subscriptions", wrap(a.registerPush))
	m.HandleFunc("DELETE /api/notifications/push/subscriptions/{id}", wrap(a.removePush))
	m.HandleFunc("POST /api/notifications/push/subscriptions/{id}/test", wrap(a.testPush))
}

func validPushEndpoint(raw string) bool {
	if len(raw) > 2048 {
		return false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Fragment != "" || (u.Port() != "" && u.Port() != "443") {
		return false
	}
	host := strings.ToLower(u.Hostname())
	// Subscription URLs are untrusted user input, never general HTTP targets.
	// Only supported browser push providers are accepted; transport also pins
	// public DNS addresses and refuses redirects to prevent SSRF/rebinding.
	return host == "fcm.googleapis.com" || host == "updates.push.services.mozilla.com" || strings.HasSuffix(host, ".push.services.mozilla.com") || host == "web.push.apple.com" || strings.HasSuffix(host, ".notify.windows.com")
}
func validatePushSubscription(s webpush.Subscription) error {
	if !validPushEndpoint(s.Endpoint) {
		return fail(400, "Unsupported browser push endpoint")
	}
	public, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.P256dh, "="))
	if err != nil || len(public) != 65 {
		return fail(400, "Invalid browser push public key")
	}
	x, _ := elliptic.Unmarshal(elliptic.P256(), public)
	if x == nil {
		return fail(400, "Invalid browser push public key")
	}
	auth, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(s.Keys.Auth, "="))
	if err != nil || len(auth) != 16 {
		return fail(400, "Invalid browser push authentication key")
	}
	return nil
}
func pushKeysTx(tx *sql.Tx) (string, string, error) {
	var private, public string
	err := tx.QueryRow("SELECT private_key,public_key FROM notification_push_config WHERE id=1").Scan(&private, &public)
	if errors.Is(err, sql.ErrNoRows) {
		private, public, err = webpush.GenerateVAPIDKeys()
		if err == nil {
			_, err = tx.Exec("INSERT INTO notification_push_config VALUES(1,?,?)", private, public)
		}
	}
	return private, public, err
}
func (a *App) pushSettings(w http.ResponseWriter, r *http.Request) error {
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var public string
	err = tx.QueryRow("SELECT public_key FROM notification_push_config WHERE id=1").Scan(&public)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	// The public application key is created by explicit registration setup POST,
	// never by a read, so enabling has no consent implications on page load.
	items, err := data(tx, "SELECT id,endpoint,enabled,created_at FROM notification_push_subscriptions WHERE user_id=? ORDER BY id DESC", Current(r).ID)
	if err != nil {
		return err
	}
	for _, item := range items {
		item["endpoint_hash"] = hash(item["endpoint"].(string))
		delete(item, "endpoint")
	}
	preferences := []notificationPreference{}
	for _, kind := range notificationTypes {
		pref := notificationPreference{Type: kind, Channel: "push"}
		err := tx.QueryRow("SELECT enabled,version FROM notification_push_preferences WHERE user_id=? AND type=?", Current(r).ID, kind).Scan(&pref.Enabled, &pref.Version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		preferences = append(preferences, pref)
	}
	send(w, map[string]any{"public_key": public, "items": items, "preferences": preferences})
	return nil
}

// A setup request obtains the public key before the browser creates its
// subscription. The empty input is explicit; ordinary GET never generates keys.
func (a *App) registerPush(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Setup    bool         `json:"setup"`
		Endpoint string       `json:"endpoint"`
		Keys     webpush.Keys `json:"keys"`
	}
	if err := decode(r, &input); err != nil {
		return err
	}
	if input.Setup && (input.Endpoint != "" || input.Keys.Auth != "" || input.Keys.P256dh != "") {
		return fail(400, "Invalid push setup")
	}
	sub := webpush.Subscription{Endpoint: input.Endpoint, Keys: input.Keys}
	if !input.Setup {
		if err := validatePushSubscription(sub); err != nil {
			return err
		}
	}
	var id int64
	var public string
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		_, key, err := pushKeysTx(tx)
		if err != nil {
			return err
		}
		public = key
		if input.Setup {
			return nil
		}
		uid := u.ID
		var owner int64
		err = tx.QueryRow("SELECT id,user_id FROM notification_push_subscriptions WHERE endpoint=?", sub.Endpoint).Scan(&id, &owner)
		if err == nil && owner != uid {
			return fail(409, "This browser subscription belongs to another user; reset its browser subscription first")
		}
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if id == 0 {
			if queryInt(tx, "SELECT COUNT(*) FROM notification_push_subscriptions WHERE user_id=?", uid) >= 10 {
				return fail(400, "Remove a device before adding another (maximum 10)")
			}
			result, err := tx.Exec("INSERT INTO notification_push_subscriptions(user_id,endpoint,p256dh,auth,created_at) VALUES(?,?,?,?,?)", uid, sub.Endpoint, sub.Keys.P256dh, sub.Keys.Auth, time.Now().Unix())
			if err != nil {
				return err
			}
			id, err = result.LastInsertId()
			if err != nil {
				return err
			}
		} else {
			if _, err := tx.Exec("UPDATE notification_push_subscriptions SET p256dh=?,auth=?,enabled=1 WHERE id=? AND user_id=?", sub.Keys.P256dh, sub.Keys.Auth, id, uid); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "notification_push", id, "registered", map[string]any{"device_id": id})
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"id": id, "public_key": public})
	return nil
}
func (a *App) removePush(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	if id <= 0 {
		return fail(400, "Choose a device")
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		result, err := tx.Exec("DELETE FROM notification_push_subscriptions WHERE id=? AND user_id=?", id, u.ID)
		if err != nil {
			return err
		}
		n, _ := result.RowsAffected()
		if n == 0 {
			return fail(404, "Device unavailable")
		}
		return audit(tx, u, nil, "notification_push", id, "removed", map[string]any{"device_id": id})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) testPush(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if queryInt(tx, "SELECT COUNT(*) FROM notification_push_subscriptions WHERE id=? AND user_id=? AND enabled=1", id, u.ID) != 1 {
			return fail(404, "Device unavailable")
		}
		now := time.Now()
		if queryInt(tx, "SELECT COUNT(*) FROM notification_push_outbox WHERE subscription_id=? AND type='system' AND notification_id IS NULL AND created_at>?", id, now.Add(-5*time.Minute).Unix()) > 0 {
			return fail(429, "Wait five minutes before another test")
		}
		_, err := tx.Exec("INSERT INTO notification_push_outbox(subscription_id,type,key_hash,next_attempt,created_at) VALUES(?,'system',?,?,?)", id, hash(randomToken()), now.Unix(), now.Unix())
		return err
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
