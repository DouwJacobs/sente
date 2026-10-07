package app

import (
	"net/http"
)

func (a *App) transactionAudit(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	id := parseID(r)
	var account int64
	if err := a.DB.QueryRow("SELECT account_id FROM transactions WHERE id=?", id).Scan(&account); err != nil {
		return fail(404, "Transaction not found")
	}
	if !a.can(a.DB, u, account, false) {
		return fail(403, "Account access required")
	}
	if r.URL.Query().Has("page") {
		return a.metadataPage(w, r, "SELECT audit.id,audit.action,audit.details,audit.created_at,u.username FROM audit LEFT JOIN users u ON u.id=audit.user_id WHERE entity='transaction' AND entity_id=?", []any{id}, "action", "id")
	}
	rows, err := data(a.DB, "SELECT audit.id,audit.action,audit.details,audit.created_at,u.username FROM audit LEFT JOIN users u ON u.id=audit.user_id WHERE entity='transaction' AND entity_id=? ORDER BY audit.id LIMIT 100", id)
	if err != nil {
		return err
	}
	send(w, rows)
	return nil
}
