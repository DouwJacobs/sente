package app

import (
	"database/sql"
	"encoding/json"
	"strings"
)

// Removed source occurrences remain private import evidence, never ledger entries.
// A merge aliases an occurrence to its survivor; deletion retains a tombstone.
func migrateTransactionRemovals(tx *sql.Tx, _ migrationOrigin) error {
	_, err := tx.Exec(`CREATE TABLE transaction_removals(
 id INTEGER PRIMARY KEY, original_id INTEGER NOT NULL,
 account_id INTEGER NOT NULL REFERENCES accounts(id),
 retained_id INTEGER REFERENCES transactions(id) ON DELETE SET NULL,
 occurrence_group INTEGER,
 fitid TEXT, source_date TEXT NOT NULL, source_amount INTEGER NOT NULL,
 source_description TEXT NOT NULL, source_key TEXT NOT NULL,
 provenance TEXT NOT NULL, import_id INTEGER REFERENCES imports(id),
 snapshot TEXT NOT NULL, reason TEXT NOT NULL, user_id INTEGER NOT NULL REFERENCES users(id),
 created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP);
 CREATE INDEX transaction_removal_source ON transaction_removals(account_id,source_key);
 CREATE INDEX transaction_removal_fitid ON transaction_removals(account_id,fitid);
 CREATE INDEX transaction_removal_original ON transaction_removals(original_id);`)
	return err
}

// Negative IDs are private tombstone handles, stripped from candidate output.
const transactionImportSources = `(SELECT id,account_id,fitid,source_date,source_amount,source_description,source_key,provenance,import_id FROM transactions
 UNION ALL SELECT COALESCE(retained_id,-COALESCE(occurrence_group,id)),account_id,fitid,source_date,source_amount,source_description,source_key,provenance,import_id FROM transaction_removals)`

func publicImportCandidate(m map[string]any) map[string]any {
	out := map[string]any{}
	for _, key := range []string{"id", "date", "amount_cents", "description"} {
		if value, ok := m[key]; ok {
			out[key] = value
		}
	}
	if num(out["id"]) < 0 {
		delete(out, "id")
		out["previously_removed"] = true
	}
	return out
}

type transactionRemovalSnapshot struct {
	ID          int64            `json:"id"`
	AccountID   int64            `json:"account_id"`
	Record      map[string]any   `json:"record"`
	Allocations []map[string]any `json:"allocations"`
	Tags        []map[string]any `json:"tags"`
}

func (a *App) transactionRemovalSnapshot(tx *sql.Tx, u User, item mcpSeenItem) (transactionRemovalSnapshot, error) {
	var out transactionRemovalSnapshot
	if item.ID <= 0 || item.Version <= 0 {
		return out, fail(400, "Provide positive transaction IDs and current versions")
	}
	// Reuse the shared current-account editor boundary, including hidden accounts.
	current, err := a.mcpTransaction(tx, u, item.ID)
	if err != nil {
		return out, err
	}
	if current.Version != item.Version {
		return out, fail(409, "Transaction changed; prepare a new proposal")
	}
	if queryInt(tx, "SELECT COUNT(*) FROM transfer_links WHERE left_id=? OR right_id=?", item.ID, item.ID) != 0 {
		return out, fail(400, "Unlink the transfer before removing a transaction")
	}
	rows, err := data(tx, "SELECT * FROM transactions WHERE id=?", item.ID)
	if err != nil {
		return out, err
	}
	out.ID, out.AccountID, out.Record = item.ID, num(rows[0]["account_id"]), rows[0]
	out.Allocations, err = data(tx, "SELECT * FROM allocations WHERE transaction_id=? ORDER BY id", item.ID)
	if err != nil {
		return out, err
	}
	out.Tags, err = data(tx, "SELECT tag_id FROM transaction_tags WHERE transaction_id=? ORDER BY tag_id", item.ID)
	return out, err
}

