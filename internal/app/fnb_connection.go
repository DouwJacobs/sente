package app

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

type fnbCredentials struct {
	Username            string   `json:"username"`
	Password            string   `json:"password"`
	Hidden              []string `json:"hidden,omitempty"`
	TransactionAccounts []string `json:"transaction_accounts,omitempty"`
	RunID               string   `json:"run_id,omitempty"`
}
type fnbAccountSnapshot struct {
	Name    string  `json:"name"`
	BankID  string  `json:"bank_id"`
	Balance *string `json:"balance_decimal"`
}
type fnbSnapshot struct {
	Accounts    []fnbAccountSnapshot `json:"accounts"`
	Reports     []FNBReport          `json:"reports,omitempty"`
	Skipped     int                  `json:"skipped"`
	Error       string               `json:"error,omitempty"`
	Diagnostics map[string]int       `json:"diagnostics,omitempty"`
}

var fnbNumber = regexp.MustCompile(`^[0-9]{3,64}$`)

func (a *App) fnbKey(create bool) ([]byte, error) {
	path := a.fnbKeyPath
	if path == "" {
		path = os.Getenv("FNB_KEY_FILE")
	}
	if path == "" {
		home, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}
		path = filepath.Join(home, "finance-tracker", "fnb.key")
	}
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("invalid key location")
	}
	if create {
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return nil, err
		}
	}
	parent, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	path = filepath.Join(parent, filepath.Base(path))
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, directory := range []string{cwd, a.BackupDir} {
		if directory == "" {
			continue
		}
		absolute, err := filepath.Abs(directory)
		if err != nil {
			return nil, err
		}
		rel, err := filepath.Rel(absolute, path)
		if err != nil {
			return nil, err
		}
		if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return nil, fmt.Errorf("key must be outside workspace and backups")
		}
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return nil, fmt.Errorf("invalid key permissions")
		}
		key, err := os.ReadFile(path)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("invalid key")
		}
		return key, nil
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if !create {
		return nil, fmt.Errorf("key unavailable")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	if _, err = file.Write(key); err != nil {
		return nil, err
	}
	if err = file.Sync(); err != nil {
		return nil, err
	}
	return key, nil
}
func fnbSeal(key, plain []byte, owner int64) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, g.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}
	return g.Seal(nonce, nonce, plain, []byte(fmt.Sprintf("fnb-v1:%d", owner))), nil
}
func fnbUnseal(key, secret []byte, owner int64) (fnbCredentials, error) {
	var out fnbCredentials
	block, err := aes.NewCipher(key)
	if err != nil {
		return out, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil || len(secret) < g.NonceSize() {
		return out, fmt.Errorf("invalid secret")
	}
	plain, err := g.Open(nil, secret[:g.NonceSize()], secret[g.NonceSize():], []byte(fmt.Sprintf("fnb-v1:%d", owner)))
	if err != nil {
		return out, err
	}
	err = json.Unmarshal(plain, &out)
	clear(plain)
	return out, err
}
func (a *App) fnbStatus(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	rows, err := data(a.DB, "SELECT interval_hours,state,last_attempt,last_success,next_due,last_error,last_skipped,debug_browser,last_diagnostics,version FROM fnb_connections WHERE user_id=?", u.ID)
	if err != nil {
		return err
	}
	discoveries := []map[string]any{}
	if r.URL.Query().Get("summary") != "1" {
		discoveries, err = data(a.DB, "SELECT bank_id,name,balance_cents,balance_date,hidden,account_id FROM fnb_discoveries WHERE user_id=? ORDER BY name,bank_id LIMIT 100", u.ID)
		if err != nil {
			return err
		}
	}
	var connection any = nil
	if len(rows) > 0 {
		connection = rows[0]
		var diagnostics map[string]int
		if text, ok := rows[0]["last_diagnostics"].(string); ok {
			json.Unmarshal([]byte(text), &diagnostics)
		}
		rows[0]["last_diagnostics"] = safeFNBDiagnostics(diagnostics)
	}
	refreshingID := int64(0)
	kind := ""
	if len(rows) > 0 && rows[0]["state"] == "refreshing" {
		refreshingID = a.fnbRefreshAccount.Load()
		kind = "accounts"
		if a.fnbRefreshTransactions.Load() {
			kind = "transactions"
		}
	}
	send(w, map[string]any{"connection": connection, "accounts": discoveries, "refreshing_account_id": refreshingID, "refresh_kind": kind})
	return nil
}
func (a *App) fnbConnect(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	if !a.Secure && !strings.HasPrefix(a.PublicURL, "http://127.0.0.1:") && !strings.HasPrefix(a.PublicURL, "http://localhost:") {
		return fail(400, "Use HTTPS to enter bank credentials")
	}
	var b struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Version  int64  `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if strings.TrimSpace(b.Username) == "" || len(b.Username) > 200 || b.Password == "" || len(b.Password) > 512 {
		return fail(400, "Enter your FNB username and password")
	}
	a.fnbMu.Lock()
	defer a.fnbMu.Unlock()
	exists := queryInt(a.DB, "SELECT COUNT(*) FROM fnb_connections WHERE user_id=?", u.ID) > 0
	key, err := a.fnbKey(!exists && queryInt(a.DB, "SELECT COUNT(*) FROM fnb_connections") == 0)
	if err != nil {
		return fail(503, "FNB encryption key unavailable. Restore the key before saving credentials.")
	}
	plain, _ := json.Marshal(fnbCredentials{Username: b.Username, Password: b.Password})
	secret, err := fnbSeal(key, plain, u.ID)
	clear(plain)
	clear(key)
	if err != nil {
		return err
	}
	err = a.write(func(tx *sql.Tx) error {
		if exists {
			res, err := tx.Exec("UPDATE fnb_connections SET secret=?,state='ready',last_error='',next_due=NULL,interval_hours=0,version=version+1 WHERE user_id=? AND version=?", secret, u.ID, b.Version)
			if err != nil {
				return err
			}
			if err = affected(res); err != nil {
				return err
			}
		} else {
			if b.Version != 0 {
				return fail(409, "Connection changed. Reload before saving.")
			}
			if _, err := tx.Exec("INSERT INTO fnb_connections(user_id,secret) VALUES(?,?)", u.ID, secret); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "credentials_replaced", map[string]bool{"schedule_paused": true})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbSchedule(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Hours   int   `json:"interval_hours"`
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Hours != 0 && b.Hours != 6 && b.Hours != 12 && b.Hours != 24 && b.Hours != 168 {
		return fail(400, "Choose a supported refresh interval")
	}
	var due any = nil
	if b.Hours > 0 {
		due = time.Now().Add(time.Duration(b.Hours) * time.Hour).Unix()
	}
	err := a.write(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE fnb_connections SET interval_hours=?,next_due=?,version=version+1 WHERE user_id=? AND version=? AND state!='refreshing'", b.Hours, due, u.ID, b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "schedule_updated", map[string]int{"interval_hours": b.Hours})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbDisconnect(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	a.fnbMu.Lock()
	defer a.fnbMu.Unlock()
	err := a.write(func(tx *sql.Tx) error {
		if _, err := tx.Exec("DELETE FROM fnb_connections WHERE user_id=?", u.ID); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "disconnected", nil)
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbVisibility(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		BankID string `json:"bank_id"`
		Hidden bool   `json:"hidden"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		var id sql.NullInt64
		if err := tx.QueryRow("SELECT account_id FROM fnb_discoveries WHERE user_id=? AND bank_id=?", u.ID, b.BankID).Scan(&id); err != nil {
			return fail(404, "Discovered account not found")
		}
		if id.Valid {
			if !a.can(tx, u, id.Int64, true) {
				return fail(403, "Account editor access required")
			}
			if _, err := tx.Exec("UPDATE accounts SET sync_hidden=?,version=version+1 WHERE id=?", b.Hidden, id.Int64); err != nil {
				return err
			}
		}
		if _, err := tx.Exec("UPDATE fnb_discoveries SET hidden=? WHERE user_id=? AND bank_id=?", b.Hidden, u.ID, b.BankID); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "account_visibility_updated", map[string]any{"hidden": b.Hidden})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) fnbRefresh(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		AccountID int64 `json:"account_id"`
	}
	if r.ContentLength != 0 {
		if err := decode(r, &b); err != nil {
			return err
		}
	}
	if b.AccountID < 0 {
		return fail(400, "Choose an account")
	}
	if _, err := a.runFNB(u.ID, true, false, b.AccountID); err != nil {
		return err
	}
	success(w)
	return nil
}
func (a *App) refreshFNB(owner int64, manual bool) error {
	_, err := a.runFNB(owner, manual, false)
	return err
}
func (a *App) runFNB(owner int64, manual, transactions bool, scope ...int64) ([]ParsedFile, error) {
	select {
	case <-a.stop:
		return nil, fail(503, "Tracker is stopping")
	default:
	}
	if !a.fnbMu.TryLock() {
		return nil, fail(409, "An FNB refresh is already running")
	}
	defer a.fnbMu.Unlock()
	selected := int64(0)
	if len(scope) > 0 {
		selected = scope[0]
	}
	a.fnbRefreshTransactions.Store(transactions)
	defer a.fnbRefreshTransactions.Store(false)
	a.fnbRefreshAccount.Store(selected)
	defer a.fnbRefreshAccount.Store(0)
	var u User
	var disabled bool
	if err := a.DB.QueryRow("SELECT id,username,admin,budget_member,disabled FROM users WHERE id=?", owner).Scan(&u.ID, &u.Username, &u.Admin, &u.Member, &disabled); err != nil || disabled || !u.Admin {
		return nil, fail(403, "Connection owner must be an enabled administrator")
	}
	var secret []byte
	var interval int
	var debug bool
	var state string
	if err := a.DB.QueryRow("SELECT secret,interval_hours,state,debug_browser FROM fnb_connections WHERE user_id=?", owner).Scan(&secret, &interval, &state, &debug); err != nil {
		return nil, fail(404, "Connect FNB first")
	}
	if !manual && state != "ready" {
		return nil, nil
	}
	var targets []fnbTarget
	if transactions {
		var err error
		targets, err = a.fnbTargets(a.DB, u)
		if err != nil {
			return nil, err
		}
		if len(targets) == 0 {
			return nil, fail(400, "No enabled FNB accounts with editor access. Refresh accounts first.")
		}
	}
	var selectedBank string
	if selected > 0 {
		if !ruleAccess(a.DB, a, u, selected) {
			return nil, fail(403, "Account editor access required")
		}
		if err := a.DB.QueryRow("SELECT a.bank_id FROM accounts a JOIN fnb_discoveries d ON d.account_id=a.id AND d.bank_id=a.bank_id WHERE a.id=? AND d.user_id=? AND d.hidden=0", selected, owner).Scan(&selectedBank); err != nil {
			return nil, fail(400, "Account is not connected to your FNB profile")
		}
	}
	key, err := a.fnbKey(false)
	var credentials fnbCredentials
	if err == nil {
		credentials, err = fnbUnseal(key, secret, owner)
	}
	clear(key)
	if err != nil {
		a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error='KEY_UNAVAILABLE',next_due=NULL,version=version+1 WHERE user_id=?", owner)
		return nil, fail(503, "FNB encryption key unavailable")
	}
	hiddenRows, err := data(a.DB, "SELECT d.bank_id FROM fnb_discoveries d LEFT JOIN accounts a ON a.id=d.account_id WHERE d.user_id=? AND (d.hidden=1 OR a.sync_hidden=1)", owner)
	if err != nil {
		return nil, err
	}
	if selected > 0 {
		others, e := data(a.DB, "SELECT bank_id FROM fnb_discoveries WHERE user_id=? AND bank_id!=?", owner, selectedBank)
		if e != nil {
			return nil, e
		}
		hiddenRows = append(hiddenRows, others...)
	}
	for _, row := range hiddenRows {
		credentials.Hidden = append(credentials.Hidden, row["bank_id"].(string))
	}
	if transactions {
		credentials.RunID = randomToken()
		for _, target := range targets {
			credentials.TransactionAccounts = append(credentials.TransactionAccounts, target.BankID)
		}
	}
	runID := credentials.RunID
	now := time.Now().UTC().Format(time.RFC3339)
	if _, err := a.DB.Exec("UPDATE fnb_connections SET state='refreshing',last_attempt=?,last_error='',last_diagnostics='{}',version=version+1 WHERE user_id=?", now, owner); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-a.stop:
			cancel()
		case <-done:
		}
	}()
	provider := a.fnbProvider
	if provider == nil {
		provider = runFNBProvider
	}
	var previews []ParsedFile
	snapshot, err := provider(ctx, credentials, debug)
	credentials = fnbCredentials{}
	code := "REFRESH_FAILED"
	if err == nil && snapshot.Error != "" {
		code = snapshot.Error
		err = fmt.Errorf("provider failure")
	}
	switch code {
	case "SESSION_CONFLICT", "LOGIN_LAYOUT_CHANGED", "APPROVAL_REQUIRED", "ACCOUNT_LAYOUT_CHANGED", "BROWSER_NOT_FOUND", "KEY_UNAVAILABLE", "BALANCE_LAYOUT_CHANGED", "LOGOUT_REQUIRED", "TRANSACTION_LAYOUT_CHANGED", "TRANSACTION_ACCOUNT_MISMATCH", "TRANSACTION_FEE_REVIEW_REQUIRED", "TRANSACTION_ACCOUNT_UNSUPPORTED":
	default:
		code = "REFRESH_FAILED"
	}
	if err == nil {
		now = time.Now().UTC().Format(time.RFC3339)
		if transactions {
			previews, err = a.stageFNBTransactions(u, targets, snapshot, runID)
		} else {
			if selected > 0 {
				filtered := snapshot.Accounts[:0]
				for _, account := range snapshot.Accounts {
					if account.BankID == selectedBank {
						filtered = append(filtered, account)
					}
				}
				snapshot.Accounts = filtered
				if !ruleAccess(a.DB, a, u, selected) {
					err = fail(403, "Account editor access required")
				} else {
					err = a.applyFNBSnapshot(u, snapshot, now)
				}
			} else {
				err = a.applyFNBSnapshot(u, snapshot, now)
			}
		}
		if err != nil {
			code = "ACCOUNT_LAYOUT_CHANGED"
			if transactions {
				code = "TRANSACTION_LAYOUT_CHANGED"
			}
		}
	}
	if err != nil {
		a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error=?,last_diagnostics=?,next_due=NULL,version=version+1 WHERE user_id=?", code, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
		return nil, fail(502, "FNB refresh needs attention: "+code)
	}
	var due any = nil
	if interval > 0 {
		due = time.Now().Add(time.Duration(interval) * time.Hour).Unix()
	}
	if transactions {
		// Account balance dates and last successful balance refresh remain accurate.
		_, err = a.DB.Exec("UPDATE fnb_connections SET state='ready',next_due=?,last_error='',last_diagnostics=?,version=version+1 WHERE user_id=?", due, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
		return previews, err
	}
	_, err = a.DB.Exec("UPDATE fnb_connections SET state='ready',last_success=?,next_due=?,last_error='',last_skipped=?,last_diagnostics=?,version=version+1 WHERE user_id=?", now, due, snapshot.Skipped, fnbDiagnosticJSON(snapshot.Diagnostics), owner)
	return previews, err
}
func (a *App) applyFNBSnapshot(u User, snapshot fnbSnapshot, observed string) error {
	if len(snapshot.Accounts) == 0 || len(snapshot.Accounts) > 100 || snapshot.Skipped < 0 || snapshot.Skipped > 100 {
		return fmt.Errorf("invalid discovery")
	}
	seen := map[string]bool{}
	for _, row := range snapshot.Accounts {
		if !fnbNumber.MatchString(row.BankID) || strings.TrimSpace(row.Name) == "" || len(row.Name) > 100 || seen[row.BankID] {
			return fmt.Errorf("invalid discovery")
		}
		seen[row.BankID] = true
		if row.Balance != nil {
			if _, err := Cents(*row.Balance); err != nil {
				return fmt.Errorf("invalid balance")
			}
		}
	}
	return a.write(func(tx *sql.Tx) error {
		// Recheck permissions after network work; no grant or preference changes occur during sync.
		var active bool
		if err := tx.QueryRow("SELECT admin=1 AND disabled=0,budget_member FROM users WHERE id=?", u.ID).Scan(&active, &u.Member); err != nil || !active {
			return fail(403, "Connection access revoked")
		}
		for _, row := range snapshot.Accounts {
			var balance any = nil
			observedTime, err := time.Parse(time.RFC3339, observed)
			if err != nil {
				return err
			}
			loc, _ := time.LoadLocation("Africa/Johannesburg")
			date := observedTime.In(loc).Format("2006-01-02")
			if row.Balance != nil {
				cents, _ := Cents(*row.Balance)
				balance = cents
			} else {
				date = ""
			}
			var hidden bool
			var linked sql.NullInt64
			err = tx.QueryRow("SELECT hidden,account_id FROM fnb_discoveries WHERE user_id=? AND bank_id=?", u.ID, row.BankID).Scan(&hidden, &linked)
			if err != nil && err != sql.ErrNoRows {
				return err
			}
			if !hidden {
				if !linked.Valid {
					err := tx.QueryRow("SELECT id FROM accounts WHERE bank_id=?", row.BankID).Scan(&linked)
					if err != nil && err != sql.ErrNoRows {
						return err
					}
				}
				if linked.Valid {
					if !a.can(tx, u, linked.Int64, true) {
						return fail(403, "Account editor access required")
					}
					var accountHidden bool
					tx.QueryRow("SELECT sync_hidden FROM accounts WHERE id=?", linked.Int64).Scan(&accountHidden)
					hidden = accountHidden
				} else {
					res, err := tx.Exec("INSERT INTO accounts(name,bank_id,household) VALUES(?,?,0)", row.Name, row.BankID)
					if err != nil {
						return err
					}
					id, _ := res.LastInsertId()
					linked = sql.NullInt64{Int64: id, Valid: true}
					if _, err = tx.Exec("INSERT INTO grants VALUES(?,?,'editor')", u.ID, id); err != nil {
						return err
					}
				}
				if !hidden {
					if _, err := tx.Exec("UPDATE accounts SET name=?,balance_cents=CASE WHEN ? IS NULL THEN balance_cents ELSE ? END,balance_date=CASE WHEN ? IS NULL THEN balance_date ELSE ? END,version=version+1 WHERE id=?", row.Name, balance, balance, balance, date, linked.Int64); err != nil {
						return err
					}
				}
			}
			if _, err := tx.Exec("INSERT INTO fnb_discoveries(user_id,bank_id,name,balance_cents,balance_date,hidden,account_id) VALUES(?,?,?,?,?,?,?) ON CONFLICT(user_id,bank_id) DO UPDATE SET name=excluded.name,balance_cents=CASE WHEN fnb_discoveries.hidden=1 THEN fnb_discoveries.balance_cents ELSE excluded.balance_cents END,balance_date=CASE WHEN fnb_discoveries.hidden=1 THEN fnb_discoveries.balance_date ELSE excluded.balance_date END,account_id=excluded.account_id,hidden=excluded.hidden", u.ID, row.BankID, row.Name, balance, date, hidden, linked); err != nil {
				return err
			}
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "accounts_refreshed", map[string]int{"returned": len(snapshot.Accounts), "skipped_unsupported": snapshot.Skipped})
	})
}

