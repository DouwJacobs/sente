package app

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type fnbTarget struct {
	ID     int64
	BankID string
}

// Only previously discovered, explicitly visible accounts under current editor
// access are sent to the browser. Fetching creates no accounts or grants.
func (a *App) fnbTargets(q queryer, u User) ([]fnbTarget, error) {
	rows, err := data(q, "SELECT a.id,a.bank_id FROM fnb_discoveries d JOIN accounts a ON a.id=d.account_id AND a.bank_id=d.bank_id WHERE d.user_id=? AND d.hidden=0 AND a.sync_hidden=0 ORDER BY a.id", u.ID)
	if err != nil {
		return nil, err
	}
	out := []fnbTarget{}
	for _, row := range rows {
		id := num(row["id"])
		if ruleAccess(q, a, u, id) {
			out = append(out, fnbTarget{id, row["bank_id"].(string)})
		}
	}
	return out, nil
}
func (a *App) fnbTransactions(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	previews, err := a.runFNB(u.ID, true, true)
	if err != nil {
		return err
	}
	sendImportPreviews(w, r, previews)
	return nil
}

func (a *App) stageFNBTransactions(u User, targets []fnbTarget, snapshot fnbSnapshot, runID string) ([]ParsedFile, error) {
	if snapshot.Error != "" || runID == "" || len(targets) == 0 || len(snapshot.Reports) != len(targets) {
		return nil, fmt.Errorf("invalid transaction response")
	}
	reports := map[string]FNBReport{}
	for _, report := range snapshot.Reports {
		if _, exists := reports[report.BankID]; exists || report.RunID != runID || len(report.Transactions) > 150 {
			return nil, fmt.Errorf("invalid transaction response")
		}
		reports[report.BankID] = report
	}
	previews := []ParsedFile{}
	err := a.write(func(tx *sql.Tx) error {
		var active bool
		if err := tx.QueryRow("SELECT admin=1 AND disabled=0,budget_member FROM users WHERE id=?", u.ID).Scan(&active, &u.Member); err != nil || !active {
			return fail(403, "Connection access revoked")
		}
		current, err := a.fnbTargets(tx, u)
		if err != nil {
			return err
		}
		allowed := map[int64]string{}
		for _, target := range current {
			allowed[target.ID] = target.BankID
		}
		for _, target := range targets {
			if allowed[target.ID] != target.BankID {
				return fail(403, "Account access or visibility changed")
			}
			report, exists := reports[target.BankID]
			if !exists {
				return fmt.Errorf("transaction account mismatch")
			}
			normalized, err := NormalizeFNBReport(report, target.BankID)
			if err != nil {
				return err
			}
			p := normalized.File
			p.RunID, p.Coverage = runID, &normalized.Coverage
			// A transaction fetch cannot overwrite a fresher Accounts balance.
			p.Balance, p.BalanceDate = nil, ""
			p.Name = "FNB live transactions"
			content, err := json.Marshal(report.Transactions)
			if err != nil {
				return err
			}
			p.Hash = hash("fnb-live-v1|" + target.BankID + "|" + report.Currency + "|" + report.AccountType + "|" + strconv.Itoa(report.PageRows) + "|" + string(content))
			// Reuse a durable snapshot, including its original run provenance. The
			// content hash does not identify individual transactions across formats.
			var raw string
			err = tx.QueryRow("SELECT id,data FROM imports WHERE account_id=? AND format='fnb-live' AND hash=? ORDER BY id DESC LIMIT 1", target.ID, p.Hash).Scan(&p.ID, &raw)
			if err == nil {
				id := p.ID
				if err = json.Unmarshal([]byte(raw), &p); err != nil {
					return err
				}
				p.ID = id
			} else if err != sql.ErrNoRows {
				return err
			}
			if err = a.annotate(tx, target.ID, &p); err != nil {
				return err
			}
			if p.ID == 0 {
				if err = stageImport(tx, u, target.ID, &p); err != nil {
					return err
				}
				if err = audit(tx, u, target.ID, "import", p.ID, "fnb_staged", map[string]any{"run_id": runID, "rows": len(p.Rows), "possible_gap": true}); err != nil {
					return err
				}
			}
			if _, err = tx.Exec("INSERT INTO account_import_checks VALUES(?,CURRENT_TIMESTAMP,?) ON CONFLICT(account_id) DO UPDATE SET last_checked=excluded.last_checked,import_id=excluded.import_id", target.ID, p.ID); err != nil {
				return err
			}
			previews = append(previews, p)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return previews, nil
}
