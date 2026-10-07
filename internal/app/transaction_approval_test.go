package app

import (
	"fmt"
	"testing"
)

func TestSaveAndApproveAtomic(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -10000, "2026-10-21", nil)
	path := fmt.Sprintf("/api/transactions/%d", id)
	cat := int64(1)
	body := editBody(1, -10000, []Allocation{{CategoryID: nil, Amount: -10000, Note: ""}})
	body["approve"] = true
	status(t, e.req(t, 1, path, "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 1 {
		t.Fatal("uncategorized approval saved edits")
	}
	body["allocations"] = []Allocation{{CategoryID: &cat, Amount: -10000, Note: ""}}
	body["rule"] = map[string]any{"pattern": " "}
	status(t, e.req(t, 1, path, "PUT", body), 400)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='transaction' AND entity_id=?", id) != 0 {
		t.Fatal("invalid rule saved edit or approval")
	}
	body["rule"] = map[string]any{"pattern": "Market"}
	status(t, e.req(t, 3, path, "PUT", body), 403)
	status(t, e.req(t, 1, path, "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=? AND version=2 AND review_state='approved' AND reviewed_by=1 AND reviewed_at IS NOT NULL", id) != 1 {
		t.Fatal("approval metadata not saved")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM audit WHERE entity='transaction' AND entity_id=? AND action IN ('edited','approved')", id) != 2 {
		t.Fatal("missing edit/approval audits")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM rules WHERE account_id=1 AND category_id=1 AND direction='debit' AND pattern='Market'") != 1 {
		t.Fatal("rule not saved atomically")
	}
	status(t, e.req(t, 1, path, "PUT", body), 409)
	if queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", id) != 2 {
		t.Fatal("stale approval changed transaction")
	}
	// Complete future matches are automatically accepted but remain unseen.
	p := e.stage(t, "future.ofx", ofx(ofxRow("future-rule", "20261022", "-12.34", "Market new reference 12345")))
	if p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != cat {
		t.Fatal("future rule did not match")
	}
	status(t, e.commit(t, p.ID, nil, false), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE fitid='future-rule' AND review_state='approved'") != 1 {
		t.Fatal("rule did not accept categorized import")
	}
	// Saving categorized entries automatically accepts them.
	body["version"] = 2
	body["approve"] = false
	body["rule"] = nil
	status(t, e.req(t, 1, path, "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=? AND review_state='approved' AND reviewed_by IS NULL AND reviewed_at IS NULL", id) != 1 {
		t.Fatal("save-only did not automatically accept classification")
	}
}

func TestSaveAndApproveSplitsAndTransfers(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	id := seedTransaction(t, e, 1, -10000, "2026-10-21", nil)
	path := fmt.Sprintf("/api/transactions/%d", id)
	body := editBody(1, -10000, []Allocation{{CategoryID: &cat, Amount: -6000, Note: ""}, {CategoryID: nil, Amount: -4000, Note: ""}})
	body["approve"] = true
	status(t, e.req(t, 1, path, "PUT", body), 400)
	body["allocations"] = []Allocation{{CategoryID: &cat, Amount: -6000, Note: ""}, {CategoryID: &cat, Amount: -3999, Note: ""}}
	status(t, e.req(t, 1, path, "PUT", body), 400)
	body["allocations"] = []Allocation{{CategoryID: &cat, Amount: -6000, Note: ""}, {CategoryID: &cat, Amount: -4000, Note: ""}}
	status(t, e.req(t, 1, path, "PUT", body), 200)
	body["version"] = 2
	body["is_transfer"] = true
	body["allocations"] = []Allocation{{CategoryID: nil, Amount: -10000, Note: ""}}
	status(t, e.req(t, 1, path, "PUT", body), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE id=? AND is_transfer=1 AND review_state='approved'", id) != 1 {
		t.Fatal("explicit transfer approval failed")
	}
}
