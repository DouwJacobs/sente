package app

import (
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// A condition consumes an occurrence even when disabled. Enabling a type does
// not replay old crossings. Cooldown suppresses a new cycle until its next pass.
func (a *App) evaluateConditionTx(tx *sql.Tx, e notificationEvent, scope string, active bool, cooldown time.Duration, now time.Time, inactiveReason ...string) error {
	key := hash(scope)
	var prior, consumed bool
	var cycle, last int64
	err := tx.QueryRow("SELECT active,cycle,consumed,last_sent FROM notification_conditions WHERE user_id=? AND type=? AND scope_hash=?", e.RecipientID, e.Type, key).Scan(&prior, &cycle, &consumed, &last)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	status := "resolved"
	if active {
		if !prior {
			cycle++
			consumed = false
		}
		status = "deduplicated"
		if !consumed {
			if last > 0 && now.Unix()-last < int64(cooldown.Seconds()) {
				status = "cooldown_suppressed"
			} else {
				e.DedupeKey = fmt.Sprintf("%s:%d", key, cycle)
				e.OccurredAt = now
				result, err := a.notifyTx(tx, e, now)
				if err != nil {
					return err
				}
				status = result.Status
				consumed = true
				if status == "delivered" {
					last = now.Unix()
					if queryInt(tx, "SELECT in_app_enabled FROM notifications WHERE id=?", result.ID) == 0 {
						status = "disabled"
					}
				}
			}
		}
	} else {
		consumed = false
	}
	_, err = tx.Exec(`INSERT INTO notification_conditions(user_id,type,scope_hash,active,cycle,consumed,last_sent,evaluated_at,evaluated_run) VALUES(?,?,?,?,?,?,?,?,?)
 ON CONFLICT(user_id,type,scope_hash) DO UPDATE SET active=excluded.active,cycle=excluded.cycle,consumed=excluded.consumed,last_sent=excluded.last_sent,evaluated_at=excluded.evaluated_at,evaluated_run=excluded.evaluated_run`, e.RecipientID, e.Type, key, active, cycle, consumed, last, now.Unix(), queryInt(tx, "SELECT run FROM notification_evaluation_runs WHERE id=1"))
	if err != nil {
		return err
	}
	if status == "deduplicated" || (!active && !prior) {
		return nil
	}
	code := ""
	if !active && len(inactiveReason) > 0 {
		code = inactiveReason[0]
	}
	if status == "disabled" {
		code = "preference_disabled"
	}
	return notificationDiagnosticTx(tx, e.RecipientID, e.Type, key, "in_app", status, code, now)
}

func notificationDiagnosticTx(tx *sql.Tx, uid int64, kind, key, channel, status, code string, now time.Time) error {
	// In-app diagnostics record transitions; push diagnostics record attempts.
	// Only fixed statuses/codes, never provider text or financial payloads.
	if channel == "in_app" {
		var previousStatus, previousCode string
		err := tx.QueryRow("SELECT status,error_code FROM notification_diagnostics WHERE user_id=? AND type=? AND key_hash=? AND channel=? ORDER BY id DESC LIMIT 1", uid, kind, key, channel).Scan(&previousStatus, &previousCode)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && previousStatus == status && previousCode == code {
			return nil
		}
	}
	_, err := tx.Exec("INSERT INTO notification_diagnostics(user_id,type,key_hash,evaluated_at,channel,status,error_code) VALUES(?,?,?,?,?,?,?)", uid, kind, key, now.Unix(), channel, status, code)
	return err
}

func (a *App) evaluateNotificationAlerts(now time.Time) error {
	now = now.UTC().Truncate(time.Second)
	return a.writeQuiet(func(tx *sql.Tx) error {
		if err := pruneNotificationsTx(tx, now); err != nil {
			return err
		}
		if _, err := tx.Exec("UPDATE notification_evaluation_runs SET run=run+1 WHERE id=1"); err != nil {
			return err
		}
		run := queryInt(tx, "SELECT run FROM notification_evaluation_runs WHERE id=1")
		users, err := data(tx, "SELECT id,username,admin,budget_member FROM users WHERE disabled=0 AND deleted_at IS NULL ORDER BY id")
		if err != nil {
			return err
		}
		for _, row := range users {
			u := User{ID: num(row["id"]), Member: num(row["budget_member"]) == 1}
			if u.Member {
				if err := a.evaluateBudgetAlertsTx(tx, u, now); err != nil {
					return err
				}
			}
			if err := a.evaluateSpendingAlertsTx(tx, u, now); err != nil {
				return err
			}
		}
		// Removed budgets, moved transactions and revoked scopes reset silently.
		_, err = tx.Exec("UPDATE notification_conditions SET active=0,consumed=0 WHERE evaluated_run<>?", run)
		if err != nil {
			return err
		}
		return pruneAlertStateTx(tx, now)
	})
}
func pruneAlertStateTx(tx *sql.Tx, now time.Time) error {
	for _, query := range []string{
		"DELETE FROM notification_diagnostics WHERE evaluated_at<=?",
		"DELETE FROM notification_diagnostics WHERE id NOT IN (SELECT id FROM notification_diagnostics ORDER BY id DESC LIMIT 10000)",
		"DELETE FROM notification_conditions WHERE evaluated_at<=?",
		"DELETE FROM notification_push_outbox WHERE created_at<=?",
	} {
		var args []any
		switch query {
		case "DELETE FROM notification_diagnostics WHERE evaluated_at<=?":
			args = []any{now.Add(-14 * 24 * time.Hour).Unix()}
		case "DELETE FROM notification_conditions WHERE evaluated_at<=?":
			args = []any{now.Add(-notificationReceiptRetention).Unix()}
		case "DELETE FROM notification_push_outbox WHERE created_at<=?":
			args = []any{now.Add(-7 * 24 * time.Hour).Unix()}
		}
		if _, err := tx.Exec(query, args...); err != nil {
			return err
		}
	}
	return nil
}
