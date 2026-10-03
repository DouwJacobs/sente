package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func transactionTargets(t *testing.T, e *testEnv) []fnbTarget {
	t.Helper()
	if err := e.a.applyFNBSnapshot(e.owner, fnbSnapshot{Accounts: []fnbAccountSnapshot{{Name: "Shared", BankID: "12345678901"}, {Name: "Private", BankID: "22222222222"}}}, "2026-10-03T08:00:00Z"); err != nil {
		t.Fatal(err)
	}
	targets, err := e.a.fnbTargets(e.a.DB, e.owner)
	if err != nil {
		t.Fatal(err)
	}
	return targets
}
func transactionReport(bank, run string) FNBReport {
	r := prototypeReport()
	r.BankID = bank
	r.RunID = run
	r.Balance = nil
	r.BalanceDate = ""
	return r
}
func TestFNBLiveStagingReuseCommitProvenanceAndReview(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	e.a.DB.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction,spending_group_id) VALUES(1,1,'Synthetic purchase',1,0,'debit',1)")
	report := transactionReport(targets[0].BankID, "run-one")
	report.Transactions = append(report.Transactions[:1], report.Transactions[0]) // legitimate identical purchases
	snapshot := fnbSnapshot{Reports: []FNBReport{report}}
	previews, err := e.a.stageFNBTransactions(e.owner, targets, snapshot, "run-one")
	if err != nil {
		t.Fatal(err)
	}
	p := previews[0]
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 || p.Rows[0].CategoryID == nil || *p.Rows[0].CategoryID != 1 || p.Rows[0].SpendingGroupID == nil || p.Rows[1].Duplicate != "possible" || p.Rows[0].FITID != "" || p.Coverage == nil || !p.Coverage.PossibleGap {
		t.Fatal("staging/classification/duplicate invariant failed", p)
	}
	snapshot.Reports[0].RunID = "run-two"
	reused, err := e.a.stageFNBTransactions(e.owner, targets, snapshot, "run-two")
	if err != nil || reused[0].ID != p.ID || reused[0].RunID != "run-one" || queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 1 {
		t.Fatal("repeat snapshot lost durable provenance", err)
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{}), 409)
	status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{"decisions": map[string]string{"2": "keep"}, "classification_version": p.ClassificationVersion}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review' AND amount_cents=-29 AND spending_group_id=1") != 2 {
		t.Fatal("amounts, review or identical purchases lost")
	}
	var raw string
	e.a.DB.QueryRow("SELECT provenance FROM transactions ORDER BY id LIMIT 1").Scan(&raw)
	if !strings.Contains(raw, `"run_id":"run-one"`) || !strings.Contains(raw, `"source_reference":"bank-reference"`) || !strings.Contains(raw, `"possible_gap":true`) {
		t.Fatal("source provenance lost", raw)
	}
	committed, err := e.a.stageFNBTransactions(e.owner, targets, snapshot, "run-two")
	if err != nil || !committed[0].AlreadyImported {
		t.Fatal("committed snapshot not idempotent", err)
	}
	var items []map[string]any
	response := e.req(t, 1, "/api/imports", "GET", nil)
	status(t, response, 200)
	json.Unmarshal(response.Body.Bytes(), &items)
	retained := items[0]["preview"].(map[string]any)
	if retained["run_id"] != "run-one" || retained["coverage"] == nil {
		t.Fatal("history lost coverage")
	}
}
func TestFNBLiveRejectsAtomicMismatchHiddenRevocationAndInvalidMoney(t *testing.T) {
	for _, kind := range []string{"mismatch", "run", "currency", "money", "fee", "hidden", "owner", "permission", "bank"} {
		t.Run(kind, func(t *testing.T) {
			e := setup(t)
			targets := transactionTargets(t, e)
			snapshot := fnbSnapshot{Reports: []FNBReport{transactionReport(targets[0].BankID, "run"), transactionReport(targets[1].BankID, "run")}}
			switch kind {
			case "mismatch":
				snapshot.Reports[1].BankID = "999999"
			case "run":
				snapshot.Reports[1].RunID = "wrong"
			case "currency":
				snapshot.Reports[1].Currency = "USD"
			case "money":
				snapshot.Reports[1].Transactions[0].Amount = "1.234"
			case "fee":
				snapshot.Reports[1].Transactions[0].ServiceFee = "-0.10"
			case "hidden":
				e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=2")
			case "owner":
				e.a.DB.Exec("UPDATE users SET disabled=1 WHERE id=1")
			case "permission":
				e.a.DB.Exec("DELETE FROM grants WHERE account_id=2")
			case "bank":
				e.a.DB.Exec("UPDATE accounts SET bank_id='99999' WHERE id=2")
			}
			if _, err := e.a.stageFNBTransactions(e.owner, targets, snapshot, "run"); err == nil {
				t.Fatal("invalid/unauthorized snapshot accepted")
			}
			if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
				t.Fatal("partial write")
			}
		})
	}
}
func TestFNBLiveRouteSerializesFiltersTargetsAndSanitizesFailures(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	transactionTargets(t, e)
	e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=2")
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, debug bool) (fnbSnapshot, error) {
		if len(c.TransactionAccounts) != 1 || c.TransactionAccounts[0] != "12345678901" || c.RunID == "" {
			t.Fatal("hidden account fetched", c.TransactionAccounts)
		}
		status(t, e.req(t, 1, "/api/fnb/refresh", "POST", nil), 409)
		return fnbSnapshot{Reports: []FNBReport{transactionReport(c.TransactionAccounts[0], c.RunID)}}, nil
	}
	status(t, e.req(t, 2, "/api/fnb/transactions", "POST", nil), 403)
	status(t, e.req(t, 1, "/api/fnb/transactions", "POST", nil), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
		t.Fatal("route auto-committed")
	}
	e.a.fnbProvider = func(context.Context, fnbCredentials, bool) (fnbSnapshot, error) {
		return fnbSnapshot{Error: "LOGOUT_REQUIRED", Reports: []FNBReport{transactionReport("12345678901", "bad")}, Diagnostics: map[string]int{"transaction_rows": 2, "synthetic-secret": 1}}, nil
	}
	response := e.req(t, 1, "/api/fnb/transactions", "POST", nil)
	status(t, response, 502)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 1 {
		t.Fatal("failed logout staged data")
	}
	connection := e.req(t, 1, "/api/fnb", "GET", nil)
	if strings.Contains(connection.Body.String(), "synthetic-secret") || !strings.Contains(connection.Body.String(), "action_required") {
		t.Fatal("unsanitized or unsuspended failure")
	}
}
func TestFNBLiveCrossFormatOverlapRemainsCandidate(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	report := transactionReport(targets[0].BankID, "run")
	report.Transactions = report.Transactions[:1]
	e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance,fitid) VALUES(1,'2026-10-01',-29,'Synthetic purchase','2026-10-01',-29,'Synthetic purchase',?,'{}','ofx-identifier')", fingerprint(SourceRow{Date: "2026-10-01", Amount: -29, Description: "Synthetic purchase"}))
	p, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{report}}, "run")
	if err != nil || p[0].Rows[0].Duplicate != "possible" || p[0].Rows[0].FITID != "" {
		t.Fatal("cross-format identity guessed", err)
	}
}

