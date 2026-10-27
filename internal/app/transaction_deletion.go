package app

import (
	"database/sql"
	"net/http"
)

// Browser deletion uses the same authorized archival write as MCP cleanup.
func (a *App) deleteTransaction(w http.ResponseWriter, r *http.Request) error {
	var input struct {
		Version   int64  `json:"version"`
		Reason    string `json:"reason"`
		Confirmed bool   `json:"confirmed"`
	}
	if err := decode(r, &input); err != nil {
		return err
	}
	if !input.Confirmed {
		return fail(400, "Confirm deleting this transaction")
	}
	if err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		snapshot, err := a.transactionRemovalSnapshot(tx, u, mcpSeenItem{ID: parseID(r), Version: input.Version})
		if err != nil {
			return err
		}
		return a.removeTransactionsTx(tx, u, nil, []transactionRemovalSnapshot{snapshot}, input.Reason)
	}); err != nil {
		return err
	}
	success(w)
	return nil
}
