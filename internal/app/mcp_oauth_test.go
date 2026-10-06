package app

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"
)

const testVerifier = "synthetic-pkce-verifier-with-at-least-forty-three-characters"
const testRedirect = "http://127.0.0.1:43210/callback"

func TestMCPOAuthRepeatedResource(t *testing.T) {
	e := setup(t)
	client, _ := oauthStart(t, e)
	digest := sha256.Sum256([]byte(testVerifier))
	q := url.Values{"client_id": {client}, "redirect_uri": {testRedirect}, "response_type": {"code"}, "resource": {e.a.mcpResource(), e.a.mcpResource()}, "scope": {mcpReadScope + " " + mcpWriteScope}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "state": {"synthetic-state"}}
	w := e.req(t, 0, "/oauth/authorize?"+q.Encode(), "GET", nil)
	status(t, w, 303)
	location, _ := url.Parse(w.Header().Get("Location"))
	code := oauthApprove(t, e, 1, location.Query().Get("request"), true)
	copyValues := func(input url.Values) url.Values {
		result := url.Values{}
		for key, values := range input {
			result[key] = append([]string(nil), values...)
		}
		return result
	}
	for _, key := range []string{"client_id", "redirect_uri", "state", "code_challenge", "scope", "response_type"} {
		bad := copyValues(q)
		bad.Add(key, bad.Get(key))
		status(t, e.req(t, 0, "/oauth/authorize?"+bad.Encode(), "GET", nil), 400)
	}
	for _, resources := range [][]string{{e.a.mcpResource(), "https://foreign.example/api/mcp"}, {"https://foreign.example/api/mcp", e.a.mcpResource()}, {e.a.mcpResource(), ""}, {"https://foreign.example/api/mcp", "https://foreign.example/api/mcp"}} {
		bad := copyValues(q)
		bad["resource"] = resources
		status(t, e.req(t, 0, "/oauth/authorize?"+bad.Encode(), "GET", nil), 400)
	}
	form := url.Values{"grant_type": {"authorization_code"}, "client_id": {client}, "code": {code}, "code_verifier": {testVerifier}, "redirect_uri": {testRedirect}, "resource": {e.a.mcpResource(), e.a.mcpResource()}}
	for _, key := range []string{"client_id", "code", "code_verifier", "redirect_uri", "grant_type"} {
		bad := copyValues(form)
		bad.Add(key, bad.Get(key))
		status(t, oauthPost(e, "/oauth/token", bad), 400)
	}
	bad := copyValues(form)
	bad.Add("resource", "https://foreign.example/api/mcp")
	status(t, oauthPost(e, "/oauth/token", bad), 400)
	w = oauthPost(e, "/oauth/token", form)
	status(t, w, 200)
	credentials := oauthJSON(t, w)
	identity, err := e.a.mcpIdentity(credentials["access_token"].(string))
	if err != nil || identity.User.ID != 1 || !identity.Write {
		t.Fatal("repeated resources changed authorization", err)
	}
	refresh := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {credentials["refresh_token"].(string)}, "resource": {e.a.mcpResource(), e.a.mcpResource()}}
	status(t, oauthPost(e, "/oauth/token", refresh), 200)
}

func oauthJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var v map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &v); err != nil {
		t.Fatal(w.Body.String())
	}
	return v
}
func oauthPost(e *testEnv, path string, q url.Values) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(q.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://agent.example.test")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	return w
}
func oauthStart(t *testing.T, e *testEnv) (string, string) {
	t.Helper()
	w := e.req(t, 0, "/oauth/register", "POST", map[string]any{"client_name": "Synthetic agent", "redirect_uris": []string{testRedirect}, "token_endpoint_auth_method": "none"})
	status(t, w, 201)
	client := oauthJSON(t, w)["client_id"].(string)
	digest := sha256.Sum256([]byte(testVerifier))
	q := url.Values{"client_id": {client}, "redirect_uri": {testRedirect}, "response_type": {"code"}, "resource": {e.a.mcpResource()}, "scope": {mcpReadScope + " " + mcpWriteScope}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}, "state": {"synthetic-state"}}
	w = e.req(t, 0, "/oauth/authorize?"+q.Encode(), "GET", nil)
	status(t, w, 303)
	u, _ := url.Parse(w.Header().Get("Location"))
	return client, u.Query().Get("request")
}
func oauthApprove(t *testing.T, e *testEnv, user int, request string, write bool) string {
	t.Helper()
	status(t, e.req(t, user, "/api/mcp/authorization/"+request, "GET", nil), 200)
	w := e.req(t, user, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true, "can_write": write})
	status(t, w, 200)
	u, _ := url.Parse(oauthJSON(t, w)["redirect"].(string))
	if u.Query().Get("state") != "synthetic-state" || u.Query().Get("iss") != e.a.PublicURL {
		t.Fatal("lost state/issuer")
	}
	return u.Query().Get("code")
}
func oauthExchange(e *testEnv, client, code, verifier string) *httptest.ResponseRecorder {
	return oauthPost(e, "/oauth/token", url.Values{"grant_type": {"authorization_code"}, "client_id": {client}, "code": {code}, "code_verifier": {verifier}, "redirect_uri": {testRedirect}, "resource": {e.a.mcpResource()}})
}

