package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

func TestMCPFinancialSummarySplitsRefundsTransfersAndFilters(t *testing.T) {
	e := setup(t)
	if _, err := e.a.DB.Exec(`
 INSERT INTO categories(id,name,group_name,kind) VALUES(3,'Dining','Living','expense');
 INSERT INTO transactions(id,account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id,is_transfer) VALUES
 (1,1,'2026-10-21',-1000,'Synthetic split 12345678901','2026-10-21',-1000,'secret source','{}',1,0),
 (2,1,'2026-10-22',200,'Refund','2026-10-22',200,'secret source','{}',1,0),
 (3,1,'2026-10-23',5000,'Salary','2026-10-23',5000,'secret source','{}',1,0),
 (4,1,'2026-10-24',-300,'Transfer','2026-10-24',-300,'secret source','{}',1,1),
 (5,2,'2026-10-24',-9000,'Private','2026-10-24',-9000,'secret source','{}',NULL,0),
 (6,1,'2026-10-24',-50,'Missing category','2026-10-24',-50,'secret source','{}',1,0);
 INSERT INTO allocations(transaction_id,category_id,amount_cents,note) VALUES
 (1,1,-600,'secret note'),(1,3,-400,''),(2,1,200,''),(3,2,5000,''),(4,1,-300,''),(5,1,-9000,''),(6,NULL,-50,'');
 `); err != nil {
		t.Fatal(err)
	}
	token := mcpToken(t, e, 1, false)
	value, failed := mcpCall(t, e, token, "get_financial_summary", map[string]any{"period_id": 1, "group_by": "category", "page_size": 1})
	if failed {
		t.Fatal(value)
	}
	totals := value["totals"].(map[string]any)
	for key, want := range map[string]float64{"income_cents": 5000, "spent_cents": 850, "net_cents": 4150, "cash_in_cents": 5200, "cash_out_cents": 1350, "cash_net_cents": 3850, "transaction_count": 5} {
		if totals[key] != want {
			t.Fatalf("%s: %v want %v", key, totals, want)
		}
	}
	if value["total"] != float64(4) || value["more"] != true || len(value["items"].([]any)) != 1 {
		t.Fatal(value)
	}
	cat, failed := mcpCall(t, e, token, "get_financial_summary", map[string]any{"period_id": 1, "category_id": 1, "group_by": "category"})
	if failed {
		t.Fatal(cat)
	}
	ct := cat["totals"].(map[string]any)
	if ct["spent_cents"] != float64(400) || ct["cash_out_cents"] != float64(1300) || ct["transaction_count"] != float64(3) {
		t.Fatal("category includes other split or duplicates parent", cat)
	}
	date, failed := mcpCall(t, e, token, "get_financial_summary", map[string]any{"date_from": "2026-10-22", "date_to": "2026-10-22"})
	if failed || date["totals"].(map[string]any)["spent_cents"] != float64(-200) {
		t.Fatal("refund semantics", date)
	}
	searched, failed := mcpCall(t, e, token, "get_financial_summary", map[string]any{"q": "split", "group_by": "month"})
	if failed || searched["totals"].(map[string]any)["spent_cents"] != float64(1000) || searched["items"].([]any)[0].(map[string]any)["name"] != "2026-10" {
		t.Fatal(searched)
	}
	raw, _ := json.Marshal(value)
	for _, secret := range []string{"secret note", "secret source", "12345678901", "Private", "bank_id", "provenance", "description"} {
		if strings.Contains(string(raw), secret) {
			t.Fatal("aggregate disclosure", secret, string(raw))
		}
	}
}
func TestMCPFinancialSummaryPermissionsPeriodsAndPaging(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	owner := mcpToken(t, e, 1, false)
	viewer := mcpToken(t, e, 3, false)
	private, failed := mcpCall(t, e, viewer, "get_financial_summary", map[string]any{"account_id": 2})
	if failed || private["totals"].(map[string]any)["transaction_count"] != float64(0) {
		t.Fatal("private disclosure", private)
	}
	for _, input := range []map[string]any{{"period_id": 1}, {"merchant_id": 1}, {"group_by": "merchant"}, {"date_from": "2026-02-30"}, {"date_from": "2026-11-01", "date_to": "2026-10-01"}, {"account_id": -1}, {"group_by": "description"}, {"page_size": 0}, {"page_size": 101}, {"page": -1}} {
		if v, failed := mcpCall(t, e, viewer, "get_financial_summary", input); !failed {
			t.Fatal("accepted invalid or unauthorized", input, v)
		}
	}
	identity, err := e.a.mcpIdentity(owner)
	if err != nil {
		t.Fatal(err)
	}
	p := legacyMCPPermissions(false)
	p.ReadMerchants = true
	p.Constraints = mcpConstraints{SelectedAccounts: true, AccountIDs: []int64{1}}
	raw, _ := json.Marshal(p)
	e.a.DB.Exec("UPDATE mcp_tokens SET permissions=? WHERE id=?", string(raw), identity.TokenID)
	e.a.DB.Exec("INSERT INTO merchants(id,name,account_id) VALUES(1,'Synthetic shop 12345678901',1); UPDATE transactions SET merchant_id=1 WHERE id=1")
	merchant, failed := mcpCall(t, e, owner, "get_financial_summary", map[string]any{"group_by": "merchant", "merchant_id": 1})
	if failed || merchant["totals"].(map[string]any)["spent_cents"] != float64(101) || !strings.Contains(fmt.Sprint(merchant), "[redacted]") {
		t.Fatal(merchant)
	}
	all, failed := mcpCall(t, e, owner, "get_financial_summary", map[string]any{})
	if failed || all["totals"].(map[string]any)["spent_cents"] != float64(303) {
		t.Fatal("selected-account scope", all)
	}
	e.a.DB.Exec("INSERT INTO periods(id,name,start_date,end_date) VALUES(2,'November','2026-11-20','2026-12-19')")
	compared, failed := mcpCall(t, e, owner, "compare_financial_periods", map[string]any{"period_ids": []int64{2, 1}})
	if failed {
		t.Fatal(compared)
	}
	rows := compared["items"].([]any)
	if rows[0].(map[string]any)["totals"].(map[string]any)["spent_cents"] != float64(0) || rows[1].(map[string]any)["totals"].(map[string]any)["spent_cents"] != float64(303) {
		t.Fatal(compared)
	}
	for _, ids := range [][]int64{{1}, {1, 1}, {1, 999}, {0, 1}} {
		if v, failed := mcpCall(t, e, owner, "compare_financial_periods", map[string]any{"period_ids": ids}); !failed {
			t.Fatal(ids, v)
		}
	}
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1")
	all, failed = mcpCall(t, e, owner, "get_financial_summary", map[string]any{})
	if failed || all["totals"].(map[string]any)["transaction_count"] != float64(0) {
		t.Fatal("hidden account included", all)
	}
}

func TestMCPAggregateNetOverflow(t *testing.T) {
	for _, pair := range [][2]int64{{math.MaxInt64, -1}, {math.MinInt64, 1}} {
		if _, err := mcpAggregateNet(pair[0], pair[1]); err == nil {
			t.Fatal("overflow accepted", pair)
		}
	}
	if net, err := mcpAggregateNet(100, -25); err != nil || net != 125 {
		t.Fatal(net, err)
	}
}
