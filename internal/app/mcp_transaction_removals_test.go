package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func seedRemovalDuplicates(t *testing.T, e *testEnv) {
	t.Helper()
	seedMCPTransactions(t, e)
	for id, ref := range map[int]string{1: "initial-reference", 2: "cleared-reference"} {
		provenance, _ := json.Marshal(map[string]any{"format": "fnb-live", "source_component": "transaction", "source_bank_description": "Synthetic salary", "source_reference": ref})
		_, err := e.a.DB.Exec("UPDATE transactions SET date='2026-10-21',amount_cents=10001,description='Synthetic salary',source_date='2026-10-21',source_amount=10001,source_description='Synthetic salary',source_key=?,provenance=? WHERE id=?", fingerprint(SourceRow{Date: "2026-10-21", Amount: 10001, Description: "Synthetic salary"}), string(provenance), id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.a.DB.Exec("UPDATE allocations SET amount_cents=10001 WHERE transaction_id=?", id); err != nil {
			t.Fatal(err)
		}
	}
}

func removalChange(merge bool, ids ...int64) map[string]any {
	items := []map[string]any{}
	for _, id := range ids {
		items = append(items, map[string]any{"id": id, "version": 1})
	}
	input := map[string]any{"items": items, "reason": "Confirmed synthetic duplicate"}
	operation := "delete_transactions"
	if merge {
		operation = "merge_transactions"
		input["keep"] = map[string]any{"id": 1, "version": 1}
	}
	return map[string]any{"operation": operation, "removal": input}
}

func TestMCPRemovalMergeExactConsentAuditAndReplay(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	migrationExec(t, e.a.DB, `UPDATE allocations SET amount_cents=6001 WHERE transaction_id=1;
 INSERT INTO allocations(transaction_id,amount_cents,note,group_override) VALUES(1,4000,'Second split',1);
 INSERT INTO tags(id,account_id,name) VALUES(91,1,'Synthetic keep'); INSERT INTO transaction_tags VALUES(1,91);`)
	token := mcpToken(t, e, 1, true)
	change := removalChange(true, 2)
	if v, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
		t.Fatal("legacy grant allowed removal", v)
	}
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{MergeTransactions: true}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, change)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals") != 0 {
		t.Fatal("preview wrote data")
	}
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("pending removal applied", v)
	}
	mcpApprove(t, e, 1, id)
	result, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id})
	if failed || num(result["removed"]) != 1 || num(result["retained_id"]) != 1 || num(result["version"]) != 2 {
		t.Fatal(result)
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions WHERE account_id=1") != 10001 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM allocations WHERE transaction_id=1") != 10001 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals WHERE retained_id=1") != 1 {
		t.Fatal("merge totals or source retention wrong")
	}
	var note string
	if err := e.a.DB.QueryRow("SELECT note FROM allocations WHERE transaction_id=1").Scan(&note); err != nil || note != "secret note" {
		t.Fatal("keeper metadata changed", err)
	}
	encoded, _ := json.Marshal(result)
	if strings.Contains(string(encoded), "secret") || strings.Contains(string(encoded), "reference") {
		t.Fatal("private evidence leaked", result)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='transaction' AND entity_id=2 AND action='removed'") != 1 {
		t.Fatal("missing removal audit")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=1") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=1 AND group_override=1 AND spending_group_id IS NULL") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_tags WHERE transaction_id=1 AND tag_id=91") != 1 {
		t.Fatal("keeper split groups or tags changed")
	}
	if replay, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed || num(replay["removed"]) != 1 {
		t.Fatal("replay failed", replay)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals") != 1 {
		t.Fatal("replay removed again")
	}
	// Each historical bank reference resolves to the same survivor, once per page.
	for _, ref := range []string{"initial-reference", "cleared-reference"} {
		file := ParsedFile{Format: "fnb-live", Rows: []SourceRow{{Date: "2026-10-21", Amount: 10001, Description: "Synthetic salary", SourceReference: ref, SourceBankDescription: "Synthetic salary", SourceComponent: "transaction"}}}
		if err := e.a.annotate(e.a.DB, 1, &file); err != nil || file.Rows[0].Duplicate != "exact_source" {
			t.Fatal("merged reference re-imported", err, file)
		}
	}
	// Current account/consent still gates an already applied result.
	p.Constraints.AccountIDs = []int64{2}
	setTestMCPPermissions(t, e, token, p)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("replay bypassed scope", v)
	}
}

