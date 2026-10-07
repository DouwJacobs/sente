package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	problemerror "finance-tracker/internal/problem"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

type App struct {
	RequestRestart func()

	mcpCursorOnce          sync.Once
	mcpCursorKey           []byte
	mcpCursorError         error
	mcpOnce                sync.Once
	mcpHTTP                http.Handler
	fnbMu                  sync.Mutex
	fnbRefreshAccount      atomic.Int64
	fnbRefreshTransactions atomic.Bool
	fnbWG                  sync.WaitGroup
	fnbKeyPath             string
	fnbProvider            func(context.Context, fnbCredentials, bool) (fnbSnapshot, error)
	DB                     *sql.DB
	networkEnvironment     networkValues
	networkSource          string
	activeNetworkTrusted   string
	trustedProxies         []netip.Prefix
	PublicURL              string
	Secure                 bool
	BackupDir              string
	mu                     sync.Mutex
	loginMu                sync.Mutex
	attempts               map[string][]time.Time
	stop                   chan struct{}
	backupError            string
	setupToken             string
	backupWG               sync.WaitGroup
	lock                   *os.File
}
type User struct {
	MCPAccounts []int64 `json:"-"`
	ID          int64   `json:"id"`
	Username    string  `json:"username"`
	Admin       bool    `json:"admin"`
	Member      bool    `json:"budget_member"`
}
type authContext struct {
	User User
	CSRF string
}
type ctxKey int

const authKey ctxKey = 0

type problem = problemerror.Error

func fail(code int, s string) error { return problemerror.New(code, s) }

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
	Exec(string, ...any) (sql.Result, error)
}

