package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestSplitGroupsFinancialScopeAndPreservation(t *testing.T) {
	e := setup(t)
	p := e.stage(t, "split-groups.ofx", ofx(ofxRow("split-groups", "20261021", "-500.00", "Split groups")))
	status(t, e.commit(t, p.ID, nil, true), 200)
	id := queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='split-groups'")
	path := fmt.Sprintf("/api/transactions/%d", id)
	body := map[string]any{"version": 1, "date": "2026-10-21", "amount_cents": -50000, "description": "Split groups", "assignment": "auto", "spending_group_id": 1, "allocations": []map[string]any{
		{"category_id": 1, "amount_cents": -30000, "note": "keep", "spending_group_id": 2},
		{"category_id": 1, "amount_cents": -12500, "spending_group_id": 0},
		{"category_id": 2, "amount_cents": -7500},
	}}
	status(t, e.req(t, 3, path, "PUT", body), 403)
	status(t, e.req(t, 1, path, "PUT", body), 200)
	status(t, e.req(t, 1, path, "PUT", body), 409)
	body["version"] = 2
	body["allocations"].([]map[string]any)[0]["spending_group_id"] = 999999
	status(t, e.req(t, 1, path, "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 2 {
		t.Fatal("invalid group mutated transaction")
	}
	body["allocations"].([]map[string]any)[0]["spending_group_id"] = 2
	read := func(path string) map[string]any {
		w := e.req(t, 1, path, "GET", nil)
		status(t, w, 200)
		var out map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	d := read("/api/dashboard?period=1")
	if num(d["spent_cents"]) != 42500 {
		t.Fatalf("expense split total: %v", d)
	}
	got := map[int64]int64{}
	for _, raw := range d["spending_breakdown"].([]any) {
		row := raw.(map[string]any)
		got[num(row["group_id"])] += num(row["spent_cents"])
	}
	if got[2] != 30000 || got[0] != 12500 || got[1] != 0 {
		t.Fatal(got)
	}
	for _, check := range []struct {
		query          string
		total, matched int64
	}{
		{"spending_group=2&category=1", 1, 30000},
		{"spending_group=unassigned&category=1", 1, 12500},
		{"spending_group=1&category=1", 0, 0},
	} {
		out := read("/api/transactions?period=1&dashboard_spending=1&" + check.query)
		if num(out["total"]) != check.total {
			t.Fatalf("%s: %v", check.query, out)
		}
		if check.total > 0 && num(out["items"].([]any)[0].(map[string]any)["matched_spend_cents"]) != check.matched {
			t.Fatal(out)
		}
	}
	for _, check := range []struct {
		group string
		spent int64
	}{{"2", 30000}, {"0", 12500}, {"1", 0}} {
		report := read("/api/budget/reports?period=1&spending_group=" + check.group + "&category=1")
		if num(report["spent_cents"]) != check.spent {
			t.Fatalf("report %s: %v", check.group, report)
		}
	}
	// Unrelated shared browser/MCP/bulk edits preserve overrides in their exact snapshots.
	tx, err := e.a.DB.Begin()
	if err != nil {
		t.Fatal(err)
	}
	before, err := e.a.mcpTransaction(tx, User{ID: 1, Admin: true, Member: true}, id)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if before.Allocations[0].SpendingGroupID == nil || *before.Allocations[0].SpendingGroupID != 2 || before.Allocations[1].SpendingGroupID == nil || *before.Allocations[1].SpendingGroupID != 0 || before.Allocations[2].SpendingGroupID != nil {
		tx.Rollback()
		t.Fatal(before.Allocations)
	}
	before.Description = "Changed description"
	_, err = e.a.editTransactionTx(tx, User{ID: 1, Admin: true, Member: true}, id, before)
	if err != nil {
		tx.Rollback()
		t.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM allocations WHERE transaction_id=?", id) != -50000 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND group_override=1", id) != 2 {
		t.Fatal("money or group overrides lost")
	}
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
}

func TestSplitGroupMigrationRollbackAndRetry(t *testing.T) {
	db := migrationDB(t)
	if err := runSchemaMigrations(db, schemaMigrations[:12], 34); err != nil {
		t.Fatal(err)
	}
	migrationExec(t, db, `INSERT INTO users(id,username,password,admin) VALUES(1,'synthetic','not a credential',1);
 INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Synthetic','synthetic',1);
 INSERT INTO spending_groups(id,name,color) VALUES(1,'Test','slate');
 INSERT INTO grants VALUES(1,1,'editor');
 INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,spending_group_id,version) VALUES(1,1,'2026-10-21',-50000,'Split','2026-10-21',-50000,'Split','{}',1,7);
 INSERT INTO allocations(transaction_id,amount_cents,note) VALUES(1,-30000,'Keep'),(1,-20000,'');`)
	queries := []string{"SELECT * FROM transactions", "SELECT id,transaction_id,category_id,amount_cents,note FROM allocations", "SELECT * FROM grants"}
	before := migrationSnapshot(t, db, queries...)
	steps := append([]schemaMigration{}, schemaMigrations[:13]...)
	original := steps[len(steps)-1].apply
	steps[len(steps)-1].apply = func(tx *sql.Tx, origin migrationOrigin) error {
		if err := original(tx, origin); err != nil {
			return err
		}
		return errors.New("injected split migration failure")
	}
	if err := runSchemaMigrations(db, steps, 35); err == nil || !strings.Contains(err.Error(), "injected split") {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT COUNT(*) FROM pragma_table_info('allocations') WHERE name='group_override'") != 0 || queryInt(db, "SELECT MAX(version) FROM migrations") != 34 {
		t.Fatal("failed migration leaked")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if migrationSnapshot(t, db, queries...) != before {
		t.Fatal("upgrade changed money/history/access")
	}
	if queryInt(db, "SELECT COUNT(*) FROM allocations WHERE group_override=0") != 2 {
		t.Fatal("legacy group inheritance lost")
	}
	if err := migrate(db); err != nil {
		t.Fatal(err)
	}
	if queryInt(db, "SELECT version FROM transactions WHERE id=1") != 7 {
		t.Fatal("restart rewrote financial version")
	}
}
