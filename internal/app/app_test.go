package app

import (
	"archive/zip"
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type testEnv struct {
	a      *App
	h      http.Handler
	owner  User
	other  User
	viewer User
}

func setup(t *testing.T) *testEnv {
	t.Helper()
	dir := t.TempDir()
	a, err := Open(filepath.Join(dir, "finance.sqlite"), "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.Close() })
	// Synthetic groups are explicit fixtures; fresh installations contain no classification.
	for _, group := range [][2]string{{"Day-to-day", "blue"}, {"Recurring", "amber"}, {"Invest-save-repay", "purple"}, {"Exceptions", "orange"}, {"Income", "teal"}, {"Transfer", "slate"}, {"Bank Fees", "orange"}, {"Communications", "purple"}, {"Debt", "rose"}, {"Utilities", "blue"}, {"Insurance", "teal"}} {
		if _, err := a.DB.Exec("INSERT INTO spending_groups(name,color) VALUES(?,?)", group[0], group[1]); err != nil {
			t.Fatal(err)
		}
	}
	// These legacy fixtures deliberately omit starter classification.
	if _, err := a.DB.Exec("DELETE FROM builtin_rules; DELETE FROM categories"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{
		"INSERT INTO users(id,username,password,admin,budget_member) VALUES(1,'owner','unused',1,1),(2,'other','unused',0,0),(3,'viewer','unused',0,0)",
		"INSERT INTO accounts(id,name,bank_id,household) VALUES(1,'Shared','12345678901',1),(2,'Private','22222222222',0)",
		"INSERT INTO grants VALUES(1,1,'editor'),(1,2,'editor'),(3,1,'viewer')",
		"INSERT INTO categories(id,name,group_name,kind) VALUES(1,'Groceries','Living','expense'),(2,'Salary','Income','income')",
		"INSERT INTO periods(id,name,start_date,end_date) VALUES(1,'October','2026-10-20','2026-11-19')",
	} {
		if _, err := a.DB.Exec(s); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range []int{1, 2, 3} {
		token := fmt.Sprint("token", id)
		if _, err := a.DB.Exec("INSERT INTO sessions VALUES(?,?,?,?)", hash(token), id, "csrf", time.Now().Add(time.Hour).Unix()); err != nil {
			t.Fatal(err)
		}
	}
	return &testEnv{a, a.Handler(t.TempDir()), User{ID: 1, Username: "owner", Admin: true, Member: true}, User{ID: 2, Username: "other"}, User{ID: 3, Username: "viewer"}}
}
func (e *testEnv) req(t *testing.T, uid int, path, method string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(b)
	}
	r := httptest.NewRequest(method, path, reader)
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: fmt.Sprint("token", uid)})
	r.Header.Set("X-CSRF-Token", "csrf")
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "http://localhost:8080")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}
func status(t *testing.T, w *httptest.ResponseRecorder, want int) {
	t.Helper()
	if w.Code != want {
		t.Fatalf("status %d want %d: %s", w.Code, want, w.Body.String())
	}
}
func (e *testEnv) upload(t *testing.T, uid int, account int64, name string, content []byte) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	mw.WriteField("account_id", fmt.Sprint(account))
	f, _ := mw.CreateFormFile("files", name)
	f.Write(content)
	mw.Close()
	r := httptest.NewRequest("POST", "/api/imports/preview", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("X-CSRF-Token", "csrf")
	r.AddCookie(&http.Cookie{Name: "finance_session", Value: fmt.Sprint("token", uid)})
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}
func ofx(rows ...string) string {
	return "OFXHEADER:100\nDATA:OFXSGML\nVERSION:102\nENCODING:USASCII\nCHARSET:1252\n\n<OFX><BANKMSGSRSV1><STMTRS><CURDEF>ZAR\n<BANKACCTFROM><BANKID>250655\n<ACCTID>12345678901\n<ACCTTYPE>CHECKING\n</BANKACCTFROM><BANKTRANLIST><DTSTART>20261020\n<DTEND>20261119\n" + strings.Join(rows, "") + "\n</BANKTRANLIST><LEDGERBAL><BALAMT>900.00\n<DTASOF>20261119\n</LEDGERBAL></STMTRS></BANKMSGSRSV1></OFX>"
}
func ofxRow(id, date, amount, desc string) string {
	kind := "CREDIT"
	if strings.HasPrefix(amount, "-") {
		kind = "DEBIT"
	}
	return "<STMTTRN><TRNTYPE>" + kind + "\n<DTPOSTED>" + date + "\n<TRNAMT>" + amount + "\n<FITID>" + id + "\n<MEMO>" + desc + "\n</STMTTRN>"
}
func csvFile(rows ...string) string {
	return "ACCOUNT TRANSACTION HISTORY\n\nName:,Example User\nAccount:,12345678901,Fusion\nBalance:,900.00\n\nDate, Amount, Balance, Description\n" + strings.Join(rows, "\n") + "\n"
}
func (e *testEnv) stage(t *testing.T, name, content string) ParsedFile {
	t.Helper()
	w := e.upload(t, 1, 1, name, []byte(content))
	status(t, w, 200)
	var p []ParsedFile
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	return p[0]
}
func (e *testEnv) commit(t *testing.T, id int64, decisions map[string]string, confirm bool) *httptest.ResponseRecorder {
	return e.req(t, 1, fmt.Sprintf("/api/imports/%d/commit", id), "POST", map[string]any{"decisions": decisions, "confirm_valid_rows": confirm})
}
func TestFNBAdapters(t *testing.T) {
	p := parseFile(inputFile{Name: "test.csv", Content: []byte(csvFile("2026/10/21,-100.00,900.00,Market"))})
	if p.Error != "" || p.AccountID != "12345678901" || len(p.Rows) != 1 || p.Rows[0].Amount != -10000 || p.BalanceDate != "2026-10-21" {
		t.Fatalf("%+v", p)
	}
	o := parseFile(inputFile{Name: "test.ofx", Content: []byte(ofx(ofxRow("id1", "20261021", "-100.00", "Market")))})
	if o.Error != "" || o.Currency != "ZAR" || o.AccountType != "CHECKING" || o.Rows[0].FITID != "id1" || o.Balance == nil {
		t.Fatalf("%+v", o)
	}
	invalid := parseFile(inputFile{Name: "bad.csv", Content: []byte(csvFile("2026/10/21,invalid,900.00,Market", "bad,-1.00,899.00,Shop"))})
	if invalid.Rows[0].Error == "" || invalid.Rows[1].Error == "" {
		t.Fatal("invalid rows were accepted")
	}
}
func TestZipSafety(t *testing.T) {
	for _, name := range []string{"../test.ofx", "/test.ofx", "dir\\test.csv"} {
		var b bytes.Buffer
		z := zip.NewWriter(&b)
		f, _ := z.Create(name)
		f.Write([]byte("bad"))
		z.Close()
		if _, err := expand([]inputFile{{Name: "files.zip", Content: b.Bytes()}}); err == nil {
			t.Fatal("accepted unsafe ZIP")
		}
	}
	var b bytes.Buffer
	z := zip.NewWriter(&b)
	f, _ := z.Create("safe/test.ofx")
	f.Write([]byte(ofx(ofxRow("id1", "20261021", "-100.00", "Market"))))
	z.Close()
	files, err := expand([]inputFile{{Name: "files.zip", Content: b.Bytes()}})
	if err != nil || len(files) != 1 {
		t.Fatal(err)
	}
}
func TestImportIdentityAndPermissions(t *testing.T) {
	e := setup(t)
	status(t, e.upload(t, 2, 1, "test.ofx", []byte(ofx())), 403)
	status(t, e.upload(t, 3, 1, "test.ofx", []byte(ofx())), 403)
	p := e.stage(t, "wrong.ofx", strings.ReplaceAll(ofx(ofxRow("id1", "20261021", "-100.00", "Market")), "12345678901", "33333333333"))
	if p.Error == "" {
		t.Fatal("wrong account accepted")
	}
	status(t, e.commit(t, p.ID, nil, false), 400)
	p = e.stage(t, "foreign.ofx", strings.ReplaceAll(ofx(ofxRow("id1", "20261021", "-100.00", "Market")), "ZAR", "USD"))
	status(t, e.commit(t, p.ID, nil, false), 400)
}
func TestImportDuplicatesAndConflict(t *testing.T) {
	e := setup(t)
	source := ofx(ofxRow("id1", "20261021", "-100.00", "Market"))
	p := e.stage(t, "test.ofx", source)
	status(t, e.commit(t, p.ID, nil, false), 200)
	same := e.stage(t, "same.ofx", source)
	if !same.AlreadyImported {
		t.Fatal("missing file hash check")
	}
	status(t, e.commit(t, same.ID, nil, false), 409)
	overlap := e.stage(t, "overlap.ofx", ofx(ofxRow("id1", "20261021", "-100.00", "Market"), ofxRow("id2", "20261022", "-10.00", "Cafe")))
	if overlap.Rows[0].Duplicate != "exact_id" {
		t.Fatalf("%+v", overlap.Rows)
	}
	status(t, e.commit(t, overlap.ID, nil, false), 200)
	conflict := e.stage(t, "conflict.ofx", ofx(ofxRow("id1", "20261021", "-101.00", "Market")))
	if conflict.Rows[0].Duplicate != "conflict" {
		t.Fatal("ID conflict not found")
	}
	status(t, e.commit(t, conflict.ID, nil, false), 409)
	status(t, e.commit(t, conflict.ID, map[string]string{"1": "skip"}, false), 200)
	cross := e.stage(t, "test.csv", csvFile("2026/10/21,-100.00,900.00,Market"))
	if cross.Rows[0].Duplicate != "possible" {
		t.Fatal("cross-format candidate not found")
	}
	status(t, e.commit(t, cross.ID, nil, false), 409)
	status(t, e.commit(t, cross.ID, map[string]string{"8": "skip"}, false), 200)
	if n := queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions"); n != 2 {
		t.Fatalf("%d transactions", n)
	}
}
func TestLegitimateIdenticalAndInvalidRows(t *testing.T) {
	e := setup(t)
	p := e.stage(t, "test.csv", csvFile("2026/10/21,-10.00,900.00,Cafe", "2026/10/21,-10.00,890.00,Cafe", "2026/10/22,broken,890.00,Invalid"))
	if p.Rows[1].Duplicate != "possible" || p.Rows[2].Error == "" {
		t.Fatal("missing duplicate/error")
	}
	status(t, e.commit(t, p.ID, map[string]string{"9": "keep"}, false), 400)
	status(t, e.commit(t, p.ID, map[string]string{"9": "keep"}, true), 200)
	if n := queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions"); n != 2 {
		t.Fatal(n)
	}
	w := e.req(t, 1, "/api/imports", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), "Invalid") || !strings.Contains(w.Body.String(), "invalid money") {
		t.Fatal("rejected rows not retained")
	}
}
func TestConcurrentImport(t *testing.T) {
	e := setup(t)
	source := ofx(ofxRow("id1", "20261021", "-100.00", "Market"))
	one := e.stage(t, "one.ofx", source)
	two := e.stage(t, "two.ofx", source)
	var wg sync.WaitGroup
	codes := make(chan int, 2)
	for _, id := range []int64{one.ID, two.ID} {
		wg.Add(1)
		go func(id int64) { defer wg.Done(); codes <- e.commit(t, id, nil, false).Code }(id)
	}
	wg.Wait()
	close(codes)
	ok, conflict := 0, 0
	for c := range codes {
		if c == 200 {
			ok++
		}
		if c == 409 {
			conflict++
		}
	}
	if ok != 1 || conflict != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM transactions") != 1 {
		t.Fatal("concurrent duplicate insertion")
	}
}
func seedTransaction(t *testing.T, e *testEnv, account, amount int64, date string, category *int64) int64 {
	t.Helper()
	res, err := e.a.DB.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance,period_id) VALUES(?,?,?,'Synthetic',?,?,'Synthetic','{}',?)", account, date, amount, date, amount, autoPeriod(e.a.DB, account, date))
	if err != nil {
		t.Fatal(err)
	}
	id, _ := res.LastInsertId()
	if _, err := e.a.DB.Exec("INSERT INTO allocations(transaction_id,category_id,amount_cents) VALUES(?,?,?)", id, category, amount); err != nil {
		t.Fatal(err)
	}
	return id
}
func editBody(version, amount int64, alloc []Allocation) map[string]any {
	return map[string]any{"version": version, "date": "2026-10-21", "amount_cents": amount, "description": "Market", "allocations": alloc, "is_transfer": false, "assignment": "auto", "period_id": nil}
}
func TestReviewSplitsAndOptimisticEdits(t *testing.T) {
	e := setup(t)
	id := seedTransaction(t, e, 1, -10000, "2026-10-21", nil)
	path := fmt.Sprintf("/api/transactions/%d", id)
	status(t, e.req(t, 1, "/api/review", "POST", map[string]any{"items": []map[string]int64{{"id": id, "version": 1}}}), 400)
	cat := int64(1)
	status(t, e.req(t, 1, path, "PUT", editBody(1, -10000, []Allocation{{CategoryID: &cat, Amount: -6000, Note: ""}, {CategoryID: &cat, Amount: -3000, Note: ""}})), 400)
	status(t, e.req(t, 1, path, "PUT", editBody(1, -10000, []Allocation{{CategoryID: &cat, Amount: -6000, Note: "Food"}, {CategoryID: &cat, Amount: -4000, Note: "Other"}})), 200)
	// Filling all split categories accepts automatically.
	status(t, e.req(t, 1, path, "PUT", editBody(1, -10000, []Allocation{{CategoryID: &cat, Amount: -10000, Note: ""}})), 409)
	status(t, e.req(t, 1, path, "PUT", editBody(2, -10000, []Allocation{{CategoryID: &cat, Amount: -10000, Note: ""}})), 200)
	var state string
	e.a.DB.QueryRow("SELECT review_state FROM transactions WHERE id=?", id).Scan(&state)
	if state != "approved" {
		t.Fatal("categorized edit did not stay accepted")
	}
	if queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM transactions") != -10000 || queryInt(e.a.DB, "SELECT SUM(amount_cents) FROM allocations") != -10000 {
		t.Fatal("split doubled cash movement")
	}
}
func TestPermissionBoundaries(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	shared := seedTransaction(t, e, 1, -10000, "2026-10-21", &cat)
	private := seedTransaction(t, e, 2, -77777, "2026-10-21", &cat)
	status(t, e.req(t, 2, "/api/transactions", "GET", nil), 200)
	w := e.req(t, 2, "/api/transactions", "GET", nil)
	if strings.Contains(w.Body.String(), "Synthetic") {
		t.Fatal("unauthorized transaction leaked")
	}
	w = e.req(t, 3, "/api/transactions", "GET", nil)
	if strings.Contains(w.Body.String(), "77777") || strings.Contains(w.Body.String(), "Private") {
		t.Fatal("private transaction leaked")
	}
	status(t, e.req(t, 3, fmt.Sprintf("/api/transactions/%d", shared), "PUT", editBody(1, -10000, []Allocation{{CategoryID: &cat, Amount: -10000, Note: ""}})), 403)
	status(t, e.req(t, 2, fmt.Sprintf("/api/audit/%d", private), "GET", nil), 403)
	status(t, e.req(t, 2, "/api/dashboard", "GET", nil), 403)
	status(t, e.req(t, 2, "/api/periods", "GET", nil), 403)
	status(t, e.req(t, 2, "/api/users", "GET", nil), 403)
	status(t, e.req(t, 2, "/api/accounts/manage", "GET", nil), 403)
	status(t, e.req(t, 1, "/api/grants", "PUT", map[string]any{"user_id": 2, "account_id": 2, "role": "viewer"}), 200)
	w = e.req(t, 2, "/api/transactions", "GET", nil)
	if !strings.Contains(w.Body.String(), "77777") {
		t.Fatal("grant not effective")
	}
	status(t, e.req(t, 1, "/api/grants", "PUT", map[string]any{"user_id": 2, "account_id": 2, "role": ""}), 200)
	w = e.req(t, 2, "/api/transactions", "GET", nil)
	if strings.Contains(w.Body.String(), "77777") {
		t.Fatal("revoked access remained")
	}
}
func TestDashboardRefundsAndPrivateExclusion(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	salary := int64(2)
	seedTransaction(t, e, 1, -10000, "2026-10-21", &cat)
	seedTransaction(t, e, 1, 2000, "2026-10-22", &cat)
	seedTransaction(t, e, 1, 50000, "2026-10-23", &salary)
	seedTransaction(t, e, 2, -99999, "2026-10-21", &cat)
	e.a.DB.Exec("INSERT INTO targets VALUES(1,1,20000)")
	w := e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	status(t, w, 200)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	if num(v["spent_cents"]) != 8000 || num(v["income_cents"]) != 50000 || num(v["remaining_cents"]) != 12000 || num(v["pending_count"]) != 3 {
		t.Fatalf("%s", w.Body.String())
	}
}
func TestTransferRedactionAndExclusion(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	left := seedTransaction(t, e, 1, -10000, "2026-10-21", &cat)
	right := seedTransaction(t, e, 2, 10000, "2026-10-21", &cat)
	status(t, e.req(t, 1, "/api/transfers", "POST", map[string]any{"left_id": left, "right_id": right, "left_version": 1, "right_version": 1}), 200)
	w := e.req(t, 3, "/api/transactions", "GET", nil)
	status(t, w, 200)
	if !strings.Contains(w.Body.String(), "transfer_counterpart_hidden") || strings.Contains(w.Body.String(), "transfer_counterpart_id") {
		t.Fatal("counterpart was not redacted")
	}
	w = e.req(t, 1, "/api/dashboard?period=1", "GET", nil)
	if !strings.Contains(w.Body.String(), "\"spent_cents\":0") {
		t.Fatal(w.Body.String())
	}
	status(t, e.req(t, 1, fmt.Sprintf("/api/transfers/%d", left), "DELETE", nil), 200)
	if queryInt(e.a.DB, "SELECT SUM(is_transfer) FROM transactions") != 0 {
		t.Fatal("unlink did not reset transfer designation")
	}
}
func (e *testEnv) newPeriod(t *testing.T, name, start, end string) int64 {
	t.Helper()
	b := periodInput{Name: name, Start: start, End: end}
	preview := e.req(t, 1, "/api/periods/new/preview", "POST", b)
	status(t, preview, 200)
	var v map[string]any
	json.Unmarshal(preview.Body.Bytes(), &v)
	b.PreviewToken = v["preview_token"].(string)
	w := e.req(t, 1, "/api/periods", "POST", b)
	status(t, w, 200)
	json.Unmarshal(w.Body.Bytes(), &v)
	return num(v["id"])
}
func TestBudgetGapsOverlapsAndManualAssignments(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	overlap := seedTransaction(t, e, 1, -100, "2026-11-19", &cat)
	gap := seedTransaction(t, e, 1, -200, "2026-11-20", &cat)
	second := e.newPeriod(t, "Early payday", "2026-11-19", "2026-12-18")
	if queryInt(e.a.DB, "SELECT period_id FROM transactions WHERE id=?", overlap) != second || queryInt(e.a.DB, "SELECT period_id FROM transactions WHERE id=?", gap) != second {
		t.Fatal("latest-starting assignment failed")
	}
	// Explicit manual membership is preserved even outside the period's date range.
	version := queryInt(e.a.DB, "SELECT version FROM transactions WHERE id=?", gap)
	b := editBody(version, -200, []Allocation{{CategoryID: &cat, Amount: -200, Note: ""}})
	b["date"] = "2026-11-20"
	b["assignment"] = "manual"
	b["period_id"] = 1
	status(t, e.req(t, 1, fmt.Sprintf("/api/transactions/%d", gap), "PUT", b), 200)
	oldVersion := queryInt(e.a.DB, "SELECT version FROM periods WHERE id=?", second)
	change := periodInput{Name: "Later", Start: "2026-11-22", End: "2026-12-18", Version: oldVersion}
	w := e.req(t, 1, fmt.Sprintf("/api/periods/%d/preview", second), "POST", change)
	status(t, w, 200)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	change.PreviewToken = v["preview_token"].(string)
	status(t, e.req(t, 1, fmt.Sprintf("/api/periods/%d", second), "PUT", change), 200)
	if queryInt(e.a.DB, "SELECT period_id FROM transactions WHERE id=?", gap) != 1 {
		t.Fatal("manual assignment was rewritten")
	}
	unassigned := seedTransaction(t, e, 1, -50, "2026-11-21", &cat)
	if queryInt(e.a.DB, "SELECT period_id FROM transactions WHERE id=?", unassigned) != 0 {
		t.Fatal("gap was assigned")
	}
	w = e.req(t, 1, "/api/transactions?unassigned=1", "GET", nil)
	if !strings.Contains(w.Body.String(), "2026-11-21") {
		t.Fatal("missing assignment queue")
	}
	if clamped(2026, time.February, 31).Format("2006-01-02") != "2026-02-28" {
		t.Fatal("month clamp failed")
	}
}
func TestPeriodPreviewStaleAndTargetCopy(t *testing.T) {
	e := setup(t)
	e.a.DB.Exec("INSERT INTO targets VALUES(1,1,12345)")
	cat := int64(1)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	b := periodInput{Name: "Changed", Start: "2026-10-22", End: "2026-11-19", Version: 1}
	w := e.req(t, 1, "/api/periods/1/preview", "POST", b)
	status(t, w, 200)
	var v map[string]any
	json.Unmarshal(w.Body.Bytes(), &v)
	b.PreviewToken = v["preview_token"].(string)
	e.a.DB.Exec("UPDATE transactions SET version=version+1 WHERE id=?", id)
	status(t, e.req(t, 1, "/api/periods/1", "PUT", b), 409)
	second := e.newPeriod(t, "Next", "2026-11-20", "2026-12-19")
	if queryInt(e.a.DB, "SELECT amount_cents FROM targets WHERE period_id=?", second) != 12345 {
		t.Fatal("targets not copied")
	}
}
func TestAuthenticationCSRFAndRecovery(t *testing.T) {
	e := setup(t)
	id, err := e.a.CreateUser("actual", "long-test-password", false, true)
	if err != nil {
		t.Fatal(err)
	}
	w := e.req(t, 1, "/api/login", "POST", map[string]string{"username": "actual", "password": "long-test-password"})
	status(t, w, 200)
	cookie := w.Result().Cookies()[0]
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode {
		t.Fatal("weak cookie options")
	}
	r := httptest.NewRequest("POST", "/api/logout", nil)
	r.AddCookie(cookie)
	denied := httptest.NewRecorder()
	e.h.ServeHTTP(denied, r)
	status(t, denied, 403)
	r = httptest.NewRequest("POST", "/api/login", strings.NewReader("{}"))
	r.Header.Set("Origin", "https://evil.example")
	denied = httptest.NewRecorder()
	e.h.ServeHTTP(denied, r)
	status(t, denied, 403)
	if err := e.a.ResetPassword("actual", "another-long-password"); err != nil {
		t.Fatal(err)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM sessions WHERE user_id=?", id) != 0 {
		t.Fatal("reset left sessions active")
	}
	if _, err := e.a.CreateUser("short", "bad", false, true); err == nil {
		t.Fatal("weak password accepted")
	}
}
func TestBackupRestoreAndRetention(t *testing.T) {
	e := setup(t)
	mcpToken(t, e, 1, true)
	cat := int64(1)
	seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	var snapshot string
	for i := 0; i < 15; i++ {
		path, err := e.a.Backup()
		if err != nil {
			t.Fatal(err)
		}
		snapshot = path
	}
	files, _ := filepath.Glob(filepath.Join(e.a.BackupDir, "finance-*.sqlite"))
	if len(files) != 14 {
		t.Fatal("retention:", len(files))
	}
	target := filepath.Join(t.TempDir(), "restored.sqlite")
	if err := Restore(target, snapshot); err != nil {
		t.Fatal(err)
	}
	restored, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if queryInt(restored, "SELECT COUNT(*) FROM transactions") != 1 || queryInt(restored, "SELECT COUNT(*) FROM sessions") != 0 {
		t.Fatal("restore data/session invariants failed")
	}
	for _, table := range []string{"mcp_tokens", "mcp_proposals", "mcp_oauth_clients", "mcp_oauth_requests", "mcp_oauth_codes", "mcp_access_tokens", "mcp_refresh_tokens"} {
		if queryInt(restored, "SELECT COUNT(*) FROM "+table) != 0 {
			t.Fatal("restore retained agent credentials", table)
		}
	}
	if err := Restore(filepath.Join(t.TempDir(), "bad.sqlite"), filepath.Join(t.TempDir(), "missing.sqlite")); err == nil {
		t.Fatal("missing backup accepted")
	}
	if _, err := os.Stat(snapshot); err != nil {
		t.Fatal(err)
	}
	if err := Restore(filepath.Join(filepath.Dir(e.a.BackupDir), "finance.sqlite"), snapshot); err == nil {
		t.Fatal("restored over active service")
	}
}