func TestMCPOAuthIsolationAndLifecycle(t *testing.T) {
	e := setup(t)
	seedMCPTransactions(t, e)
	w := e.req(t, 0, "/.well-known/oauth-protected-resource/api/mcp", "GET", nil)
	status(t, w, 200)
	if oauthJSON(t, w)["resource"] != e.a.mcpResource() {
		t.Fatal(w.Body.String())
	}
	w = e.req(t, 0, "/.well-known/oauth-authorization-server", "GET", nil)
	status(t, w, 200)
	if oauthJSON(t, w)["issuer"] != e.a.PublicURL {
		t.Fatal(w.Body.String())
	}
	for _, uri := range []string{"http://evil.example/callback", "https://user:password@evil.example/callback", "javascript:alert(1)", "https://agent.example/callback#fragment"} {
		status(t, e.req(t, 0, "/oauth/register", "POST", map[string]any{"redirect_uris": []string{uri}}), 400)
	}
	client, request := oauthStart(t, e)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "GET", nil), 200)
	status(t, e.req(t, 2, "/api/mcp/authorization/"+request, "GET", nil), 403)
	status(t, e.req(t, 2, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true}), 403)
	code := oauthApprove(t, e, 1, request, true)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true}), 404)
	status(t, oauthExchange(e, client, code, "wrong-verifier"), 400)
	status(t, oauthExchange(e, "wrong-client", code, testVerifier), 400)
	w = oauthExchange(e, client, code, testVerifier)
	status(t, w, 200)
	credentials := oauthJSON(t, w)
	status(t, oauthExchange(e, client, code, testVerifier), 400)
	access := credentials["access_token"].(string)
	refresh := credentials["refresh_token"].(string)
	if _, err := e.a.mcpIdentity(access); err != nil {
		t.Fatal(err)
	}
	viewer := mcpToken(t, e, 3, false)
	v, failed := mcpCall(t, e, viewer, "list_transactions", map[string]any{})
	if failed || len(v["items"].([]any)) != 2 {
		t.Fatal(v)
	}
	ownerSettings := oauthJSON(t, e.req(t, 1, "/api/mcp/settings", "GET", nil))
	otherSettings := oauthJSON(t, e.req(t, 3, "/api/mcp/settings", "GET", nil))
	if len(ownerSettings["connections"].([]any)) != 1 || len(otherSettings["connections"].([]any)) != 1 {
		t.Fatal("connection isolation")
	}
	requestBody := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"prepare_change","arguments":{}}}`
	r := httptest.NewRequest("POST", "/api/mcp", strings.NewReader(requestBody))
	r.Header.Set("Authorization", "Bearer "+viewer)
	w = httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	if !strings.Contains(w.Header().Get("WWW-Authenticate"), "insufficient_scope") {
		t.Fatal(w.Header())
	}
	q := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {refresh}, "resource": {e.a.mcpResource()}}
	q.Set("resource", "https://wrong.example/api/mcp")
	status(t, oauthPost(e, "/oauth/token", q), 400)
	q.Set("resource", e.a.mcpResource())
	w = oauthPost(e, "/oauth/token", q)
	status(t, w, 200)
	rotated := oauthJSON(t, w)
	if rotated["refresh_token"] == refresh {
		t.Fatal("refresh token did not rotate")
	}
	status(t, oauthPost(e, "/oauth/token", q), 400)
	if _, err := e.a.mcpIdentity(rotated["access_token"].(string)); err == nil {
		t.Fatal("refresh reuse did not revoke connection")
	}
	if _, err := e.a.mcpIdentity(viewer); err != nil {
		t.Fatal("revoked another user's connection")
	}
}
func TestMCPOAuthConsentExpiryAndRevocation(t *testing.T) {
	e := setup(t)
	client, request := oauthStart(t, e)
	w := e.req(t, 1, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": false})
	status(t, w, 200)
	u, _ := url.Parse(oauthJSON(t, w)["redirect"].(string))
	if u.Query().Get("error") != "access_denied" || u.Query().Get("code") != "" {
		t.Fatal(u)
	}
	if queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_tokens") != 0 {
		t.Fatal("denial created connection")
	}
	client, request = oauthStart(t, e)
	code := oauthApprove(t, e, 1, request, false)
	w = oauthExchange(e, client, code, testVerifier)
	status(t, w, 200)
	v := oauthJSON(t, w)
	identity, err := e.a.mcpIdentity(v["access_token"].(string))
	if err != nil || identity.Write {
		t.Fatal("default access was not read-only", err)
	}
	status(t, oauthPost(e, "/oauth/revoke", url.Values{"client_id": {"wrong"}, "token": {v["access_token"].(string)}}), 200)
	if _, err := e.a.mcpIdentity(v["access_token"].(string)); err != nil {
		t.Fatal("foreign client revoked connection")
	}
	status(t, oauthPost(e, "/oauth/revoke", url.Values{"client_id": {client}, "token": {v["refresh_token"].(string)}}), 200)
	if _, err := e.a.mcpIdentity(v["access_token"].(string)); err == nil {
		t.Fatal("revocation failed")
	}
	client, request = oauthStart(t, e)
	code = oauthApprove(t, e, 1, request, true)
	e.a.DB.Exec("UPDATE mcp_oauth_codes SET expires_at=?", time.Now().Unix()-1)
	status(t, oauthExchange(e, client, code, testVerifier), 400)
	_, request = oauthStart(t, e)
	e.a.DB.Exec("UPDATE mcp_oauth_requests SET expires_at=?", time.Now().Unix()-1)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "GET", nil), 404)
}

func TestMCPOAuthRequestValidationAndAtomicCode(t *testing.T) {
	e := setup(t)
	client, request := oauthStart(t, e)
	digest := sha256.Sum256([]byte(testVerifier))
	q := url.Values{"client_id": {client}, "redirect_uri": {testRedirect}, "response_type": {"code"}, "resource": {e.a.mcpResource()}, "scope": {mcpReadScope}, "code_challenge_method": {"S256"}, "code_challenge": {base64.RawURLEncoding.EncodeToString(digest[:])}}
	for key, value := range map[string]string{"redirect_uri": "https://other.example/callback", "resource": "https://other.example/api/mcp", "scope": "finance:admin", "code_challenge_method": "plain", "code_challenge": "short", "response_type": "token"} {
		copy := url.Values{}
		for k, v := range q {
			copy[k] = append([]string(nil), v...)
		}
		copy.Set(key, value)
		status(t, e.req(t, 0, "/oauth/authorize?"+copy.Encode(), "GET", nil), 400)
	}
	q.Add("client_id", client)
	status(t, e.req(t, 0, "/oauth/authorize?"+q.Encode(), "GET", nil), 400)
	// Cookie authentication alone cannot approve a connection without CSRF.
	r := httptest.NewRequest("POST", "/api/mcp/authorization/"+request, strings.NewReader(`{"allow":true}`))
	r.Header.Set("Cookie", "finance_session=token1")
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, r)
	status(t, w, 403)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "GET", nil), 200)
	e.a.DB.Exec("UPDATE mcp_oauth_requests SET session_hash='different-session' WHERE id=?", request)
	status(t, e.req(t, 1, "/api/mcp/authorization/"+request, "POST", map[string]any{"allow": true}), 403)
	client, request = oauthStart(t, e)
	code := oauthApprove(t, e, 1, request, true)
	responses := make(chan *httptest.ResponseRecorder, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); responses <- oauthExchange(e, client, code, testVerifier) }()
	}
	wg.Wait()
	close(responses)
	successful := 0
	var credentials map[string]any
	for response := range responses {
		if response.Code == 200 {
			successful++
			credentials = oauthJSON(t, response)
		} else {
			status(t, response, 400)
		}
	}
	if successful != 1 || queryInt(e.a.DB, "SELECT COUNT(*) FROM mcp_access_tokens") != 1 {
		t.Fatal("authorization code was not consumed atomically")
	}
	token := credentials["access_token"].(string)
	e.a.DB.Exec("UPDATE mcp_access_tokens SET expires_at=?", time.Now().Unix()-1)
	if _, err := e.a.mcpIdentity(token); err == nil {
		t.Fatal("expired access token accepted")
	}
	refresh := credentials["refresh_token"].(string)
	refreshForm := url.Values{"grant_type": {"refresh_token"}, "client_id": {client}, "refresh_token": {refresh}, "resource": {e.a.mcpResource()}}
	refreshForm.Set("scope", mcpReadScope)
	status(t, oauthPost(e, "/oauth/token", refreshForm), 400)
	refreshForm.Del("scope")
	originalURL := e.a.PublicURL
	e.a.PublicURL = "https://different.example"
	refreshForm.Set("resource", e.a.mcpResource())
	status(t, oauthPost(e, "/oauth/token", refreshForm), 400)
	e.a.PublicURL = originalURL
	refreshForm.Set("resource", e.a.mcpResource())
	w = oauthPost(e, "/oauth/token", refreshForm)
	status(t, w, 200)
	token = oauthJSON(t, w)["access_token"].(string)
	if err := e.a.ResetPassword("owner", "synthetic-reset-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.mcpIdentity(token); err == nil {
		t.Fatal("password reset left agent authorized")
	}
	status(t, oauthPost(e, "/oauth/token", refreshForm), 400)
	// Retain existing legacy credentials until their normal expiry/revocation.
	legacy := randomToken()
	if _, err := e.a.DB.Exec("INSERT INTO mcp_tokens(user_id,name,token_hash,expires_at) VALUES(1,'Legacy',?,?)", hash(legacy), time.Now().Add(time.Hour).Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := e.a.mcpIdentity(legacy); err != nil {
		t.Fatal("legacy connection lost", err)
	}
}