func TestFNBLiveFeesAcrossAccountsCommitOnceWithClassificationAndProvenance(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	transactionTargets(t, e)
	e.a.DB.Exec("INSERT INTO categories(id,name,group_name,kind) VALUES(3,'Bank charges','','expense')")
	for _, id := range []int{1, 2} {
		e.a.DB.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction,spending_group_id) VALUES(1,?,'Service Fees',3,0,'debit',?)", id, queryInt(e.a.DB, "SELECT id FROM spending_groups WHERE name='Bank Fees'"))
	}
	e.a.fnbProvider = func(ctx context.Context, c fnbCredentials, debug bool) (fnbSnapshot, error) {
		if len(c.TransactionAccounts) != 2 {
			t.Fatal("not fetching both accounts")
		}
		snapshot := fnbSnapshot{}
		for _, bank := range c.TransactionAccounts {
			r := transactionReport(bank, c.RunID)
			r.Transactions[0].ServiceFee = "0.05"
			snapshot.Reports = append(snapshot.Reports, r)
		}
		return snapshot, nil
	}
	response := e.req(t, 1, "/api/fnb/transactions", "POST", nil)
	status(t, response, 200)
	var previews []ParsedFile
	json.Unmarshal(response.Body.Bytes(), &previews)
	if len(previews) != 2 {
		t.Fatal("second account omitted")
	}
	for _, p := range previews {
		if len(p.Rows) != 3 || p.Coverage.ServiceFeeRows != 1 || p.Rows[1].CategoryID == nil || *p.Rows[1].CategoryID != 3 {
			t.Fatal("fee classification or rows lost", p)
		}
		status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{"classification_version": p.ClassificationVersion}), 200)
		status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p.ID), "POST", map[string]any{}), 409)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review'") != 6 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != 2400 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE amount_cents=-5") != 2 {
		t.Fatal("fee double counted or approved")
	}
	var provenance string
	e.a.DB.QueryRow("SELECT provenance FROM transactions WHERE amount_cents=-5 LIMIT 1").Scan(&provenance)
	if !strings.Contains(provenance, `"source_component":"service_fee"`) || !strings.Contains(provenance, `"source_service_fee_cents":5`) || !strings.Contains(provenance, `"source_bank_row":1`) {
		t.Fatal("fee provenance lost", provenance)
	}
}
func TestFNBLiveCombinedExportOverlapFlagsBothFeeComponents(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	report := transactionReport(targets[0].BankID, "run")
	report.Transactions = report.Transactions[:1]
	report.Transactions[0].ServiceFee = "0.05"
	e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance) VALUES(1,'2026-10-01',-34,'Synthetic purchase','2026-10-01',-34,'Synthetic purchase',?,'{}')", fingerprint(SourceRow{Date: "2026-10-01", Amount: -34, Description: "Synthetic purchase"}))
	p, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{report}}, "run")
	if err != nil || len(p[0].Rows) != 2 || p[0].Rows[0].Duplicate != "possible" || p[0].Rows[1].Duplicate != "possible" {
		t.Fatal("combined export silently duplicated", err)
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p[0].ID), "POST", map[string]any{}), 409)
	status(t, e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", p[0].ID), "POST", map[string]any{"decisions": map[string]string{"1": "skip", "2": "skip"}}), 200)
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 1 {
		t.Fatal("combined transaction charged again")
	}
}

