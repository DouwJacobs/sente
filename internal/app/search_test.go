package app

import (
	"encoding/json"
	"testing"
)

func TestGlobalSearch(t *testing.T) {
	e := setup(t)

	// Insert test spending group and connect it
	if _, err := e.a.DB.Exec("INSERT INTO spending_groups(id, name, color) VALUES(999, 'Everyday Living', '#3b82f6')"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.DB.Exec("UPDATE categories SET spending_group_id=999 WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	// Insert transaction
	if _, err := e.a.DB.Exec("INSERT INTO transactions(id, account_id, date, description, amount_cents, source_date, source_amount, source_description, provenance) VALUES(100, 1, '2026-10-01', 'Woolworths Food', -45000, '2026-10-01', -45000, 'Woolworths Food', 'fnb')"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.DB.Exec("INSERT INTO allocations(transaction_id, category_id, amount_cents) VALUES(100, 1, -45000)"); err != nil {
		t.Fatal(err)
	}
	// Insert merchant rule
	if _, err := e.a.DB.Exec("INSERT INTO merchants(id, name) VALUES(5, 'Woolies')"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.DB.Exec("INSERT INTO merchant_rules(id, merchant_id, pattern) VALUES(20, 5, 'Woolworths*')"); err != nil {
		t.Fatal(err)
	}
	// Insert custom rule
	if _, err := e.a.DB.Exec("INSERT INTO rules(id, user_id, account_id, pattern, category_id) VALUES(30, 1, 1, 'WW Food', 1)"); err != nil {
		t.Fatal(err)
	}

	// 1. Short query returns empty buckets
	w := e.req(t, 1, "/api/search?q=a", "GET", nil)
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var emptyRes globalSearchResult
	if err := json.Unmarshal(w.Body.Bytes(), &emptyRes); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(emptyRes.Categories) != 0 || len(emptyRes.SpendingGroups) != 0 || len(emptyRes.Transactions) != 0 {
		t.Fatalf("expected empty lists for 1 char query, got %+v", emptyRes)
	}

	// 2. Search for "Everyday" -> finds spending group
	w = e.req(t, 1, "/api/search?q=Everyday", "GET", nil)
	var res globalSearchResult
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.SpendingGroups) == 0 || res.SpendingGroups[0]["name"] != "Everyday Living" {
		t.Fatalf("expected Everyday Living spending group, got: %+v", res.SpendingGroups)
	}

	// 3. Search for "Groceries" -> finds category
	w = e.req(t, 1, "/api/search?q=Groceries", "GET", nil)
	res = globalSearchResult{}
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Categories) == 0 || res.Categories[0]["name"] != "Groceries" {
		t.Fatalf("expected Groceries category, got: %+v", res.Categories)
	}

	// 4. Search for "Woolworths" -> finds transaction and merchant rule
	w = e.req(t, 1, "/api/search?q=Woolworths", "GET", nil)
	res = globalSearchResult{}
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Transactions) == 0 || res.Transactions[0]["description"] != "Woolworths Food" {
		t.Fatalf("expected Woolworths Food transaction, got: %+v", res.Transactions)
	}
	if len(res.MerchantRules) == 0 || res.MerchantRules[0]["pattern"] != "Woolworths*" {
		t.Fatalf("expected Woolworths* merchant rule, got: %+v", res.MerchantRules)
	}

	// 5. Search for "450" (amount in rand) -> finds transaction with -45000 cents
	w = e.req(t, 1, "/api/search?q=450", "GET", nil)
	res = globalSearchResult{}
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Transactions) == 0 || res.Transactions[0]["id"] != float64(100) {
		t.Fatalf("expected transaction 100 by amount search, got: %+v", res.Transactions)
	}

	// 6. Search for "WW Food" -> finds rule
	w = e.req(t, 1, "/api/search?q=WW+Food", "GET", nil)
	res = globalSearchResult{}
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Rules) == 0 || res.Rules[0]["pattern"] != "WW Food" {
		t.Fatalf("expected WW Food rule, got: %+v", res.Rules)
	}

	// 7. Permission boundary: User 2 has no access to Account 1
	w = e.req(t, 2, "/api/search?q=Woolworths", "GET", nil)
	res = globalSearchResult{}
	json.Unmarshal(w.Body.Bytes(), &res)
	if len(res.Transactions) != 0 {
		t.Fatalf("user 2 should not see transactions from unauthorized account, got: %+v", res.Transactions)
	}
}