// Shared authorized service: callers supply exact snapshots and own one SQL
// transaction for authorization, archival, deletion, survivor version and audit.
func (a *App) removeTransactionsTx(tx *sql.Tx, u User, keep *transactionRemovalSnapshot, removed []transactionRemovalSnapshot, reason string) error {
	if err := validateTransactionRemovalSelection(tx, keep, removed, reason); err != nil {
		return err
	}
	all := append([]transactionRemovalSnapshot(nil), removed...)
	if keep != nil {
		all = append(all, *keep)
	}
	for _, expected := range all {
		actual, err := a.transactionRemovalSnapshot(tx, u, mcpSeenItem{expected.ID, num(expected.Record["version"])})
		if err != nil {
			return err
		}
		before, _ := json.Marshal(expected)
		now, _ := json.Marshal(actual)
		if string(before) != string(now) {
			return fail(409, "Transaction details changed; prepare a new proposal")
		}
	}
	var retained any
	if keep != nil {
		retained = keep.ID
	}
	for _, snapshot := range removed {
		raw, err := json.Marshal(snapshot)
		if err != nil {
			return err
		}
		result, err := tx.Exec(`INSERT INTO transaction_removals(original_id,account_id,retained_id,fitid,source_date,source_amount,source_description,source_key,provenance,import_id,snapshot,reason,user_id)
 SELECT id,account_id,?,fitid,source_date,source_amount,source_description,source_key,provenance,import_id,?,?,? FROM transactions WHERE id=?`, retained, string(raw), reason, u.ID, snapshot.ID)
		if err != nil {
			return err
		}
		// Preserve aliases from an earlier merge when its survivor is removed.
		// All aliases still represent ONE occurrence after deleting that survivor.
		var group any
		if keep == nil {
			group, err = result.LastInsertId()
			if err != nil {
				return err
			}
		}
		if _, err = tx.Exec("UPDATE transaction_removals SET retained_id=?,occurrence_group=? WHERE retained_id=?", retained, group, snapshot.ID); err != nil {
			return err
		}
		if _, err = tx.Exec("DELETE FROM transactions WHERE id=?", snapshot.ID); err != nil {
			return err
		}
		if err = audit(tx, u, snapshot.AccountID, "transaction", snapshot.ID, "removed", map[string]any{"before": snapshot, "retained_id": retained, "reason": reason}); err != nil {
			return err
		}
	}
	if keep != nil {
		_, err := tx.Exec("UPDATE transactions SET version=version+1 WHERE id=?", keep.ID)
		return err
	}
	return nil
}

func validateTransactionRemovalSelection(q queryer, keep *transactionRemovalSnapshot, removed []transactionRemovalSnapshot, reason string) error {
	if len(removed) < 1 || len(removed) > 100 || strings.TrimSpace(reason) == "" || len(reason) > 500 {
		return fail(400, "Select 1–100 transactions and provide a reason of at most 500 bytes")
	}
	seen := map[int64]bool{}
	if keep != nil {
		seen[keep.ID] = true
	}
	for _, snapshot := range removed {
		if snapshot.ID <= 0 || seen[snapshot.ID] {
			return fail(400, "Each transaction can appear only once, including the keeper")
		}
		seen[snapshot.ID] = true
		if keep == nil {
			continue
		}
		k, r := keep.Record, snapshot.Record
		if keep.AccountID != snapshot.AccountID || num(k["amount_cents"]) != num(r["amount_cents"]) || num(k["source_amount"]) != num(r["source_amount"]) || k["source_date"] != r["source_date"] || normalize(k["source_description"].(string)) != normalize(r["source_description"].(string)) || num(k["is_transfer"]) != num(r["is_transfer"]) {
			return fail(400, "Duplicates must have the same account, amount, original date and description, and transfer status")
		}
		if queryInt(q, "SELECT COUNT(*) FROM transactions WHERE id IN (?,?) AND COALESCE(json_extract(provenance,'$.source_service_fee_cents'),0)<>0", keep.ID, snapshot.ID) != 0 {
			return fail(400, "Fee-bearing duplicates require explicit deletion of the duplicate principal and its fee together")
		}
	}
	return nil
}
