package app

import (
	"time"
)

func (a *App) StartFNBScheduler() {
	a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error='INTERRUPTED',next_due=NULL,version=version+1 WHERE state='refreshing'")
	a.fnbWG.Add(1)
	go func() {
		defer a.fnbWG.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-a.stop:
				return
			case <-ticker.C:
				a.refreshDueFNB(time.Now())
			}
		}
	}()
}

func (a *App) refreshDueFNB(now time.Time) {
	rows, err := data(a.DB, "SELECT c.user_id FROM fnb_connections c JOIN users u ON u.id=c.user_id WHERE c.state='ready' AND c.interval_hours>0 AND c.next_due<=? AND u.admin=1 AND u.disabled=0", now.Unix())
	if err != nil {
		return
	}
	for _, row := range rows {
		select {
		case <-a.stop:
			return
		default:
		}
		a.refreshFNB(num(row["user_id"]), false)
	}
}