func TestUserManagementAndRevocation(t *testing.T) {
	e := setup(t)
	body := map[string]any{"username": "owner", "admin": true, "budget_member": true, "disabled": true, "version": 1}
	status(t, e.req(t, 1, "/api/users/1", "PUT", body), 400)
	body = map[string]any{"username": "other", "admin": false, "budget_member": true, "disabled": false, "version": 1}
	status(t, e.req(t, 1, "/api/users/2", "PUT", body), 200)
	status(t, e.req(t, 2, "/api/periods", "GET", nil), 200)
	body["disabled"] = true
	body["version"] = 2
	status(t, e.req(t, 1, "/api/users/2", "PUT", body), 200)
	status(t, e.req(t, 2, "/api/accounts", "GET", nil), 401)
	status(t, e.req(t, 1, "/api/users/2", "PUT", body), 409)
}
func TestNonmemberCannotRewriteBudgetAssignment(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	id := seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	e.a.DB.Exec("INSERT INTO grants VALUES(2,1,'editor')")
	e.a.DB.Exec("UPDATE transactions SET assignment='manual',period_id=1 WHERE id=?", id)
	w := e.req(t, 2, "/api/transactions", "GET", nil)
	status(t, w, 200)
	if strings.Contains(w.Body.String(), "\"period_name\":\"October\"") {
		t.Fatal("budget period metadata leaked")
	}
	b := editBody(1, -100, []Allocation{{CategoryID: &cat, Amount: -100, Note: ""}})
	b["assignment"] = "outside"
	status(t, e.req(t, 2, fmt.Sprintf("/api/transactions/%d", id), "PUT", b), 200)
	var assignment string
	e.a.DB.QueryRow("SELECT assignment FROM transactions WHERE id=?", id).Scan(&assignment)
	if assignment != "manual" || queryInt(e.a.DB, "SELECT period_id FROM transactions WHERE id=?", id) != 1 {
		t.Fatal("nonmember rewrote household budget assignment")
	}
}
func TestInitialSchemaMigration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "legacy.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	legacy := strings.ReplaceAll(schema, ",disabled INTEGER NOT NULL DEFAULT 0,version INTEGER NOT NULL DEFAULT 1", "")
	legacy = strings.ReplaceAll(legacy, ",source_key TEXT NOT NULL DEFAULT ''", "")
	legacy = strings.ReplaceAll(legacy, "CREATE INDEX IF NOT EXISTS transaction_source_match ON transactions(account_id,source_key);", "")
	if _, err := db.Exec(legacy); err != nil {
		t.Fatal(err)
	}
	db.Exec("INSERT INTO users(id,username,password) VALUES(1,'legacy','unused')")
	db.Exec("INSERT INTO accounts(id,name,bank_id) VALUES(1,'Legacy','12345')")
	db.Exec("INSERT INTO transactions(account_id,date,amount_cents,description,source_date,source_amount,source_description,provenance) VALUES(1,'2026-10-21',-100,'Edited','2026-10-20',-100,'Original','{}')")
	db.Close()
	a, err := Open(path, "http://localhost:8080", filepath.Join(dir, "backups"))
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if queryInt(a.DB, "SELECT MAX(version) FROM migrations") != schemaVersion {
		t.Fatal("migration version not recorded")
	}
	var key, description string
	if err := a.DB.QueryRow("SELECT source_key,description FROM transactions").Scan(&key, &description); err != nil {
		t.Fatal(err)
	}
	if key == "" || description != "Edited" {
		t.Fatal("migration lost source/edited data")
	}
}
func TestRestorePreservesPreviousDatabase(t *testing.T) {
	e := setup(t)
	cat := int64(1)
	seedTransaction(t, e, 1, -100, "2026-10-21", &cat)
	snapshot, err := e.a.Backup()
	if err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "restored.sqlite")
	if err := Restore(target, snapshot); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	db.Exec("UPDATE transactions SET description='Before restore marker'")
	db.Close()
	if err := Restore(target, snapshot); err != nil {
		t.Fatal(err)
	}
	previous, _ := filepath.Glob(target + ".before-restore-*")
	if len(previous) != 1 {
		t.Fatal("previous database not preserved")
	}
	old, err := sql.Open("sqlite", previous[0])
	if err != nil {
		t.Fatal(err)
	}
	defer old.Close()
	var description string
	old.QueryRow("SELECT description FROM transactions LIMIT 1").Scan(&description)
	if description != "Before restore marker" {
		t.Fatal("previous contents lost")
	}
	fresh, err := sql.Open("sqlite", target)
	if err != nil {
		t.Fatal(err)
	}
	defer fresh.Close()
	fresh.QueryRow("SELECT description FROM transactions LIMIT 1").Scan(&description)
	if description != "Synthetic" {
		t.Fatal("restored data incorrect")
	}
}
