package app

import "database/sql"

// Owner Settings uses exactly the same scope/consent/privacy checks as MCP.
func (a *App) browserBankRefreshJobs(u User) ([]bankRefreshStatus, error) {
	jobs := []bankRefreshStatus{}
	if !u.Admin {
		return jobs, nil
	}
	tx, err := a.DB.Begin()
	if err != nil {
		return jobs, err
	}
	defer tx.Rollback()
	err = func(tx *sql.Tx) error {
		rows, err := data(tx, "SELECT id,connection_id FROM bank_refresh_jobs WHERE user_id=? AND status!='result_expired' ORDER BY created_at DESC,id DESC LIMIT 50", u.ID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			out, err := a.readBankRefreshStatus(tx, mcpIdentity{User: u, TokenID: num(row["connection_id"])}, row["id"].(string))
			if err == nil {
				jobs = append(jobs, out)
			}
		}
		return nil
	}(tx)
	return jobs, err
}
