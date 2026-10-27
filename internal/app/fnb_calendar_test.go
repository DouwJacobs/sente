package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestFNBCalendarNextOccurrence(t *testing.T) {
	for _, tc := range []struct {
		now, want string
		schedule  fnbCalendarSchedule
	}{
		{"2026-10-15T10:00:00Z", "2026-10-15T16:00:00Z", fnbCalendarSchedule{Frequency: "daily", Times: []string{"18:00", "12:00"}}},
		{"2026-10-16T17:00:00Z", "2026-10-19T05:00:00Z", fnbCalendarSchedule{Frequency: "weekly", Times: []string{"07:00"}, Weekdays: []int{1, 2, 3, 4, 5}}},
		{"2026-10-16T17:00:00Z", "2026-10-16T22:00:00Z", fnbCalendarSchedule{Frequency: "weekly", Times: []string{"00:00"}, Weekdays: []int{6}}},
		{"2026-01-31T10:00:00Z", "2026-03-31T10:00:00Z", fnbCalendarSchedule{Frequency: "monthly", Times: []string{"12:00"}, MonthDays: []int{31}}},
		{"2028-02-01T00:00:00Z", "2028-02-29T10:00:00Z", fnbCalendarSchedule{Frequency: "monthly", Times: []string{"12:00"}, LastDay: true}},
		{"2026-02-01T00:00:00Z", "2026-02-28T10:00:00Z", fnbCalendarSchedule{Frequency: "monthly", Times: []string{"12:00"}, MonthDays: []int{31}, LastDay: true}},
		{"2026-12-31T22:00:00Z", "2027-01-01T10:00:00Z", fnbCalendarSchedule{Frequency: "monthly", Times: []string{"12:00"}, MonthDays: []int{1, 15}}},
	} {
		if err := tc.schedule.validate(); err != nil {
			t.Fatal(err)
		}
		now, _ := time.Parse(time.RFC3339, tc.now)
		want, _ := time.Parse(time.RFC3339, tc.want)
		if got := nextFNBCalendarDue(now, &tc.schedule); got != want.Unix() {
			t.Fatalf("%+v: got %v", tc, got)
		}
	}
}

func TestFNBCalendarRejectsInvalidSchedules(t *testing.T) {
	for _, raw := range []string{
		`{"frequency":"yearly","times":["12:00"]}`,
		`{"frequency":"daily","times":[]}`,
		`{"frequency":"daily","times":["12:00","12:00"]}`,
		`{"frequency":"daily","times":["24:00"]}`,
		`{"frequency":"daily","times":["12:00"],"weekdays":[1]}`,
		`{"frequency":"weekly","times":["12:00"]}`,
		`{"frequency":"weekly","times":["12:00"],"weekdays":[1,1]}`,
		`{"frequency":"weekly","times":["12:00"],"weekdays":[7]}`,
		`{"frequency":"monthly","times":["12:00"]}`,
		`{"frequency":"monthly","times":["12:00"],"month_days":[0,32]}`,
		`{"frequency":"monthly","times":["12:00"],"month_days":[1,1]}`,
		`{"frequency":"monthly","times":["12:00"],"weekdays":[1],"last_day":true}`,
	} {
		if _, err := parseFNBCalendar(raw); err == nil {
			t.Fatal("accepted", raw)
		}
	}
	many := &fnbCalendarSchedule{Frequency: "daily", Times: make([]string, 13)}
	if many.validate() == nil {
		t.Fatal("unbounded times")
	}
}

