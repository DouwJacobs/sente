package app

import (
	"database/sql"
	"log"
	"time"
)

// Evaluate source changes and Joburg date rollover, with independent push deadlines.
func (a *App) StartNotificationMaintenance() {
	a.notificationOnce.Do(func() {
		a.notificationWG.Add(1)
		go func() {
			defer a.notificationWG.Done()
			if err := a.writeQuiet(func(tx *sql.Tx) error { return recoverUncertainPushTx(tx, time.Now()) }); err != nil {
				log.Print("Notification dispatcher recovery failed")
			}
			lastRevision := int64(-1)
			lastDay := ""
			loc, _ := time.LoadLocation("Africa/Johannesburg")
			for {
				now := time.Now()
				day := now.In(loc).Format("2006-01-02")
				revision := queryInt(a.DB, "SELECT revision FROM notification_source_revision WHERE id=1")
				if revision != lastRevision || day != lastDay {
					if err := a.evaluateNotificationAlerts(now); err != nil {
						log.Print("Notification evaluation failed")
					} else {
						lastRevision = revision
						lastDay = day
					}
				}
				if err := a.dispatchNotificationPush(now); err != nil {
					log.Print("Notification dispatch failed")
				}
				now = time.Now()
				delay := nextNotificationDay(now).Sub(now)
				var next sql.NullInt64
				err := a.DB.QueryRow("SELECT MIN(next_attempt) FROM notification_push_outbox WHERE status='pending'").Scan(&next)
				if err != nil || lastRevision != revision || lastDay != day {
					delay = min(delay, time.Minute)
				} else if next.Valid {
					delay = min(delay, max(time.Second, time.Until(time.Unix(next.Int64, 0))))
				}
				timer := time.NewTimer(max(time.Second, delay))
				select {
				case <-a.stop:
					timer.Stop()
					return
				case <-timer.C:
				case <-a.notificationWake:
					timer.Stop()
					debounce := time.NewTimer(2 * time.Second)
					select {
					case <-a.stop:
						debounce.Stop()
						return
					case <-debounce.C:
					}
					select {
					case <-a.notificationWake:
					default:
					}
				}
			}
		}()
	})
}
func nextNotificationDay(now time.Time) time.Time {
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	local := now.In(loc)
	return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, loc)
}
