package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"
)

func workflowJSON(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var v map[string]any
	if e := json.Unmarshal(b, &v); e != nil {
		t.Fatal(e, string(b))
	}
	return v
}
func TestCoreBulkAtomicSplitPreservationAndPrivacy(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -1000, "2026-10-21", nil)
	private := seedTransaction(t, e, 2, -2000, "2026-10-22", nil)
	workflowExec(t, e, "DELETE FROM allocations WHERE transaction_id=?", id)
	workflowExec(t, e, "INSERT INTO allocations(transaction_id,category_id,amount_cents,note) VALUES(?,1,-400,'keep'),(?,NULL,-600,'missing')", id, id)
	request := map[string]any{"items": []map[string]any{{"id": id, "version": 1}}, "operation": "category", "category_id": 2, "mode": "missing"}
	preview := e.req(t, 1, "/api/transactions/bulk/preview", "POST", request)
	status(t, preview, 200)
	token := workflowJSON(t, preview.Body.Bytes())["token"]
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id IS NULL", id) != 1 {
		t.Fatal("preview wrote ledger")
	}
	result := e.req(t, 1, "/api/transactions/bulk/apply", "POST", map[string]any{"token": token})
	status(t, result, 200)
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM allocations WHERE transaction_id=?", id) != -1000 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=? AND category_id=1 AND note='keep'", id) != 1 {
		t.Fatal("split or money changed")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_seen WHERE transaction_id=? AND user_id=1 AND transaction_version=2", id) != 1 {
		t.Fatal("manual bulk not seen")
	}
	status(t, e.req(t, 1, "/api/transactions/bulk/apply", "POST", map[string]any{"token": token}), 409)
	request["items"] = []map[string]any{{"id": id, "version": 2}, {"id": private, "version": 1}}
	status(t, e.req(t, 3, "/api/transactions/bulk/preview", "POST", request), 403)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 2 {
		t.Fatal("failed batch changed record")
	}
}
func TestCoreNavigationOriginalAnchorAndProcessed(t *testing.T) {
	e := setup(t)
	a := seedTransaction(t, e, 1, -100, "2026-10-25", nil)
	b := seedTransaction(t, e, 1, -100, "2026-10-24", nil)
	c := seedTransaction(t, e, 1, -100, "2026-10-23", nil)
	seedTransaction(t, e, 2, -100, "2026-10-24", nil)
	workflowExec(t, e, "UPDATE transactions SET date='2026-10-01' WHERE id=?", a)
	w := e.req(t, 1, fmt.Sprintf("/api/transactions/navigation?account=1&anchor_id=%d&anchor_date=2026-10-25&processed=%d", a, b), "GET", nil)
	status(t, w, 200)
	v := workflowJSON(t, w.Body.Bytes())
	if num(v["next"].(map[string]any)["id"]) != c {
		t.Fatal(v)
	}
	status(t, e.req(t, 3, fmt.Sprintf("/api/transactions/navigation?anchor_id=%d&anchor_date=2026-10-24", queryInt(e.a.DB, "SELECT MAX(id) FROM transactions")), "GET", nil), 404)
}
func TestCoreRebalanceRechecksSpendingAndPreservesTotals(t *testing.T) {
	e := setup(t)
	workflowExec(t, e, "INSERT INTO categories(id,name,group_name,kind)VALUES(3,'Other','','expense')")
	workflowExec(t, e, "INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward)VALUES(1,1,1000,0),(1,3,2000,0)")
	body := map[string]any{"version": 1, "from_group": 0, "from_category": 1, "to_group": 0, "to_category": 3, "amount_cents": 500}
	w := e.req(t, 1, "/api/periods/1/rebalance/preview", "POST", body)
	status(t, w, 200)
	token := workflowJSON(t, w.Body.Bytes())["token"]
	seedTransaction(t, e, 1, -100, "2026-10-22", workflowPtr(int64(1)))
	status(t, e.req(t, 1, "/api/periods/1/rebalance/apply", "POST", map[string]any{"token": token}), 409)
	w = e.req(t, 1, "/api/periods/1/rebalance/preview", "POST", body)
	status(t, w, 200)
	status(t, e.req(t, 1, "/api/periods/1/rebalance/apply", "POST", map[string]any{"token": workflowJSON(t, w.Body.Bytes())["token"]}), 200)
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM group_targets WHERE period_id=1") != 3000 || queryInt(e.a.DB, "SELECT SUM(carry_forward) FROM group_targets WHERE period_id=1") != 0 {
		t.Fatal("rebalance changed totals or carry-forward")
	}
	status(t, e.req(t, 3, "/api/periods/1/rebalance/preview", "POST", body), 403)
}
func TestCoreReportRefundsExportsAndMetadataIsolation(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -1000, "2026-10-21", workflowPtr(int64(1)))
	seedTransaction(t, e, 1, 200, "2026-10-22", workflowPtr(int64(1)))
	seedTransaction(t, e, 2, -9000, "2026-10-22", workflowPtr(int64(1)))
	workflowExec(t, e, "UPDATE transactions SET description='=SUM(A1)',note='private note' WHERE id=?", id)
	w := e.req(t, 1, "/api/budget/reports?period=1&count=1", "GET", nil)
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["spent_cents"]) != 800 {
		t.Fatal(w.Body.String())
	}
	w = e.req(t, 3, "/api/transactions/export", "GET", nil)
	status(t, w, 200)
	z, err := zip.NewReader(bytes.NewReader(w.Body.Bytes()), int64(w.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range z.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		r.Close()
		if strings.Contains(string(b), "Private") || strings.Contains(string(b), "22222222222") {
			t.Fatal("private data exported")
		}
		if f.Name == "transactions.csv" && !strings.Contains(string(b), "'=SUM(A1)") {
			t.Fatal("formula not escaped", string(b))
		}
	}
	safe := mcpSafe(map[string]any{"id": id, "note": "secret", "merchant_id": 12, "tags": []any{"secret"}}).(map[string]any)
	if len(safe) != 1 {
		t.Fatal("metadata leaked to MCP", safe)
	}
	label := e.req(t, 1, "/api/labels", "POST", map[string]any{"account_id": 1, "kind": "tag", "name": "Synthetic label"})
	status(t, label, 200)
	status(t, e.req(t, 1, "/api/labels?kind=tag&page=0&q=Syn", "GET", nil), 200)
	status(t, e.req(t, 1, "/api/merchant-rules?page=0&q=Market", "GET", nil), 200)
	status(t, e.req(t, 3, "/api/labels", "POST", map[string]any{"account_id": 1, "kind": "tag", "name": "Test"}), 403)
}
func TestCoreMerchantRulesConflictsAndArchiveGuards(t *testing.T) {
	e := setup(t)
	workflowExec(t, e, "INSERT INTO merchants(id,account_id,name)VALUES(1,1,'Market'),(2,1,'Other')")
	workflowExec(t, e, "INSERT INTO merchant_rules(account_id,merchant_id,pattern,direction,priority)VALUES(1,1,'market','any',5),(1,2,'MARKET','any',5)")
	m, err := matchMerchant(e.a.DB, 1, "Market purchase", -100)
	if err != nil || m != 0 {
		t.Fatal("conflict picked a merchant")
	}
	workflowExec(t, e, "UPDATE merchant_rules SET priority=6 WHERE merchant_id=1")
	m, err = matchMerchant(e.a.DB, 1, " market   purchase ", -100)
	if err != nil || m != 1 {
		t.Fatal("normalised priority match failed")
	}
	workflowExec(t, e, "INSERT INTO rules(user_id,account_id,pattern,category_id)VALUES(1,1,'shop',1)")
	b := map[string]any{"name": "Groceries", "archived": true, "version": 1}
	status(t, e.req(t, 1, "/api/categories/1", "PUT", b), 409)
	workflowExec(t, e, "UPDATE rules SET enabled=0 WHERE category_id=1")
	status(t, e.req(t, 1, "/api/categories/1", "PUT", b), 200)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	body := editBody(1, -100, []Allocation{{CategoryID: workflowPtr(int64(1)), Amount: -100}})
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 400)
}

