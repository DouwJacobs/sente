package app

import (
	"encoding/json"
	"testing"
)

func TestAccountsExposeImportTypeWithinAccountPermissions(t *testing.T) {
	e := setup(t)
	targets := transactionTargets(t, e)
	reports := []FNBReport{transactionReport(targets[0].BankID, "typed-run"), transactionReport(targets[1].BankID, "typed-run")}
	reports[1].AccountType = "Credit"
	if _, err := e.a.stageFNBTransactions(e.owner, targets, fnbSnapshot{Reports: reports}, "typed-run"); err != nil {
		t.Fatal(err)
	}
	response := e.req(t, 1, "/api/accounts?page=0", "GET", nil)
	status(t, response, 200)
	var page struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	types := map[string]string{}
	for _, item := range page.Items {
		types[item["bank_id"].(string)] = item["account_type"].(string)
	}
	if types[targets[0].BankID] != "Cheque" || types[targets[1].BankID] != "Credit" {
		t.Fatal("verified types missing", types)
	}
	e.a.DB.Exec("DELETE FROM grants WHERE account_id=? AND user_id=1", targets[1].ID)
	response = e.req(t, 1, "/api/accounts?page=0", "GET", nil)
	status(t, response, 200)
	if err := json.Unmarshal(response.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	for _, item := range page.Items {
		if item["bank_id"] == targets[1].BankID {
			t.Fatal("unauthorized type disclosed")
		}
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 0 {
		t.Fatal("metadata committed transactions")
	}
}
