package statements

import (
	"html"
	"regexp"
	"strings"

	"finance-tracker/internal/money"
)

var ofxFields = regexp.MustCompile(`(?is)<([A-Z0-9_.]+)>([^<\r\n]*)`)

func fields(s string) map[string]string {
	v := map[string]string{}
	for _, m := range ofxFields.FindAllStringSubmatch(s, -1) {
		v[strings.ToUpper(m[1])] = html.UnescapeString(strings.TrimSpace(m[2]))
	}
	return v
}

var ofxTransactions = regexp.MustCompile(`(?is)<STMTTRN>(.*?)</STMTTRN>`)
var ledgerBlock = regexp.MustCompile(`(?is)<LEDGERBAL>(.*?)</LEDGERBAL>`)

func parseOFX(p *ParsedFile, s string) {
	f := fields(s)
	p.AccountID = f["ACCTID"]
	p.Currency = f["CURDEF"]
	p.AccountType = f["ACCTTYPE"]
	p.StartDate = dateValue(f["DTSTART"])
	p.EndDate = dateValue(f["DTEND"])
	if f["CODE"] != "" && f["CODE"] != "0" {
		p.Error = "OFX reports a bank error"
		return
	}
	blocks := ofxTransactions.FindAllStringSubmatch(s, -1)
	if len(blocks) > 100000 {
		p.Error = "File exceeds 100,000 transactions"
		return
	}
	for i, b := range blocks {
		values := fields(b[1])
		row := SourceRow{Row: i + 1, Date: dateValue(values["DTPOSTED"]), FITID: values["FITID"], Description: values["MEMO"]}
		if row.Description == "" {
			row.Description = values["NAME"]
		}
		amount, err := money.Cents(values["TRNAMT"])
		row.Amount = amount
		if row.Date == "" {
			row.Error = "Invalid transaction date"
		} else if err != nil {
			row.Error = err.Error()
		} else if row.Description == "" {
			row.Error = "Description is missing"
		} else if row.FITID == "" {
			row.Error = "OFX transaction ID is missing"
		}
		if (values["TRNTYPE"] == "DEBIT" && amount > 0) || (values["TRNTYPE"] == "CREDIT" && amount < 0) {
			row.Error = "Transaction type conflicts with signed amount"
		}
		p.Rows = append(p.Rows, row)
	}
	if m := ledgerBlock.FindStringSubmatch(s); m != nil {
		v := fields(m[1])
		if n, err := money.Cents(v["BALAMT"]); err == nil {
			p.Balance = &n
			p.BalanceDate = dateValue(v["DTASOF"])
		} else {
			p.Error = "Invalid OFX ledger balance"
		}
	}
}
