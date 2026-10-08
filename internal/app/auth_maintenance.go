package app

import (
	"database/sql"
	"log"
	"time"
)

const authBucketLimit = 4096
const authWindow = 15 * time.Minute

// Both login and OAuth prune independently. At capacity, reject new peers rather
// than evicting live buckets and letting a fresh-IP flood reset throttling.
func (a *App) authAttempt(key string, limit int, now time.Time) bool {
	a.loginMu.Lock()
	defer a.loginMu.Unlock()
	if a.attempts == nil {
		a.attempts = map[string][]time.Time{}
	}
	for k, times := range a.attempts {
		first := 0
		for first < len(times) && !times[first].After(now.Add(-authWindow)) {
			first++
		}
		if first == len(times) {
			delete(a.attempts, k)
		} else if first > 0 {
			a.attempts[k] = append([]time.Time(nil), times[first:]...)
		}
	}
	recent := a.attempts[key]
	if len(recent) >= limit || len(recent) == 0 && len(a.attempts) >= authBucketLimit {
		return false
	}
	a.attempts[key] = append(recent, now)
	return true
}

// Used refresh hashes survive while their credential can still be replayed.
// Expired connections cascade credentials/proposals; audits are preserved.
func pruneCredentialsTx(tx *sql.Tx, now time.Time) error {
	for _, query := range []string{
		"DELETE FROM sessions WHERE expires_at<=?",
		"DELETE FROM mcp_oauth_requests WHERE expires_at<=?",
		"DELETE FROM mcp_oauth_codes WHERE expires_at<=?",
		"DELETE FROM mcp_access_tokens WHERE expires_at<=?",
		"DELETE FROM mcp_refresh_tokens WHERE expires_at<=?",
		"DELETE FROM mcp_tokens WHERE expires_at<=?",
		"DELETE FROM mcp_oauth_clients WHERE expires_at<=? AND NOT EXISTS(SELECT 1 FROM mcp_tokens WHERE client_id=mcp_oauth_clients.id)",
	} {
		if _, err := tx.Exec(query, now.Unix()); err != nil {
			return err
		}
	}
	return nil
}

// Maintenance runs only when serving, including quiet password-only deployments.
func (a *App) StartCredentialMaintenance() {
	a.authMaintenanceOnce.Do(func() {
		a.authMaintenanceWG.Add(1)
		go func() {
			defer a.authMaintenanceWG.Done()
			ticker := time.NewTicker(time.Hour)
			defer ticker.Stop()
			for {
				if err := a.write(func(tx *sql.Tx) error { return pruneCredentialsTx(tx, time.Now()) }); err != nil {
					log.Print("Credential retention cleanup failed")
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
