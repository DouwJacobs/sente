package app

import (
	"database/sql"
	"net/http"
	"strings"
	"time"
)

func (a *App) notificationRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/notifications/diagnostics", wrap(a.notificationDiagnostics))
	a.notificationPushRoutes(m)
	m.HandleFunc("GET /api/notifications/push/messages/{id}", wrap(a.pushNotificationMessage))
	m.HandleFunc("GET /api/notifications", wrap(a.listNotifications))
	m.HandleFunc("GET /api/notifications/unread-count", wrap(a.notificationUnreadCount))
	m.HandleFunc("POST /api/notifications/read-all", wrap(a.readAllNotifications))
	m.HandleFunc("POST /api/notifications/{id}/read", wrap(a.readNotification))
	m.HandleFunc("POST /api/notifications/{id}/dismiss", wrap(a.dismissNotification))
	m.HandleFunc("GET /api/notifications/preferences", wrap(a.notificationPreferences))
	m.HandleFunc("PUT /api/notifications/preferences", wrap(a.updateNotificationPreference))
	m.HandleFunc("PUT /api/notifications/preferences/batch", wrap(a.updateNotificationPreferencesBatch))
}

// Apply recipient, active-user and dependency authorization BEFORE count/page.
// Recheck stored dependencies plus a transaction's current account. An account
// becoming private/hidden, grants changing, or a source disappearing hides the
// entire old payload. Administrators get no cross-user inbox exception.
func notificationVisibilitySQL() string {
	return `n.in_app_enabled=1 AND n.user_id=? AND n.dismissed_at IS NULL AND n.created_at>? AND
 EXISTS(SELECT 1 FROM users u WHERE u.id=n.user_id AND u.disabled=0 AND u.deleted_at IS NULL) AND
 NOT EXISTS(SELECT 1 FROM notification_accounts d LEFT JOIN accounts a ON a.id=d.account_id
 WHERE d.notification_id=n.id AND (a.id IS NULL OR a.sync_hidden=1 OR
 (n.source_kind='budget' AND a.household=0) OR NOT (
 (a.household=1 AND EXISTS(SELECT 1 FROM users u WHERE u.id=n.user_id AND u.budget_member=1)) OR
 EXISTS(SELECT 1 FROM grants g WHERE g.user_id=n.user_id AND g.account_id=a.id AND g.role IN ('viewer','editor'))))) AND
 (n.source_kind='system' OR
 (n.source_kind='account' AND EXISTS(SELECT 1 FROM notification_accounts d WHERE d.notification_id=n.id AND d.account_id=n.source_id)) OR
 (n.source_kind='transaction' AND EXISTS(SELECT 1 FROM transactions t JOIN notification_accounts d ON d.account_id=t.account_id WHERE t.id=n.source_id AND d.notification_id=n.id)) OR
 (n.source_kind='budget' AND EXISTS(SELECT 1 FROM periods p WHERE p.id=n.source_id) AND EXISTS(SELECT 1 FROM users u WHERE u.id=n.user_id AND u.budget_member=1)))`
}
func notificationPushVisibilitySQL() string {
	return strings.TrimPrefix(notificationVisibilitySQL(), "n.in_app_enabled=1 AND ")
}

func (a *App) pushNotificationMessage(w http.ResponseWriter, r *http.Request) error {
	id := parseID(r)
	if id <= 0 {
		return fail(400, "Choose a notification")
	}
	items, err := data(a.DB, `SELECT n.id,n.type,n.severity,n.title,n.message,n.source_kind,n.source_id,n.occurred_at,n.created_at,n.read_at,n.dismissible FROM notifications n WHERE n.id=? AND `+notificationPushVisibilitySQL(), append([]any{id}, notificationReadArgs(Current(r).ID)...)...)
	if err != nil {
		return err
	}
	if len(items) != 1 {
		return fail(404, "Notification unavailable")
	}
	send(w, items[0])
	return nil
}

