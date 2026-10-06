package app

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"fmt"
	"golang.org/x/text/encoding/charmap"
	"html"
	"io"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type SourceRow struct {
	TransactionID         int64                `json:"transaction_id,omitempty"`
	SourceBankRow         int                  `json:"source_bank_row,omitempty"`
	SourceComponent       string               `json:"source_component,omitempty"`
	SourceBankDescription string               `json:"source_bank_description,omitempty"`
	SourceServiceFee      *int64               `json:"source_service_fee_cents,omitempty"`
	SourceNetKey          string               `json:"source_net_key,omitempty"`
	SourceReference       string               `json:"source_reference,omitempty"`
	SpendingGroupID       *int64               `json:"spending_group_id,omitempty"`
	RuleMatchCount        int                  `json:"rule_match_count,omitempty"`
	RuleMatches           []classificationRule `json:"rule_matches,omitempty"`
	RuleConflict          bool                 `json:"rule_conflict,omitempty"`
	Row                   int                  `json:"row"`
	Date                  string               `json:"date"`
	Amount                int64                `json:"amount_cents"`
	Description           string               `json:"description"`
	FITID                 string               `json:"fitid,omitempty"`
	Balance               *int64               `json:"balance_cents,omitempty"`
	CategoryID            *int64               `json:"category_id,omitempty"`
	MerchantID            *int64               `json:"merchant_id,omitempty"`
	MerchantName          string               `json:"merchant_name,omitempty"`
	Suggestion            string               `json:"suggestion,omitempty"`
	Error                 string               `json:"error,omitempty"`
	Duplicate             string               `json:"duplicate,omitempty"`
	Candidates            []map[string]any     `json:"candidates,omitempty"`
}
type ParsedFile struct {
	AccountName           string            `json:"account_name,omitempty"`
	RunID                 string            `json:"run_id,omitempty"`
	Coverage              *FNBCoverage      `json:"coverage,omitempty"`
	ClassificationVersion string            `json:"classification_version,omitempty"`
	Decisions             map[string]string `json:"decisions,omitempty"`
	Inserted              int               `json:"inserted"`
	Skipped               int               `json:"skipped"`
	ID                    int64             `json:"id"`
	Name                  string            `json:"name"`
	Hash                  string            `json:"hash"`
	Format                string            `json:"format"`
	AccountID             string            `json:"bank_id"`
	Currency              string            `json:"currency"`
	AccountType           string            `json:"account_type,omitempty"`
	StartDate             string            `json:"start_date,omitempty"`
	EndDate               string            `json:"end_date,omitempty"`
	Balance               *int64            `json:"balance_cents,omitempty"`
	BalanceDate           string            `json:"balance_date,omitempty"`
	Rows                  []SourceRow       `json:"rows"`
	Error                 string            `json:"error,omitempty"`
	AlreadyImported       bool              `json:"already_imported"`
}
type inputFile struct {
	Name    string
	Content []byte
}

