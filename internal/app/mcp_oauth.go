package app

import (
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const mcpReadScope = "finance:read"
const mcpWriteScope = "finance:propose"

var pkceVerifier = regexp.MustCompile(`^[A-Za-z0-9._~-]{43,128}$`)
var pkceChallenge = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func (a *App) mcpResource() string { return a.PublicURL + "/api/mcp" }
func (a *App) mcpChallenge(scope string) string {
	return `Bearer resource_metadata="` + a.PublicURL + `/.well-known/oauth-protected-resource/api/mcp", scope="` + scope + `"`
}
func oauthError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": code})
}
func safeOAuthURL(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.Fragment != "" {
		return false
	}
	return u.Scheme == "https" || u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")
}
func oauthScope(raw string) (string, bool) {
	read, write := false, false
	for _, s := range strings.Fields(raw) {
		switch s {
		case mcpReadScope:
			read = true
		case mcpWriteScope:
			write = true
		default:
			return "", false
		}
	}
	if !read {
		return "", false
	}
	if write {
		return mcpReadScope + " " + mcpWriteScope, true
	}
	return mcpReadScope, true
}

// RFC 8707 allows repeated resource indicators. This server has one audience:
// identical resource values are harmless; all other duplicate parameters remain
// ambiguous and must reject before client, redirect, PKCE or scope validation.
func normalizeOAuthParameters(values url.Values) bool {
	for key, entries := range values {
		if len(entries) == 1 {
			continue
		}
		if key != "resource" || len(entries) == 0 || entries[0] == "" {
			return false
		}
		for _, entry := range entries[1:] {
			if entry != entries[0] {
				return false
			}
		}
		values.Set(key, entries[0])
	}
	return true
}
func (a *App) oauthRoutes(mux *http.ServeMux) {
	metadata := func(w http.ResponseWriter, r *http.Request) {
		send(w, map[string]any{"resource": a.mcpResource(), "authorization_servers": []string{a.PublicURL}, "scopes_supported": []string{mcpReadScope, mcpWriteScope}, "bearer_methods_supported": []string{"header"}, "resource_name": "Sente"})
	}
	mux.HandleFunc("GET /.well-known/oauth-protected-resource/api/mcp", metadata)
	mux.HandleFunc("GET /.well-known/oauth-protected-resource", metadata)
	mux.HandleFunc("GET /.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		send(w, map[string]any{"issuer": a.PublicURL, "authorization_endpoint": a.PublicURL + "/oauth/authorize", "token_endpoint": a.PublicURL + "/oauth/token", "registration_endpoint": a.PublicURL + "/oauth/register", "revocation_endpoint": a.PublicURL + "/oauth/revoke", "scopes_supported": []string{mcpReadScope, mcpWriteScope}, "response_types_supported": []string{"code"}, "grant_types_supported": []string{"authorization_code", "refresh_token"}, "token_endpoint_auth_methods_supported": []string{"none"}, "code_challenge_methods_supported": []string{"S256"}, "authorization_response_iss_parameter_supported": true})
	})
	mux.HandleFunc("POST /oauth/register", a.oauthRegister)
	mux.HandleFunc("GET /oauth/authorize", a.oauthAuthorize)
	mux.HandleFunc("POST /oauth/token", a.oauthToken)
	mux.HandleFunc("POST /oauth/revoke", a.oauthRevoke)
	for _, path := range []string{"/oauth/register", "/oauth/token", "/oauth/revoke"} {
		mux.HandleFunc("OPTIONS "+path, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			w.WriteHeader(204)
		})
	}
}

// Separate buckets from password sign-in; no cookie credentials authorize these endpoints.
func (a *App) oauthRate(r *http.Request, limit int) bool {
	return a.authAttempt("oauth:"+r.URL.Path+":"+a.clientIP(r), limit, time.Now())
}

