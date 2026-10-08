package app

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNotificationScheduleUpgradePreservesHistoryAndRollsBack(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:4], 26); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password) VALUES(1,'synthetic','unused'); INSERT INTO accounts(id,name,bank_id) VALUES(1,'Synthetic','synthetic'); INSERT INTO grants VALUES(1,1,'viewer'); INSERT INTO notification_diagnostics(user_id,type,key_hash,evaluated_at,channel,status) VALUES(1,'system','synthetic',1,'in_app','deduplicated')`)
	steps := append([]schemaMigration(nil), schemaMigrations...)
	steps[4].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := migrateNotificationSchedule(tx, origin); err != nil {
			return err
		}
		return errors.New("synthetic failure")
	}
	if err := runSchemaMigrations(db, steps, 27); err == nil {
		t.Fatal("upgrade must fail")
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='notification_source_revision'") != 0 {
		t.Fatal("failed migration leaked schema")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 26 {
		t.Fatal("failed upgrade changed history")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM notification_diagnostics") != 1 || queryInt(db, "SELECT COUNT(*) FROM grants WHERE user_id=1 AND account_id=1") != 1 {
		t.Fatal("history or grants lost")
	}
	migrationExec(t, db, "UPDATE accounts SET household=1 WHERE id=1")
	revision := queryInt(db, "SELECT revision FROM notification_source_revision")
	if revision == 0 {
		t.Fatal("source mutation not recorded")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT revision FROM notification_source_revision") != revision {
		t.Fatal("restart reapplied migration")
	}
}

func TestNotificationMaintenanceOnlyEvaluatesSourceChanges(t *testing.T) {
	e := alertFixture(t)
	e.a.StartNotificationMaintenance()
	awaitRun := func(previous int64) {
		t.Helper()
		deadline := time.Now().Add(8 * time.Second)
		for time.Now().Before(deadline) {
			if queryInt(e.a.DB, "SELECT run FROM notification_evaluation_runs") > previous {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
		t.Fatal("evaluation did not run")
	}
	awaitRun(0)
	// Drain startup fixture writes before checking an unrelated commit.
	time.Sleep(2500 * time.Millisecond)
	baseline := queryInt(e.a.DB, "SELECT run FROM notification_evaluation_runs")
	if err := e.a.write(func(tx *sql.Tx) error {
		_, err := tx.Exec("INSERT INTO sessions(token,user_id,csrf,expires_at) VALUES('synthetic-extra',1,'synthetic',1)")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2500 * time.Millisecond)
	if queryInt(e.a.DB, "SELECT run FROM notification_evaluation_runs") != baseline {
		t.Fatal("session write reran financial checks")
	}
	revision := queryInt(e.a.DB, "SELECT revision FROM notification_source_revision")
	if err := e.a.write(func(tx *sql.Tx) error {
		_, err := tx.Exec("UPDATE targets SET amount_cents=20000")
		if err != nil {
			return err
		}
		return errors.New("synthetic rollback")
	}); err == nil {
		t.Fatal("expected rollback")
	}
	if queryInt(e.a.DB, "SELECT revision FROM notification_source_revision") != revision {
		t.Fatal("rolled-back change dirtied source")
	}
	if err := e.a.write(func(tx *sql.Tx) error { _, err := tx.Exec("UPDATE targets SET amount_cents=20000"); return err }); err != nil {
		t.Fatal(err)
	}
	awaitRun(baseline)
}

func TestNotificationDiagnosticsOnlyMeaningfulTransitions(t *testing.T) {
	e := alertFixture(t)
	now := alertTestTime()
	evaluateAlerts(t, e, now)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics WHERE channel='in_app'") != 0 {
		t.Fatal("negative checks logged")
	}
	alertTransaction(t, e, 1, 1, -8000, "2026-10-15")
	evaluateAlerts(t, e, now)
	count := queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics")
	evaluateAlerts(t, e, now.Add(2*time.Hour))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics") != count {
		t.Fatal("unchanged conditions logged again")
	}
	changeAlertAmount(t, e, -1000)
	evaluateAlerts(t, e, now.Add(3*time.Hour))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM notification_diagnostics WHERE status='resolved'") == 0 {
		t.Fatal("resolution missing")
	}
	// Existing noisy history remains inspectable explicitly, hidden by default.
	migrationExec(t, e.a.DB, "INSERT INTO notification_diagnostics(user_id,type,key_hash,evaluated_at,channel,status) VALUES(1,'system','old',strftime('%s','now'),'in_app','deduplicated')")
	w := e.req(t, 1, "/api/notifications/diagnostics?type=system", "GET", nil)
	status(t, w, 200)
	if containsDiagnosticStatus(w.Body.String(), "deduplicated") {
		t.Fatal("historical noise visible by default")
	}
	w = e.req(t, 1, "/api/notifications/diagnostics?type=system&status=deduplicated", "GET", nil)
	status(t, w, 200)
	if !containsDiagnosticStatus(w.Body.String(), "deduplicated") {
		t.Fatal("historical records erased")
	}
}
func containsDiagnosticStatus(body, status string) bool { return strings.Contains(body, status) }

func TestNotificationDailyDeadlineUsesJoburg(t *testing.T) {
	now := time.Date(2026, 10, 8, 21, 59, 0, 0, time.UTC)
	expected := time.Date(2026, 10, 8, 22, 0, 0, 0, time.UTC)
	if !nextNotificationDay(now).Equal(expected) {
		t.Fatal(nextNotificationDay(now))
	}
	if !nextNotificationDay(expected).Equal(expected.Add(24 * time.Hour)) {
		t.Fatal("midnight deadline did not advance")
	}
}
