package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"
)

func (a *App) configurationApply(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		PreviewID    string `json:"preview_id"`
		AllowUpdates bool   `json:"allow_updates"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	summary := &RulesetImportSummary{}
	err := a.write(func(tx *sql.Tx) error {
		u, err := rulesetActor(tx, ptrUser(Current(r)))
		if err != nil {
			return err
		}
		var fingerprint, payload, source string
		err = tx.QueryRow("SELECT fingerprint,payload,source FROM configuration_previews WHERE id=? AND user_id=? AND expires_at>=?", b.PreviewID, u.ID, time.Now().Unix()).Scan(&fingerprint, &payload, &source)
		if err != nil {
			return fail(409, "This preview expired or was already used. Preview the source again.")
		}
		var cfg RulesetConfig
		var s configurationSource
		if err = json.Unmarshal([]byte(payload), &cfg); err != nil {
			return err
		}
		if err = json.Unmarshal([]byte(source), &s); err != nil {
			return err
		}
		before, current, err := configurationSnapshot(tx)
		if err != nil {
			return err
		}
		if current != fingerprint {
			return fail(409, "Configuration or access changed. Preview the source again.")
		}
		if err = a.importRulesetTx(tx, u, cfg, summary); err != nil {
			return err
		}
		after, _, err := configurationSnapshot(tx)
		if err != nil {
			return err
		}
		changes := configurationChanges(before, after)
		for _, change := range changes {
			if change.Action == "update" && !b.AllowUpdates {
				return fail(409, "Review and confirm replacements of existing configuration")
			}
		}
		if s.ID == 0 {
			if queryInt(tx, "SELECT COUNT(*) FROM configuration_sources") >= 50 {
				return fail(400, "Keep at most 50 configuration sources")
			}
			res, e := tx.Exec("INSERT INTO configuration_sources(name,kind,repository_url,ref,path,last_revision,last_sync) VALUES(?,?,?,?,?,?,?)", s.Name, s.Kind, s.RepositoryURL, s.Ref, s.Path, s.Revision, time.Now().UTC().Format(time.RFC3339))
			if e != nil {
				return e
			}
			s.ID, err = res.LastInsertId()
			if err != nil {
				return err
			}
		} else {
			res, e := tx.Exec("UPDATE configuration_sources SET last_revision=?,last_sync=?,version=version+1 WHERE id=? AND version=?", s.Revision, time.Now().UTC().Format(time.RFC3339), s.ID, s.Version)
			if e != nil {
				return e
			}
			if e = affected(res); e != nil {
				return e
			}
		}
		if err = audit(tx, u, nil, "configuration_source", s.ID, "imported", map[string]any{"revision": s.Revision, "summary": summary}); err != nil {
			return err
		}
		_, err = tx.Exec("DELETE FROM configuration_previews WHERE id=?", b.PreviewID)
		return err
	})
	if err != nil {
		return err
	}
	send(w, summary)
	return nil
}
func (a *App) configurationForget(w http.ResponseWriter, r *http.Request) error {
	var b struct {
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		u, err := rulesetActor(tx, ptrUser(Current(r)))
		if err != nil {
			return err
		}
		res, err := tx.Exec("DELETE FROM configuration_sources WHERE id=? AND version=?", parseID(r), b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "configuration_source", parseID(r), "forgotten", nil)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
