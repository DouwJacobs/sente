package app

import (
	"net/http"
	"strconv"
	"time"
)

func (a *App) notificationDiagnostics(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	page, size, err := listPage(r)
	if err != nil {
		return err
	}
	days := 7
	if raw := r.URL.Query().Get("days"); raw != "" {
		days, err = strconv.Atoi(raw)
		if err != nil || days < 1 || days > 14 {
			return fail(400, "Choose a range of 1 to 14 days")
		}
	}
	where := "evaluated_at>?"
	args := []any{time.Now().Add(-time.Duration(days) * 24 * time.Hour).Unix()}
	if kind := r.URL.Query().Get("type"); kind != "" {
		if !knownNotificationType(kind) {
			return fail(400, "Choose a notification type")
		}
		where += " AND type=?"
		args = append(args, kind)
	}
	if status := r.URL.Query().Get("status"); status != "" {
		allowed := map[string]bool{"resolved": true, "delivered": true, "duplicate": true, "disabled": true, "deduplicated": true, "cooldown_suppressed": true, "no_longer_triggered": true, "coalesced": true, "pending": true, "sent": true, "channel_failure": true, "expired": true, "permission_revoked": true, "uncertain": true, "no_device": true}
		if !allowed[status] {
			return fail(400, "Choose a delivery status")
		}
		where += " AND status=?"
		args = append(args, status)
	}
	if r.URL.Query().Get("status") == "" {
		where += " AND status NOT IN ('deduplicated','no_longer_triggered')"
	}
	if raw := r.URL.Query().Get("user"); raw != "" {
		uid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || uid <= 0 {
			return fail(400, "Choose a user ID")
		}
		where += " AND user_id=?"
		args = append(args, uid)
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	// Recheck current admin role within the snapshot, including concurrent removal.
	if queryInt(tx, "SELECT COUNT(*) FROM users WHERE id=? AND admin=1 AND disabled=0 AND deleted_at IS NULL", Current(r).ID) != 1 {
		return fail(403, "Administrator access required")
	}
	var total int64
	if err := tx.QueryRow("SELECT COUNT(*) FROM notification_diagnostics WHERE "+where, args...).Scan(&total); err != nil {
		return err
	}
	items, err := data(tx, "SELECT id,user_id,(SELECT username FROM users WHERE users.id=notification_diagnostics.user_id) AS username,type,key_hash,evaluated_at,channel,status,error_code FROM notification_diagnostics WHERE "+where+" ORDER BY id DESC LIMIT ? OFFSET ?", append(args, size, page*size)...)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items, "total": total, "page": page, "page_size": size})
	return nil
}
