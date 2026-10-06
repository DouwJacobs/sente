package app

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestFNBAutomaticOverlappingPagesPreserveMultiplicityFeesAndClassification(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	e.a.DB.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction) VALUES(1,1,'Synthetic purchase',1,0,'debit')")
	run := func(r FNBReport) ParsedFile {
		t.Helper()
		ps, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{r}}, r.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.a.autoCommitFNB(e.owner, ps); err != nil {
			t.Fatal(err)
		}
		return ps[0]
	}
	r := transactionReport(targets[0].BankID, "one")
	r.Transactions = r.Transactions[:1]
	r.Transactions[0].ServiceFee = "0.05"
	r.Transactions[0].Balance = fnbDecimal("100.00")
	p := run(r)
	if p.Inserted != 2 || p.Skipped != 0 {
		t.Fatal("first run", p.Inserted, p.Skipped)
	}
	r.RunID = "two"
	p = run(r)
	if p.Inserted != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 2 {
		t.Fatal("identical snapshot replay")
	}
	// A real second purchase with the same date/amount/description/reference,
	// but a different running balance, shifts the old row's page position.
	newPurchase := r.Transactions[0]
	newPurchase.Balance = fnbDecimal("99.66")
	r.RunID = "three"
	r.Transactions = append([]FNBTransaction{newPurchase}, r.Transactions...)
	p = run(r)
	if p.Inserted != 2 || p.Skipped != 2 {
		t.Fatal("overlap lost identical purchase or duplicated fee", p.Inserted, p.Skipped)
	}
	// Simulate legacy fee provenance without a copied running balance.
	e.a.DB.Exec("UPDATE transactions SET provenance=json_set(provenance,'$.balance_cents',NULL) WHERE json_extract(provenance,'$.source_component')='service_fee'")
	// User edits must never alter source identity used for repeat matching.
	e.a.DB.Exec("UPDATE transactions SET description='Owner edited',date='2026-10-02' WHERE id=1")
	r.RunID = "four"
	r.Transactions = append(r.Transactions, FNBTransaction{Date: "2026-10-02", Amount: "1.00", Description: "Unclassified refund", Status: "posted"})
	p = run(r)
	if p.Inserted != 1 || p.Skipped != 4 {
		t.Fatal("source identity after edit", p.Inserted, p.Skipped)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 2 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='pending_review'") != 3 {
		t.Fatal("only missing categories should need review")
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != 32 {
		t.Fatal("principal/fee totals wrong")
	}
	// Dropping older rows from a rolling page must not bring them back twice.
	r.RunID = "five"
	r.Transactions = r.Transactions[1:]
	p = run(r)
	if p.Inserted != 0 || p.Skipped != 3 {
		t.Fatal("rolling page duplicated transactions", p.Inserted, p.Skipped)
	}
}

func TestFNBAutomaticIdenticalRowsWithoutBalanceCountOccurrences(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	r := transactionReport(targets[0].BankID, "one")
	r.Transactions = []FNBTransaction{r.Transactions[0], r.Transactions[0]}
	for n := 2; n <= 4; n++ {
		r.RunID = fmt.Sprint(n)
		ps, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{r}}, r.RunID)
		if err != nil {
			t.Fatal(err)
		}
		if err = e.a.autoCommitFNB(e.owner, ps); err != nil {
			t.Fatal(err)
		}
		if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != int64(n) {
			t.Fatal("identical occurrences lost or duplicated", n)
		}
		r.Transactions = append(r.Transactions, r.Transactions[0])
	}
}

func TestFNBScheduledTransactionsAutomaticallyCommitWithoutUI(t *testing.T) {
	e := setup(t)
	connectFNBTest(t, e)
	transactionTargets(t, e)
	e.a.DB.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction) VALUES(1,1,'Synthetic purchase',1,0,'debit')")
	calls := 0
	e.a.fnbProvider = func(_ context.Context, c fnbCredentials, _ bool) (fnbSnapshot, error) {
		calls++
		if len(c.TransactionAccounts) != 2 {
			t.Fatal("scheduled accounts not requested")
		}
		s := fnbSnapshot{}
		for _, bank := range c.TransactionAccounts {
			s.Accounts = append(s.Accounts, fnbAccountSnapshot{Name: "Synthetic", BankID: bank, Balance: fnbDecimal("10.00")})
			r := transactionReport(bank, c.RunID)
			if calls > 1 {
				r.Transactions = append(r.Transactions, FNBTransaction{Date: "2026-10-02", Amount: "2.00", Description: "New scheduled transaction", Status: "posted"})
			}
			s.Reports = append(s.Reports, r)
		}
		return s, nil
	}
	for i := 0; i < 3; i++ {
		e.a.DB.Exec("UPDATE fnb_connections SET interval_hours=24,next_due=1")
		e.a.refreshDueFNB(time.Now())
	}
	if calls != 3 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 6 || queryInt(e.a.DB, "SELECT COUNT(*) FROM imports WHERE status='staged'") != 0 {
		t.Fatal("scheduler needs UI or duplicates rows")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 1 {
		t.Fatal("scheduled rule classification not accepted")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM accounts WHERE balance_cents=1000") != 2 {
		t.Fatal("scheduled balances not updated")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE state='ready' AND next_due>0 AND last_success IS NOT NULL") != 1 {
		t.Fatal("schedule not checkpointed")
	}
}