func (a *App) oauthRegister(w http.ResponseWriter, r *http.Request) {
	if !a.oauthRate(r, 60) {
		oauthError(w, 429, "temporarily_unavailable")
		return
	}
	if !safeOAuthURL(a.PublicURL) {
		oauthError(w, 400, "invalid_request")
		return
	}
	var b struct {
		Name      string   `json:"client_name"`
		Redirects []string `json:"redirect_uris"`
		Auth      string   `json:"token_endpoint_auth_method"`
		Grants    []string `json:"grant_types"`
		Responses []string `json:"response_types"`
	}
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if json.NewDecoder(r.Body).Decode(&b) != nil || len(b.Redirects) < 1 || len(b.Redirects) > 5 || (b.Auth != "" && b.Auth != "none") {
		oauthError(w, 400, "invalid_client_metadata")
		return
	}
	for _, uri := range b.Redirects {
		if len(uri) > 2048 || !safeOAuthURL(uri) {
			oauthError(w, 400, "invalid_redirect_uri")
			return
		}
	}
	for _, g := range b.Grants {
		if g != "authorization_code" && g != "refresh_token" {
			oauthError(w, 400, "invalid_client_metadata")
			return
		}
	}
	for _, t := range b.Responses {
		if t != "code" {
			oauthError(w, 400, "invalid_client_metadata")
			return
		}
	}
	b.Name = strings.TrimSpace(b.Name)
	if b.Name == "" {
		b.Name = "MCP agent"
	}
	if len(b.Name) > 80 {
		oauthError(w, 400, "invalid_client_metadata")
		return
	}
	id := randomToken()
	uris, _ := json.Marshal(b.Redirects)
	err := a.write(func(tx *sql.Tx) error {
		tx.Exec("DELETE FROM mcp_oauth_requests WHERE expires_at<=?", time.Now().Unix())
		tx.Exec("DELETE FROM mcp_oauth_clients WHERE expires_at<=? AND NOT EXISTS(SELECT 1 FROM mcp_tokens WHERE client_id=mcp_oauth_clients.id)", time.Now().Unix())
		if queryInt(tx, "SELECT COUNT(*) FROM mcp_oauth_clients") >= 1000 {
			return fail(429, "Registration limit reached")
		}
		_, err := tx.Exec("INSERT INTO mcp_oauth_clients VALUES(?,?,?,?)", id, b.Name, string(uris), time.Now().Add(90*24*time.Hour).Unix())
		return err
	})
	if err != nil {
		oauthError(w, 429, "temporarily_unavailable")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(201)
	send(w, map[string]any{"client_id": id, "client_name": b.Name, "redirect_uris": b.Redirects, "token_endpoint_auth_method": "none", "grant_types": []string{"authorization_code", "refresh_token"}, "response_types": []string{"code"}})
}
func (a *App) oauthAuthorize(w http.ResponseWriter, r *http.Request) {
	if !a.oauthRate(r, 120) {
		oauthError(w, 429, "temporarily_unavailable")
		return
	}
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || len(r.URL.RawQuery) > 8192 || !safeOAuthURL(a.PublicURL) {
		oauthError(w, 400, "invalid_request")
		return
	}
	if !normalizeOAuthParameters(q) {
		oauthError(w, 400, "invalid_request")
		return
	}
	var uris string
	if a.DB.QueryRow("SELECT redirect_uris FROM mcp_oauth_clients WHERE id=? AND expires_at>?", q.Get("client_id"), time.Now().Unix()).Scan(&uris) != nil {
		oauthError(w, 400, "invalid_client")
		return
	}
	var redirects []string
	json.Unmarshal([]byte(uris), &redirects)
	matched := false
	for _, uri := range redirects {
		matched = matched || uri == q.Get("redirect_uri")
	}
	scope, valid := oauthScope(q.Get("scope"))
	if !matched || q.Get("response_type") != "code" || q.Get("resource") != a.mcpResource() || !valid || q.Get("code_challenge_method") != "S256" || !pkceChallenge.MatchString(q.Get("code_challenge")) || len(q.Get("state")) > 1024 {
		oauthError(w, 400, "invalid_request")
		return
	}
	id := randomToken()
	err = a.write(func(tx *sql.Tx) error {
		tx.Exec("DELETE FROM mcp_oauth_requests WHERE expires_at<=?", time.Now().Unix())
		if queryInt(tx, "SELECT COUNT(*) FROM mcp_oauth_requests") >= 1000 {
			return fail(429, "Too many authorization requests")
		}
		_, err := tx.Exec("INSERT INTO mcp_oauth_requests(id,client_id,redirect_uri,state,challenge,scope,expires_at) VALUES(?,?,?,?,?,?,?)", id, q.Get("client_id"), q.Get("redirect_uri"), q.Get("state"), q.Get("code_challenge"), scope, time.Now().Add(10*time.Minute).Unix())
		return err
	})
	if err != nil {
		oauthError(w, 429, "temporarily_unavailable")
		return
	}
	http.Redirect(w, r, a.PublicURL+"/mcp/authorize?request="+id, http.StatusSeeOther)
}

type oauthRequest struct {
	ID, Client, Name, Redirect, State, Challenge, Scope, Session string
	User                                                         sql.NullInt64
	Expires                                                      int64
}

func (a *App) oauthRequest(tx *sql.Tx, r *http.Request) (oauthRequest, error) {
	var b oauthRequest
	b.ID = r.PathValue("id")
	err := tx.QueryRow(`SELECT r.client_id,c.name,r.redirect_uri,r.state,r.challenge,r.scope,r.expires_at,r.user_id,r.session_hash FROM mcp_oauth_requests r JOIN mcp_oauth_clients c ON c.id=r.client_id WHERE r.id=? AND r.expires_at>? AND c.expires_at>?`, b.ID, time.Now().Unix(), time.Now().Unix()).Scan(&b.Client, &b.Name, &b.Redirect, &b.State, &b.Challenge, &b.Scope, &b.Expires, &b.User, &b.Session)
	if err != nil {
		return b, fail(404, "Connection request expired or completed. Start again from your agent.")
	}
	cookie, _ := r.Cookie("finance_session")
	session := hash(cookie.Value)
	if b.User.Valid && (b.User.Int64 != Current(r).ID || b.Session != session) {
		return b, fail(403, "This request belongs to another sign-in session. Start a new connection from your agent.")
	}
	if !b.User.Valid {
		_, err = tx.Exec("UPDATE mcp_oauth_requests SET user_id=?,session_hash=? WHERE id=?", Current(r).ID, session, b.ID)
	}
	return b, err
}
func (a *App) mcpAuthorization(w http.ResponseWriter, r *http.Request) error {
	var b oauthRequest
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error { var err error; b, err = a.oauthRequest(tx, r); return err })
	if err != nil {
		return err
	}
	uri, _ := url.Parse(b.Redirect)
	accounts, err := a.mcpPermissionAccounts(a.DB, Current(r))
	if err != nil {
		return err
	}
	send(w, map[string]any{"accounts": accounts, "client_name": b.Name, "redirect_origin": uri.Scheme + "://" + uri.Host, "can_propose": strings.Contains(b.Scope, mcpWriteScope), "expires_at": b.Expires, "username": Current(r).Username})
	return nil
}
func (a *App) decideMCPAuthorization(w http.ResponseWriter, r *http.Request) error {
	var decision struct {
		Allow       bool            `json:"allow"`
		Write       bool            `json:"can_write"`
		Permissions *mcpPermissions `json:"permissions,omitempty"`
	}
	if err := decode(r, &decision); err != nil {
		return err
	}
	callback := ""
	err := a.browserWrite(r, func(tx *sql.Tx, u User) error {
		b, err := a.oauthRequest(tx, r)
		if err != nil {
			return err
		}
		uri, _ := url.Parse(b.Redirect)
		q := uri.Query()
		q.Set("state", b.State)
		q.Set("iss", a.PublicURL)
		if decision.Allow {
			p := legacyMCPPermissions(decision.Write)
			if decision.Permissions != nil {
				p, err = a.validateMCPPermissions(tx, u, *decision.Permissions)
				if err != nil {
					return err
				}
				decision.Write = p.writes()
			}
			if decision.Write && !strings.Contains(b.Scope, mcpWriteScope) {
				return fail(400, "Agent did not request change proposals")
			}
			if queryInt(tx, "SELECT COUNT(*) FROM mcp_tokens WHERE user_id=? AND expires_at>?", u.ID, time.Now().Unix()) >= 20 {
				return fail(400, "Revoke an existing connection before adding another")
			}
			permissions, _ := json.Marshal(p)
			res, err := tx.Exec("INSERT INTO mcp_tokens(user_id,name,token_hash,client_id,can_write,expires_at,permissions) VALUES(?,?,?,?,?,?,?)", u.ID, b.Name, hash(randomToken()), b.Client, decision.Write, time.Now().Add(90*24*time.Hour).Unix(), string(permissions))
			if err != nil {
				return err
			}
			id, _ := res.LastInsertId()
			code := randomToken()
			if _, err = tx.Exec("INSERT INTO mcp_oauth_codes(token_hash,connection_id,client_id,redirect_uri,challenge,resource,expires_at) VALUES(?,?,?,?,?,?,?)", hash(code), id, b.Client, b.Redirect, b.Challenge, a.mcpResource(), time.Now().Add(5*time.Minute).Unix()); err != nil {
				return err
			}
			if err = audit(tx, u, nil, "mcp", id, "connection_approved", map[string]any{"client_name": b.Name, "can_write": decision.Write, "permissions": p}); err != nil {
				return err
			}
			q.Set("code", code)
		} else {
			q.Set("error", "access_denied")
		}
		if _, err = tx.Exec("DELETE FROM mcp_oauth_requests WHERE id=?", b.ID); err != nil {
			return err
		}
		uri.RawQuery = q.Encode()
		callback = uri.String()
		return nil
	})
	if err != nil {
		return err
	}
	send(w, map[string]string{"redirect": callback})
	return nil
}

