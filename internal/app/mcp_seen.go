package app

import "database/sql"

type mcpSeenItem struct {
	ID      int64 `json:"id"`
	Version int64 `json:"version"`
}
type mcpSeenState struct {
	mcpSeenItem
	AccountID int64 `json:"account_id"`
	Seen      bool  `json:"seen"`
}

func (a *App) prepareMCPSeen(tx *sql.Tx, identity mcpIdentity, b mcpChange) ([]mcpSeenState, error) {
	if b.Seen == nil || len(b.SeenItems) < 1 || len(b.SeenItems) > 100 {
		return nil, fail(400, "Choose seen or unseen and select 1–100 transactions")
	}
	states := []mcpSeenState{}
	ids := map[int64]bool{}
	for _, item := range b.SeenItems {
		if ids[item.ID] || item.ID <= 0 || item.Version <= 0 {
			return nil, fail(400, "Select distinct transaction IDs and current versions")
		}
		ids[item.ID] = true
		var account, version int64
		if err := tx.QueryRow("SELECT account_id,version FROM transactions WHERE id=?", item.ID).Scan(&account, &version); err != nil {
			return nil, fail(404, "Transaction not found")
		}
		if !a.can(tx, identity.User, account, false) || queryInt(tx, "SELECT COUNT(*) FROM accounts WHERE id=? AND sync_hidden=0", account) != 1 {
			return nil, fail(403, "Account access required")
		}
		if version != item.Version {
			return nil, fail(409, "Transaction changed; reload before proposing seen state")
		}
		seen := queryInt(tx, "SELECT COUNT(*) FROM transaction_seen WHERE user_id=? AND transaction_id=? AND transaction_version=?", identity.User.ID, item.ID, version) == 1
		states = append(states, mcpSeenState{mcpSeenItem: item, AccountID: account, Seen: seen})
	}
	return states, nil
}
func (a *App) applyMCPSeen(tx *sql.Tx, identity mcpIdentity, exact mcpExactChange) error {
	current, err := a.prepareMCPSeen(tx, identity, exact.Change)
	if err != nil {
		return err
	}
	if len(current) != len(exact.SeenStates) {
		return fail(409, "Seen selection changed")
	}
	for i, item := range current {
		if item != exact.SeenStates[i] {
			return fail(409, "Seen state changed; prepare a new proposal")
		}
		if *exact.Change.Seen {
			err = markSeen(tx, identity.User.ID, item.ID, item.Version)
		} else {
			_, err = tx.Exec("DELETE FROM transaction_seen WHERE user_id=? AND transaction_id=?", identity.User.ID, item.ID)
		}
		if err != nil {
			return err
		}
	}
	return nil
}
