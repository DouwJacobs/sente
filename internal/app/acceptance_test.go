package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

func seenRequest(t *testing.T, e *testEnv, user int, seen bool, id, version int64) int {
	return e.req(t, user, "/api/transactions/seen", "POST", map[string]any{"seen": seen, "items": []map[string]int64{{"id": id, "version": version}}}).Code
}
func transactionFor(t *testing.T, e *testEnv, user int, id int64) map[string]any {
	t.Helper()
	w := e.req(t, user, fmt.Sprintf("/api/transactions?id=%d", id), "GET", nil)
	status(t, w, 200)
	var b struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	if len(b.Items) != 1 {
		t.Fatal(w.Body.String())
	}
	return b.Items[0]
}
func TestAutomaticAcceptanceAndPersonalSeen(t *testing.T) {
	e := setup(t)
	createTestRule(t, e, ruleBody(1, 1, "Merchant", "debit", nil, 0, true, 0))
	p := e.stage(t, "accept.ofx", ofx(ofxRow("accepted", "20261021", "-12.34", "Merchant synthetic"), ofxRow("missing", "20261021", "-2.34", "Unknown synthetic")))
	status(t, e.commit(t, p.ID, nil, false), 200)
	id := queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='accepted'")
	missing := queryInt(e.a.DB, "SELECT id FROM transactions WHERE fitid='missing'")
	row := transactionFor(t, e, 1, id)
	if row["review_state"] != "approved" || row["seen"] != float64(0) || row["reviewed_by"] != nil {
		t.Fatal(row)
	}
	if transactionFor(t, e, 1, missing)["review_state"] != "pending_review" {
		t.Fatal("missing category accepted")
	}
	// Seeing is personal and never changes financial/review versions or totals.
	before := e.req(t, 1, "/api/dashboard?period=1", "GET", nil).Body.String()
	if seenRequest(t, e, 3, true, id, 1) != 200 {
		t.Fatal("viewer cannot mark own seen")
	}
	if transactionFor(t, e, 3, id)["seen"] != float64(1) || transactionFor(t, e, 1, id)["seen"] != float64(0) {
		t.Fatal("seen leaked between users")
	}
	if seenRequest(t, e, 1, true, id, 1) != 200 || seenRequest(t, e, 1, false, id, 1) != 200 {
		t.Fatal("seen toggling failed")
	}
	if e.req(t, 1, "/api/dashboard?period=1", "GET", nil).Body.String() != before {
		t.Fatal("seen changed reporting")
	}
	if seenRequest(t, e, 2, true, id, 1) != 403 {
		t.Fatal("unauthorized seen allowed")
	}
	cat := int64(1)
	body := editBody(1, -1234, []Allocation{{&cat, -1234, "updated"}})
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if transactionFor(t, e, 1, id)["seen"] != float64(1) || transactionFor(t, e, 3, id)["seen"] != float64(0) {
		t.Fatal("edit did not reset other users' seen versions")
	}
	if seenRequest(t, e, 3, true, id, 1) != 409 {
		t.Fatal("stale details marked seen")
	}
	// Missing split categories remain pending; filling the last split accepts it.
	body["version"] = 2
	body["allocations"] = []Allocation{{&cat, -1000, ""}, {nil, -234, ""}}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if transactionFor(t, e, 1, id)["review_state"] != "pending_review" {
		t.Fatal("partial split accepted")
	}
	body["version"] = 3
	body["allocations"] = []Allocation{{&cat, -1000, ""}, {&cat, -234, ""}}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if transactionFor(t, e, 1, id)["review_state"] != "approved" {
		t.Fatal("complete split not accepted")
	}
	body["version"] = 4
	body["is_transfer"] = true
	body["allocations"] = []Allocation{{nil, -1234, ""}}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", id), "PUT", body), 200)
	if transactionFor(t, e, 1, id)["review_state"] != "approved" {
		t.Fatal("explicit transfer not accepted")
	}
}
func TestSeenBatchAtomicityAndFilters(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", nil)
	private := seedTransaction(t, e, 2, -100, "2026-10-21", nil)
	status(t, e.req(t, 3, "/api/transactions/seen", "POST", map[string]any{"seen": true, "items": []map[string]int64{{"id": id, "version": 1}, {"id": private, "version": 1}}}), 403)
	if transactionFor(t, e, 3, id)["seen"] != float64(0) {
		t.Fatal("batch partially marked seen")
	}
	if seenRequest(t, e, 1, true, id, 1) != 200 {
		t.Fatal("mark failed")
	}
	for filter, want := range map[string]int{"1": 1, "0": 1} {
		w := e.req(t, 1, "/api/transactions?seen="+filter, "GET", nil)
		status(t, w, 200)
		var b struct {
			Total int `json:"total"`
		}
		json.Unmarshal(w.Body.Bytes(), &b)
		if b.Total != want {
			t.Fatal(w.Body.String())
		}
	}
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=2")
	if seenRequest(t, e, 1, true, private, 1) != 403 {
		t.Fatal("hidden account marked seen")
	}
	status(t, e.req(t, 1, "/api/transactions?seen=bad", "GET", nil), 400)
	status(t, e.req(t, 1, "/api/transactions/seen", "POST", map[string]any{"items": []map[string]int64{{"id": id, "version": 1}}}), 400)
}
func TestAcceptanceMigrationOnceAndReviewerSeen(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	pending := seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	approved := seedTransaction(t, e, 1, -200, "2026-10-21", &cat)
	missing := seedTransaction(t, e, 1, -300, "2026-10-21", nil)
	malformed := seedTransaction(t, e, 1, -400, "2026-10-21", &cat)
	e.a.DB.Exec("UPDATE allocations SET amount_cents=-399 WHERE transaction_id=?", malformed)
	e.a.DB.Exec("UPDATE transactions SET review_state='approved',reviewed_by=1,reviewed_at=CURRENT_TIMESTAMP WHERE id=?", approved)
	e.a.DB.Exec("DELETE FROM migrations WHERE version>=10;INSERT INTO migrations VALUES(9)")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if transactionFor(t, e, 1, pending)["review_state"] != "approved" || transactionFor(t, e, 1, pending)["seen"] != float64(0) {
		t.Fatal("pending migration incorrectly seen or pending")
	}
	if transactionFor(t, e, 1, approved)["seen"] != float64(1) || transactionFor(t, e, 3, approved)["seen"] != float64(0) {
		t.Fatal("reviewer seen not preserved personally")
	}
	if transactionFor(t, e, 1, missing)["review_state"] != "pending_review" {
		t.Fatal("uncategorized migration accepted")
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != -1000 {
		t.Fatal("migration changed money")
	}
	if transactionFor(t, e, 1, malformed)["review_state"] != "pending_review" {
		t.Fatal("invalid split accepted by migration")
	}
	versions := queryInt(e.a.DB, "SELECT SUM(version) FROM transactions")
	if err := migrate(e.a.DB); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT SUM(version) FROM transactions") != versions {
		t.Fatal("migration repeated")
	}
}