func oauthForm(w http.ResponseWriter, r *http.Request) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16384)
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/x-www-form-urlencoded") || r.ParseForm() != nil || len(r.URL.RawQuery) > 0 {
		oauthError(w, 400, "invalid_request")
		return false
	}
	if !normalizeOAuthParameters(r.PostForm) {
		oauthError(w, 400, "invalid_request")
		return false
	}
	return true
}
func (a *App) oauthToken(w http.ResponseWriter, r *http.Request) {
	if !a.oauthRate(r, 240) {
		oauthError(w, 429, "temporarily_unavailable")
		return
	}
	if !oauthForm(w, r) {
		return
	}
	q := r.PostForm
	if q.Get("grant_type") != "authorization_code" && q.Get("grant_type") != "refresh_token" {
		oauthError(w, 400, "unsupported_grant_type")
		return
	}
	if r.Header.Get("Authorization") != "" || q.Get("client_secret") != "" {
		oauthError(w, 401, "invalid_client")
		return
	}
	if q.Get("resource") != a.mcpResource() {
		oauthError(w, 400, "invalid_target")
		return
	}
	access, refresh := randomToken(), randomToken()
	scope := ""
	invalid, reused := false, false
	err := a.write(func(tx *sql.Tx) error {
		var id, expiry int64
		var write bool
		switch q.Get("grant_type") {
		case "authorization_code":
			var challenge string
			err := tx.QueryRow(`SELECT c.connection_id,c.challenge,t.expires_at,t.can_write FROM mcp_oauth_codes c JOIN mcp_tokens t ON t.id=c.connection_id JOIN users u ON u.id=t.user_id WHERE c.token_hash=? AND c.client_id=? AND c.redirect_uri=? AND c.resource=? AND c.expires_at>? AND t.expires_at>? AND u.disabled=0`, hash(q.Get("code")), q.Get("client_id"), q.Get("redirect_uri"), a.mcpResource(), time.Now().Unix(), time.Now().Unix()).Scan(&id, &challenge, &expiry, &write)
			digest := sha256.Sum256([]byte(q.Get("code_verifier")))
			actual := base64.RawURLEncoding.EncodeToString(digest[:])
			if err != nil || !pkceVerifier.MatchString(q.Get("code_verifier")) || subtle.ConstantTimeCompare([]byte(challenge), []byte(actual)) != 1 {
				invalid = true
				return nil
			}
			if _, err = tx.Exec("DELETE FROM mcp_oauth_codes WHERE token_hash=?", hash(q.Get("code"))); err != nil {
				return err
			}
		case "refresh_token":
			var used bool
			err := tx.QueryRow(`SELECT f.connection_id,f.used,t.expires_at,t.can_write FROM mcp_refresh_tokens f JOIN mcp_tokens t ON t.id=f.connection_id JOIN users u ON u.id=t.user_id WHERE f.token_hash=? AND f.client_id=? AND f.resource=? AND f.expires_at>? AND t.expires_at>? AND u.disabled=0`, hash(q.Get("refresh_token")), q.Get("client_id"), a.mcpResource(), time.Now().Unix(), time.Now().Unix()).Scan(&id, &used, &expiry, &write)
			if err != nil {
				invalid = true
				return nil
			}
			if used {
				reused = true
				_, err = tx.Exec("DELETE FROM mcp_tokens WHERE id=?", id)
				return err
			}
			if requested := q.Get("scope"); requested != "" {
				canonical, ok := oauthScope(requested)
				if !ok || strings.Contains(canonical, mcpWriteScope) && !write {
					invalid = true
					return nil
				}
				if strings.Contains(canonical, mcpWriteScope) != write {
					invalid = true
					return nil
				}
			}
			if _, err = tx.Exec("UPDATE mcp_refresh_tokens SET used=1 WHERE token_hash=?", hash(q.Get("refresh_token"))); err != nil {
				return err
			}
		default:
			invalid = true
			return nil
		}
		scope = mcpReadScope
		if write {
			scope += " " + mcpWriteScope
		}
		// Remove expired access/code rows; retain used refresh hashes for replay detection.
		tx.Exec("DELETE FROM mcp_access_tokens WHERE expires_at<=?", time.Now().Unix())
		tx.Exec("DELETE FROM mcp_oauth_codes WHERE expires_at<=?", time.Now().Unix())
		if _, err := tx.Exec("INSERT INTO mcp_access_tokens VALUES(?,?,?,?)", hash(access), id, a.mcpResource(), time.Now().Add(time.Hour).Unix()); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO mcp_refresh_tokens(token_hash,connection_id,client_id,resource,expires_at,used) VALUES(?,?,?,?,?,0)", hash(refresh), id, q.Get("client_id"), a.mcpResource(), expiry)
		return err
	})
	if err != nil {
		oauthError(w, 500, "server_error")
		return
	}
	if invalid || reused {
		oauthError(w, 400, "invalid_grant")
		return
	}
	send(w, map[string]any{"access_token": access, "refresh_token": refresh, "token_type": "Bearer", "expires_in": 3600, "scope": scope})
}
func (a *App) oauthRevoke(w http.ResponseWriter, r *http.Request) {
	if !oauthForm(w, r) {
		return
	}
	err := a.write(func(tx *sql.Tx) error {
		_, err := tx.Exec(`DELETE FROM mcp_tokens WHERE client_id=? AND id IN (SELECT connection_id FROM mcp_access_tokens WHERE token_hash=? UNION SELECT connection_id FROM mcp_refresh_tokens WHERE token_hash=?)`, r.PostForm.Get("client_id"), hash(r.PostForm.Get("token")), hash(r.PostForm.Get("token")))
		return err
	})
	if err != nil {
		oauthError(w, 500, "server_error")
		return
	}
	w.WriteHeader(200)
}
