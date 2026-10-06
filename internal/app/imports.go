package app

import (
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"
)

func fingerprint(row SourceRow) string {
	return hash(row.Date + "|" + strconv.FormatInt(row.Amount, 10) + "|" + normalize(row.Description))
}
func normalize(s string) string { return strings.ToLower(strings.Join(strings.Fields(s), " ")) }
func (a *App) annotate(q queryer, account int64, p *ParsedFile) error {
	if err := q.QueryRow("SELECT name FROM accounts WHERE id=?", account).Scan(&p.AccountName); err != nil {
		return err
	}
	p.AlreadyImported = queryInt(q, "SELECT COUNT(*) FROM imports WHERE account_id=? AND hash=? AND status='committed'", account, p.Hash) > 0
	rules, err := loadRules(q, account)
	if err != nil {
		return err
	}
	ruleState, _ := json.Marshal(rules)
	p.ClassificationVersion = hash(string(ruleState))
	seen := map[string]SourceRow{}
	fingerprints := map[string][]map[string]any{}
	for i := range p.Rows {
		row := &p.Rows[i]
		row.Duplicate = ""
		row.Candidates = nil
		if row.Error != "" {
			continue
		}
		row.CategoryID = nil
		row.SpendingGroupID = nil
		row.MerchantID = nil
		row.MerchantName = ""
		row.Suggestion = ""
		mm, err := matchMerchantDetails(q, account, row.Description, row.Amount)
		if err != nil {
			return err
		}
		if mm != nil {
			row.MerchantID = &mm.ID
			row.MerchantName = mm.Name
			if mm.CategoryID != nil {
				row.CategoryID = mm.CategoryID
				row.SpendingGroupID = mm.SpendingGroupID
				row.Suggestion = "Merchant: " + mm.Name
			}
		}
		if row.CategoryID == nil {
			classify(row, rules)
		}
		if p.Format == "fnb-live" && row.FITID == "" {
			continue
		}
		if row.FITID != "" {
			matches, err := data(q, "SELECT id,source_date date,source_amount amount_cents,source_description description FROM transactions WHERE account_id=? AND fitid=?", account, row.FITID)
			if err != nil {
				return err
			}
			if len(matches) > 0 {
				row.Candidates = matches
				v := matches[0]
				if v["date"] == row.Date && num(v["amount_cents"]) == row.Amount && normalize(v["description"].(string)) == normalize(row.Description) {
					row.Duplicate = "exact_id"
				} else {
					row.Duplicate = "conflict"
				}
			}
			if prior, ok := seen[row.FITID]; ok {
				row.Duplicate = "exact_id"
				if prior.Date != row.Date || prior.Amount != row.Amount || normalize(prior.Description) != normalize(row.Description) {
					row.Duplicate = "conflict"
				}
				row.Candidates = append(row.Candidates, map[string]any{"row": prior.Row, "date": prior.Date, "amount_cents": prior.Amount, "description": prior.Description})
			}
			seen[row.FITID] = *row
		}
		if row.Duplicate == "" {
			matches, err := data(q, "SELECT id,source_date date,source_amount amount_cents,source_description description FROM transactions WHERE account_id=? AND source_key=? LIMIT 5", account, fingerprint(*row))
			if err != nil {
				return err
			}
			for _, m := range matches {
				if normalize(m["description"].(string)) == normalize(row.Description) {
					row.Candidates = append(row.Candidates, m)
				}
			}
			// Some exports combine the bank amount and fee into one debit.
			// Flag that record on both components so neither silently duplicates it.
			if row.SourceNetKey != "" && row.SourceNetKey != fingerprint(*row) {
				netMatches, err := data(q, "SELECT id,source_date date,source_amount amount_cents,source_description description FROM transactions WHERE account_id=? AND source_key=? LIMIT 5", account, row.SourceNetKey)
				if err != nil {
					return err
				}
				row.Candidates = append(row.Candidates, netMatches...)
			}
			row.Candidates = append(row.Candidates, fingerprints[fingerprint(*row)]...)
			if len(row.Candidates) > 0 {
				row.Duplicate = "possible"
			}
		}
		key := fingerprint(*row)
		if len(fingerprints[key]) < 5 {
			fingerprints[key] = append(fingerprints[key], map[string]any{"row": row.Row, "date": row.Date, "amount_cents": row.Amount, "description": row.Description})
		}
	}
	if p.Format == "fnb-live" {
		return a.annotateLiveDuplicates(q, account, p)
	}
	return nil
}
func (a *App) previewImport(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	r.Body = http.MaxBytesReader(w, r.Body, 26<<20)
	if err := r.ParseMultipartForm(26 << 20); err != nil {
		return fail(400, "Upload must be at most 25 MiB")
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	account, _ := strconv.ParseInt(r.FormValue("account_id"), 10, 64)
	if !ruleAccess(a.DB, a, u, account) {
		return fail(403, "Account editor access required")
	}
	var bank string
	a.DB.QueryRow("SELECT bank_id FROM accounts WHERE id=?", account).Scan(&bank)
	headers := r.MultipartForm.File["files"]
	if len(headers) == 0 || len(headers) > 20 {
		return fail(400, "Choose 1–20 files")
	}
	incoming := []inputFile{}
	total := int64(0)
	for _, header := range headers {
		f, err := header.Open()
		if err != nil {
			return err
		}
		b, err := io.ReadAll(io.LimitReader(f, (25<<20)-total+1))
		f.Close()
		if err != nil {
			return err
		}
		total += int64(len(b))
		if total > 25<<20 {
			return fail(400, "Upload exceeds 25 MiB")
		}
		incoming = append(incoming, inputFile{header.Filename, b})
	}
	files, err := expand(incoming)
	if err != nil {
		return err
	}
	result := []ParsedFile{}
	batchSeen := map[string]map[string]any{}
	err = a.write(func(tx *sql.Tx) error {
		if !ruleAccess(tx, a, u, account) {
			return fail(403, "Account access changed")
		}
		for _, f := range files {
			p := parseFile(f)
			if p.AccountID != bank {
				p.Error = "File account number does not match the selected account"
			}
			if err := a.annotate(tx, account, &p); err != nil {
				return err
			}

			for i := range p.Rows {
				row := &p.Rows[i]
				if row.Error != "" {
					continue
				}
				key := fingerprint(*row)
				if prior, ok := batchSeen[key]; ok && row.Duplicate == "" {
					row.Duplicate = "possible"
					row.Candidates = append(row.Candidates, prior)
				}
				if p.Error == "" {
					batchSeen[key] = map[string]any{"file": p.Name, "row": row.Row, "date": row.Date, "amount_cents": row.Amount, "description": row.Description}
				}
			}

			if err := stageImport(tx, u, account, &p); err != nil {
				return err
			}
			result = append(result, p)
		}
		return nil
	})
	if err != nil {
		return err
	}
	sendImportPreviews(w, r, result)
	return nil
}
func (a *App) imports(w http.ResponseWriter, r *http.Request) error {
	if r.URL.Query().Has("page") {
		return a.importPages(w, r)
	}
	u := Current(r)
	items, err := data(a.DB, "SELECT i.id,i.account_id,i.name,i.format,i.status,i.created_at,i.data FROM imports i JOIN accounts a ON a.id=i.account_id WHERE "+accessSQL+" ORDER BY i.id DESC LIMIT 100", u.Member, u.ID)
	if err != nil {
		return err
	}
	for _, item := range items {
		var p ParsedFile
		if err := json.Unmarshal([]byte(item["data"].(string)), &p); err != nil {
			return err
		}
		p.ID = num(item["id"])
		if item["status"] == "staged" {
			if err := a.annotate(a.DB, num(item["account_id"]), &p); err != nil {
				return err
			}
		}
		item["preview"] = previewPage(p, 0, 100)
		delete(item, "data")
	}
	send(w, items)
	return nil
}

type importCommitOptions struct {
	PreviewVersion        string            `json:"preview_version"`
	SkipAllCandidates     bool              `json:"skip_all_candidates"`
	ConfirmValid          bool              `json:"confirm_valid_rows"`
	ClassificationVersion string            `json:"classification_version"`
	Decisions             map[string]string `json:"decisions"`
}

func (a *App) commitImport(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	id := parseID(r)
	var b importCommitOptions
	if err := decode(r, &b); err != nil {
		return err
	}
	inserted := 0
	skipped := 0
	err := a.write(func(tx *sql.Tx) error {
		return a.commitStagedImport(tx, u, id, b, &inserted, &skipped)
	})
	if err != nil {
		return err
	}
	send(w, map[string]int{"inserted": inserted, "skipped": skipped})
	return nil
}

// Shared by the HTTP endpoint and automatic connector imports, under a write transaction.
func (a *App) commitStagedImport(tx *sql.Tx, u User, id int64, b importCommitOptions, inserted, skipped *int) error {
	var account int64
	var status, raw string
	if err := tx.QueryRow("SELECT account_id,status,data FROM imports WHERE id=?", id).Scan(&account, &status, &raw); err != nil {
		return fail(404, "Import not found")
	}
	if !ruleAccess(tx, a, u, account) {
		return fail(403, "Account editor access required")
	}
	if status == "committed" {
		return fail(409, "This import is already committed")
	}
	var p ParsedFile
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		return err
	}
	if p.Error != "" {
		return fail(400, p.Error)
	}
	var bank string
	if err := tx.QueryRow("SELECT bank_id FROM accounts WHERE id=?", account).Scan(&bank); err != nil {
		return err
	}
	if p.AccountID != bank || p.Currency != "ZAR" {
		return fail(409, "Account identity changed; preview this file again against the correct account")
	}
	expected := b.ClassificationVersion
	if expected == "" {
		expected = p.ClassificationVersion
	}
	if err := a.annotate(tx, account, &p); err != nil {
		return err
	}

	if b.PreviewVersion != "" && b.PreviewVersion != importPreviewVersion(p) {
		return fail(409, "Import preview changed. Reload it before confirming.")
	}

	if b.SkipAllCandidates {
		if b.Decisions == nil {
			b.Decisions = map[string]string{}
		}
		for _, row := range p.Rows {
			if row.Error == "" && (row.Duplicate == "possible" || row.Duplicate == "conflict") {
				key := strconv.Itoa(row.Row)
				if b.Decisions[key] == "" {
					b.Decisions[key] = "skip"
				}
			}
		}
	}
	if expected != p.ClassificationVersion {
		return fail(409, "Classification rules changed. Inspect the current preview before importing.")
	}
	if p.AlreadyImported {
		return fail(409, "This exact file has already been imported")
	}
	invalid := false
	for _, row := range p.Rows {
		if row.Error != "" {
			invalid = true
		}
	}
	if invalid && !b.ConfirmValid {
		return fail(400, "Confirm importing valid rows while retaining rejected rows")
	}
	for i := range p.Rows {
		row := &p.Rows[i]
		decision := b.Decisions[strconv.Itoa(row.Row)]
		if decision != "" && decision != "keep" && decision != "skip" {
			return fail(400, "Invalid duplicate decision")
		}
		if row.Error != "" {
			(*skipped)++
			continue
		}
		if p.Format != "fnb-live" {
			one := ParsedFile{Hash: p.Hash, Rows: []SourceRow{*row}}
			if err := a.annotate(tx, account, &one); err != nil {
				return err
			}
			*row = one.Rows[0]
		}
		switch row.Duplicate {
		case "conflict":
			if decision != "skip" {
				return fail(409, "A bank transaction ID has conflicting details; inspect and skip the conflicting row")
			}
		case "exact_id", "exact_source":
			decision = "skip"
		case "possible":
			if decision == "" {
				return fail(409, "Resolve all possible duplicates before importing")
			}
		}
		if decision == "skip" {
			(*skipped)++
			continue
		}
		source, _ := json.Marshal(map[string]any{"file": p.Name, "hash": p.Hash, "format": p.Format, "row": row.Row, "date": row.Date, "amount_cents": row.Amount, "description": row.Description, "fitid": row.FITID, "balance_cents": row.Balance, "source_reference": row.SourceReference, "run_id": p.RunID, "coverage": p.Coverage, "source_bank_row": row.SourceBankRow, "source_component": row.SourceComponent, "source_bank_description": row.SourceBankDescription, "source_service_fee_cents": row.SourceServiceFee})
		var fit any
		if row.FITID != "" {
			fit = row.FITID
		}
		period := autoPeriod(tx, account, row.Date)
		res, err := tx.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,fitid,source_date,source_amount,source_description,source_key,provenance,import_id,period_id,spending_group_id,merchant_id) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)", account, row.Date, row.Amount, row.Description, fit, row.Date, row.Amount, row.Description, fingerprint(*row), string(source), id, period, row.SpendingGroupID, row.MerchantID)
		if err != nil {
			return err
		}
		tid, _ := res.LastInsertId()
		if row.MerchantID == nil {
			if err := a.importMerchant(tx, tid, account, row.Description, row.Amount); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,?,?)", tid, row.CategoryID, row.Amount); err != nil {
			return err
		}
		if err := acceptCategorized(tx, u, account, tid); err != nil {
			return err
		}
		if err := audit(tx, u, account, "transaction", tid, "imported", map[string]any{"import_id": id, "row": row.Row, "suggestion": row.Suggestion}); err != nil {
			return err
		}
		(*inserted)++
	}
	if p.Balance != nil && validDate(p.BalanceDate) {
		if _, err := tx.Exec("UPDATE accounts SET balance_cents=?,balance_date=?,version=version+1 WHERE id=? AND (balance_date IS NULL OR balance_date<=?)", *p.Balance, p.BalanceDate, account, p.BalanceDate); err != nil {
			return err
		}
	}
	p.Decisions, p.Inserted, p.Skipped = b.Decisions, *inserted, *skipped
	result, _ := json.Marshal(p)
	if _, err := tx.Exec("UPDATE imports SET status='committed',data=?,committed_at=CURRENT_TIMESTAMP WHERE id=?", string(result), id); err != nil {
		return err
	}
	return audit(tx, u, account, "import", id, "committed", map[string]int{"inserted": *inserted, "skipped": *skipped})
}

// Both file uploads and connector snapshots enter the same durable preview.
func stageImport(tx *sql.Tx, u User, account int64, p *ParsedFile) error {
	b, err := json.Marshal(p)
	if err != nil {
		return err
	}
	res, err := tx.Exec("INSERT INTO imports(user_id,account_id,name,hash,format,data) VALUES(?,?,?,?,?,?)", u.ID, account, p.Name, p.Hash, p.Format, string(b))
	if err != nil {
		return err
	}
	p.ID, err = res.LastInsertId()
	return err
}