func TestFNBAutoCommitRechecksOwnerPermissionAndRules(t *testing.T) {
	for _, kind := range []string{"owner", "permission", "hidden", "rules"} {
		t.Run(kind, func(t *testing.T) {
			e := setup(t)
			targets := transactionTargets(t, e)[:1]
			if kind == "permission" {
				targets, _ = e.a.fnbTargets(e.a.DB, e.owner)
				targets = targets[1:2]
			}
			r := transactionReport(targets[0].BankID, "run")
			ps, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{r}}, r.RunID)
			if err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "owner":
				e.a.DB.Exec("UPDATE users SET disabled=1 WHERE id=1")
			case "permission":
				e.a.DB.Exec("DELETE FROM grants WHERE account_id=2")
			case "hidden":
				e.a.DB.Exec("UPDATE accounts SET sync_hidden=1 WHERE id=1")
			case "rules":
				e.a.DB.Exec("INSERT INTO rules(user_id,account_id,pattern,category_id,priority,direction) VALUES(1,1,'Synthetic purchase',1,0,'debit')")
			}
			err = e.a.autoCommitFNB(e.owner, ps)
			if kind == "rules" {
				if err != nil || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions WHERE review_state='approved'") != 1 {
					t.Fatal("current rules not applied", err)
				}
			} else if err == nil || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
				t.Fatal("revoked automatic write")
			}
		})
	}
}

func TestFNBAutomaticBatchRollsBackWhenLaterAccountAccessRevoked(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)
	s := fnbSnapshot{}
	for _, target := range targets {
		s.Reports = append(s.Reports, transactionReport(target.BankID, "run"))
	}
	ps, err := e.a.stageFNBTransactions(e.owner, targets, s, "run")
	if err != nil {
		t.Fatal(err)
	}
	e.a.DB.Exec("DELETE FROM grants WHERE account_id=2")
	if err = e.a.autoCommitFNB(e.owner, ps); err == nil {
		t.Fatal("revoked account imported")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM imports WHERE status='committed'") != 0 {
		t.Fatal("automatic batch partially committed")
	}
}

func TestFNBLiveIgnoresUnkeyedAndOtherAccountTransactions(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance) VALUES(1,'2026-10-01',-29,'Unrelated manual entry','2026-10-01',-29,'Unrelated manual entry','','{}')")
	e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,source_key,provenance) VALUES(2,'2026-10-01',-29,'Synthetic purchase','2026-10-01',-29,'Synthetic purchase',?,'{}')", fingerprint(SourceRow{Date: "2026-10-01", Amount: -29, Description: "Synthetic purchase"}))
	r := transactionReport(targets[0].BankID, "run")
	ps, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{r}}, "run")
	if err != nil {
		t.Fatal(err)
	}
	if err = e.a.autoCommitFNB(e.owner, ps); err != nil {
		t.Fatal(err)
	}
	if ps[0].Inserted != 2 || ps[0].Skipped != 0 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 4 {
		t.Fatal("unrelated entry treated as a duplicate")
	}
}

func TestFNBConcurrentOverlappingAutomaticImportsCommitEachOccurrenceOnce(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)[:1]
	first := transactionReport(targets[0].BankID, "first")
	second := transactionReport(targets[0].BankID, "second")
	second.Transactions = append(second.Transactions, FNBTransaction{Date: "2026-10-02", Amount: "1.00", Description: "New purchase", Status: "posted"})
	one, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{first}}, "first")
	if err != nil {
		t.Fatal(err)
	}
	two, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: []FNBReport{second}}, "second")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 2)
	go func() { done <- e.a.autoCommitFNB(e.owner, one) }()
	go func() { done <- e.a.autoCommitFNB(e.owner, two) }()
	for i := 0; i < 2; i++ {
		if err = <-done; err != nil {
			t.Fatal(err)
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 3 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != 1305 {
		t.Fatal("concurrent overlap duplicated or lost money")
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM imports WHERE status='committed'") != 2 {
		t.Fatal("concurrent run not durably completed")
	}
}
