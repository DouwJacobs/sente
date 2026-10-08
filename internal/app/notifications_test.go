package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func syntheticNotification(uid int64, key string) notificationEvent {
	return notificationEvent{RecipientID: uid, Type: "system", Severity: "info", Title: "Sente update", Message: "A synthetic system event is available.", SourceKind: "system", DedupeKey: key, OccurredAt: time.Now().Add(-time.Minute).UTC().Truncate(time.Second), Dismissible: true}
}
func deliverSynthetic(t *testing.T, a *App, e notificationEvent) notificationDelivery {
	t.Helper()
	result, err := a.notify(e)
	if err != nil {
		t.Fatal(err)
	}
	return result
}
func TestNotificationConcurrentRetryAndImmutablePayload(t *testing.T) {
	e := setup(t)
	event := syntheticNotification(1, "concurrent")
	const workers = 12
	results := make(chan notificationDelivery, workers)
	failures := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); result, err := e.a.notify(event); results <- result; failures <- err }()
	}
	wg.Wait()
	close(results)
	close(failures)
	for err := range failures {
		if err != nil {
			t.Fatal(err)
		}
	}
	created := 0
	var id int64
	for result := range results {
		if id == 0 {
			id = result.ID
		}
		if result.ID != id {
			t.Fatal("retry returned another record")
		}
		if result.Status == "delivered" {
			created++
		}
	}
	if created != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_receipts") != 1 {
		t.Fatal("duplicate logical event")
	}
	event.Message = "Changed retry"
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("changed retry payload accepted")
	}
	event = syntheticNotification(2, "concurrent")
	deliverSynthetic(t, e.a, event)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 2 {
		t.Fatal("dedupe merged recipients")
	}
	w := e.req(t, 1, "/api/notifications", "GET", nil)
	status(t, w, 200)
	for _, secret := range []string{"dedupe_hash", "event_hash", "receipt_id", "user_id", "AccountIDs", "concurrent"} {
		if strings.Contains(w.Body.String(), secret) {
			t.Fatal("internal delivery fields exposed", secret)
		}
	}
}
func TestNotificationInboxReadDismissAndOwnership(t *testing.T) {
	e := setup(t)
	first := syntheticNotification(1, "one")
	a := deliverSynthetic(t, e.a, first)
	second := syntheticNotification(1, "two")
	second.Dismissible = false
	b := deliverSynthetic(t, e.a, second)
	deliverSynthetic(t, e.a, syntheticNotification(2, "other"))
	w := e.req(t, 1, "/api/notifications?page_size=1", "GET", nil)
	status(t, w, 200)
	body := w.Body.String()
	if !strings.Contains(body, `"total":2`) || !strings.Contains(body, `"unread_count":2`) {
		t.Fatal(body)
	}
	status(t, e.req(t, 2, fmt.Sprintf("/api/notifications/%d/read", a.ID), "POST", nil), 404)
	status(t, e.req(t, 3, fmt.Sprintf("/api/notifications/%d/dismiss", a.ID), "POST", nil), 404)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/read", a.ID), "POST", nil), 200)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/read", a.ID), "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=1 AND read_at IS NOT NULL") != 1 {
		t.Fatal("read modified other messages")
	}
	w = e.req(t, 1, "/api/notifications?state=read", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"total":1`) {
		t.Fatal(w.Body.String())
	}
	status(t, e.req(t, 1, "/api/notifications/read-all", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications WHERE user_id=2 AND read_at IS NULL") != 1 {
		t.Fatal("mark all crossed recipient boundary")
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/dismiss", b.ID), "POST", nil), 404)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/dismiss", a.ID), "POST", nil), 200)
	retry := deliverSynthetic(t, e.a, first)
	if retry.Status != "duplicate" || retry.ID != a.ID || queryInt(e.a.DB, "SELECT dismissed_at IS NOT NULL FROM notifications WHERE id=?", a.ID) != 1 {
		t.Fatal("retry resurrected dismissed event")
	}
	status(t, e.req(t, 1, "/api/notifications?state=bad", "GET", nil), 400)
	status(t, e.req(t, 1, "/api/notifications?page_size=101", "GET", nil), 400)
	status(t, e.req(t, 1, "/api/notifications/0/read", "POST", nil), 400)
}
func TestNotificationPrivacyRevocationAndSourceChanges(t *testing.T) {
	e := setup(t)
	migrationExec(t, e.a.DB, `INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance) VALUES(1,2,'2026-10-01',-1234,'Synthetic private','2026-10-01',-1234,'Synthetic private','synthetic','{}')`)
	event := syntheticNotification(3, "private")
	event.Type = "unusual_spending"
	event.SourceKind = "transaction"
	event.SourceID = 1
	event.AccountIDs = []int64{2}
	event.Title = "Private alert"
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("private event created without grant")
	}
	migrationExec(t, e.a.DB, "INSERT INTO grants VALUES(3,2,'viewer')")
	item := deliverSynthetic(t, e.a, event)
	status(t, e.req(t, 3, "/api/notifications", "GET", nil), 200)
	// Administrator does not gain another user's inbox or private grant by role.
	if strings.Contains(e.req(t, 1, "/api/notifications", "GET", nil).Body.String(), "Private alert") {
		t.Fatal("administrator read another inbox")
	}
	migrationExec(t, e.a.DB, "DELETE FROM grants WHERE user_id=3 AND account_id=2")
	w := e.req(t, 3, "/api/notifications", "GET", nil)
	status(t, w, 200)
	if strings.Contains(w.Body.String(), "Private alert") || !strings.Contains(w.Body.String(), `"unread_count":0`) {
		t.Fatal("revoked access leaked list or count", w.Body.String())
	}
	status(t, e.req(t, 3, fmt.Sprintf("/api/notifications/%d/read", item.ID), "POST", nil), 404)
	status(t, e.req(t, 3, "/api/notifications/read-all", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT read_at IS NULL FROM notifications WHERE id=?", item.ID) != 1 {
		t.Fatal("read-all touched inaccessible event")
	}
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("retry bypassed revoked grant")
	}
	migrationExec(t, e.a.DB, "INSERT INTO grants VALUES(3,2,'viewer'); UPDATE accounts SET sync_hidden=1 WHERE id=2")
	if strings.Contains(e.req(t, 3, "/api/notifications", "GET", nil).Body.String(), "Private alert") {
		t.Fatal("hidden account leaked event")
	}
	migrationExec(t, e.a.DB, "UPDATE accounts SET sync_hidden=0 WHERE id=2; UPDATE transactions SET account_id=1 WHERE id=1")
	if strings.Contains(e.req(t, 3, "/api/notifications", "GET", nil).Body.String(), "Private alert") {
		t.Fatal("changed source scope exposed stale target")
	}
	migrationExec(t, e.a.DB, "DELETE FROM transactions WHERE id=1")
	if strings.Contains(e.req(t, 3, "/api/notifications", "GET", nil).Body.String(), "Private alert") {
		t.Fatal("deleted source leaked payload")
	}
}
func TestNotificationBudgetMembershipAndSharedScope(t *testing.T) {
	e := setup(t)
	event := syntheticNotification(1, "budget")
	event.Type = "budget_threshold"
	event.SourceKind = "budget"
	event.SourceID = 1
	event.AccountIDs = []int64{1}
	deliverSynthetic(t, e.a, event)
	event.DedupeKey = "private-budget"
	event.AccountIDs = []int64{2}
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("private account used in shared budget event")
	}
	event.DedupeKey = "viewer-budget"
	event.RecipientID = 3
	event.AccountIDs = []int64{1}
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("viewer got household budget event")
	}
	migrationExec(t, e.a.DB, "UPDATE users SET budget_member=0 WHERE id=1")
	if !strings.Contains(e.req(t, 1, "/api/notifications/unread-count", "GET", nil).Body.String(), `"unread_count":0`) {
		t.Fatal("budget membership not rechecked")
	}
	migrationExec(t, e.a.DB, "UPDATE users SET budget_member=1 WHERE id=1; UPDATE accounts SET household=0 WHERE id=1")
	if !strings.Contains(e.req(t, 1, "/api/notifications/unread-count", "GET", nil).Body.String(), `"unread_count":0`) {
		t.Fatal("account became private but budget alert remained")
	}
}
func TestNotificationPreferencesConsentAndAuditRollback(t *testing.T) {
	e := setup(t)
	event := syntheticNotification(1, "preference")
	w := e.req(t, 1, "/api/notifications/preferences", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), `"enabled":true`) || strings.Contains(w.Body.String(), "push") {
		t.Fatal(w.Body.String())
	}
	pref := map[string]any{"type": "system", "channel": "in_app", "enabled": false, "version": 0}
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", pref), 200)
	result := deliverSynthetic(t, e.a, event)
	if result.Status != "disabled" || queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 0 {
		t.Fatal("disabled channel delivered")
	}
	deliverSynthetic(t, e.a, syntheticNotification(2, "preference"))
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", pref), 409)
	pref["channel"] = "push"
	pref["version"] = 1
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", pref), 400)
	pref["channel"] = "in_app"
	pref["enabled"] = true
	migrationExec(t, e.a.DB, `CREATE TRIGGER reject_notification_audit BEFORE INSERT ON audit BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END;`)
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", pref), 500)
	if queryInt(e.a.DB, "SELECT enabled FROM notification_preferences WHERE user_id=1") != 0 {
		t.Fatal("preference survived failed audit")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_notification_audit")
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", pref), 200)
	if deliverSynthetic(t, e.a, event).Status != "delivered" {
		t.Fatal("re-enabled event not delivered")
	}
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", map[string]any{"type": "system", "channel": "in_app", "version": 2}), 400)
}
func TestNotificationDeliveryFailureRollbackAndRetry(t *testing.T) {
	e := setup(t)
	event := syntheticNotification(1, "failure")
	migrationExec(t, e.a.DB, `CREATE TRIGGER reject_notification BEFORE INSERT ON notification_accounts BEGIN SELECT RAISE(ABORT,'synthetic dependency failure'); END;`)
	event.Type = "unusual_spending"
	event.SourceKind = "account"
	event.SourceID = 2
	event.AccountIDs = []int64{2}
	if _, err := e.a.notify(event); err == nil {
		t.Fatal("delivery failure ignored")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_receipts") != 0 {
		t.Fatal("delivery failure left partial receipt or payload")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_notification")
	if deliverSynthetic(t, e.a, event).Status != "delivered" {
		t.Fatal("retry failed")
	}
	// Event insertion also rolls back with its surrounding financial write.
	event.DedupeKey = "atomic"
	err := e.a.write(func(tx *sql.Tx) error {
		if _, err := e.a.notifyTx(tx, event, time.Now()); err != nil {
			return err
		}
		return errors.New("synthetic financial rollback")
	})
	if err == nil || queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 1 {
		t.Fatal("notification outlived rolled-back trigger")
	}
}
func TestNotificationRetentionCapacityAndNoResurrection(t *testing.T) {
	e := setup(t)
	first := syntheticNotification(1, "retained")
	item := deliverSynthetic(t, e.a, first)
	migrationExec(t, e.a.DB, fmt.Sprintf("UPDATE notifications SET created_at=%d WHERE id=%d", time.Now().Add(-notificationRetention-time.Hour).Unix(), item.ID))
	if !strings.Contains(e.req(t, 1, "/api/notifications/unread-count", "GET", nil).Body.String(), `"unread_count":0`) {
		t.Fatal("expired event visible")
	}
	status(t, e.req(t, 1, "/api/notifications/read-all", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 0 {
		t.Fatal("expired payload not cleaned")
	}
	if deliverSynthetic(t, e.a, first).Status != "duplicate" || queryInt(e.a.DB, "SELECT COUNT(*) FROM notifications") != 0 {
		t.Fatal("receipt did not prevent resurrection")
	}
	old := syntheticNotification(1, "expired")
	old.OccurredAt = time.Now().Add(-notificationRetention - time.Hour)
	if deliverSynthetic(t, e.a, old).Status != "expired" {
		t.Fatal("old event recreated")
	}
	// Seed only metadata for the capacity boundary; no owner data involved.
	if err := e.a.write(func(tx *sql.Tx) error {
		for i := 0; i < notificationReceiptLimit-1; i++ {
			if _, err := tx.Exec("INSERT INTO notification_receipts(user_id,type,dedupe_hash,event_hash,created_at) VALUES(1,'system',?,'synthetic',?)", fmt.Sprint(i), time.Now().Unix()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.notify(syntheticNotification(1, "full")); err == nil {
		t.Fatal("unbounded receipt growth")
	}
	migrationExec(t, e.a.DB, fmt.Sprintf("UPDATE notification_receipts SET created_at=%d", time.Now().Add(-notificationReceiptRetention-time.Hour).Unix()))
	deliverSynthetic(t, e.a, syntheticNotification(1, "after-cleanup"))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_receipts") != 1 {
		t.Fatal("receipt cleanup failed")
	}
}
func TestNotificationSecurityAndUserDeletion(t *testing.T) {
	e := setup(t)
	deliverSynthetic(t, e.a, syntheticNotification(2, "deleted"))
	status(t, e.req(t, 2, "/api/notifications/preferences", "PUT", map[string]any{"type": "system", "channel": "in_app", "enabled": false, "version": 0}), 200)
	r := httptest.NewRequest("GET", "/api/notifications", nil)
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 401)
	r = httptest.NewRequest("POST", "/api/notifications/read-all", nil)
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token2"})
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	status(t, e.req(t, 1, "/api/users/2", "DELETE", map[string]any{"confirm_username": "other", "version": 1}), 200)
	for _, table := range []string{"notifications", "notification_receipts", "notification_preferences"} {
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM "+table+" WHERE user_id=2") != 0 {
			t.Fatal("deleted user retained notifications", table)
		}
	}
	if _, err := e.a.notify(syntheticNotification(2, "post-delete")); err == nil {
		t.Fatal("deleted recipient accepted")
	}
	status(t, e.req(t, 2, "/api/notifications", "GET", nil), 401)
}
func TestNotificationMigrationUpgradeRestartAndRollback(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:1], 23); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused'); INSERT INTO accounts(id,name,bank_id) VALUES(1,'Synthetic','synthetic'); INSERT INTO grants VALUES(1,1,'viewer'); INSERT INTO sessions VALUES('synthetic',1,'synthetic',9999999999); CREATE TRIGGER reject_notification_version BEFORE INSERT ON migrations WHEN NEW.version=24 BEGIN SELECT RAISE(ABORT,'synthetic record failure'); END;`)
	before := migrationSnapshot(t, db, "SELECT * FROM users", "SELECT * FROM accounts", "SELECT * FROM grants", "SELECT * FROM sessions")
	if err := migrate(db); err == nil {
		t.Fatal("migration record failure ignored")
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='notifications'") != 0 {
		t.Fatal("failed migration left tables")
	}
	migrationExec(t, db, "DROP TRIGGER reject_notification_version")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, "SELECT * FROM users", "SELECT * FROM accounts", "SELECT * FROM grants", "SELECT * FROM sessions") {
		t.Fatal("migration changed protected data")
	}
	a := &App{DB: db}
	deliverSynthetic(t, a, syntheticNotification(1, "restart"))
	preserved := migrationSnapshot(t, db, "SELECT * FROM notifications", "SELECT * FROM notification_receipts")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if preserved != migrationSnapshot(t, db, "SELECT * FROM notifications", "SELECT * FROM notification_receipts") {
		t.Fatal("restart changed inbox")
	}
	// Close/reopen the SQLite file to prove state persists beyond a connection.
	var path string
	if err := db.QueryRow("SELECT file FROM pragma_database_list WHERE name='main'").Scan(&path); err != nil {
		t.Fatal(err)
	}
	db.Close()
	reopened, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err := migrate(reopened); err != nil {
		t.Fatal(err)
	}
	if preserved != migrationSnapshot(t, reopened, "SELECT * FROM notifications", "SELECT * FROM notification_receipts") {
		t.Fatal("restart lost state")
	}
	snapshot, err := os.ReadFile(filepath.Join("testdata", "migrations", "v22.sql"))
	if err != nil {
		t.Fatal(err)
	}
	legacy := migrationDB(t)
	migrationExec(t, legacy, string(snapshot))
	if err := migrate(legacy); err != nil {
		t.Fatal("legacy to notification schema", err)
	}
	if queryInt(legacy, "SELECT MAX(version) FROM migrations") != 24 || queryInt(legacy, "SELECT COUNT(*) FROM notifications") != 0 {
		t.Fatal("legacy notification upgrade incorrect")
	}
}
func TestNotificationEventValidation(t *testing.T) {
	e := setup(t)
	mutations := []func(*notificationEvent){
		func(v *notificationEvent) {
			v.Type = "budget_overspend"
			v.SourceKind = "account"
			v.SourceID = 2
			v.AccountIDs = []int64{2}
		},
		func(v *notificationEvent) { v.Type = "unknown" }, func(v *notificationEvent) { v.Title = strings.Repeat("x", 121) },
		func(v *notificationEvent) { v.OccurredAt = time.Time{} }, func(v *notificationEvent) { v.OccurredAt = time.Now().Add(time.Hour) },
		func(v *notificationEvent) { v.AccountIDs = []int64{1} }, func(v *notificationEvent) { v.SourceKind = "transaction"; v.SourceID = 1 },
		func(v *notificationEvent) { v.DedupeKey = "" }, func(v *notificationEvent) { v.Message = "\x00" },
	}
	for _, mutate := range mutations {
		event := syntheticNotification(1, "invalid")
		mutate(&event)
		if _, err := e.a.notify(event); err == nil {
			t.Fatal("invalid event accepted")
		}
	}
	event := syntheticNotification(1, "canonical")
	event.Type = "unusual_spending"
	event.SourceKind = "account"
	event.SourceID = 2
	event.AccountIDs = []int64{2, 1, 2}
	first := deliverSynthetic(t, e.a, event)
	event.AccountIDs = []int64{1, 2}
	if second := deliverSynthetic(t, e.a, event); second.ID != first.ID || second.Status != "duplicate" {
		t.Fatal("scope ordering broke retry")
	}
}