func TestMCPRemovalStaleScopeValidationAndAtomicRollback(t *testing.T) {
	for _, kind := range []string{"version", "metadata", "scope", "consent", "audit"} {
		t.Run(kind, func(t *testing.T) {
			e := setup(t)
			seedRemovalDuplicates(t, e)
			token := mcpToken(t, e, 1, true)
			p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{MergeTransactions: true}}
			setTestMCPPermissions(t, e, token, p)
			id := mcpPrepare(t, e, token, removalChange(true, 2))
			mcpApprove(t, e, 1, id)
			switch kind {
			case "version":
				migrationExec(t, e.a.DB, "UPDATE transactions SET version=2 WHERE id=1")
			case "metadata":
				migrationExec(t, e.a.DB, "UPDATE allocations SET note='changed' WHERE transaction_id=2")
			case "scope":
				p.Constraints.AccountIDs = []int64{2}
				setTestMCPPermissions(t, e, token, p)
			case "consent":
				p.Capabilities.MergeTransactions = false
				setTestMCPPermissions(t, e, token, p)
			case "audit":
				migrationExec(t, e.a.DB, "CREATE TRIGGER reject_removal_audit BEFORE INSERT ON audit WHEN NEW.action='removed' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
			}
			if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
				t.Fatal("invalid removal applied", v)
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals") != 0 {
				t.Fatal("partial removal")
			}
			if kind == "audit" {
				migrationExec(t, e.a.DB, "DROP TRIGGER reject_removal_audit")
				if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
					t.Fatal("retry failed", v)
				}
			}
		})
	}
}

func TestMCPRemovalDeleteTombstonesFITIDMultiplicityAndIDReuse(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{DeleteTransactions: true}, Automatic: mcpCapabilities{DeleteTransactions: true}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, removalChange(false, 2, 3))
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals WHERE retained_id IS NULL") != 2 {
		t.Fatal("delete failed")
	}
	file := ParsedFile{Format: "ofx", Rows: []SourceRow{{Date: "2026-10-23", Amount: -303, Description: "Private merchant", FITID: "private-account-ref"}}}
	if err := e.a.annotate(e.a.DB, 2, &file); err != nil || file.Rows[0].Duplicate != "exact_id" {
		t.Fatal("deleted FITID recreated", err, file)
	}
	if _, ok := file.Rows[0].Candidates[0]["id"]; ok {
		t.Fatal("tombstone exposed as ledger ID")
	}
	// A tombstone suppresses one occurrence, not all future identical purchases.
	r := SourceRow{Date: "2026-10-21", Amount: 10001, Description: "Synthetic salary", SourceReference: "cleared-reference", SourceBankDescription: "Synthetic salary", SourceComponent: "transaction"}
	live := ParsedFile{Format: "fnb-live", Rows: []SourceRow{r, r}}
	if err := e.a.annotate(e.a.DB, 1, &live); err != nil || live.Rows[0].Duplicate != "exact_source" || live.Rows[1].Duplicate != "possible" {
		t.Fatal("multiplicity or ambiguity lost", err, live)
	}
	// New ledger IDs must never reuse a removed ID (history/notifications reference IDs).
	staged := e.stage(t, "fresh.ofx", ofx(ofxRow("fresh-id", "20261024", "1.00", "Fresh synthetic entry")))
	status(t, e.commit(t, staged.ID, nil, true), 200)
	if queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='fresh-id'") <= 3 {
		t.Fatal("removed transaction ID reused")
	}
}

