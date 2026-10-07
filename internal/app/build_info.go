package app

import (
	"net/http"

	"finance-tracker/internal/buildinfo"
)

func (a *App) buildInfo(w http.ResponseWriter, r *http.Request) error {
	send(w, buildinfo.Current())
	return nil
}
