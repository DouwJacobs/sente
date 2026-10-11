package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestDashboardSpendingCompleteBreakdown(t *testing.T) {
	rows := []map[string]any{}
	for group := 1; group <= 21; group++ {
		for category := 1; category <= 21; category++ {
			rows = append(rows, map[string]any{"spending_group_id": int64(group), "spending_group_name": fmt.Sprintf("Group %02d", group), "category_id": int64(category), "name": fmt.Sprintf("Category %02d", category), "kind": "expense", "amount_cents": int64(-100)})
		}
	}
	result := dashboardSpending(rows)
	if len(result) != 441 {
		t.Fatal("flow was paginated")
	}
	var sum int64
	for _, entry := range result {
		sum += num(entry["spent_cents"])
	}
	if sum != 44100 {
		t.Fatal("flow lost spending")
	}
}

func TestDashboardSpendingScopeAndRefunds(t *testing.T) {
	e := setup(t)
	statements := []string{
		"INSERT INTO categories(id,name,group_name,kind) VALUES(3,'Transport','','expense')",
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id,spending_group_id,is_transfer) VALUES(1,1,'2026-10-20',-1000,'Split','2026-10-20',-1000,'Split','{}',1,1,0),(2,1,'2026-11-19',100,'Refund','2026-11-19',100,'Refund','{}',1,1,0),(3,1,'2026-10-21',-200,'Missing','2026-10-21',-200,'Missing','{}',1,NULL,0),(4,1,'2026-10-21',-900,'Transfer','2026-10-21',-900,'Transfer','{}',1,1,1),(5,2,'2026-10-20',-800,'Private','2026-10-20',-800,'Private','{}',NULL,1,0),(6,2,'2026-11-19',-300,'End','2026-11-19',-300,'End','{}',NULL,2,0),(7,2,'2026-10-19',-500,'Before','2026-10-19',-500,'Before','{}',NULL,1,0),(8,2,'2026-11-20',-500,'After','2026-11-20',-500,'After','{}',NULL,1,0),(9,1,'2026-10-21',600,'Net refund','2026-10-21',600,'Net refund','{}',1,2,0),(10,1,'2026-10-21',1000,'Income','2026-10-21',1000,'Income','{}',1,1,0)",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-600),(1,3,-400),(2,1,100),(3,NULL,-200),(4,1,-900),(5,1,-800),(6,1,-300),(7,1,-500),(8,1,-500),(9,3,600),(10,2,1000)",
	}
	for _, s := range statements {
		if _, err := e.a.DB.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	read := func(path string) map[string]any {
		w := e.req(t, 1, path, "GET", nil)
		status(t, w, 200)
		var d map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
			t.Fatal(err)
		}
		var sum int64
		for _, raw := range d["spending_breakdown"].([]any) {
			sum += num(raw.(map[string]any)["spent_cents"])
		}
		if sum != num(d["spent_cents"]) {
			t.Fatalf("flow does not reconcile: %v", d)
		}
		return d
	}
	d := read("/api/dashboard?period=1&group_page=1&category_page=1")
	if num(d["spent_cents"]) != 500 || len(d["spending_breakdown"].([]any)) != 4 {
		t.Fatal(d)
	}
	missing, refund := false, false
	for _, raw := range d["spending_breakdown"].([]any) {
		entry := raw.(map[string]any)
		if num(entry["category_id"]) == 0 {
			missing = entry["category_name"] == "Uncategorised" && num(entry["spent_cents"]) == 200
		}
		if num(entry["spent_cents"]) == -600 {
			refund = true
		}
	}
	if !missing || !refund {
		t.Fatal("uncategorised/net refunds disappeared")
	}
	d = read("/api/dashboard?period=1&account=2")
	if num(d["spent_cents"]) != 1100 {
		t.Fatal("custom inclusive dates changed")
	}
	status(t, e.req(t, 2, "/api/dashboard?period=1&account=2", "GET", nil), 403)
	status(t, e.req(t, 3, "/api/dashboard?period=1&account=2", "GET", nil), 403)
	if _, err := e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	d = read("/api/dashboard?period=1")
	if len(d["spending_breakdown"].([]any)) != 0 {
		t.Fatal("hidden account disclosed")
	}
}

func TestDashboardSpendingZeroAndUnclassifiedIncome(t *testing.T) {
	rows := []map[string]any{
		{"category_id": int64(1), "name": "Food", "kind": "expense", "amount_cents": int64(-100)},
		{"category_id": int64(1), "name": "Food", "kind": "expense", "amount_cents": int64(100)},
		{"amount_cents": int64(200)},
		{"amount_cents": int64(-300), "is_transfer": int64(1)},
	}
	result := dashboardSpending(rows)
	if len(result) != 1 || num(result[0]["spent_cents"]) != 0 {
		t.Fatal(result)
	}
}
