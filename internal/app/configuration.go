package app

import (
	"bytes"
	_ "embed"
	"net/http"
	"os"
)

//go:embed configuration/starter.json
var starterConfiguration []byte

type configurationSource struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Kind          string `json:"kind"`
	RepositoryURL string `json:"repository_url"`
	Ref           string `json:"ref"`
	Path          string `json:"path"`
	Revision      string `json:"revision"`
	Version       int64  `json:"version"`
}
type configurationChange struct {
	Entity string         `json:"entity"`
	Name   string         `json:"name"`
	Action string         `json:"action"`
	Before map[string]any `json:"before"`
	After  map[string]any `json:"after"`
}

func (a *App) configurationRoutes(m *http.ServeMux) {
	m.HandleFunc("GET /api/configuration", wrap(a.configurationStatus))
	m.HandleFunc("GET /api/configuration/export", wrap(a.configurationExport))
	m.HandleFunc("POST /api/configuration/preview", wrap(a.configurationPreview))
	m.HandleFunc("POST /api/configuration/apply", wrap(a.configurationApply))
	m.HandleFunc("DELETE /api/configuration/sources/{id}", wrap(a.configurationForget))
}
func defaultConfigurationSource() configurationSource {
	repository := os.Getenv("SENTE_DEFAULT_CONFIG_REPOSITORY")
	if repository == "" {
		repository = "https://github.com/DouwJacobs/sente-config.git"
	}
	ref := os.Getenv("SENTE_DEFAULT_CONFIG_REF")
	path := os.Getenv("SENTE_DEFAULT_CONFIG_PATH")
	if path == "" {
		path = "sente.json"
	}
	return configurationSource{Name: "Sente starter", Kind: "starter", RepositoryURL: repository, Ref: ref, Path: path}
}
func (a *App) configurationStatus(w http.ResponseWriter, r *http.Request) error {
	if _, err := rulesetActor(a.DB, ptrUser(Current(r))); err != nil {
		return err
	}
	rows, err := data(a.DB, "SELECT id,name,kind,repository_url,ref,path,last_revision,last_sync,version FROM configuration_sources ORDER BY id")
	if err != nil {
		return err
	}
	send(w, map[string]any{"sources": rows, "default_source": defaultConfigurationSource()})
	return nil
}
func ptrUser(u User) *User { return &u }
func (a *App) configurationExport(w http.ResponseWriter, r *http.Request) error {
	var buf bytes.Buffer
	if err := a.exportRulesetFor(&buf, ptrUser(Current(r))); err != nil {
		return err
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="sente-configuration.json"`)
	_, err := w.Write(buf.Bytes())
	return err
}