func Open(path, publicURL, backupDir string) (*App, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	lock, err := lockDB(path)
	if err != nil {
		return nil, err
	}
	opened := false
	defer func() {
		if !opened {
			lock.Close()
		}
	}()
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	file.Close()
	if err := os.Chmod(path, 0600); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err = migrate(db); err != nil {
		db.Close()
		return nil, err
	}
	u, err := url.Parse(publicURL)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		db.Close()
		return nil, errors.New("PUBLIC_URL must be an absolute URL")
	}
	opened = true
	return &App{DB: db, PublicURL: strings.TrimRight(publicURL, "/"), Secure: u.Scheme == "https", BackupDir: backupDir, setupToken: randomToken(), attempts: map[string][]time.Time{}, stop: make(chan struct{}), lock: lock}, nil
}
func (a *App) Close() error {
	close(a.stop)
	a.backupWG.Wait()
	a.fnbWG.Wait()
	a.fnbMu.Lock()
	defer a.fnbMu.Unlock()
	err := a.DB.Close()
	a.lock.Close()
	return err
}
func randomToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func hash(s string) string         { v := sha256.Sum256([]byte(s)); return hex.EncodeToString(v[:]) }
func Current(r *http.Request) User { return r.Context().Value(authKey).(authContext).User }
func (a *App) CreateUser(username, password string, admin, member bool) (int64, error) {
	username = strings.TrimSpace(username)
	if len(username) < 2 || len(username) > 80 || len(password) < 12 || len(password) > 72 {
		return 0, fail(400, "Use a username of 2–80 characters and password of 12–72 bytes")
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}
	result, err := a.DB.Exec("INSERT INTO users(username,password,admin,budget_member) VALUES(?,?,?,?)", username, string(digest), admin, member)
	if err != nil {
		return 0, fail(409, "Username already exists")
	}
	return result.LastInsertId()
}
func (a *App) ResetPassword(username, password string) error {
	if len(password) < 12 || len(password) > 72 {
		return fail(400, "Password must be 12–72 bytes")
	}
	digest, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return a.write(func(tx *sql.Tx) error {
		var id int64
		if err := tx.QueryRow("SELECT id FROM users WHERE username=? AND deleted_at IS NULL", username).Scan(&id); err != nil {
			return errors.New("User not found")
		}
		if err := setUserPasswordTx(tx, id, string(digest), ""); err != nil {
			return err
		}
		return audit(tx, User{ID: id, Username: username}, nil, "user", id, "password_recovered", map[string]any{"method": "offline", "all_sessions_revoked": true})
	})
}
func (a *App) write(fn func(*sql.Tx) error) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	tx, err := a.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err = fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
func data(q queryer, s string, args ...any) ([]map[string]any, error) {
	rows, err := q.Query(s, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := []map[string]any{}
	for rows.Next() {
		v := make([]any, len(cols))
		ptr := make([]any, len(cols))
		for i := range v {
			ptr[i] = &v[i]
		}
		if err := rows.Scan(ptr...); err != nil {
			return nil, err
		}
		item := map[string]any{}
		for i, c := range cols {
			if b, ok := v[i].([]byte); ok {
				v[i] = string(b)
			}
			item[c] = v[i]
		}
		result = append(result, item)
	}
	return result, rows.Err()
}
func num(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return int64(x)
	case int:
		return int64(x)
	}
	return 0
}
func parseID(r *http.Request) int64 { v, _ := strconv.ParseInt(r.PathValue("id"), 10, 64); return v }
func decode(r *http.Request, v any) error {
	d := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		return fail(400, "Invalid request fields")
	}
	if d.Decode(new(any)) != io.EOF {
		return fail(400, "Request must contain one JSON value")
	}
	return nil
}
func send(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

type handler func(http.ResponseWriter, *http.Request) error

func wrap(h handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := h(w, r); err != nil {
			code := 500
			message := "Request could not be completed"
			var p problem
			if errors.As(err, &p) {
				code = p.Code
				message = p.Message
			} else {
				log.Printf("request failed: %T", err)
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(map[string]string{"error": message})
		}
	}
}
func (a *App) can(q queryer, u User, account int64, edit bool) bool {
	if !mcpAccountAllowed(u, account) {
		return false
	}
	var role string
	err := q.QueryRow("SELECT CASE WHEN a.household=1 AND ?=1 THEN 'editor' ELSE COALESCE(g.role,'') END FROM accounts a LEFT JOIN grants g ON g.account_id=a.id AND g.user_id=? WHERE a.id=?", u.Member, u.ID, account).Scan(&role)
	return err == nil && (role == "editor" || (!edit && role == "viewer"))
}
func requireAdmin(u User) error {
	if !u.Admin {
		return fail(403, "Administrator access required")
	}
	return nil
}
func requireMember(u User) error {
	if !u.Member {
		return fail(403, "Household budget membership required")
	}
	return nil
}
func audit(q queryer, u User, account any, entity string, id int64, action string, detail any) error {
	b, _ := json.Marshal(detail)
	_, err := q.Exec("INSERT INTO audit(user_id,account_id,entity,entity_id,action,details) VALUES(?,?,?,?,?,?)", u.ID, account, entity, id, action, string(b))
	return err
}
func validDate(s string) bool {
	t, err := time.Parse("2006-01-02", s)
	return err == nil && t.Format("2006-01-02") == s
}
func (a *App) Handler(static string) http.Handler {
	mux := http.NewServeMux()
	a.oauthRoutes(mux)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) { send(w, map[string]string{"status": "ok"}) })
	mux.HandleFunc("GET /api/setup", wrap(a.setupStatus))
	mux.HandleFunc("POST /api/setup", wrap(a.setupAdmin))
	mux.HandleFunc("POST /api/login", wrap(a.login))
	mux.Handle("/api/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mcpOnce.Do(func() { a.mcpHTTP = a.mcpHandler() })
		a.mcpHTTP.ServeHTTP(w, r)
	}))
	mux.HandleFunc("/api/", a.protect(a.routes()))
	fs := http.FileServer(http.Dir(static))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" && r.Method != "HEAD" {
			http.NotFound(w, r)
			return
		}
		path := filepath.Join(static, filepath.Clean("/"+r.URL.Path))
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			fs.ServeHTTP(w, r)
			return
		}
		if strings.Contains(filepath.Base(r.URL.Path), ".") {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, filepath.Join(static, "index.html"))
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "same-origin")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Cache-Control", "no-store")
		}
		if strings.HasPrefix(r.URL.Path, "/oauth/") || strings.HasPrefix(r.URL.Path, "/.well-known/") {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		if r.URL.Path == "/mcp/authorize" {
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.URL.Path != "/oauth/token" && r.URL.Path != "/oauth/register" && r.URL.Path != "/oauth/revoke" {
			if origin := r.Header.Get("Origin"); origin != "" && origin != a.PublicURL {
				wrap(func(http.ResponseWriter, *http.Request) error { return fail(403, "Origin not allowed") })(w, r)
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *App) protect(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("finance_session")
		if err != nil {
			wrap(func(http.ResponseWriter, *http.Request) error { return fail(401, "Please sign in") })(w, r)
			return
		}
		var auth authContext
		err = a.DB.QueryRow("SELECT u.id,u.username,u.admin,u.budget_member,s.csrf FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=? AND s.expires_at>? AND u.disabled=0", hash(cookie.Value), time.Now().Unix()).Scan(&auth.User.ID, &auth.User.Username, &auth.User.Admin, &auth.User.Member, &auth.CSRF)
		if err != nil {
			wrap(func(http.ResponseWriter, *http.Request) error { return fail(401, "Session expired") })(w, r)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("X-CSRF-Token") != auth.CSRF {
			wrap(func(http.ResponseWriter, *http.Request) error { return fail(403, "Invalid security token") })(w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authKey, auth)))
	}
}
func (a *App) login(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decode(r, &body); err != nil {
		return err
	}
	host := a.clientIP(r)
	a.loginMu.Lock()
	now := time.Now()
	recent := []time.Time{}
	for _, t := range a.attempts[host] {
		if now.Sub(t) < 15*time.Minute {
			recent = append(recent, t)
		}
	}
	if len(recent) >= 10 {
		a.loginMu.Unlock()
		return fail(429, "Too many attempts; try again in 15 minutes")
	}
	a.attempts[host] = append(recent, now)
	a.loginMu.Unlock()
	var u User
	var password string
	err := a.DB.QueryRow("SELECT id,username,password,admin,budget_member FROM users WHERE username=? AND disabled=0", body.Username).Scan(&u.ID, &u.Username, &password, &u.Admin, &u.Member)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(password), []byte(body.Password)) != nil {
		return fail(401, "Username or password is incorrect")
	}
	a.loginMu.Lock()
	delete(a.attempts, host)
	a.loginMu.Unlock()
	token := randomToken()
	csrf := randomToken()
	expires := time.Now().Add(7 * 24 * time.Hour)
	if err := a.write(func(tx *sql.Tx) error {
		// A reset/deletion that raced bcrypt verification must win over this login.
		if err := tx.QueryRow("SELECT username,admin,budget_member FROM users WHERE id=? AND password=? AND disabled=0 AND deleted_at IS NULL", u.ID, password).Scan(&u.Username, &u.Admin, &u.Member); err != nil {
			return fail(401, "Username or password is incorrect")
		}
		_, err := tx.Exec("INSERT INTO sessions VALUES(?,?,?,?)", hash(token), u.ID, csrf, expires.Unix())
		return err
	}); err != nil {
		return err
	}
	http.SetCookie(w, &http.Cookie{Name: "finance_session", Value: token, Path: "/", HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode, Expires: expires})
	send(w, map[string]any{"user": u, "csrf": csrf})
	return nil
}
func (a *App) routes() http.Handler {
	m := http.NewServeMux()
	a.workflowRoutes(m)
	m.HandleFunc("GET /api/build", wrap(a.buildInfo))
	m.HandleFunc("GET /api/mcp/settings", wrap(a.mcpSettings))
	m.HandleFunc("GET /api/mcp/authorization/{id}", wrap(a.mcpAuthorization))
	m.HandleFunc("POST /api/mcp/authorization/{id}", wrap(a.decideMCPAuthorization))
	m.HandleFunc("DELETE /api/mcp/connections/{id}", wrap(a.revokeMCPToken))
	m.HandleFunc("PUT /api/mcp/connections/{id}/permissions", wrap(a.updateMCPPermissions))
	m.HandleFunc("DELETE /api/mcp/tokens/{id}", wrap(a.revokeMCPToken))
	m.HandleFunc("POST /api/mcp/proposals/{id}", wrap(a.decideMCPProposal))
	m.HandleFunc("POST /api/mcp/proposals/batch", wrap(a.decideMCPProposalBatch))
	m.HandleFunc("GET /api/network", wrap(a.networkStatus))
	m.HandleFunc("PUT /api/network", wrap(a.saveNetwork))
	m.HandleFunc("POST /api/network/restart", wrap(a.restartNetwork))
	m.HandleFunc("GET /api/me", wrap(func(w http.ResponseWriter, r *http.Request) error {
		v := r.Context().Value(authKey).(authContext)
		send(w, map[string]any{"user": v.User, "csrf": v.CSRF})
		return nil
	}))
	m.HandleFunc("POST /api/logout", wrap(func(w http.ResponseWriter, r *http.Request) error {
		c, _ := r.Cookie("finance_session")
		_, err := a.DB.Exec("DELETE FROM sessions WHERE token=?", hash(c.Value))
		http.SetCookie(w, &http.Cookie{Name: "finance_session", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: a.Secure, SameSite: http.SameSiteStrictMode})
		success(w)
		return err
	}))
	m.HandleFunc("POST /api/password", wrap(a.changePassword))
	m.HandleFunc("GET /api/fnb", wrap(a.fnbStatus))
	m.HandleFunc("PUT /api/fnb", wrap(a.fnbConnect))
	m.HandleFunc("DELETE /api/fnb", wrap(a.fnbDisconnect))
	m.HandleFunc("PUT /api/fnb/debug", wrap(a.fnbDebug))
	m.HandleFunc("PUT /api/fnb/schedule", wrap(a.fnbSchedule))
	m.HandleFunc("POST /api/fnb/refresh", wrap(a.fnbRefresh))
	m.HandleFunc("POST /api/fnb/transactions", wrap(a.fnbTransactions))
	m.HandleFunc("PUT /api/fnb/accounts", wrap(a.fnbVisibility))
	m.HandleFunc("GET /api/accounts/manage", wrap(a.manageAccounts))
	m.HandleFunc("PUT /api/accounts/{id}/visibility", wrap(a.accountVisibility))
	m.HandleFunc("GET /api/accounts", wrap(a.accounts))
	m.HandleFunc("POST /api/accounts", wrap(a.createAccount))
	m.HandleFunc("PUT /api/accounts/{id}", wrap(a.updateAccount))
	m.HandleFunc("GET /api/users", wrap(a.users))
	m.HandleFunc("POST /api/users", wrap(a.createUser))
	m.HandleFunc("PUT /api/users/{id}", wrap(a.updateUser))
	m.HandleFunc("POST /api/users/{id}/password", wrap(a.adminPassword))
	m.HandleFunc("DELETE /api/users/{id}", wrap(a.deleteUser))
	m.HandleFunc("PUT /api/grants", wrap(a.grant))
	m.HandleFunc("GET /api/grants", wrap(a.grants))
	m.HandleFunc("GET /api/spending-groups", wrap(a.spendingGroups))
	m.HandleFunc("POST /api/spending-groups", wrap(a.saveSpendingGroup))
	m.HandleFunc("PUT /api/spending-groups/{id}", wrap(a.saveSpendingGroup))
	m.HandleFunc("GET /api/categories", wrap(a.categories))
	m.HandleFunc("POST /api/categories", wrap(a.createCategory))
	m.HandleFunc("GET /api/rules", wrap(a.rules))
	m.HandleFunc("POST /api/rules", wrap(a.saveRule))
	m.HandleFunc("DELETE /api/rules/{id}", wrap(a.deleteRule))
	m.HandleFunc("PUT /api/rules/{id}", wrap(a.saveRule))
	m.HandleFunc("POST /api/rules/preview", wrap(a.previewRule))
	m.HandleFunc("POST /api/rules/batch", wrap(a.batchRules))
	m.HandleFunc("DELETE /api/rules/batch", wrap(a.batchRules))
	m.HandleFunc("GET /api/transactions", wrap(a.transactions))
	m.HandleFunc("PUT /api/transactions/{id}", wrap(a.editTransaction))
	m.HandleFunc("POST /api/review", wrap(a.review))
	m.HandleFunc("POST /api/transactions/seen", wrap(a.transactionSeen))
	m.HandleFunc("POST /api/transfers", wrap(a.linkTransfer))
	m.HandleFunc("DELETE /api/transfers/{id}", wrap(a.unlinkTransfer))
	m.HandleFunc("GET /api/audit/{id}", wrap(a.transactionAudit))
	m.HandleFunc("GET /api/imports", wrap(a.imports))
	m.HandleFunc("GET /api/imports/{id}", wrap(a.importRows))
	m.HandleFunc("GET /api/imports/{id}/rows/{row}/suggestions", wrap(a.importSuggestions))
	m.HandleFunc("POST /api/imports/preview", wrap(a.previewImport))
	m.HandleFunc("POST /api/imports/{id}/commit", wrap(a.commitImport))
	m.HandleFunc("GET /api/periods", wrap(a.periods))
	m.HandleFunc("GET /api/periods/{id}/targets", wrap(a.targetPages))
	m.HandleFunc("GET /api/periods/{id}/budget-groups", wrap(a.budgetGroupPages))
	m.HandleFunc("PUT /api/periods/{id}/budget", wrap(a.updateBudgetBuilder))
	m.HandleFunc("POST /api/periods", wrap(a.createPeriod))
	m.HandleFunc("POST /api/periods/{id}/preview", wrap(a.previewPeriod))
	m.HandleFunc("PUT /api/periods/{id}", wrap(a.updatePeriod))
	m.HandleFunc("PUT /api/targets/{id}", wrap(a.updateTargets))
	m.HandleFunc("GET /api/branding", wrap(a.branding))
	m.HandleFunc("PUT /api/branding", wrap(a.updateBranding))
	m.HandleFunc("GET /api/settings", wrap(a.settings))
	m.HandleFunc("PUT /api/settings", wrap(a.updateSettings))
	m.HandleFunc("GET /api/dashboard", wrap(a.dashboard))
	m.HandleFunc("GET /api/search", wrap(a.globalSearch))
	m.HandleFunc("GET /api/backups", wrap(a.backups))
	m.HandleFunc("POST /api/backups", wrap(a.backupNow))
	return m
}
func queryInt(q queryer, s string, args ...any) int64 {
	var n int64
	q.QueryRow(s, args...).Scan(&n)
	return n
}
func (a *App) Initialize() error {
	if queryInt(a.DB, "SELECT COUNT(*) FROM periods") > 0 {
		return nil
	}
	loc, _ := time.LoadLocation("Africa/Johannesburg")
	now := time.Now().In(loc)
	start := time.Date(now.Year(), now.Month(), 20, 0, 0, 0, 0, loc)
	if now.Before(start) {
		start = start.AddDate(0, -1, 0)
	}
	end := start.AddDate(0, 1, 0).AddDate(0, 0, -1)
	_, err := a.DB.Exec("INSERT INTO periods(name,start_date,end_date) VALUES(?,?,?)", start.Format("January 2006"), start.Format("2006-01-02"), end.Format("2006-01-02"))
	return err
}
func success(w http.ResponseWriter) { send(w, map[string]bool{"ok": true}) }
func affected(result sql.Result) error {
	n, _ := result.RowsAffected()
	if n == 0 {
		return fail(409, "This record changed. Reload before saving.")
	}
	return nil
}
