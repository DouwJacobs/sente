package app

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"testing"
)

func TestDashboardBucketLineage(t *testing.T) {
	e := setup(t)
	for _, statement := range []string{
		"INSERT INTO targets VALUES(1,1,2000)",
		"INSERT INTO group_targets(period_id,category_id,spending_group_id,amount_cents) VALUES(1,1,1,1200),(1,1,2,800)",
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id,spending_group_id,is_transfer) VALUES(1,1,'2026-10-21',-1000,'Split','2026-10-21',-1000,'Split','{}',1,1,0),(2,1,'2026-10-21',-500,'Other group','2026-10-21',-500,'Other group','{}',1,2,0),(3,1,'2026-10-21',100,'Refund','2026-10-21',100,'Refund','{}',1,1,0),(4,1,'2026-10-21',-200,'Missing','2026-10-21',-200,'Missing','{}',1,NULL,0),(5,1,'2026-10-21',-900,'Transfer','2026-10-21',-900,'Transfer','{}',1,1,1),(6,2,'2026-10-21',-800,'Private','2026-10-21',-800,'Private','{}',1,1,0)",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-600),(1,1,-400),(2,1,-500),(3,1,100),(4,NULL,-200),(5,1,-900),(6,1,-800)",
	} {
		if _, err := e.a.DB.Exec(statement); err != nil {
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
		return d
	}
	d := read("/api/dashboard?period=1&category_page=0")
	groups := d["spending_groups"].([]any)
	if len(groups) != 3 || num(d["spent_cents"]) != 1600 || num(d["budget_cents"]) != 2000 {
		t.Fatalf("wrong totals: %v", d)
	}
	var sum int64
	for _, value := range groups {
		g := value.(map[string]any)
		sum += num(g["spent_cents"])
		cats := g["categories"].([]any)
		if len(cats) != 1 {
			t.Fatal("same-category split counted as separate children")
		}
		c := cats[0].(map[string]any)
		if num(g["id"]) != 0 && (num(c["id"]) != 1 || num(c["total_spent_cents"]) != 1400 || num(c["total_target_cents"]) != 2000 || num(c["target_cents"]) != map[int64]int64{1: 1200, 2: 800}[num(g["id"])]) {
			t.Fatalf("lost shared category lineage: %v", c)
		}
		if num(g["id"]) == 1 && num(c["spent_cents"]) != 900 {
			t.Fatal("refund or split counted incorrectly")
		}
		if num(g["id"]) == 0 && (num(c["id"]) != 0 || num(c["spent_cents"]) != 200) {
			t.Fatal("missing classification excluded")
		}
	}
	if sum != num(d["spent_cents"]) {
		t.Fatal("buckets do not reconcile")
	}
	d = read("/api/dashboard?period=1&account=2&category_page=0")
	if d["has_targets"] != false || num(d["spent_cents"]) != 800 {
		t.Fatal("private scope/limits changed")
	}
	c := d["spending_groups"].([]any)[0].(map[string]any)["categories"].([]any)[0].(map[string]any)
	if num(c["target_cents"]) != 0 || num(c["total_spent_cents"]) != 800 {
		t.Fatal("private bucket leaked household totals")
	}
	status(t, e.req(t, 2, "/api/dashboard?period=1", "GET", nil), 403)
	status(t, e.req(t, 1, "/api/dashboard?account=999", "GET", nil), 403)
}

func TestDashboardBucketPagination(t *testing.T) {
	rows := []map[string]any{}
	for group := 1; group <= 21; group++ {
		for category := 1; category <= 21; category++ {
			rows = append(rows, map[string]any{"spending_group_id": int64(group), "spending_group_name": fmt.Sprintf("Group %02d", group), "category_id": int64(category), "name": fmt.Sprintf("Category %02d", category), "kind": "expense", "amount_cents": int64(-100)})
		}
	}
	groups, total, err := dashboardGroups(httptest.NewRequest("GET", "/?group_id=1&group_category_page=1", nil), rows, nil, nil)
	if err != nil || total != 21 || len(groups) != 20 || len(groups[0]["categories"].([]map[string]any)) != 1 || num(groups[0]["spent_cents"]) != 2100 {
		t.Fatal("child pagination lost totals")
	}
	groups, total, err = dashboardGroups(httptest.NewRequest("GET", "/?group_page=1", nil), rows, nil, nil)
	if err != nil || total != 21 || len(groups) != 1 {
		t.Fatal("group pagination lost groups")
	}
	for _, query := range []string{"group_page=-1", "group_category_page=bad"} {
		if _, _, err := dashboardGroups(httptest.NewRequest("GET", "/?"+query, nil), rows, nil, nil); err == nil {
			t.Fatal("invalid page accepted")
		}
	}
}