func TestFNBCalendarScheduleOwnerVersionAndLifecycle(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	schedule := map[string]any{"frequency": "weekly", "times": []string{"18:00", "07:00"}, "weekdays": []int{5, 1}}
	body := map[string]any{"schedule": schedule, "version": 1}
	status(t, e.req(t, 2, "/api/fnb/schedule", "PUT", body), 403)
	body["interval_hours"] = 24
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", body), 400)
	body["interval_hours"] = 0
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", body), 200)
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", body), 409)
	response := e.req(t, 1, "/api/fnb", "GET", nil)
	status(t, response, 200)
	if !strings.Contains(response.Body.String(), `"times":["07:00","18:00"]`) || strings.Contains(response.Body.String(), "schedule_json") {
		t.Fatal("status not canonical", response.Body.String())
	}
	calls := 0
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		calls++
		return fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Synthetic", BankID: "12345678901", Balance: fnbDecimal("1.00")}}}, nil
	}
	e.a.DB.Exec("UPDATE fnb_connections SET next_due=1")
	e.a.refreshDueFNB(time.Now())
	e.a.refreshDueFNB(time.Now())
	if calls != 1 {
		t.Fatalf("overdue calls %d", calls)
	}
	due := time.Unix(queryInt(e.a.DB, "SELECT next_due FROM fnb_connections"), 0).In(time.FixedZone("SAST", 7200))
	if (due.Weekday() != time.Monday && due.Weekday() != time.Friday) || (due.Hour() != 7 && due.Hour() != 18) || due.Minute() != 0 || !due.After(time.Now()) {
		t.Fatal("wrong next run", due)
	}
	status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT next_due FROM fnb_connections") != due.Unix() {
		t.Fatal("manual refresh moved clock")
	}
	version := queryInt(e.a.DB, "SELECT version FROM fnb_connections")
	e.a.DB.Exec("UPDATE fnb_connections SET state='refreshing'")
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"version": version}), 409)
	e.a.DB.Exec("UPDATE fnb_connections SET state='ready'")
	status(t, e.req(t, 1, "/api/fnb/schedule", "PUT", map[string]any{"version": version, "interval_hours": 3}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE schedule_json='' AND daily_time='' AND interval_hours=3 AND next_due>0") != 1 {
		t.Fatal("interval did not replace calendar")
	}
	version = queryInt(e.a.DB, "SELECT version FROM fnb_connections")
	status(t, e.req(t, 1, "/api/fnb", "PUT", map[string]any{"username": "synthetic", "password": "synthetic", "version": version}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE schedule_json='' AND daily_time='' AND interval_hours=0 AND next_due IS NULL") != 1 {
		t.Fatal("replacement did not pause schedules")
	}
}

func TestFNBCalendarUpgradePreservesLegacyAndRollsBack(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:33-migrationBaseline+1], 33); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password,admin) VALUES(1,'synthetic','synthetic',1);
 INSERT INTO fnb_connections(user_id,secret,interval_hours,daily_time,next_due,version) VALUES(1,x'1234',0,'12:00',12345,7);`)
	protected := []string{"SELECT user_id,secret,interval_hours,daily_time,next_due,version FROM fnb_connections", "SELECT * FROM users", "SELECT * FROM grants"}
	before := migrationSnapshot(t, db, protected...)
	steps := append([]schemaMigration{}, schemaMigrations[:34-migrationBaseline+1]...)
	steps[34-migrationBaseline].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := migrateFNBCalendarSchedule(tx, origin); err != nil {
			return err
		}
		return errors.New("synthetic failure")
	}
	if runSchemaMigrations(db, steps, 34) == nil {
		t.Fatal("expected failure")
	}
	if queryInt(db, "SELECT MAX(version) FROM migrations") != 33 {
		t.Fatal("partial migration")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if before != migrationSnapshot(t, db, protected...) {
		t.Fatal("legacy changes")
	}
	var raw string
	if err := db.QueryRow("SELECT schedule_json FROM fnb_connections").Scan(&raw); err != nil || raw != "" {
		t.Fatal("legacy schedule replaced", err)
	}
	once := migrationSnapshot(t, db, "SELECT * FROM fnb_connections", "SELECT * FROM migrations")
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if once != migrationSnapshot(t, db, "SELECT * FROM fnb_connections", "SELECT * FROM migrations") {
		t.Fatal("repeated upgrade")
	}
	// Canonical calendar schedules round-trip as structured browser status data.
	encoded, _ := json.Marshal(fnbCalendarSchedule{Frequency: "daily", Times: []string{"12:00"}})
	if _, err := parseFNBCalendar(string(encoded)); err != nil {
		t.Fatal(err)
	}
}
