package app

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestFNBDailyDue(t *testing.T) {
	for _, tc := range []struct{ now, clock, want string }{
		{"2026-10-15T09:59:59Z", "12:00", "2026-10-15T10:00:00Z"},
		{"2026-10-15T10:00:00Z", "12:00", "2026-10-16T10:00:00Z"},
		{"2026-12-31T23:00:00Z", "00:00", "2027-01-01T22:00:00Z"},
		{"2026-10-15T22:30:00Z", "23:59", "2026-10-16T21:59:00Z"},
	} {
		now, _ := time.Parse(time.RFC3339, tc.now)
		want, _ := time.Parse(time.RFC3339, tc.want)
		if got := nextFNBDue(now, 0, tc.clock); got != want.Unix() {
			t.Fatalf("%+v: got %v", tc, got)
		}
	}
	for _, clock := range []string{"24:00", "9:00", "12:60", "12:00:00", " 12:00"} {
		if validFNBDailyTime(clock) {
			t.Fatalf("accepted %q", clock)
		}
	}
	if nextFNBDue(time.Now(), 0, "") != nil {
		t.Fatal("off has next run")
	}
}

func TestFNBDailyScheduleLifecycle(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	for _, body := range []map[string]any{
		{"daily_time": "24:00", "version": 1},
		{"daily_time": "12:00", "interval_hours": 24, "version": 1},
	} {
		status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", body), 400)
	}
	status(t, e.req(t, 2, "/api/fnb/schedule", "PUT", map[string]any{"daily_time": "12:00", "version": 1}), 403)
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"daily_time": "12:00", "version": 1}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE interval_hours=0 AND daily_time='12:00' AND next_due>0") != 1 {
		t.Fatal("daily not saved")
	}
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"interval_hours": 6, "version": 1}), 409)
	calls := 0
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		calls++
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic", BankID: "12345678901", Balance: fnbDecimal("1.00")}}}, nil
	}
	e.a.DB.Exec("UPDATE fnb_connections SET next_due=1")
	e.a.refreshDueFNB(time.Now())
	e.a.refreshDueFNB(time.Now())
	if calls != 1 {
		t.Fatalf("overdue refresh calls: %d", calls)
	}
	due := time.Unix(queryInt(e.a.DB, "SELECT next_due FROM fnb_connections"), 0).In(time.FixedZone("SAST", 7200))
	if due.Hour() != 12 || due.Minute() != 0 || !due.After(time.Now()) {
		t.Fatal("lost daily clock time", due)
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 200)
	manualDue := time.Unix(queryInt(e.a.DB, "SELECT next_due FROM fnb_connections"), 0).In(time.FixedZone("SAST", 7200))
	if manualDue.Hour() != 12 || manualDue.Minute() != 0 {
		t.Fatal("manual refresh moved daily clock", manualDue)
	}
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{Error: "APPROVAL_REQUIRED"}, nil
	}
	e.a.DB.Exec("UPDATE fnb_connections SET next_due=1")
	e.a.refreshDueFNB(time.Now())
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required' AND next_due IS NULL AND daily_time='12:00'") != 1 {
		t.Fatal("failure not paused")
	}
	version := queryInt(e.a.DB, "SELECT version FROM fnb_connections")
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"daily_time": "09:00", "version": version}), 200)
	e.a.refreshDueFNB(time.Now().Add(48 * time.Hour))
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='action_required'") != 1 {
		t.Fatal("saving schedule resumed paused connection")
	}
	version = queryInt(e.a.DB, "SELECT version FROM fnb_connections")
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"interval_hours": 0, "version": version}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE daily_time='' AND next_due IS NULL") != 1 {
		t.Fatal("off did not clear daily time")
	}
}

func TestFNBDailyScheduleUpgradeRollbackAndRetry(t *testing.T) {
	db := migrationDB(t)
	steps := schemaMigrations[:32-migrationBaseline+1]
	if err := runSchemaMigrations(db, steps, 32); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password,admin) VALUES(1,'synthetic','synthetic',1);
 INSERT INTO fnb_connections(user_id,secret,interval_hours,next_due,version) VALUES(1,x'1234',24,12345,7);`)
	protected := []string{"SELECT user_id,secret,interval_hours,next_due,version FROM fnb_connections", "SELECT * FROM users", "SELECT * FROM grants"}
	before := migrationSnapshot(t, db, protected...)
	failing := append([]schemaMigration{}, schemaMigrations[:33-migrationBaseline+1]...)
	failing[len(failing)-1].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := migrateFNBDailySchedule(tx, origin); err != nil {
			return err
		}
		return errors.New("synthetic migration failure")
	}
	if err := runSchemaMigrations(db, failing, 33); err == nil {
		t.Fatal("expected rollback")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 32 {
		t.Fatal("recorded failed migration")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, protected...) {
		t.Fatal("upgrade changed connection or authorization")
	}
	if queryInt(db, "SELECT COUNT(*) FROM fnb_connections WHERE daily_time=''") != 1 {
		t.Fatal("legacy schedule gained daily setting")
	}
	once := migrationSnapshot(t, db, "SELECT * FROM fnb_connections", "SELECT * FROM migrations")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if once != migrationSnapshot(t, db, "SELECT * FROM fnb_connections", "SELECT * FROM migrations") {
		t.Fatal("migration repeated")
	}
}