func expand(files []inputFile) ([]inputFile, error) {
	result := []inputFile{}
	var total int64
	for _, f := range files {
		if strings.EqualFold(path.Ext(f.Name), ".zip") {
			z, err := zip.NewReader(bytes.NewReader(f.Content), int64(len(f.Content)))
			if err != nil {
				return nil, fail(400, "Invalid ZIP archive")
			}
			if len(z.File) > 100 {
				return nil, fail(400, "ZIP contains too many entries")
			}
			for _, entry := range z.File {
				if entry.FileInfo().IsDir() {
					continue
				}
				name := entry.Name
				if strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || strings.Contains(name, "..") || entry.Mode()&os.ModeSymlink != 0 {
					return nil, fail(400, "ZIP contains an unsafe entry")
				}
				ext := strings.ToLower(path.Ext(name))
				if ext != ".csv" && ext != ".ofx" {
					continue
				}
				if entry.UncompressedSize64 > 100<<20 {
					return nil, fail(400, "ZIP expansion exceeds 100 MiB")
				}
				rc, err := entry.Open()
				if err != nil {
					return nil, err
				}
				b, err := io.ReadAll(io.LimitReader(rc, (100<<20)-total+1))
				rc.Close()
				if err != nil {
					return nil, fail(400, "Could not read ZIP entry")
				}
				total += int64(len(b))
				if total > 100<<20 {
					return nil, fail(400, "ZIP expansion exceeds 100 MiB")
				}
				result = append(result, inputFile{path.Base(name), b})
			}
		} else {
			ext := strings.ToLower(path.Ext(f.Name))
			if ext != ".csv" && ext != ".ofx" {
				return nil, fail(400, "Upload FNB CSV, OFX, or ZIP files")
			}
			total += int64(len(f.Content))
			result = append(result, f)
		}
		if len(result) > 20 {
			return nil, fail(400, "Upload at most 20 CSV/OFX files per batch")
		}
	}
	if len(result) == 0 {
		return nil, fail(400, "No CSV or OFX files found")
	}
	return result, nil
}
func text(b []byte) string {
	b = bytes.TrimPrefix(b, []byte{0xef, 0xbb, 0xbf})
	if !utf8.Valid(b) {
		v, _ := charmap.Windows1252.NewDecoder().Bytes(b)
		return string(v)
	}
	return string(b)
}

var decimalRE = regexp.MustCompile(`^[+-]?[0-9]+(\.[0-9]{1,2})?$`)

func Cents(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if !decimalRE.MatchString(s) {
		return 0, fmt.Errorf("invalid money: use a signed decimal with at most two fractional digits")
	}
	negative := strings.HasPrefix(s, "-")
	s = strings.TrimLeft(s, "+-")
	parts := strings.SplitN(s, ".", 2)
	whole, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || whole > 9000000000000 {
		return 0, fmt.Errorf("amount is too large")
	}
	fraction := int64(0)
	if len(parts) > 1 {
		v := parts[1]
		if len(v) == 1 {
			v += "0"
		}
		fraction, _ = strconv.ParseInt(v, 10, 64)
	}
	v := whole*100 + fraction
	if negative {
		v = -v
	}
	return v, nil
}
func dateValue(s string) string {
	s = strings.TrimSpace(s)
	for _, format := range []string{"2006/01/02", "2006-01-02", "20060102"} {
		if t, err := time.Parse(format, s); err == nil {
			return t.Format("2006-01-02")
		}
	}
	if regexp.MustCompile(`^[0-9]{8}([0-9]{6}(\.[0-9]+)?(\[[-+][0-9]+(:[A-Z]+)?\])?)?$`).MatchString(s) {
		if t, err := time.Parse("20060102", s[:8]); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return ""
}
func parseFile(f inputFile) ParsedFile {
	p := ParsedFile{Name: f.Name, Hash: hash(string(f.Content)), Rows: []SourceRow{}}
	switch strings.ToLower(path.Ext(f.Name)) {
	case ".csv":
		p.Format = "csv"
		parseCSV(&p, text(f.Content))
	case ".ofx":
		p.Format = "ofx"
		parseOFX(&p, text(f.Content))
	default:
		p.Error = "Unsupported file format"
	}
	if p.AccountID == "" && p.Error == "" {
		p.Error = "Bank account identity is missing"
	}
	if p.Currency != "ZAR" && p.Error == "" {
		p.Error = "Only ZAR imports are supported"
	}
	if len(p.Rows) == 0 && p.Error == "" {
		p.Error = "No transactions found"
	}
	return p
}
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
		amount, err := Cents(record[1])
		item.Amount = amount
		if item.Date == "" {
			item.Error = "Invalid transaction date"
		} else if err != nil {
			item.Error = err.Error()
		} else if item.Description == "" {
			item.Error = "Description is missing"
		}
		if record[2] != "" {
			balance, err := Cents(record[2])
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
		amount, err := Cents(values["TRNAMT"])
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
		if n, err := Cents(v["BALAMT"]); err == nil {
			p.Balance = &n
			p.BalanceDate = dateValue(v["DTASOF"])
		} else {
			p.Error = "Invalid OFX ledger balance"
		}
	}
}
