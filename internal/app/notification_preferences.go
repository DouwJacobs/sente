package app

import (
	"database/sql"
	"errors"
	"net/http"
)

type notificationPreference struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
	Version int64  `json:"version"`
}

func readNotificationPreferences(q queryer, uid int64) ([]notificationPreference, error) {
	items := make([]notificationPreference, 0, len(notificationTypes))
	for _, kind := range notificationTypes {
		item := notificationPreference{Type: kind, Channel: "in_app", Enabled: true}
		err := q.QueryRow("SELECT enabled,version FROM notification_preferences WHERE user_id=? AND type=? AND channel='in_app'", uid, kind).Scan(&item.Enabled, &item.Version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}
func (a *App) notificationPreferences(w http.ResponseWriter, r *http.Request) error {
	items, err := readNotificationPreferences(a.DB, Current(r).ID)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items})
	return nil
}
func (a *App) updateNotificationPreference(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Type    string `json:"type"`
		Channel string `json:"channel"`
		Enabled *bool  `json:"enabled"`
		Version *int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if !knownNotificationType(b.Type) || b.Channel != "in_app" || b.Enabled == nil || b.Version == nil || *b.Version < 0 {
		return fail(400, "Choose a valid notification type, channel, preference and version")
	}
	uid := Current(r).ID
	err := a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, false); err != nil {
			return err
		}
		var version int64
		err := tx.QueryRow("SELECT version FROM notification_preferences WHERE user_id=? AND type=? AND channel=?", uid, b.Type, b.Channel).Scan(&version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if version != *b.Version {
			return fail(409, "Notification preference changed; reload before saving")
		}
		if version == 0 {
			_, err = tx.Exec("INSERT INTO notification_preferences(user_id,type,channel,enabled) VALUES(?,?,?,?)", uid, b.Type, b.Channel, *b.Enabled)
		} else {
			_, err = tx.Exec("UPDATE notification_preferences SET enabled=?,version=version+1 WHERE user_id=? AND type=? AND channel=?", *b.Enabled, uid, b.Type, b.Channel)
		}
		if err != nil {
			return err
		}
		return audit(tx, Current(r), nil, "notification_preferences", uid, "updated", map[string]any{"type": b.Type, "channel": b.Channel, "enabled": *b.Enabled, "version": version + 1})
	})
	if err != nil {
		return err
	}
	return a.notificationPreferences(w, r)
}