func TestDashboardSpendingTransactionScope(t *testing.T) {
	e := setup(t)
	for _, statement := range []string{
		"INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id,spending_group_id,is_transfer) VALUES(1,1,'2026-10-21',-1000,'Split','2026-10-21',-1000,'Split','{}',1,1,0),(2,1,'2026-10-21',-500,'Other group','2026-10-21',-500,'Other group','{}',1,2,0),(3,1,'2026-10-21',100,'Refund','2026-10-21',100,'Refund','{}',1,1,0),(4,1,'2026-10-21',-200,'Missing','2026-10-21',-200,'Missing','{}',1,NULL,0),(5,1,'2026-10-21',-900,'Transfer','2026-10-21',-900,'Transfer','{}',1,1,1),(6,2,'2026-10-21',-800,'Private','2026-10-21',-800,'Private','{}',NULL,1,0),(7,1,'2026-10-21',1000,'Income','2026-10-21',1000,'Income','{}',1,1,0)",
		"INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-600),(1,NULL,-400),(2,1,-500),(3,1,100),(4,NULL,-200),(5,1,-900),(6,1,-800),(7,2,1000)",
	} {
		if _, err := e.a.DB.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	read := func(scope string) ([]any, map[string]any) {
		w := e.req(t, 1, "/api/transactions?dashboard_spending=1&period=1"+scope, "GET", nil)
		status(t, w, 200)
		var result map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		return result["items"].([]any), result
	}
	rows, _ := read("&spending_group=1&category=1")
	if len(rows) != 2 {
		t.Fatalf("lineage leak: %v", rows)
	}
	var spend int64
	for _, raw := range rows {
		r := raw.(map[string]any)
		spend += num(r["matched_spend_cents"])
		if num(r["id"]) == 1 && num(r["matched_spend_cents"]) != 600 {
			t.Fatal("split counted parent instead of allocation")
		}
	}
	if spend != 500 {
		t.Fatal("refund not deducted")
	}
	rows, _ = read("&spending_group=1")
	if len(rows) != 2 {
		t.Fatal("income/transfer/private leaked")
	}
	rows, _ = read("&spending_group=unassigned&category=uncategorized")
	if len(rows) != 1 || num(rows[0].(map[string]any)["matched_spend_cents"]) != 200 {
		t.Fatal("missing lineage wrong")
	}
	rows, _ = read("&account=2&spending_group=1&category=1")
	if len(rows) != 1 || num(rows[0].(map[string]any)["id"]) != 6 {
		t.Fatal("private date scope changed")
	}
	status(t, e.req(t, 2, "/api/transactions?dashboard_spending=1&period=1", "GET", nil), 403)
	status(t, e.req(t, 1, "/api/transactions?dashboard_spending=1", "GET", nil), 400)
	status(t, e.req(t, 1, "/api/transactions?dashboard_spending=bad&period=1", "GET", nil), 400)
	for id := 20; id < 45; id++ {
		_, err := e.a.DB.Exec("INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id,spending_group_id) VALUES(?,1,'2026-10-21',-10,'Paged','2026-10-21',-10,'Paged','{}',1,1)", id)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,1,-10)", id); err != nil {
			t.Fatal(err)
		}
	}
	rows, result := read("&spending_group=1&category=1")
	if len(rows) != 20 || num(result["total"]) != 27 {
		t.Fatal("pagination loses full count")
	}
	rows, _ = read("&spending_group=1&category=1&offset=20&list_version=" + result["list_version"].(string))
	if len(rows) != 7 {
		t.Fatal("second page wrong")
	}
	if _, err := e.a.DB.Exec("UPDATE transactions SET version=version+1 WHERE id=44"); err != nil {
		t.Fatal(err)
	}
	status(t, e.req(t, 1, "/api/transactions?dashboard_spending=1&period=1&spending_group=1&offset=20&list_version="+result["list_version"].(string), "GET", nil), 409)
}