// Secrets travel over stdin, never arguments/environment or error logs. The child
// emits a strict account snapshot or an allowlisted generic failure code.
func runFNBProvider(ctx context.Context, credentials fnbCredentials, manual bool) (fnbSnapshot, error) {
	var snapshot fnbSnapshot
	cwd, err := os.Getwd()
	if err != nil {
		return snapshot, err
	}
	executable := os.Getenv("FNB_NODE_EXECUTABLE")
	runner := os.Getenv("FNB_RUNNER_PATH")
	if executable == "" {
		windows := "/mnt/c/Program Files/nodejs/node.exe"
		if _, err := os.Stat(windows); err == nil {
			executable = windows
			if runner == "" {
				runner = `\\wsl.localhost\Ubuntu` + strings.ReplaceAll(filepath.Join(cwd, "connectors/fnb/owner/refresh.mjs"), "/", `\`)
			}
		} else {
			executable = "node"
		}
	}
	if runner == "" {
		runner = filepath.Join(cwd, "connectors/fnb/owner/refresh.mjs")
	}
	payload, _ := json.Marshal(map[string]any{"username": credentials.Username, "password": credentials.Password, "visible": manual, "hidden": credentials.Hidden, "transaction_accounts": credentials.TransactionAccounts, "run_id": credentials.RunID})
	cmd := exec.CommandContext(ctx, executable, runner)
	cmd.Stdin = bytes.NewReader(payload)
	defer clear(payload)
	var output bytes.Buffer
	cmd.Stdout = &limitedFNBWriter{writer: &output, remaining: 16 << 20}
	cmd.Stderr = io.Discard
	if err = cmd.Run(); err != nil {
		return snapshot, fmt.Errorf("connector unavailable")
	}
	d := json.NewDecoder(&output)
	d.DisallowUnknownFields()
	if err = d.Decode(&snapshot); err != nil {
		return snapshot, fmt.Errorf("invalid connector result")
	}
	if d.Decode(new(any)) != io.EOF {
		return snapshot, fmt.Errorf("invalid connector result")
	}
	return snapshot, nil
}

type limitedFNBWriter struct {
	writer    io.Writer
	remaining int
}

func (w *limitedFNBWriter) Write(p []byte) (int, error) {
	if len(p) > w.remaining {
		return 0, fmt.Errorf("connector output exceeded")
	}
	w.remaining -= len(p)
	return w.writer.Write(p)
}
func (a *App) StartFNBScheduler() {
	a.DB.Exec("UPDATE fnb_connections SET state='action_required',last_error='INTERRUPTED',next_due=NULL,version=version+1 WHERE state='refreshing'")
	a.fnbWG.Add(1)
	go func() {
		defer a.fnbWG.Done()
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-a.stop:
				return
			case <-ticker.C:
				a.refreshDueFNB(time.Now())
			}
		}
	}()
}

func (a *App) refreshDueFNB(now time.Time) {
	rows, err := data(a.DB, "SELECT c.user_id FROM fnb_connections c JOIN users u ON u.id=c.user_id WHERE c.state='ready' AND c.interval_hours>0 AND c.next_due<=? AND u.admin=1 AND u.disabled=0", now.Unix())
	if err != nil {
		return
	}
	for _, row := range rows {
		select {
		case <-a.stop:
			return
		default:
		}
		a.refreshFNB(num(row["user_id"]), false)
	}
}

func safeFNBDiagnostics(input map[string]int) map[string]int {
	out := map[string]int{}
	for _, key := range []string{"name_nodes", "number_nodes", "ledger_nodes", "matched_rows", "missing_rows", "invalid_amounts", "unsupported_entries", "hidden_rows", "blank_balances", "placeholder_balances", "sign_suffix", "parenthesized", "comma_decimal", "other_format", "evaluation_failed", "trailing_minus", "currency_suffix", "unknown_text", "repeated_decimal", "balance_label", "unavailable_text", "loading_text", "logout_clicked", "logout_confirmed", "logout_unconfirmed", "balance_failure", "reward_entries", "non_zar_entries", "transaction_rows", "transaction_headers", "transaction_invalid_rows", "transaction_accounts", "transaction_failure", "transaction_identity_fields", "transaction_type_fields", "transaction_navigation", "transaction_accounts_requested", "transaction_failed_account_position", "transaction_unsupported_type", "transaction_unsupported_currency", "transaction_fee_rows", "transaction_successful_controls", "transaction_pending_controls", "transaction_selected_successful"} {
		if value, ok := input[key]; ok && value >= 0 && value <= 10000 {
			out[key] = value
		}
	}
	return out
}
func fnbDiagnosticJSON(input map[string]int) string {
	value, _ := json.Marshal(safeFNBDiagnostics(input))
	return string(value)
}
func (a *App) fnbDebug(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b struct {
		Debug   bool  `json:"debug_browser"`
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	err := a.write(func(tx *sql.Tx) error {
		res, err := tx.Exec("UPDATE fnb_connections SET debug_browser=?,version=version+1 WHERE user_id=? AND version=? AND state!='refreshing'", b.Debug, u.ID, b.Version)
		if err != nil {
			return err
		}
		if err = affected(res); err != nil {
			return err
		}
		return audit(tx, u, nil, "fnb_connection", u.ID, "browser_mode_updated", map[string]bool{"debug_browser": b.Debug})
	})
	if err != nil {
		return err
	}
	success(w)
	return nil
}