func TestTransactionRemovalMigrationRollbackRetryAndRestart(t *testing.T) {
	db := migrationDB(t)
	previous := schemaMigrations[:len(schemaMigrations)-1]
	if err := runSchemaMigrations(db, previous, 35); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password,admin) VALUES(1,'synthetic','not a credential',1);
 INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Synthetic','synthetic',0);
 INSERT INTO grants VALUES(1,1,'editor');
 INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,version) VALUES(1,1,'2026-10-21',10001,'Synthetic salary','2026-10-21',10001,'Synthetic salary','{}',7);
 INSERT INTO allocations(transaction_id,amount_cents,note,group_override) VALUES(1,10001,'Keep',1);`)
	queries := []string{"SELECT * FROM users ORDER BY id", "SELECT * FROM transactions ORDER BY id", "SELECT * FROM mcp_tokens ORDER BY id"}
	before := migrationSnapshot(t, db, queries...)
	steps := append([]schemaMigration(nil), schemaMigrations...)
	original := steps[len(steps)-1].apply
	steps[len(steps)-1].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := original(tx, origin); err != nil {
			return err
		}
		return errors.New("injected removal migration failure")
	}
	if err := runSchemaMigrations(db, steps, 36); err == nil || !strings.Contains(err.Error(), "injected removal") {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM sqlite_master WHERE name='transaction_removals'") != 0 || queryInt(db, "SELECT MAX(version) FROM migrations") != 35 {
		t.Fatal("failed migration leaked")
	}
	for i := 0; i < 2; i++ {
		if err := migrate(db); err != nil {
			t.Fatal(err)
		}
	}
	if migrationSnapshot(t, db, queries...) != before || queryInt(db, "SELECT COUNT(*) FROM transaction_removals") != 0 {
		t.Fatal("migration changed financial/access state")
	}
}

func TestMCPRemovalRejectsInvalidSelectionsAndRevokedAutomaticConsent(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{MergeTransactions: true, DeleteTransactions: true}, Automatic: mcpCapabilities{MergeTransactions: true, DeleteTransactions: true}}
	setTestMCPPermissions(t, e, token, p)
	bad := []map[string]any{removalChange(true, 1), removalChange(true, 2, 2), removalChange(true, 3), removalChange(false), removalChange(false, 9999)}
	for _, change := range bad {
		if v, failed := mcpCall(t, e, token, "prepare_change", change); !failed {
			t.Fatal("invalid selection accepted", v, change)
		}
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET amount_cents=10002 WHERE id=2")
	if v, failed := mcpCall(t, e, token, "prepare_change", removalChange(true, 2)); !failed {
		t.Fatal("different amounts merged", v)
	}
	migrationExec(t, e.a.DB, "UPDATE transactions SET amount_cents=10001 WHERE id=2; INSERT INTO transfer_links(left_id,right_id) VALUES(1,2)")
	if v, failed := mcpCall(t, e, token, "prepare_change", removalChange(false, 2)); !failed {
		t.Fatal("linked transfer removed", v)
	}
	migrationExec(t, e.a.DB, "DELETE FROM transfer_links")
	id := mcpPrepare(t, e, token, removalChange(true, 2))
	p.Automatic = mcpCapabilities{}
	setTestMCPPermissions(t, e, token, p)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); !failed {
		t.Fatal("revoked automatic consent ignored", v)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 3 {
		t.Fatal("validation mutated ledger")
	}
}

func TestMCPRemovalDeletingMergedSurvivorRetainsOneOccurrence(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	token := mcpToken(t, e, 1, true)
	p := mcpPermissions{SchemaVersion: 1, Preset: "custom", Capabilities: mcpCapabilities{MergeTransactions: true, DeleteTransactions: true}, Automatic: mcpCapabilities{MergeTransactions: true, DeleteTransactions: true}}
	setTestMCPPermissions(t, e, token, p)
	id := mcpPrepare(t, e, token, removalChange(true, 2))
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	change := removalChange(false, 1)
	change["removal"].(map[string]any)["items"].([]map[string]any)[0]["version"] = 2
	id = mcpPrepare(t, e, token, change)
	if v, failed := mcpCall(t, e, token, "apply_change", map[string]any{"proposal_id": id}); failed {
		t.Fatal(v)
	}
	r := SourceRow{Date: "2026-10-21", Amount: 10001, Description: "Synthetic salary", SourceReference: "cleared-reference", SourceBankDescription: "Synthetic salary", SourceComponent: "transaction"}
	file := ParsedFile{Format: "fnb-live", Rows: []SourceRow{r, r}}
	if err := e.a.annotate(e.a.DB, 1, &file); err != nil || file.Rows[0].Duplicate != "exact_source" || file.Rows[1].Duplicate != "" {
		t.Fatal("aliases suppressed more than one occurrence", err, file)
	}
}
