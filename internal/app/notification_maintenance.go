package app

import (
	"database/sql"
	"log"
	"time"
)

// Serving cleans up at startup and daily, even when nobody opens the inbox.
// Shutdown waits for the last serialized cleanup before closing SQLite.
func (a *App) StartNotificationMaintenance() {
	a.notificationOnce.Do(func() {
		a.notificationWG.Add(1)
		go func() {
			defer a.notificationWG.Done()
			ticker := time.NewTicker(24 * time.Hour)
			defer ticker.Stop()
			for {
				if err := a.write(func(tx *sql.Tx) error { return pruneNotificationsTx(tx, time.Now()) }); err != nil {
					log.Print("Notification retention cleanup failed") // No payload/provider errors.
				}
				select {
				case <-a.stop:
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
