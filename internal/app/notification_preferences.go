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
func readAllNotificationPreferences(q queryer, uid int64) ([]notificationPreference, error) {
	items, err := readNotificationPreferences(q, uid)
	if err != nil {
		return nil, err
	}
	for _, kind := range notificationTypes {
		pref := notificationPreference{Type: kind, Channel: "push"}
		err := q.QueryRow("SELECT enabled,version FROM notification_push_preferences WHERE user_id=? AND type=?", uid, kind).Scan(&pref.Enabled, &pref.Version)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		items = append(items, pref)
	}
	return items, nil
}
func (a *App) notificationPreferences(w http.ResponseWriter, r *http.Request) error {
	read := readNotificationPreferences
	if r.URL.Query().Get("channels") == "all" {
		read = readAllNotificationPreferences
	}
	items, err := read(a.DB, Current(r).ID)
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items})
	return nil
}

type notificationPreferenceInput struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	Enabled *bool  `json:"enabled"`
	Version *int64 `json:"version"`
}

func (a *App) updateNotificationPreference(w http.ResponseWriter, r *http.Request) error {
	var input notificationPreferenceInput
	if err := decode(r, &input); err != nil {
		return err
	}
	return a.saveNotificationPreferences(w, r, []notificationPreferenceInput{input})
}

func (a *App) updateNotificationPreferencesBatch(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Items []notificationPreferenceInput `json:"items"`
	}
	if err := decode(r, &input); err != nil {
		return err
	}
	return a.saveNotificationPreferences(w, r, input.Items)
}

func (a *App) saveNotificationPreferences(w http.ResponseWriter, r *http.Request, inputs []notificationPreferenceInput) error {
	if len(inputs) == 0 || len(inputs) > 2*len(notificationTypes) {
		return fail(400, "Choose at least one notification preference")
	}
	seen := make(map[string]bool)
	for _, input := range inputs {
		if !knownNotificationType(input.Type) || (input.Channel != "in_app" && input.Channel != "push") || input.Enabled == nil || input.Version == nil || *input.Version < 0 || seen[input.Type+":"+input.Channel] {
			return fail(400, "Choose valid, distinct notification preferences")
		}
		seen[input.Type+":"+input.Channel] = true
	}
	uid := Current(r).ID
	var items []notificationPreference
	err := a.write(func(tx *sql.Tx) error {
		if err := userSecurityActorTx(tx, r, false); err != nil {
			return err
		}
		for _, input := range inputs {
			var version int64
			var err error
			if input.Channel == "push" {
				err = tx.QueryRow("SELECT version FROM notification_push_preferences WHERE user_id=? AND type=?", uid, input.Type).Scan(&version)
			} else {
				err = tx.QueryRow("SELECT version FROM notification_preferences WHERE user_id=? AND type=? AND channel=?", uid, input.Type, input.Channel).Scan(&version)
			}
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if version != *input.Version {
				return fail(409, "Notification preference changed; reload before saving")
			}
			if input.Channel == "push" {
				_, err = tx.Exec(`INSERT INTO notification_push_preferences(user_id,type,enabled) VALUES(?,?,?) ON CONFLICT(user_id,type) DO UPDATE SET enabled=excluded.enabled,version=version+1`, uid, input.Type, *input.Enabled)
			} else if version == 0 {
				_, err = tx.Exec("INSERT INTO notification_preferences(user_id,type,channel,enabled) VALUES(?,?,?,?)", uid, input.Type, input.Channel, *input.Enabled)
			} else {
				_, err = tx.Exec("UPDATE notification_preferences SET enabled=?,version=version+1 WHERE user_id=? AND type=? AND channel=?", *input.Enabled, uid, input.Type, input.Channel)
			}
			if err != nil {
				return err
			}
			if err := audit(tx, Current(r), nil, "notification_preferences", uid, "updated", map[string]any{"type": input.Type, "channel": input.Channel, "enabled": *input.Enabled, "version": version + 1}); err != nil {
				return err
			}
		}
		var err error
		read := readNotificationPreferences
		if r.URL.Query().Get("channels") == "all" {
			read = readAllNotificationPreferences
		}
		items, err = read(tx, uid)
		return err
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"items": items})
	return nil
}
