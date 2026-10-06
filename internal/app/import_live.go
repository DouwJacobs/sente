package app

import (
	"database/sql"
	"encoding/json"
)

// Match occurrences, never just the presence of a fingerprint: two identical
// purchases in one bank page remain two entries. Immutable source fields survive
// user edits. A bank row position and a reference alone are not transaction IDs.
func (a *App) annotateLiveDuplicates(q queryer, account int64, p *ParsedFile) error {
	used := map[int64]bool{}
	combined := map[int]int64{}
	for i := range p.Rows {
		row := &p.Rows[i]
		if row.Error != "" || row.Duplicate == "conflict" || row.Duplicate == "exact_id" {
			continue
		}
		row.Duplicate, row.Candidates = "", nil
		if id := combined[row.SourceBankRow]; id != 0 && row.SourceComponent == "service_fee" {
			row.Duplicate = "exact_source"
			row.Candidates = []map[string]any{{"id": id}}
			continue
		}
		netKey := row.SourceNetKey
		if netKey == "" {
			netKey = fingerprint(*row)
		}
		matches, err := data(q, "SELECT id,source_date date,source_amount amount_cents,source_description description,source_key,provenance,import_id FROM transactions WHERE account_id=? AND (source_key=? OR source_key=?) ORDER BY CASE WHEN source_key=? THEN 0 ELSE 1 END,id", account, fingerprint(*row), netKey, fingerprint(*row))
		if err != nil {
			return err
		}
		for _, m := range matches {
			id := num(m["id"])
			if used[id] {
				continue
			}
			var original struct {
				BankRow         int    `json:"source_bank_row"`
				Format          string `json:"format"`
				Reference       string `json:"source_reference"`
				Balance         *int64 `json:"balance_cents"`
				BankDescription string `json:"source_bank_description"`
				Component       string `json:"source_component"`
				Fee             *int64 `json:"source_service_fee_cents"`
			}
			if err := json.Unmarshal([]byte(m["provenance"].(string)), &original); err != nil {
				return err
			}
			// Older live imports did not copy the parent running balance onto
			// fees. Recover it from immutable provenance without rewriting data.
			if original.Format == "fnb-live" && original.Component == "service_fee" && original.Balance == nil && original.BankRow > 0 && m["import_id"] != nil {
				var balance sql.NullInt64
				err := q.QueryRow("SELECT json_extract(provenance,'$.balance_cents') FROM transactions WHERE account_id=? AND import_id=? AND json_extract(provenance,'$.source_component')='transaction' AND json_extract(provenance,'$.source_bank_row')=? LIMIT 1", account, m["import_id"], original.BankRow).Scan(&balance)
				if err != nil && err != sql.ErrNoRows {
					return err
				}
				if balance.Valid {
					original.Balance = &balance.Int64
				}
			}
			net := m["source_key"] != fingerprint(*row)
			if !net {
				if row.Balance != nil && original.Balance != nil && *row.Balance != *original.Balance {
					continue
				}
				if original.Format == "fnb-live" {
					if original.Reference != row.SourceReference || normalize(original.BankDescription) != normalize(row.SourceBankDescription) || original.Component != row.SourceComponent {
						continue
					}
					if (original.Fee == nil) != (row.SourceServiceFee == nil) || original.Fee != nil && *original.Fee != *row.SourceServiceFee {
						continue
					}
				}
			} else {
				// A combined export consumes both principal and fee once. Another live
				// principal is not evidence that an exported combined debit already exists.
				if original.Format == "fnb-live" || row.SourceComponent != "transaction" {
					continue
				}
				combined[row.SourceBankRow] = id
			}
			used[id] = true
			row.Duplicate = "exact_source"
			delete(m, "provenance")
			delete(m, "source_key")
			delete(m, "import_id")
			row.Candidates = []map[string]any{m}
			break
		}
	}
	return nil
}

// Importing and rule application do not depend on an open browser UI. Bad data
// and conflicting bank IDs remain in Import activity, outside category review.
func (a *App) autoCommitFNB(u User, previews []ParsedFile) error {
	return a.write(func(tx *sql.Tx) error {
		var active bool
		if err := tx.QueryRow("SELECT admin=1 AND disabled=0,budget_member FROM users WHERE id=?", u.ID).Scan(&active, &u.Member); err != nil || !active {
			return fail(403, "Connection access revoked")
		}
		targets, err := a.fnbTargets(tx, u)
		if err != nil {
			return err
		}
		allowed := map[int64]string{}
		for _, target := range targets {
			allowed[target.ID] = target.BankID
		}
		for i := range previews {
			p := &previews[i]
			account := queryInt(tx, "SELECT account_id FROM imports WHERE id=?", p.ID)
			if allowed[account] != p.AccountID {
				return fail(403, "Account access or visibility changed")
			}
			if err := a.annotate(tx, account, p); err != nil {
				return err
			}
			if p.AlreadyImported {
				p.Inserted, p.Skipped = 0, 0
				continue
			}
			blocked := p.Error != ""
			for _, row := range p.Rows {
				if row.Error != "" || row.Duplicate == "conflict" || row.Duplicate == "possible" {
					blocked = true
				}
			}
			if blocked {
				continue
			}
			inserted, skipped := 0, 0
			if err := a.commitStagedImport(tx, u, p.ID, importCommitOptions{ClassificationVersion: p.ClassificationVersion}, &inserted, &skipped); err != nil {
				return err
			}
			p.Inserted, p.Skipped, p.AlreadyImported = inserted, skipped, true
		}
		return nil
	})
}
