package statements

import (
	"strings"
	"testing"
)

func prototypeReport() FNBReport {
	balance := "912.34"
	return FNBReport{RunID: "synthetic-run", BankID: "12345678901", Currency: "ZAR", AccountType: "Cheque", Balance: &balance, BalanceDate: "2026-10-02", Transactions: []FNBTransaction{{Date: "2026-10-01", Amount: "-0.29", Description: "Synthetic purchase", Reference: "bank-reference", Status: "posted"}, {Date: "2026-09-30", Amount: "12.34", Description: "Synthetic refund", Status: "posted"}}}
}
func TestFNBPrototypeExactMoneyCoverageAndReferences(t *testing.T) {
	r := prototypeReport()
	out, err := NormalizeFNBReport(r, r.BankID)
	if err != nil {
		t.Fatal(err)
	}
	if out.File.Rows[0].Amount != -29 || out.File.Rows[1].Amount != 1234 || *out.File.Balance != 91234 {
		t.Fatal("money changed")
	}
	if out.File.Rows[0].FITID != "" || out.References[1] != "bank-reference" {
		t.Fatal("reference treated as statement identity")
	}
	if out.Coverage.ReturnedStart != "2026-09-30" || out.Coverage.ReturnedEnd != "2026-10-01" || !out.Coverage.PossibleGap || out.Coverage.PageLimitReached {
		t.Fatal(out.Coverage)
	}
	// Identical legitimate purchases are preserved, never deduplicated here.
	r.Transactions = []FNBTransaction{}
	for i := 0; i < 150; i++ {
		r.Transactions = append(r.Transactions, prototypeReport().Transactions[0])
	}
	out, err = NormalizeFNBReport(r, r.BankID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.File.Rows) != 150 || !out.Coverage.PageLimitReached {
		t.Fatal("page limit not signalled")
	}
	r.Transactions = nil
	out, err = NormalizeFNBReport(r, r.BankID)
	if err != nil || !out.Coverage.PossibleGap || out.Coverage.ReturnedStart != "" {
		t.Fatal("empty response asserted coverage")
	}
}
func TestFNBPrototypeRejectsInvalidResponsesWithoutEchoingData(t *testing.T) {
	cases := []func(*FNBReport){
		func(r *FNBReport) { r.BankID = "mismatch" }, func(r *FNBReport) { r.Currency = "USD" }, func(r *FNBReport) { r.AccountType = "Vehicle" }, func(r *FNBReport) { r.RunID = "" },
		func(r *FNBReport) { r.Transactions[0].Date = "2026-02-30" }, func(r *FNBReport) { r.Transactions[0].Amount = "1.001" }, func(r *FNBReport) { r.Transactions[0].Amount = "1e2" }, func(r *FNBReport) { r.Transactions[0].Amount = "9223372036854775807" }, func(r *FNBReport) { r.Transactions[0].Status = "pending" }, func(r *FNBReport) { r.Transactions[0].ServiceFee = "-1.00" }, func(r *FNBReport) { r.BalanceDate = "" },
	}
	for _, change := range cases {
		r := prototypeReport()
		change(&r)
		out, err := NormalizeFNBReport(r, "12345678901")
		if err == nil {
			t.Fatal("invalid response accepted")
		}
		if len(out.File.Rows) != 0 || strings.Contains(err.Error(), "Synthetic") || strings.Contains(err.Error(), "12345678901") {
			t.Fatal("partial results or source leaked")
		}
	}
}

func TestFNBPendingRowsStillSignalPageLimit(t *testing.T) {
	r := prototypeReport()
	r.PageRows = 150
	out, err := NormalizeFNBReport(r, r.BankID)
	if err != nil || !out.Coverage.PageLimitReached || out.Coverage.PendingRows != 148 {
		t.Fatal("filtered pending rows hid the page limit", err)
	}
	r.PageRows = 1
	if _, err = NormalizeFNBReport(r, r.BankID); err == nil {
		t.Fatal("inconsistent page count accepted")
	}
}

func TestFNBFeeComponentsPreserveDebitCreditTotalsAndCoverage(t *testing.T) {
	r := prototypeReport()
	r.Transactions[0].ServiceFee = "0.01"
	r.Transactions[1].ServiceFee = "1.25"
	r.PageRows = 150
	out, err := NormalizeFNBReport(r, r.BankID)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.File.Rows) != 4 || out.Coverage.ReturnedRows != 2 || out.Coverage.ServiceFeeRows != 2 || !out.Coverage.PageLimitReached {
		t.Fatal("fees distorted bank coverage", out.Coverage)
	}
	var total int64
	for i, row := range out.File.Rows {
		total += row.Amount
		if row.Row != i+1 || row.FITID != "" {
			t.Fatal("identity or row ordering changed")
		}
	}
	if total != 1079 || out.File.Rows[0].Amount != -29 || out.File.Rows[1].Amount != -1 || out.File.Rows[2].Amount != 1234 || out.File.Rows[3].Amount != -125 {
		t.Fatal("amount minus fee not preserved", total)
	}
	fee := out.File.Rows[1]
	if fee.SourceBankRow != 1 || fee.SourceComponent != "service_fee" || fee.SourceBankDescription != "Synthetic purchase" || fee.SourceServiceFee == nil || *fee.SourceServiceFee != 1 || fee.SourceReference != "bank-reference" || fee.Description != "Service Fees" {
		t.Fatal("fee source association lost", fee)
	}
}
