package app

func prototypeReport() FNBReport {
	balance := "912.34"
	return FNBReport{RunID: "synthetic-run", BankID: "12345678901", Currency: "ZAR", AccountType: "Cheque", Balance: &balance, BalanceDate: "2026-10-02", Transactions: []FNBTransaction{{Date: "2026-10-01", Amount: "-0.29", Description: "Synthetic purchase", Reference: "bank-reference", Status: "posted"}, {Date: "2026-09-30", Amount: "12.34", Description: "Synthetic refund", Status: "posted"}}}
}