func TestFNBLiveHomeLoanAmongSevenAccountsStagesWithoutChangingDebtSigns(t *testing.T) {
	e := setup(t)
	accounts := []fnbAccountSnapshot{}
	for i := 0; i < 7; i++ {
		accounts = append(accounts, fnbAccountSnapshot{Name: fmt.Sprintf("Synthetic account %d", i+1), BankID: fmt.Sprintf("%011d", 70000000000+i)})
	}
	if err := e.a.applyFNBSnapshot(e.owner, fnbSnapshot{Accounts: accounts}, "2026-10-03T08:00:00Z"); err != nil {
		t.Fatal(err)
	}
	targets, err := e.a.fnbTargets(e.a.DB, e.owner)
	if err != nil || len(targets) != 7 {
		t.Fatal("targets", err, len(targets))
	}
	reports := []FNBReport{}
	for i, target := range targets {
		r := transactionReport(target.BankID, "loan-run")
		if i == 2 {
			r.AccountType = "Home Loan"
		}
		reports = append(reports, r)
	}
	previews, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: reports}, "loan-run")
	if err != nil || len(previews) != 7 {
		t.Fatal("accounts after loan not staged", err)
	}
	loan := previews[2]
	if loan.AccountType != "Home Loan" || loan.Rows[0].Amount != -29 || loan.Rows[1].Amount != 1234 || loan.Rows[0].FITID != "" || !loan.Coverage.PossibleGap {
		t.Fatal("loan source semantics changed", loan)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports") != 7 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
		t.Fatal("staging created ledger entries or omitted accounts")
	}
}
