package app

import (
	"database/sql"
	"encoding/json"
	"sort"
	"time"
)

type fnbCalendarSchedule struct {
	Frequency string   `json:"frequency"`
	Times     []string `json:"times"`
	Weekdays  []int    `json:"weekdays,omitempty"`
	MonthDays []int    `json:"month_days,omitempty"`
	LastDay   bool     `json:"last_day,omitempty"`
}

func migrateFNBCalendarSchedule(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec("ALTER TABLE fnb_connections ADD COLUMN schedule_json TEXT NOT NULL DEFAULT ''")
	return err
}

func (s *fnbCalendarSchedule) validate() error {
	if len(s.Times) < 1 || len(s.Times) > 12 {
		return fail(400, "Choose between 1 and 12 refresh times")
	}
	seen := map[string]bool{}
	for _, clock := range s.Times {
		if !validFNBDailyTime(clock) || seen[clock] {
			return fail(400, "Choose distinct refresh times in HH:MM format")
		}
		seen[clock] = true
	}
	switch s.Frequency {
	case "daily":
		if len(s.Weekdays) > 0 || len(s.MonthDays) > 0 || s.LastDay {
			return fail(400, "Daily schedules use times only")
		}
	case "weekly":
		if len(s.Weekdays) < 1 || len(s.Weekdays) > 7 || len(s.MonthDays) > 0 || s.LastDay {
			return fail(400, "Choose at least one weekday")
		}
		if !uniqueScheduleDays(s.Weekdays, 0, 6) {
			return fail(400, "Choose distinct weekdays")
		}
	case "monthly":
		if len(s.Weekdays) > 0 || len(s.MonthDays) > 31 || (len(s.MonthDays) == 0 && !s.LastDay) || !uniqueScheduleDays(s.MonthDays, 1, 31) {
			return fail(400, "Choose month dates from 1 to 31 or the last day")
		}
	default:
		return fail(400, "Choose a daily, weekly or monthly schedule")
	}
	sort.Strings(s.Times)
	sort.Ints(s.Weekdays)
	sort.Ints(s.MonthDays)
	return nil
}

func uniqueScheduleDays(days []int, min, max int) bool {
	seen := map[int]bool{}
	for _, day := range days {
		if day < min || day > max || seen[day] {
			return false
		}
		seen[day] = true
	}
	return true
}

func parseFNBCalendar(raw string) (*fnbCalendarSchedule, error) {
	if raw == "" {
		return nil, nil
	}
	var s fnbCalendarSchedule
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil, err
	}
	if err := s.validate(); err != nil {
		return nil, err
	}
	return &s, nil
}

func nextFNBCalendarDue(now time.Time, s *fnbCalendarSchedule) any {
	loc, err := time.LoadLocation("Africa/Johannesburg")
	if err != nil {
		loc = time.FixedZone("SAST", 7200)
	}
	local := now.In(loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, loc)
	// A valid monthly schedule always has an occurrence within the next 62 days.
	for offset := 0; offset <= 62; offset++ {
		date := day.AddDate(0, 0, offset)
		matches := s.Frequency == "daily"
		if s.Frequency == "weekly" {
			for _, d := range s.Weekdays {
				matches = matches || int(date.Weekday()) == d
			}
		}
		if s.Frequency == "monthly" {
			for _, d := range s.MonthDays {
				matches = matches || date.Day() == d
			}
			matches = matches || (s.LastDay && date.AddDate(0, 0, 1).Month() != date.Month())
		}
		if !matches {
			continue
		}
		for _, clock := range s.Times {
			parsed, _ := time.Parse("15:04", clock)
			due := time.Date(date.Year(), date.Month(), date.Day(), parsed.Hour(), parsed.Minute(), 0, 0, loc)
			if due.After(now) {
				return due.Unix()
			}
		}
	}
	return nil
}