func notificationReadArgs(uid int64) []any {
	return []any{uid, time.Now().Add(-notificationRetention).Unix()}
}
func visibleNotificationCounts(q queryer, uid int64) (int64, int64, error) {
	var total, unread int64
	err := q.QueryRow("SELECT COUNT(*),COALESCE(SUM(n.read_at IS NULL),0) FROM notifications n WHERE "+notificationVisibilitySQL(), notificationReadArgs(uid)...).Scan(&total, &unread)
	return total, unread, err
}
func (a *App) listNotifications(w http.ResponseWriter, r *http.Request) error {
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	state := r.URL.Query().Get("state")
	if state != "" && state != "unread" && state != "read" {
		return fail(400, "Choose read or unread notifications")
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	uid := Current(r).ID
	total, unread, err := visibleNotificationCounts(tx, uid)
	if err != nil {
		return err
	}
	where := notificationVisibilitySQL()
	if state == "unread" {
		where += " AND n.read_at IS NULL"
	}
	if state == "read" {
		where += " AND n.read_at IS NOT NULL"
	}
	args := notificationReadArgs(uid)
	var matched int64
	if err := tx.QueryRow("SELECT COUNT(*) FROM notifications n WHERE "+where, args...).Scan(&matched); err != nil {
		return err
	}
	items, err := data(tx, `SELECT n.id,n.type,n.severity,n.title,n.message,n.source_kind,n.source_id,n.occurred_at,n.created_at,n.read_at,n.dismissible FROM notifications n WHERE `+where+" ORDER BY n.id DESC LIMIT ? OFFSET ?", append(args, size, page*size)...)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items, "total": matched, "inbox_total": total, "unread_count": unread, "page": page, "page_size": size})
	return nil
}
func (a *App) notificationUnreadCount(w http.ResponseWriter, r *http.Request) error {
	_, unread, err := visibleNotificationCounts(a.DB, Current(r).ID)
	if err != nil {
		return err
	}
	send(w, map[string]any{"unread_count": unread})
	return nil
}
func (a *App) changeNotificationState(w http.ResponseWriter, r *http.Request, operation string) error {
	id := parseID(r)
	if operation != "read_all" && id <= 0 {
		return fail(400, "Choose a notification")
	}
	var unread int64
	err := a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, false); err != nil {
			return err
		}
		uid := Current(r).ID
		if err := pruneNotificationsTx(tx, time.Now()); err != nil {
			return err
		}
		where := notificationVisibilitySQL()
		if operation != "read_all" {
			where = notificationPushVisibilitySQL()
		}
		args := notificationReadArgs(uid)
		if operation != "read_all" {
			where += " AND n.id=?"
			args = append(args, id)
		}
		if operation == "dismiss" {
			where += " AND n.dismissible=1"
		}
		if operation != "read_all" {
			var count int
			if err := tx.QueryRow("SELECT COUNT(*) FROM notifications n WHERE "+where, args...).Scan(&count); err != nil {
				return err
			}
			if count != 1 {
				return fail(404, "Notification not found or unavailable")
			}
		}
		field := "read_at"
		if operation == "dismiss" {
			field = "dismissed_at"
		}
		_, err := tx.Exec("UPDATE notifications SET "+field+"=COALESCE("+field+",?) WHERE id IN (SELECT n.id FROM notifications n WHERE "+where+")", append([]any{time.Now().Unix()}, args...)...)
		if err != nil {
			return err
		}
		_, unread, err = visibleNotificationCounts(tx, uid)
		return err
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"ok": true, "unread_count": unread})
	return nil
}
func (a *App) readNotification(w http.ResponseWriter, r *http.Request) error {
	return a.changeNotificationState(w, r, "read")
}
func (a *App) readAllNotifications(w http.ResponseWriter, r *http.Request) error {
	return a.changeNotificationState(w, r, "read_all")
}
func (a *App) dismissNotification(w http.ResponseWriter, r *http.Request) error {
	return a.changeNotificationState(w, r, "dismiss")
}
