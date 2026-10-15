package app

import (
	"database/sql"
	"time"
)

func migrateFNBDailySchedule(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec("ALTER TABLE fnb_connections ADD COLUMN daily_time TEXT NOT NULL DEFAULT ''")
	return err
}

func validFNBDailyTime(value string) bool {
	parsed, err := time.Parse("15:04", value)
	return err == nil && parsed.Format("15:04") == value
}

// Both saving and completing a run use the next future wall-clock occurrence.
// An overdue schedule runs once when serving resumes, without replaying missed days.
func nextFNBDue(now time.Time, interval int, daily string) any {
	if daily != "" {
		if !validFNBDailyTime(daily) {
			return nil
		}
		clock, _ := time.Parse("15:04", daily)
		loc, err := time.LoadLocation("Africa/Johannesburg")
		if err != nil {
			loc = time.FixedZone("SAST", 2*60*60)
		}
		local := now.In(loc)
		due := time.Date(local.Year(), local.Month(), local.Day(), clock.Hour(), clock.Minute(), 0, 0, loc)
		if !due.After(now) {
			due = due.AddDate(0, 0, 1)
		}
		return due.Unix()
	}
	if interval > 0 {
		return now.Add(time.Duration(interval) * time.Hour).Unix()
	}
	return nil
}
