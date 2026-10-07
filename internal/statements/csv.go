package statements

import (
	"encoding/csv"
	"io"
	"strings"

	"finance-tracker/internal/money"
)

func parseCSV(p *ParsedFile, s string) {
	p.Currency = "ZAR"
	reader := csv.NewReader(strings.NewReader(s))
	reader.FieldsPerRecord = -1
	reader.TrimLeadingSpace = true
	header := false
	rowNum := 0
	newest := ""
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		rowNum++
		if err == nil && len(record) > 0 {
			rowNum, _ = reader.FieldPos(0)
		}
		if err != nil {
			p.Error = "Malformed CSV: " + err.Error()
			return
		}
		for i := range record {
			record[i] = strings.TrimSpace(record[i])
		}
		if len(record) == 0 || len(record) == 1 && record[0] == "" {
			continue
		}
		if !header {
			if strings.EqualFold(record[0], "Account:") && len(record) > 1 {
				p.AccountID = record[1]
			}
			if len(record) == 4 && strings.EqualFold(record[0], "Date") && strings.EqualFold(record[1], "Amount") && strings.EqualFold(record[2], "Balance") && strings.EqualFold(record[3], "Description") {
				header = true
			}
			continue
		}
		item := SourceRow{Row: rowNum}
		if len(record) != 4 {
			item.Error = "Expected Date, Amount, Balance, Description"
			p.Rows = append(p.Rows, item)
			continue
		}
		item.Date = dateValue(record[0])
		item.Description = record[3]
		amount, err := money.Cents(record[1])
		item.Amount = amount
		if item.Date == "" {
			item.Error = "Invalid transaction date"
		} else if err != nil {
			item.Error = err.Error()
		} else if item.Description == "" {
			item.Error = "Description is missing"
		}
		if record[2] != "" {
			balance, err := money.Cents(record[2])
			if err != nil {
				item.Error = "Invalid bank balance"
			} else {
				item.Balance = &balance
			}
		}
		if item.Error == "" && item.Balance != nil && item.Date > newest {
			newest = item.Date
			p.Balance = item.Balance
			p.BalanceDate = item.Date
		}
		p.Rows = append(p.Rows, item)
		if len(p.Rows) > 100000 {
			p.Error = "File exceeds 100,000 rows"
			return
		}
	}
	if !header {
		p.Error = "FNB CSV header not found"
	}
}