func workflowPtr(v int64) *int64 { return &v }

func TestCoreMetadataScopedUpdatesLegacyPreservationAndLimits(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	w := e.req(t, 1, "/api/labels", "POST", map[string]any{"account_id": 1, "kind": "merchant", "name": "Épicerie"})
	status(t, w, 200)
	merchant := num(workflowJSON(t, w.Body.Bytes())["id"])
	w = e.req(t, 1, "/api/labels", "POST", map[string]any{"account_id": 1, "kind": "merchant", "name": "ÉPICERIE"})
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["id"]) != merchant {
		t.Fatal("Unicode duplicate name")
	}
	w = e.req(t, 1, "/api/labels", "POST", map[string]any{"account_id": 1, "kind": "tag", "name": "Weekly"})
	status(t, w, 200)
	tag := num(workflowJSON(t, w.Body.Bytes())["id"])
	body := editBody(1, -100, []Allocation{{Amount: -100}})
	body["note"] = "Whole transaction note"
	body["merchant_id"] = merchant
	body["tag_ids"] = []int64{tag}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	body = editBody(2, -100, []Allocation{{Amount: -100}})
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT merchant_id FROM transactions WHERE id=?", id) != merchant || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_tags WHERE transaction_id=?", id) != 1 {
		t.Fatal("omitted metadata lost")
	}
	body["version"] = 3
	body["note"] = strings.Repeat("x", 2001)
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 400)
	private := e.req(t, 1, "/api/labels", "POST", map[string]any{"account_id": 2, "kind": "tag", "name": "Private tag"})
	status(t, private, 200)
	body["note"] = "ok"
	body["tag_ids"] = []int64{num(workflowJSON(t, private.Body.Bytes())["id"])}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 400)
	w = e.req(t, 3, "/api/labels?kind=tag&page=0", "GET", nil)
	status(t, w, 200)
	if strings.Contains(w.Body.String(), "Private tag") {
		t.Fatal("catalogue privacy leak")
	}
	w = e.req(t, 1, fmt.Sprintf("/api/transactions?merchant=%d&tag=%d", merchant, tag), "GET", nil)
	status(t, w, 200)
	if num(workflowJSON(t, w.Body.Bytes())["total"]) != 1 {
		t.Fatal("label filters lost transaction")
	}
}
func TestCoreBulkChangedTransferMeaningRejected(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	group := queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Day-to-day'")
	w := e.req(t, 1, "/api/transactions/bulk/preview", "POST", map[string]any{"operation": "group", "group_id": group, "items": []map[string]any{{"id": id, "version": 1}}})
	status(t, w, 200)
	token := workflowJSON(t, w.Body.Bytes())["token"]
	workflowExec(t, e, "UPDATE spending_groups SET name=' TRANSFER ' WHERE id=?", group)
	status(t, e.req(t, 1, "/api/transactions/bulk/apply", "POST", map[string]any{"token": token}), 409)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 1 {
		t.Fatal("stale preview changed ledger")
	}
}
func TestCoreZeroNewCheckFreshnessAndHealthPrivacy(t *testing.T) {
	e := setup(t)
	workflowExec(t, e, "INSERT INTO imports(id,user_id,account_id,name,format,hash,data,status,created_at,committed_at)VALUES(900,1,1,'Synthetic','fnb-live','synthetic','{}','committed','2026-01-01 00:00:00','2026-01-01 00:01:00')")
	workflowExec(t, e, "INSERT INTO account_import_checks VALUES(1,'2026-10-05 10:00:00',900)")
	w := e.req(t, 3, "/api/accounts/health?page=0", "GET", nil)
	status(t, w, 200)
	v := workflowJSON(t, w.Body.Bytes())
	if num(v["total"]) != 1 {
		t.Fatal("health private accounts leaked")
	}
	row := v["items"].([]any)[0].(map[string]any)
	if row["last_imported"] != "2026-10-05 10:00:00" || row["state"] != "ready" || row["possible_gap"] != true {
		t.Fatal(row)
	}
}

func workflowExec(t *testing.T, e *testEnv, text string, args ...any) {
	t.Helper()
	if _, err := e.a.DB.Exec(text, args...); err != nil {
		t.Fatal(err)
	}
}

func TestCoreArchivedBudgetLegacyWriteCannotRestartCarry(t *testing.T) {
	e := setup(t)
	workflowExec(t, e, "INSERT INTO group_targets(period_id,category_id,amount_cents,carry_forward)VALUES(1,1,100,0)")
	status(t, e.req(t, 1, "/api/categories/1", "PUT", map[string]any{"name": "Groceries", "archived": true, "version": 1}), 200)
	body := map[string]any{"version": 1, "targets": []map[string]any{{"category_id": 1, "amount_cents": 200}}}
	status(t, e.req(t, 1, "/api/targets/1", "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT carry_forward FROM group_targets WHERE period_id=1 AND category_id=1") != 0 {
		t.Fatal("archived category started copying again")
	}
}