func TestNotificationInboxCapAndMaintenanceShutdown(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "synthetic.sqlite")
	a, err := Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	migrationExec(t, a.DB, "INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused')")
	event := syntheticNotification(1, "old-payload")
	delivered := deliverSynthetic(t, a, event)
	old := time.Now().Add(-notificationRetention - time.Hour).Unix()
	migrationExec(t, a.DB, fmt.Sprintf("UPDATE notifications SET created_at=%d WHERE id=%d", old, delivered.ID))
	// Fill an inbox in one transaction to check its hard payload limit.
	if err := a.write(func(tx *sql.Tx) error {
		for i := 0; i < notificationInboxLimit; i++ {
			key := fmt.Sprintf("cap-%d", i)
			res, err := tx.Exec("INSERT INTO notification_receipts(user_id,type,dedupe_hash,event_hash,created_at) VALUES(1,'system',?,'synthetic',?)", hash(key), time.Now().Unix())
			if err != nil {
				return err
			}
			receipt, err := res.LastInsertId()
			if err != nil {
				return err
			}
			if _, err := tx.Exec("INSERT INTO notifications(receipt_id,user_id,type,severity,title,message,source_kind,source_id,occurred_at,created_at,dismissible) VALUES(?,1,'system','info','Synthetic','Synthetic','system',0,?,?,1)", receipt, time.Now().Unix(), time.Now().Unix()); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		a.Close()
		t.Fatal(err)
	}
	last := deliverSynthetic(t, a, syntheticNotification(1, "latest"))
	if queryInt(a.DB, "SELECT COUNT(*) FROM notifications") != notificationInboxLimit || last.ID <= delivered.ID {
		a.Close()
		t.Fatal("inbox not capped")
	}
	// A receipt without its evicted payload stays a successful duplicate.
	if result := deliverSynthetic(t, a, event); result.Status != "duplicate" || result.ID != 0 {
		a.Close()
		t.Fatal("old payload resurrected")
	}
	migrationExec(t, a.DB, fmt.Sprintf("UPDATE notification_receipts SET created_at=%d", time.Now().Add(-notificationReceiptRetention-time.Hour).Unix()))
	a.StartNotificationMaintenance()
	a.StartNotificationMaintenance()
	if err := a.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if queryInt(reopened, "SELECT COUNT(*) FROM notification_receipts") != 0 || queryInt(reopened, "SELECT COUNT(*) FROM notifications") != 0 {
		t.Fatal("startup cleanup not completed before shutdown")
	}
}

func TestNotificationBackupRestorePreservesPersonalState(t *testing.T) {
	e := setup(t)
	event := syntheticNotification(1, "restore")
	delivered := deliverSynthetic(t, e.a, event)
	status(t, e.req(t, 1, fmt.Sprintf("/api/notifications/%d/read", delivered.ID), "POST", nil), 200)
	status(t, e.req(t, 1, "/api/notifications/preferences", "PUT", map[string]any{"type": "system", "channel": "in_app", "enabled": false, "version": 0}), 200)
	protected := []string{"SELECT * FROM notifications", "SELECT * FROM notification_receipts", "SELECT * FROM notification_preferences"}
	before := migrationSnapshot(t, e.a.DB, protected...)
	backup, err := e.a.Backup()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored.sqlite")
	if err := Restore(target, backup); err != nil {
		t.Fatal(err)
	}
	restored, err := Open(target, "http://localhost:8080", filepath.Join(t.TempDir(), "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if before != migrationSnapshot(t, restored.DB, protected...) {
		t.Fatal("restored inbox/read/preferences state changed")
	}
	if queryInt(restored.DB, "SELECT COUNT(*) FROM sessions") != 0 {
		t.Fatal("restore retained browser sessions")
	}
	if queryInt(restored.DB, "SELECT MAX(version) FROM migrations") != schemaVersion {
		t.Fatal("restore schema mismatch")
	}
	if result := deliverSynthetic(t, restored, event); result.Status != "duplicate" {
		t.Fatal("restore lost retry idempotency")
	}
}

func TestNotificationPreferenceBatchAtomicityAndIsolation(t *testing.T) {
	e := setup(t)
	path := "/api/notifications/preferences/batch"
	preference := func(kind string, version int) map[string]any {
		return map[string]any{"type": kind, "channel": "in_app", "enabled": false, "version": version}
	}
	// A stale second item must roll back the first item too.
	status(t, e.req(t, 1, path, "PUT", map[string]any{"items": []any{preference("system", 0), preference("budget_threshold", 1)}}), 409)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_preferences") != 0 {
		t.Fatal("partial preference save")
	}
	body := map[string]any{"items": []any{preference("system", 0), preference("budget_threshold", 0)}}
	// Audit failure on the second item must roll back every write and audit.
	migrationExec(t, e.a.DB, `CREATE TRIGGER reject_batch_audit BEFORE INSERT ON audit WHEN json_extract(NEW.details,'$.type')='budget_threshold' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END;`)
	status(t, e.req(t, 1, path, "PUT", body), 500)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_preferences") != 0 {
		t.Fatal("batch survived audit failure")
	}
	migrationExec(t, e.a.DB, "DROP TRIGGER reject_batch_audit")
	status(t, e.req(t, 1, path, "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_preferences WHERE user_id=1 AND enabled=0 AND version=1") != 2 {
		t.Fatal("batch did not save")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_preferences WHERE user_id=2") != 0 {
		t.Fatal("other user's preferences changed")
	}
	status(t, e.req(t, 1, path, "PUT", body), 409)
	status(t, e.req(t, 1, path, "PUT", map[string]any{"items": []any{preference("system", 1), preference("system", 1)}}), 400)
	status(t, e.req(t, 1, path, "PUT", map[string]any{"items": []any{}}), 400)
}
