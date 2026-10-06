package app

import (
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMCPTypedTransactionFilters(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	if _, err := e.a.DB.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<105)
 INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) SELECT 1,'2026-10-21',-100,'Synthetic','2026-10-21',-100,'Synthetic','{}' FROM n;
 INSERT INTO allocations(transaction_id,category_id,amount_cents) SELECT id,1,amount_cents FROM transactions;`); err != nil {
		t.Fatal(err)
	}
	missing := seedTransaction(t, e, 1, -200, "2026-10-22", nil)
	split := seedTransaction(t, e, 1, -300, "2026-10-22", &cat)
	e.a.DB.Exec("UPDATE allocations SET amount_cents=-100 WHERE transaction_id=?", split)
	e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,NULL,-200)", split)
	transfer := seedTransaction(t, e, 1, 100, "2026-10-23", nil)
	e.a.DB.Exec("UPDATE transactions SET is_transfer=1,review_state='approved' WHERE id=?", transfer)
	e.a.DB.Exec("UPDATE transactions SET review_state='approved' WHERE date='2026-10-21'")
	seedTransaction(t, e, 2, -200, "2026-10-22", nil)
	token := mcpToken(t, e, 3, false)
	cases := []struct {
		args  map[string]any
		total int
	}{
		{map[string]any{"date_from": "2026-10-22", "date_to": "2026-10-22"}, 2},
		{map[string]any{"category_ids": []int64{1, 2}, "date_from": "2026-10-22"}, 1},
		{map[string]any{"categorization": "uncategorized", "acceptance": "needs_category", "min_amount_cents": -300, "max_amount_cents": -200}, 2},
		{map[string]any{"categorization": "categorized", "acceptance": "accepted"}, 106},
		{map[string]any{"min_amount_cents": 0, "max_amount_cents": 100}, 1},
		{map[string]any{"min_amount_cents": -900000000000000, "max_amount_cents": 900000000000000}, 108},
		{map[string]any{"filters": map[string]string{"date_from": "2026-10-22", "date_to": "2026-10-22"}}, 2},
	}
	for _, c := range cases {
		v, failed := mcpCall(t, e, token, "list_transactions", c.args)
		if failed || v["total"] != float64(c.total) {
			t.Fatalf("%v: %v", c.args, v)
		}
	}
	first, failed := mcpCall(t, e, token, "list_transactions", map[string]any{"category_ids": []int64{1, 2}})
	if failed || first["total"] != float64(106) || len(first["items"].([]any)) != 100 {
		t.Fatal(first)
	}
	second, failed := mcpCall(t, e, token, "list_transactions", map[string]any{"category_ids": []int64{1, 2}, "filters": map[string]string{"offset": "100", "list_version": first["list_version"].(string)}})
	if failed || len(second["items"].([]any)) != 6 {
		t.Fatal(second)
	}
	for _, args := range []map[string]any{
		{"date_from": "2026-02-30"}, {"date_from": "2026-10-23", "date_to": "2026-10-22"},
		{"category_ids": []int64{1, 1}}, {"category_ids": []int64{0}}, {"category_ids": make([]int64, 101)},
		{"categorization": "pending"}, {"categorization": "categorized", "acceptance": "needs_category"}, {"categorization": "uncategorized", "acceptance": "accepted"}, {"acceptance": "unseen"}, {"min_amount_cents": 1, "max_amount_cents": 0},
		{"category_ids": []int64{1}, "filters": map[string]string{"category": "1"}},
		{"categorization": "categorized", "filters": map[string]string{"category": "uncategorized"}},
		{"acceptance": "accepted", "filters": map[string]string{"pending": "1"}},
		{"date_from": "2026-10-21", "filters": map[string]string{"date_from": "2026-10-22"}},
		{"max_amount_cents": 0, "filters": map[string]string{"direction": "in"}},
	} {
		if v, failed := mcpCall(t, e, token, "list_transactions", args); !failed {
			t.Fatalf("accepted invalid filters %v: %v", args, v)
		}
	}
	if missing <= 0 {
		t.Fatal("fixture failed")
	}
}

func TestMCPReviewQueueCursorPrivacyAndSelectors(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	// A partially categorized split remains one review parent; all allocations survive.
	if _, err := e.a.DB.Exec("UPDATE allocations SET amount_cents=-51 WHERE transaction_id=1; INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(1,1,-50)"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		seedTransaction(t, e, 1, -100, "2026-10-22", nil)
	}
	accepted := seedTransaction(t, e, 1, -100, "2026-10-24", nil)
	transfer := seedTransaction(t, e, 1, -100, "2026-10-24", nil)
	e.a.DB.Exec("UPDATE allocations SET category_id=1 WHERE transaction_id=?", accepted)
	e.a.DB.Exec("UPDATE transactions SET review_state='approved' WHERE id IN (?,?)", accepted, transfer)
	e.a.DB.Exec("UPDATE transactions SET is_transfer=1 WHERE id=?", transfer)
	e.a.DB.Exec("INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) VALUES(3,2,1)")
	token := mcpToken(t, e, 3, false)
	args := map[string]any{"selector": "unseen", "limit": 2}
	first, failed := mcpCall(t, e, token, "get_transaction_review_queue", args)
	if failed || first["total"] != float64(5) {
		t.Fatal(first)
	}
	cursor := first["next_cursor"].(string)
	if cursor == "" || first["more"] != true {
		t.Fatal(first)
	}
	ids := map[int64]bool{}
	for {
		for _, entry := range first["items"].([]any) {
			item := entry.(map[string]any)
			id := int64(item["id"].(float64))
			if ids[id] || item["review_state"] != "needs_category" || item["seen"] != float64(0) || item["currency"] != "ZAR" || item["account_label"] != "Account 1" {
				t.Fatal(item)
			}
			ids[id] = true
		}
		if first["more"] != true {
			break
		}
		args["cursor"] = first["next_cursor"]
		first, failed = mcpCall(t, e, token, "get_transaction_review_queue", args)
		if failed {
			t.Fatal(first)
		}
	}
	if len(ids) != 5 || ids[accepted] || ids[transfer] || ids[2] || ids[3] {
		t.Fatal(ids)
	}
	all, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "needs_category"})
	if failed || all["total"] != float64(6) {
		t.Fatal(all)
	}
	b, _ := json.Marshal(all)
	for _, secret := range []string{"12345678901", "owner@example.test", "secret note", "Secret source", "Private merchant", "fitid", "provenance", "account_name", "source_description", "bank_id"} {
		if strings.Contains(string(b), secret) {
			t.Fatalf("queue disclosed %s", secret)
		}
	}
	if !strings.Contains(string(b), "[redacted]") {
		t.Fatal("missing redaction", all)
	}
	for _, invalid := range []map[string]any{
		{"selector": "unseen", "limit": 2, "cursor": cursor, "q": "different"},
		{"selector": "needs_category", "limit": 2, "cursor": cursor},
		{"selector": "unseen", "limit": 3, "cursor": cursor},
		{"selector": "unseen", "limit": 2, "cursor": cursor[:len(cursor)-2] + "aa"},
		{"selector": "unseen", "cursor": "junk"}, {"selector": "all"}, {"selector": "unseen", "limit": 101}, {"selector": "unseen", "limit": 0}, {"selector": "unseen", "limit": -1},
		{"selector": "unseen", "acceptance": "accepted"}, {"selector": "unseen", "categorization": "categorized"},
	} {
		if v, failed := mcpCall(t, e, token, "get_transaction_review_queue", invalid); !failed {
			t.Fatal(invalid, v)
		}
	}
	// Another user's seen marker does not invalidate this user's snapshot.
	if _, err := e.a.DB.Exec("INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) VALUES(1,1,1)"); err != nil {
		t.Fatal(err)
	}
	if v, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 2, "cursor": cursor}); failed {
		t.Fatal("other user changed personal snapshot", v)
	}
	other := mcpToken(t, e, 3, false)
	if v, failed := mcpCall(t, e, other, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 2, "cursor": cursor}); !failed {
		t.Fatal("cross-connection cursor", v)
	}
	otherUser := mcpToken(t, e, 1, false)
	if v, failed := mcpCall(t, e, otherUser, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 2, "cursor": cursor}); !failed {
		t.Fatal("cross-user cursor", v)
	}
	e.a.DB.Exec("UPDATE transactions SET version=version+1 WHERE id=1")
	if v, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 2, "cursor": cursor}); !failed || v["error"] != "This list changed. Start from the first page." {
		t.Fatal("stale cursor", v)
	}
}

func TestMCPReviewQueueSeenAndGrantStaleness(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	token := mcpToken(t, e, 1, false)
	first, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 1})
	if failed {
		t.Fatal(first)
	}
	e.a.DB.Exec("INSERT INTO transaction_seen(user_id,transaction_id,transaction_version) VALUES(1,1,1)")
	if v, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "unseen", "limit": 1, "cursor": first["next_cursor"]}); !failed {
		t.Fatal("seen cursor not stale", v)
	}
	first, failed = mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "needs_category", "limit": 1})
	if failed {
		t.Fatal(first)
	}
	e.a.DB.Exec("DELETE FROM grants WHERE user_id=1 AND account_id=2")
	if v, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "needs_category", "limit": 1, "cursor": first["next_cursor"]}); !failed {
		t.Fatal("grant cursor not stale", v)
	}
	empty, failed := mcpCall(t, e, token, "get_transaction_review_queue", map[string]any{"selector": "needs_category", "account_id": 2})
	if failed || empty["total"] != float64(0) || len(empty["items"].([]any)) != 0 || empty["next_cursor"] != "" {
		t.Fatal(empty)
	}
}

func TestLedgerSyntheticQueryPlans(t *testing.T) {
	e := setup(t)
	if _, err := e.a.DB.Exec(`WITH RECURSIVE n(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM n WHERE x<250)
 INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) SELECT 1,'2026-10-22',-100-x,'Synthetic','2026-10-22',-100-x,'Synthetic','{}' FROM n;
 INSERT INTO allocations(transaction_id,amount_cents) SELECT id,amount_cents FROM transactions;`); err != nil {
		t.Fatal(err)
	}
	for _, filter := range []string{"date_from=2026-10-20&date_to=2026-10-23&account=1", "categorization=uncategorized&acceptance=needs_category&seen=0", "category_ids=1,2", "q=synthetic"} {
		r := httptest.NewRequest("GET", "/api/transactions?"+filter, nil)
		query, args, err := e.a.transactionQuery(e.a.DB, r, User{ID: 1, Member: true})
		if err != nil {
			t.Fatal(err)
		}
		plan, err := data(e.a.DB, "EXPLAIN QUERY PLAN "+query+" ORDER BY t.date DESC,t.id DESC LIMIT 51", args...)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("%s: %s", filter, fmt.Sprint(plan))
	}
}
