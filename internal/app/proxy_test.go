package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProxyClientIPTrustBoundary(t *testing.T) {
	for _, tc := range []struct{ name, trusted, peer, forwarded, want string }{
		{"direct spoof", "", "192.0.2.1:1234", "198.51.100.1", "192.0.2.1"},
		{"untrusted proxy", "127.0.0.1", "192.0.2.1:1234", "198.51.100.1", "192.0.2.1"},
		{"trusted edge", "127.0.0.1", "127.0.0.1:1234", "198.51.100.1", "198.51.100.1"},
		{"spoofed prefix", "127.0.0.1", "127.0.0.1:1234", "203.0.113.99, 198.51.100.1", "198.51.100.1"},
		{"trusted chain", "127.0.0.1,10.0.0.0/24", "127.0.0.1:1234", "198.51.100.1,10.0.0.2", "198.51.100.1"},
		{"malformed chain", "127.0.0.1", "127.0.0.1:1234", "bad,198.51.100.1", "127.0.0.1"},
		{"direct IPv6", "", "[2001:db8::1]:1234", "198.51.100.1", "2001:db8::1"},
		{"mapped IPv4", "127.0.0.1", "[::ffff:127.0.0.1]:1234", "::ffff:198.51.100.1", "198.51.100.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := &App{}
			if err := a.ConfigureTrustedProxies(tc.trusted); err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest("GET", "/", nil)
			r.RemoteAddr = tc.peer
			r.Header.Set("X-Forwarded-For", tc.forwarded)
			if got := a.clientIP(r); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
	for _, value := range []string{"*", "0.0.0.0/0", "::/0", "proxy.example.com", "127.0.0.1,", "10.0.0.0/33"} {
		if err := (&App{}).ConfigureTrustedProxies(value); err == nil {
			t.Fatalf("accepted invalid trust %s", value)
		}
	}
}

func TestHTTPSReverseProxySessionAndOrigin(t *testing.T) {
	e := setup(t)
	if err := e.a.ResetPassword("owner", "synthetic-proxy-password"); err != nil {
		t.Fatal(err)
	}
	if err := e.a.ConfigureTrustedProxies("127.0.0.1,::1"); err != nil {
		t.Fatal(err)
	}
	static := t.TempDir()
	os.WriteFile(filepath.Join(static, "index.html"), []byte("synthetic app"), 0600)
	backend := httptest.NewServer(e.a.Handler(static))
	defer backend.Close()
	target, _ := url.Parse(backend.URL)
	proxyHandler := httputil.NewSingleHostReverseProxy(target)
	proxy := httptest.NewTLSServer(proxyHandler)
	defer proxy.Close()
	e.a.PublicURL = proxy.URL
	e.a.Secure = true
	client := proxy.Client()
	request := func(method, path, origin, csrf string, cookie *http.Cookie) *http.Response {
		t.Helper()
		body := ""
		if path == "/api/login" {
			body = `{"username":"owner","password":"synthetic-proxy-password"}`
		}
		if path == "/api/branding" && method == "PUT" {
			body = `{"display_name":"Proxy household","version":1}`
		}
		r, _ := http.NewRequest(method, proxy.URL+path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", origin)
		r.Header.Set("X-CSRF-Token", csrf)
		// Headers must never override the configured external scheme/origin.
		r.Header.Set("X-Forwarded-Proto", "http")
		r.Header.Set("X-Forwarded-Host", "evil.example")
		r.Header.Set("Forwarded", `proto=http;host=evil.example`)
		if cookie != nil {
			r.AddCookie(cookie)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	login := request("POST", "/api/login", proxy.URL, "", nil)
	if login.StatusCode != 200 {
		t.Fatalf("login %d", login.StatusCode)
	}
	var payload struct {
		CSRF string `json:"csrf"`
	}
	json.NewDecoder(login.Body).Decode(&payload)
	login.Body.Close()
	cookies := login.Cookies()
	if len(cookies) != 1 {
		t.Fatal("missing session cookie")
	}
	cookie := cookies[0]
	if !cookie.Secure || !cookie.HttpOnly || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/" {
		t.Fatal("insecure proxied cookie")
	}
	for _, tc := range []struct {
		method, path, origin, csrf string
		want                       int
	}{
		{"GET", "/api/me", proxy.URL, "", 200},
		{"GET", "/settings", proxy.URL, "", 200},
		{"PUT", "/api/branding", proxy.URL, payload.CSRF, 200},
		{"POST", "/api/logout", "https://evil.example", payload.CSRF, 403},
		{"POST", "/api/logout", proxy.URL, "wrong", 403},
		{"POST", "/api/logout", proxy.URL, payload.CSRF, 200},
	} {
		response := request(tc.method, tc.path, tc.origin, tc.csrf, cookie)
		if response.StatusCode != tc.want {
			t.Fatalf("%s got %d want %d", tc.path, response.StatusCode, tc.want)
		}
		if tc.want == 200 && tc.path == "/api/logout" && !response.Cookies()[0].Secure {
			t.Fatal("logout cookie lost Secure")
		}
		response.Body.Close()
	}
}
func TestProxyLoginRateLimitSeparatesClientsAndRejectsSpoofing(t *testing.T) {
	e := setup(t)
	if err := e.a.ConfigureTrustedProxies("127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	login := func(peer, forwarded string) int {
		r := httptest.NewRequest("POST", "/api/login", strings.NewReader(`{"username":"absent","password":"synthetic"}`))
		r.RemoteAddr = peer
		r.Header.Set("X-Forwarded-For", forwarded)
		w := httptest.NewRecorder()
		e.h.ServeHTTP(w, r)
		return w.Code
	}
	for i := 0; i < 10; i++ {
		if got := login("127.0.0.1:1234", "198.51.100.1"); got != 401 {
			t.Fatal(got)
		}
	}
	if got := login("127.0.0.1:1234", "203.0.113.99,198.51.100.1"); got != 429 {
		t.Fatal("spoofed prefix bypassed limit")
	}
	if got := login("127.0.0.1:1234", "198.51.100.2"); got != 401 {
		t.Fatal("unrelated client blocked")
	}
	for i := 0; i < 10; i++ {
		login("192.0.2.1:1234", "198.51.100.2")
	}
	if got := login("192.0.2.1:1234", "198.51.100.3"); got != 429 {
		t.Fatal("untrusted header bypassed limit")
	}
}
