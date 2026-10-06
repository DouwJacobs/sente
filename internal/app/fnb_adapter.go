package app

// The worker extracts exact decimal bank text. References are provenance,
// never assumed to be stable statement FITIDs.
import (
	"fmt"
	"strings"
	"time"
)

type FNBTransaction struct {
	Date        string  `json:"date"`
	Amount      string  `json:"amount_decimal"`
	Description string  `json:"description"`
	Reference   string  `json:"source_reference,omitempty"`
	Status      string  `json:"status"`
	Balance     *string `json:"balance_decimal,omitempty"`
	ServiceFee  string  `json:"service_fee_decimal,omitempty"`
}
type FNBReport struct {
	PageRows     int              `json:"page_rows,omitempty"`
	RunID        string           `json:"run_id"`
	BankID       string           `json:"bank_id"`
	Currency     string           `json:"currency"`
	AccountType  string           `json:"account_type"`
	Balance      *string          `json:"balance_decimal,omitempty"`
	BalanceDate  string           `json:"balance_date,omitempty"`
	Transactions []FNBTransaction `json:"transactions"`
}
type FNBCoverage struct {
	ServiceFeeRows   int    `json:"service_fee_rows,omitempty"`
	PendingRows      int    `json:"pending_rows,omitempty"`
	ReturnedStart    string `json:"returned_start,omitempty"`
	ReturnedEnd      string `json:"returned_end,omitempty"`
	ReturnedRows     int    `json:"returned_rows"`
	PossibleGap      bool   `json:"possible_gap"`
	PageLimitReached bool   `json:"page_limit_reached"`
}
type FNBNormalizedRun struct {
	RunID      string         `json:"run_id"`
	File       ParsedFile     `json:"normalized"`
	References map[int]string `json:"source_references"`
	Coverage   FNBCoverage    `json:"coverage"`
}

func bankDate(value string) bool {
	d, err := time.Parse("2006-01-02", value)
	return err == nil && d.Format("2006-01-02") == value
}

// NormalizeFNBReport validates against an explicitly selected account identity.
// The caller must separately authorize the mapped tracker account at execution.
func NormalizeFNBReport(report FNBReport, selectedBankID string) (FNBNormalizedRun, error) {
	var out FNBNormalizedRun
	if strings.TrimSpace(report.RunID) == "" || len(report.RunID) > 128 {
		return out, fmt.Errorf("invalid connector run identity")
	}
	if selectedBankID == "" || report.BankID != selectedBankID {
		return out, fmt.Errorf("connector account mismatch")
	}
	if fnbMaskedCredit.MatchString(report.BankID) && report.AccountType != "Credit" {
		return out, fmt.Errorf("masked connector identity requires credit account")
	}
	if report.Currency != "ZAR" {
		return out, fmt.Errorf("unsupported connector currency")
	}
	switch report.AccountType {
	case "Cheque", "Savings", "Credit", "Easy", "Home Loan":
	default:
		return out, fmt.Errorf("unsupported connector account type")
	}
	if report.PageRows < 0 || (report.PageRows != 0 && report.PageRows < len(report.Transactions)) || report.PageRows > 150 {
		return out, fmt.Errorf("invalid connector page count")
	}
	if len(report.Transactions) > 100000 {
		return out, fmt.Errorf("connector row limit exceeded")
	}
	out = FNBNormalizedRun{RunID: report.RunID, References: map[int]string{}, Coverage: FNBCoverage{ReturnedRows: len(report.Transactions), PossibleGap: true, PageLimitReached: len(report.Transactions) >= 150 || report.PageRows >= 150}, File: ParsedFile{Format: "fnb-live", AccountID: report.BankID, Currency: report.Currency, AccountType: report.AccountType, Rows: []SourceRow{}}}
	if report.PageRows > len(report.Transactions) {
		out.Coverage.PendingRows = report.PageRows - len(report.Transactions)
	}
	if report.Balance != nil {
		if !bankDate(report.BalanceDate) {
			return FNBNormalizedRun{}, fmt.Errorf("invalid connector balance date")
		}
		balance, err := Cents(*report.Balance)
		if err != nil {
			return FNBNormalizedRun{}, fmt.Errorf("invalid connector balance")
		}
		out.File.Balance = &balance
		out.File.BalanceDate = report.BalanceDate
	} else if report.BalanceDate != "" {
		return FNBNormalizedRun{}, fmt.Errorf("connector balance date has no balance")
	}
	for i, row := range report.Transactions {
		invalid := func(message string) (FNBNormalizedRun, error) {
			return FNBNormalizedRun{}, fmt.Errorf("connector row %d: %s", i+1, message)
		}
		if row.Status != "posted" {
			return invalid("only verified posted entries are supported")
		}
		if !bankDate(row.Date) {
			return invalid("invalid bank date")
		}
		amount, err := Cents(row.Amount)
		if err != nil {
			return invalid("invalid exact amount")
		}
		if strings.TrimSpace(row.Description) == "" || len(row.Description) > 4000 || len(row.Reference) > 256 {
			return invalid("invalid source metadata")
		}
		var fee int64
		if row.ServiceFee != "" {
			fee, err = Cents(row.ServiceFee)
			if err != nil {
				return invalid("invalid exact fee")
			}
			if fee < 0 {
				return invalid("service fee must be nonnegative")
			}
		}
		normalized := SourceRow{Row: len(out.File.Rows) + 1, Date: row.Date, Amount: amount, Description: row.Description, SourceReference: row.Reference, SourceBankRow: i + 1, SourceComponent: "transaction", SourceBankDescription: row.Description}
		if fee != 0 {
			normalized.SourceServiceFee = &fee
			normalized.SourceNetKey = fingerprint(SourceRow{Date: row.Date, Amount: amount - fee, Description: row.Description})
		}
		if row.Balance != nil {
			balance, err := Cents(*row.Balance)
			if err != nil {
				return invalid("invalid exact balance")
			}
			normalized.Balance = &balance
		}
		out.File.Rows = append(out.File.Rows, normalized)
		if row.Reference != "" {
			out.References[normalized.Row] = row.Reference
		}
		if fee != 0 {
			feeRow := SourceRow{Row: len(out.File.Rows) + 1, Date: row.Date, Amount: -fee, Description: "Service Fees", SourceReference: row.Reference, SourceBankRow: i + 1, SourceComponent: "service_fee", SourceBankDescription: row.Description, SourceServiceFee: &fee, SourceNetKey: normalized.SourceNetKey, Balance: normalized.Balance}
			out.File.Rows = append(out.File.Rows, feeRow)
			if row.Reference != "" {
				out.References[feeRow.Row] = row.Reference
			}
			out.Coverage.ServiceFeeRows++
		}
		if out.Coverage.ReturnedStart == "" || row.Date < out.Coverage.ReturnedStart {
			out.Coverage.ReturnedStart = row.Date
		}
		if row.Date > out.Coverage.ReturnedEnd {
			out.Coverage.ReturnedEnd = row.Date
		}
	}
	out.File.StartDate = out.Coverage.ReturnedStart
	out.File.EndDate = out.Coverage.ReturnedEnd
	return out, nil
}
