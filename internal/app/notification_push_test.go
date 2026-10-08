package app

import (
	"context"
	"crypto/elliptic"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"
)

func syntheticPush(t *testing.T, suffix string) webpush.Subscription {
	t.Helper()
	_, x, y, err := elliptic.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return webpush.Subscription{Endpoint: "https://fcm.googleapis.com/fcm/send/synthetic-" + suffix, Keys: webpush.Keys{P256dh: base64.RawURLEncoding.EncodeToString(elliptic.Marshal(elliptic.P256(), x, y)), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}}
}
func registerSyntheticPush(t *testing.T, e *testEnv, uid int, sub webpush.Subscription) int64 {
	t.Helper()
	w := e.req(t, uid, "/api/notifications/push/subscriptions", "POST", sub)
	status(t, w, 200)
	var response struct {
		ID int64 `json:"id"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	return response.ID
}
func enablePushType(t *testing.T, e *testEnv, kind string) {
	t.Helper()
	status(t, e.req(t, 1, "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"items": []any{map[string]any{"type": kind, "channel": "push", "enabled": true, "version": 0}}}), 200)
}
func TestNotificationPushOwnershipConsentValidationAndCSRF(t *testing.T) {
	e := setup(t)
	sub := syntheticPush(t, "ownership")
	w := e.req(t, 1, "/api/notifications/push", "GET", nil)
	status(t, w, 200)
	if strings.Contains(w.Body.String(), "private_key") || !strings.Contains(w.Body.String(), `"public_key":""`) {
		t.Fatal("GET created or leaked private key")
	}
	id := registerSyntheticPush(t, e, 1, sub)
	if other := registerSyntheticPush(t, e, 1, sub); other != id {
		t.Fatal("repeat device duplicated")
	}
	status(t, e.req(t, 2, "/api/notifications/push/subscriptions", "POST", sub), 409)
	status(t, e.req(t, 2, fmt.Sprintf("/api/notifications/push/subscriptions/%d", id), "DELETE", nil), 404)
	status(t, e.req(t, 2, fmt.Sprintf("/api/notifications/push/subscriptions/%d/test", id), "POST", nil), 404)
	w = e.req(t, 1, "/api/notifications/push", "GET", nil)
	status(t, w, 200)
	for _, secret := range []string{sub.Endpoint, sub.Keys.Auth, sub.Keys.P256dh, "private_key"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("push secret exposed")
		}
	}
	for _, endpoint := range []string{"http://fcm.googleapis.com/test", "https://127.0.0.1/test", "https://fcm.googleapis.com.attacker.invalid/test", "https://user@fcm.googleapis.com/test", "https://web.push.apple.com:444/test", "https://example.com/test"} {
		invalid := sub
		invalid.Endpoint = endpoint
		status(t, e.req(t, 1, "/api/notifications/push/subscriptions", "POST", invalid), 400)
	}
	invalid := sub
	invalid.Keys.Auth = "bad"
	status(t, e.req(t, 1, "/api/notifications/push/subscriptions", "POST", invalid), 400)
	r := httptest.NewRequest("POST", "/api/notifications/push/subscriptions", strings.NewReader(`{"setup":true}`))
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	r = httptest.NewRequest("DELETE", fmt.Sprintf("/api/notifications/push/subscriptions/%d", id), nil)
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("X-CSRF-Token", "csrf")
	r.Header.Set("Origin", "https://attacker.invalid")
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	deliverSynthetic(t, e.a, syntheticNotification(1, "no-consent"))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox") != 0 {
		t.Fatal("device subscription silently enabled notification types")
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/push/subscriptions/%d/test", id), "POST", nil), 200)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/push/subscriptions/%d/test", id), "POST", nil), 429)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/push/subscriptions/%d", id), "DELETE", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox") != 0 {
		t.Fatal("revocation retained device jobs")
	}
}
func TestNotificationPushIndependentChannelsAtomicPreferencesAndRetry(t *testing.T) {
	e := setup(t)
	sub := syntheticPush(t, "independent")
	registerSyntheticPush(t, e, 1, sub)
	status(t, e.req(t, 1, "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"items": []any{
		map[string]any{"type": "system", "channel": "in_app", "enabled": false, "version": 0},
		map[string]any{"type": "system", "channel": "push", "enabled": true, "version": 0},
	}}), 200)
	event := syntheticNotification(1, "push-only")
	first := deliverSynthetic(t, e.a, event)
	second := deliverSynthetic(t, e.a, event)
	if first.ID != second.ID || queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox") != 1 {
		t.Fatal("retry duplicated push outbox")
	}
	w := e.req(t, 1, "/api/notifications", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"inbox_total":0`) {
		t.Fatal("push-only record entered in-app inbox", w.Body.String())
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/push/messages/%d", first.ID), "GET", nil), 200)
	status(t, e.req(t, 2, fmt.Sprintf("/api/notifications/push/messages/%d", first.ID), "GET", nil), 404)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/read", first.ID), "POST", nil), 200)
	sends := 0
	e.a.pushSender = func(context.Context, webpush.Subscription, string, string, string, string, int64) (int, error) {
		sends++
		return 201, nil
	}
	if err := e.a.dispatchNotificationPush(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatal("in-app preference disabled push channel")
	}
	if err := e.a.dispatchNotificationPush(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 1 {
		t.Fatal("dispatcher repeated sent job")
	}
	// A stale push version rolls back the simultaneously changed in-app type.
	status(t, e.req(t, 1, "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"items": []any{
		map[string]any{"type": "system", "channel": "in_app", "enabled": true, "version": 1},
		map[string]any{"type": "system", "channel": "push", "enabled": false, "version": 0},
	}}), 409)
	if queryInt(e.a.DB, "SELECT enabled FROM notification_preferences WHERE user_id=1 AND type='system'") != 0 {
		t.Fatal("mixed preference batch partly saved")
	}
}
func TestNotificationPushPermissionRecheckAndPreferenceRevocation(t *testing.T) {
	e := setup(t)
	registerSyntheticPush(t, e, 1, syntheticPush(t, "permissions"))
	enablePushType(t, e, "unusual_spending")
	event := syntheticNotification(1, "private-push")
	event.Type = "unusual_spending"
	event.SourceKind = "account"
	event.SourceID = 2
	event.AccountIDs = []int64{2}
	deliverSynthetic(t, e.a, event)
	nid := queryInt(e.a.DB, "SELECT MAX(id) FROM notifications")
	migrationExec(t, e.a.DB, "DELETE FROM grants WHERE user_id=1 AND account_id=2")
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/push/messages/%d", nid), "GET", nil), 404)
	sends := 0
	e.a.pushSender = func(context.Context, webpush.Subscription, string, string, string, string, int64) (int, error) {
		sends++
		return 201, nil
	}
	if err := e.a.dispatchNotificationPush(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox WHERE status='permission_revoked'") != 1 {
		t.Fatal("revoked account sent externally")
	}
	migrationExec(t, e.a.DB, "INSERT INTO grants VALUES(1,2,'editor')")
	event.DedupeKey = "disabled-before-send"
	deliverSynthetic(t, e.a, event)
	migrationExec(t, e.a.DB, "UPDATE notification_push_preferences SET enabled=0")
	if err := e.a.dispatchNotificationPush(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 {
		t.Fatal("revoked push preference sent")
	}
}
func TestNotificationPushFailureExpiryAndUncertainRecovery(t *testing.T) {
	for _, c := range []struct {
		name      string
		status    int
		sendError bool
		expected  string
	}{
		{"temporary provider", 503, false, "pending"}, {"expired subscription", 410, false, "channel_failure"}, {"uncertain transport", 0, true, "uncertain"}, {"rejected", 400, false, "channel_failure"},
	} {
		t.Run(c.name, func(t *testing.T) {
			e := setup(t)
			registerSyntheticPush(t, e, 1, syntheticPush(t, c.name))
			enablePushType(t, e, "system")
			deliverSynthetic(t, e.a, syntheticNotification(1, "failure"))
			e.a.pushSender = func(context.Context, webpush.Subscription, string, string, string, string, int64) (int, error) {
				if c.sendError {
					return 0, errors.New("SENSITIVE provider body endpoint auth secret")
				}
				return c.status, nil
			}
			now := time.Now()
			if err := e.a.dispatchNotificationPush(now); err != nil {
				t.Fatal(err)
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 1 {
				t.Fatal("push failure lost in-app message")
			}
			w := e.req(t, 1, "/api/notifications/diagnostics?type=system", "GET", nil)
			status(t, w, 200)
			if strings.Contains(w.Body.String(), "SENSITIVE") || strings.Contains(w.Body.String(), "auth secret") {
				t.Fatal("provider error leaked")
			}
			if c.status == 410 {
				if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_subscriptions") != 0 {
					t.Fatal("expired device retained secrets")
				}
			} else {
				if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox WHERE status=?", c.expected) != 1 {
					t.Fatal("wrong lifecycle")
				}
				if c.status == 503 {
					if err := e.a.dispatchNotificationPush(now.Add(time.Minute)); err != nil {
						t.Fatal(err)
					}
					if err := e.a.dispatchNotificationPush(now.Add(3 * time.Minute)); err != nil {
						t.Fatal(err)
					}
					if queryInt(e.a.DB, "SELECT attempts FROM notification_push_outbox") != 3 {
						t.Fatal("retry attempts unbounded/wrong")
					}
				}
			}
		})
	}
	e := setup(t)
	registerSyntheticPush(t, e, 1, syntheticPush(t, "restart"))
	enablePushType(t, e, "system")
	deliverSynthetic(t, e.a, syntheticNotification(1, "uncertain-restart"))
	migrationExec(t, e.a.DB, "UPDATE notification_push_outbox SET status='sending'")
	if err := e.a.writeQuiet(func(tx *sql.Tx) error { return recoverUncertainPushTx(tx, time.Now()) }); err != nil {
		t.Fatal(err)
	}
	sends := 0
	e.a.pushSender = func(context.Context, webpush.Subscription, string, string, string, string, int64) (int, error) {
		sends++
		return 201, nil
	}
	if err := e.a.dispatchNotificationPush(time.Now()); err != nil {
		t.Fatal(err)
	}
	if sends != 0 {
		t.Fatal("restart retried uncertain provider acceptance")
	}
}
func TestNotificationAlertMigrationRollbackRestartAndPreservation(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:2], 24); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused'); INSERT INTO notification_preferences(user_id,type,channel,enabled) VALUES(1,'system','in_app',0); CREATE TRIGGER reject_alert_migration BEFORE INSERT ON migrations WHEN NEW.version=26 BEGIN SELECT RAISE(ABORT,'synthetic'); END`)
	before := migrationSnapshot(t, db, "SELECT * FROM users", "SELECT * FROM notification_preferences")
	if err := migrate(db); err == nil {
		t.Fatal("migration failure ignored")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 24 || queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='notification_conditions'") != 0 {
		t.Fatal("pending migration batch partly committed")
	}
	migrationExec(t, db, "DROP TRIGGER reject_alert_migration")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, "SELECT * FROM users", "SELECT * FROM notification_preferences") {
		t.Fatal("upgrade changed protected state")
	}
	migrationExec(t, db, "INSERT INTO notification_conditions(user_id,type,scope_hash,active,cycle,consumed,evaluated_at) VALUES(1,'system','synthetic',1,2,1,1)")
	preserved := migrationSnapshot(t, db, "SELECT * FROM notification_conditions")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if preserved != migrationSnapshot(t, db, "SELECT * FROM notification_conditions") {
		t.Fatal("restart rewrote state")
	}
}

func TestNotificationPushOnlyReadDismissAndExpiry(t *testing.T) {
	e := setup(t)
	status(t, e.req(t, 1, "/api/notifications/preferences/batch?channels=all", "PUT", map[string]any{"items": []any{
		map[string]any{"type": "system", "channel": "in_app", "enabled": false, "version": 0}, map[string]any{"type": "system", "channel": "push", "enabled": true, "version": 0}}}), 200)
	item := deliverSynthetic(t, e.a, syntheticNotification(1, "push-link"))
	path := fmt.Sprintf("/api/notifications/push/messages/%d", item.ID)
	status(t, e.req(t, 1, path, "GET", nil), 200)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/read", item.ID), "POST", nil), 200)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/dismiss", item.ID), "POST", nil), 200)
	status(t, e.req(t, 1, path, "GET", nil), 404)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_push_outbox") != 0 {
		t.Fatal("no-device channel queued work")
	}
}
