package app

import (
	"net/http"

	"finance-tracker/internal/buildinfo"
)

func (a *App) buildInfo(w http.ResponseWriter, r *http.Request) error {
	send(w, buildinfo.Current())
	return nil
}

func (a *App) applicationUpdate(w http.ResponseWriter, r *http.Request) error {
	info := buildinfo.Current()
	send(w, a.releaseChecker.Check(r.Context(), info.Version, info.Modified, r.Method == http.MethodPost))
	return nil
}
