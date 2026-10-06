package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestImportActivityGroupsAndScopedLedgerPermissions(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)
	e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance) VALUES(1,'2026-10-01',-29,'Synthetic purchase','2026-10-01',-29,'Synthetic purchase',?,'{}')", fingerprint(SourceRow{Date: "2026-10-01", Amount: -29, Description: "Synthetic purchase"}))
	snapshot := fnbSnapshot{Reports: []FNBReport{transactionReport(targets[0].BankID, "run-a"), transactionReport(targets[1].BankID, "run-a")}}
	previews, err := e.a.stageFNBTransactions(e.owner, targets, snapshot, "run-a")
	if err != nil {
		t.Fatal(err)
	}
	get := func(uid int, path string) map[string]any {
		r := e.req(t, uid, path, "GET", nil)
		status(t, r, 200)
		var v map[string]any
		if err := json.Unmarshal(r.Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	activity := get(1, "/api/imports?page=0&page_size=1")
	if activity["total"] != float64(1) || activity["pending_total"] != float64(2) || len(activity["items"].([]any)) != 2 {
		t.Fatal("run split across pages", activity)
	}
	if num(activity["items"].([]any)[0].(map[string]any)["new_transactions_total"]) != 0 {
		t.Fatal("staged source rows counted as imported")
	}
	viewer := get(3, "/api/imports?page=0&page_size=1")
	rows := viewer["items"].([]any)
	if len(rows) != 1 || rows[0].(map[string]any)["account_name"] != "Shared" {
		t.Fatal("private metadata leaked", viewer)
	}
	for _, p := range previews {
		status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{"classification_version": p.ClassificationVersion, "skip_all_candidates": true}), 200)
	}
	completed := get(1, "/api/imports?page=0")
	if num(completed["items"].([]any)[0].(map[string]any)["new_transactions_total"]) != 3 {
		t.Fatal("run count includes skipped duplicates", completed)
	}
	visible := get(3, "/api/imports?page=0")
	if num(visible["items"].([]any)[0].(map[string]any)["new_transactions_total"]) != 1 {
		t.Fatal("new transaction count leaked private account", visible)
	}
	nested := get(1, "/api/imports?activity=run-a&page=0&page_size=1")
	if nested["total"] != float64(2) || len(nested["items"].([]any)) != 1 {
		t.Fatal("account expansion not bounded", nested)
	}
	if num(nested["items"].([]any)[0].(map[string]any)["inserted"]) != 1 {
		t.Fatal("account new count is not saved inserted count", nested)
	}
	retained := get(1, fmt.Sprintf("/api/imports/%d?page=0", previews[0].ID))
	retainedRows := retained["rows"].([]any)
	if num(retainedRows[0].(map[string]any)["transaction_id"]) != 0 || num(retainedRows[1].(map[string]any)["transaction_id"]) == 0 {
		t.Fatal("source rows not mapped to the saved ledger", retained)
	}
	status(t, e.req(t, 3, fmt.Sprintf("/api/imports/%d?page=0", previews[1].ID), "GET", nil), 404)
	private := previews[1].ID
	if get(1, fmt.Sprintf("/api/transactions?imports=%d", private))["total"].(float64) == 0 {
		t.Fatal("scope missing imported entries")
	}
	if get(3, fmt.Sprintf("/api/transactions?imports=%d", private))["total"] != float64(0) {
		t.Fatal("scope bypassed account access")
	}
	if get(1, "/api/transactions?imports=999999")["total"] != float64(0) {
		t.Fatal("scope returned unrelated entries")
	}
	status(t, e.req(t, 1, "/api/transactions?imports=1,invalid", "GET", nil), 400)
	if get(1, "/api/imports?page=0")["pending_total"] != float64(0) {
		t.Fatal("pending activity count stale")
	}
}
