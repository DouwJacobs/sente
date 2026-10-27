package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBrowserTransactionDeletionArchivesAndPreventsReimport(t *testing.T) {
	e := setup(t)
	seedRemovalDuplicates(t, e)
	r := httptest.NewRequest("DELETE", "/api/transactions/2", strings.NewReader(`{"version":1,"reason":"Duplicate","confirmed":true}`))
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: "token1"})
	r.Header.Set("Origin", "http://localhost:8080")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	body := map[string]any{"version": 1, "reason": "Duplicate transaction", "confirmed": true}
	status(t, e.req(t, 3, "/api/transactions/2", "DELETE", body), 403)
	status(t, e.req(t, 1, "/api/transactions/2", "DELETE", map[string]any{"version": 1, "reason": "Duplicate transaction"}), 400)
	status(t, e.req(t, 1, "/api/transactions/2", "DELETE", body), 200)
	status(t, e.req(t, 1, "/api/transactions/2", "DELETE", body), 404)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=2") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM allocations WHERE transaction_id=2") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals WHERE original_id=2 AND retained_id IS NULL") != 1 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions WHERE account_id=1") != 10001 {
		t.Fatal("manual deletion changed the wrong money or lost source retention")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='transaction' AND entity_id=2 AND action='removed' AND user_id=1") != 1 {
		t.Fatal("missing manual deletion audit")
	}
	p := e.stage(t, "deletion-reimport.ofx", ofx(ofxRow("other-ref", "20261021", "100.01", "Synthetic salary"), ofxRow("new-after-deletion", "20261021", "1.00", "New synthetic income")))
	if p.Rows[0].Duplicate != "exact_id" {
		t.Fatal("deleted bank identity not recognised", p.Rows)
	}
	status(t, e.commit(t, p.ID, nil, true), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE fitid='other-ref'") != 0 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions WHERE account_id=1") != 10101 {
		t.Fatal("manual deletion was recreated on import")
	}
}

func TestBrowserTransactionDeletionRechecksAccessVersionLinksAndAudit(t *testing.T) {
	for _, kind := range []string{"version", "hidden", "grant", "linked", "audit", "reason"} {
		t.Run(kind, func(t *testing.T) {
			e := setup(t)
			seedRemovalDuplicates(t, e)
			path := "/api/transactions/2"
			code := 400
			body := map[string]any{"version": 1, "reason": "Duplicate transaction", "confirmed": true}
			switch kind {
			case "version":
				migrationExec(t, e.a.DB, "UPDATE transactions SET version=2 WHERE id=2")
				code = 409
			case "hidden":
				migrationExec(t, e.a.DB, "UPDATE accounts SET sync_hidden=1 WHERE id=1")
				code = 403
			case "grant":
				path = "/api/transactions/3"
				migrationExec(t, e.a.DB, "DELETE FROM grants WHERE account_id=2 AND user_id=1")
				code = 403
			case "linked":
				migrationExec(t, e.a.DB, "INSERT INTO transfer_links(left_id,right_id) VALUES(1,2)")
			case "audit":
				migrationExec(t, e.a.DB, "CREATE TRIGGER reject_manual_removal BEFORE INSERT ON audit WHEN NEW.action='removed' BEGIN SELECT RAISE(ABORT,'synthetic audit failure'); END")
				code = 500
			case "reason":
				body["reason"] = " "
			}
			status(t, e.req(t, 1, path, "DELETE", body), code)
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transaction_removals") != 0 {
				t.Fatal("failed deletion partially mutated financial data")
			}
		})
	}
}
