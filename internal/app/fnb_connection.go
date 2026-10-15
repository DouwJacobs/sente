package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func (a *App) fnbStatus(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	rows, err := data(a.DB, "SELECT interval_hours,daily_time,schedule_json,state,last_attempt,last_success,next_due,last_error,last_skipped,debug_browser,last_diagnostics,version FROM fnb_connections WHERE user_id=?", u.ID)
	if err != nil {
		return err
	}
	discoveries := []map[string]any{}
	if r.URL.Query().Get("summary") != "1" {
		discoveries, err = data(a.DB, "SELECT bank_id,name,balance_cents,balance_date,hidden,account_id FROM fnb_discoveries WHERE user_id=? ORDER BY name,bank_id LIMIT 100", u.ID)
		if err != nil {
			return err
		}
	}
	var connection any = nil
	if len(rows) > 0 {
		connection = rows[0]
		calendar, err := parseFNBCalendar(rows[0]["schedule_json"].(string))
		if err != nil {
			return err
		}
		rows[0]["schedule"] = calendar
		delete(rows[0], "schedule_json")
		var diagnostics map[string]int
		if text, ok := rows[0]["last_diagnostics"].(string); ok {
			json.Unmarshal([]byte(text), &diagnostics)
		}
		rows[0]["last_diagnostics"] = safeFNBDiagnostics(diagnostics)
	}
	refreshingID := int64(0)
	kind := ""
	if len(rows) > 0 && rows[0]["state"] == "refreshing" {
		refreshingID = a.fnbRefreshAccount.Load()
		kind = "accounts"
		if a.fnbRefreshTransactions.Load() {
			kind = "transactions"
		}
	}
	send(w, map[string]any{"connection": connection, "accounts": discoveries, "refreshing_account_id": refreshingID, "refresh_kind": kind})
	return nil
}
func (a *App) fnbConnect(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	if !a.Secure && !strings.HasPrefix(a.PublicURL, "http://127.0.0.1:") && !strings.HasPrefix(a.PublicURL, "http://localhost:") {
		return fail(400, "Use HTTPS to enter bank credentials")
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Version  int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if strings.TrimSpace(b.Username) == "" || len(b.Username) > 200 || b.Password == "" || len(b.Password) > 512 {
		return fail(400, "Enter your FNB username and password")
	}
	a.fnbMu.Lock()
	defer a.fnbMu.Unlock()
	exists := queryInt(a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=?", u.ID) > 0
	key, err := a.fnbKey(!exists && queryInt(a.DB, "SELECT COUNT(*) FROM fnb_connections") == 0)
	if err != nil {
		return fail(503, "FNB encryption key unavailable. Restore the key before saving credentials.")
	}
	plain, _ := json.Marshal(fnbCredentials{Username: b.Username, Password: b.Password})
	secret, err := fnbSeal(key, plain, u.ID)
	clear(plain)
	clear(key)
	if err != nil {
		return err
	}
	err = a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		if exists {
			res, err := tx.Exec("UPDATE fnb_connections SET secret=?,state='ready',last_error='',next_due=NULL,interval_hours=0,daily_time='',schedule_json='',version=version+1 WHERE user_id=? AND version=?", secret, u.ID, b.Version)
			if err != nil {
				return err
			}
			if err = affected(res); err != nil {
				return err
			}
		} else {
			if b.Version != 0 {
				return fail(409, "Connection changed. Reload before saving.")
			}
			if _, err := tx.Exec("INSERT INTO fnb_connections(user_id,secret) VALUES(?,?)", u.ID, secret); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "credentials_replaced", map[string]bool{"schedule_paused": true})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbSchedule(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Hours     int                  `json:"interval_hours"`
		DailyTime string               `json:"daily_time"`
		Schedule  *fnbCalendarSchedule `json:"schedule"`
		Version   int64                `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Hours < 0 || b.Hours > 168 {
		return fail(400, "Choose a whole-hour interval from 1 to 168, or turn automatic refresh off")
	}
	if b.DailyTime != "" && (!validFNBDailyTime(b.DailyTime) || b.Hours != 0) {
		return fail(400, "Choose a daily time in HH:MM format without an interval")
	}
	due := nextFNBDue(time.Now(), b.Hours, b.DailyTime)
	calendarJSON := ""
	if b.Schedule != nil {
		if b.Hours != 0 || b.DailyTime != "" {
			return fail(400, "Choose either an interval or a calendar schedule")
		}
		if err := b.Schedule.validate(); err != nil {
			return err
		}
		encoded, err := json.Marshal(b.Schedule)
		if err != nil {
			return err
		}
		calendarJSON = string(encoded)
		due = nextFNBCalendarDue(time.Now(), b.Schedule)
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		res, err := tx.Exec("UPDATE fnb_connections SET interval_hours=?,daily_time=?,schedule_json=?,next_due=?,version=version+1 WHERE user_id=? AND version=? AND state!='refreshing'", b.Hours, b.DailyTime, calendarJSON, due, u.ID, b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "schedule_updated", map[string]any{"interval_hours": b.Hours, "daily_time": b.DailyTime, "schedule": b.Schedule})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbDisconnect(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	a.fnbMu.Lock()
	defer a.fnbMu.Unlock()
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		if _, err := tx.Exec("DELETE FROM fnb_connections WHERE user_id=?", u.ID); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "disconnected", nil)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbVisibility(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		BankID string `json:"bank_id"`
		Hidden bool   `json:"hidden"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		if err := requireAdmin(u); err != nil {
			return err
		}
		var id sql.NullInt64
		if err := tx.QueryRow("SELECT account_id FROM fnb_discoveries WHERE user_id=? AND bank_id=?", u.ID, b.BankID).Scan(&id); err != nil {
			return fail(404, "Discovered account not found")
		}
		if id.Valid {
			if !a.can(tx, u, id.Int64, true) {
				return fail(403, "Account editor access required")
			}
			if _, err := tx.Exec("UPDATE accounts SET sync_hidden=?,version=version+1 WHERE id=?", b.Hidden, id.Int64); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("UPDATE fnb_discoveries SET hidden=? WHERE user_id=? AND bank_id=?", b.Hidden, u.ID, b.BankID); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "account_visibility_updated", map[string]any{"hidden": b.Hidden})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbRefresh(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		AccountID int64 `json:"account_id"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &b); err != nil {
			return err
		}
	}
	if b.AccountID < 0 {
		return fail(400, "Choose an account")
	}
	if _, err := a.runFNBBrowser(r, false, b.AccountID); err != nil {
		return err
	}
	success(w)
	return nil
}
