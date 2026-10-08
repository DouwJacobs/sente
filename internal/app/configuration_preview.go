package app

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

func (a *App) configurationPreview(w http.ResponseWriter, r *http.Request) error {
	_, err := rulesetActor(a.DB, ptrUser(Current(r)))
	if err != nil {
		return err
	}
	var s configurationSource
	var raw []byte
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		r.Body = http.MaxBytesReader(w, r.Body, maxRulesetBytes+(1<<20))
		if err = r.ParseMultipartForm(1 << 20); err != nil {
			return fail(400, "Choose a configuration JSON file up to 32 MiB")
		}
		defer r.MultipartForm.RemoveAll()
		file, header, e := r.FormFile("file")
		if e != nil {
			return fail(400, "Choose a configuration JSON file")
		}
		defer file.Close()
		raw, err = io.ReadAll(io.LimitReader(file, maxRulesetBytes+1))
		if err != nil || len(raw) > maxRulesetBytes {
			return fail(400, "Configuration exceeds 32 MiB")
		}
		s = configurationSource{Name: header.Filename, Kind: "file"}
		if id := r.FormValue("source_id"); id != "" {
			var sourceID int64
			if _, err := fmt.Sscan(id, &sourceID); err != nil {
				return fail(400, "Choose an existing file source")
			}
			s, err = readConfigurationSource(a.DB, sourceID)
			if err != nil {
				return err
			}
			if s.Kind != "file" {
				return fail(400, "Choose a file source for a replacement file")
			}
		}
		s.Revision = hash(string(raw))
	} else {
		var b struct {
			Source   configurationSource `json:"source"`
			Starter  bool                `json:"starter"`
			Bundled  bool                `json:"bundled"`
			SourceID int64               `json:"source_id"`
		}
		if err = decode(r, &b); err != nil {
			return err
		}
		s = b.Source
		s.ID = 0
		s.Version = 0
		s.Revision = ""
		if b.SourceID != 0 {
			s, err = readConfigurationSource(a.DB, b.SourceID)
			if err != nil {
				return err
			}
			if s.Kind == "file" {
				return fail(400, "Upload the updated file to pull a file source again")
			}
		} else if b.Starter {
			s = defaultConfigurationSource()
		}
		if b.Bundled && b.SourceID == 0 {
			s = configurationSource{Name: "Sente starter", Kind: "starter", RepositoryURL: "https://github.com/DouwJacobs/sente-config.git", Path: "sente.json"}
			raw = starterConfiguration
			s.Revision = "bundled-" + hash(string(raw))[:12]
		} else {
			if s.Kind != "starter" {
				s.Kind = "repository"
			}
			raw, s.Revision, err = fetchConfigurationRepository(r.Context(), s)
			if err != nil {
				return err
			}
		}
	}
	s.Name = strings.TrimSpace(s.Name)
	if len(s.Name) < 1 || len(s.Name) > 200 {
		return fail(400, "Use a source name of 1–200 characters")
	}
	cfg, err := decodeRuleset(bytes.NewReader(raw))
	if err != nil {
		return fail(400, err.Error())
	}
	id := randomToken()
	var changes []configurationChange
	var summary *RulesetImportSummary
	err = a.browserWrite(r, func(tx *sql.Tx, u User) error {
		actor, e := rulesetActor(tx, &u)
		if e != nil {
			return e
		}
		var fingerprint string
		changes, fingerprint, summary, e = a.simulateConfiguration(tx, actor, cfg)
		if e != nil {
			var p problem
			if errors.As(e, &p) {
				return e
			}
			return fail(400, e.Error())
		}
		if s.ID != 0 {
			saved, e := readConfigurationSource(tx, s.ID)
			if e != nil {
				return e
			}
			if saved.Version != s.Version {
				return fail(409, "The source changed. Preview it again.")
			}
		}
		if _, e = tx.Exec("DELETE FROM configuration_previews WHERE expires_at<? OR user_id=?", time.Now().Unix(), u.ID); e != nil {
			return e
		}
		payload, _ := json.Marshal(cfg)
		source, _ := json.Marshal(s)
		_, e = tx.Exec("INSERT INTO configuration_previews VALUES(?,?,?,?,?,?)", id, u.ID, fingerprint, string(payload), string(source), time.Now().Add(15*time.Minute).Unix())
		return e
	})
	if err != nil {
		return err
	}
	send(w, map[string]any{"id": id, "source": s, "changes": changes, "summary": summary, "expires_in_seconds": 900})
	return nil
}
