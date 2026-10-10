package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"finance-tracker/internal/buildinfo"
)

func TestBuildInfoAuthorizationAndSafeOutput(t *testing.T) {
	e := setup(t)
	anonymous := httptest.NewRecorder()
	e.h.ServeHTTP(anonymous, httptest.NewRequest("GET", "/api/build", nil))
	status(t, anonymous, http.StatusUnauthorized)
	for _, user := range []int{1, 2, 3} {
		response := e.req(t, user, "/api/build", "GET", nil)
		status(t, response, http.StatusOK)
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("build response must not be cached")
		}
		var fields map[string]any
		if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
			t.Fatal(err)
		}
		for key := range fields {
			if key != "version" && key != "commit" && key != "built_at" && key != "modified" {
				t.Fatalf("unexpected public field %q", key)
			}
		}
		var got buildinfo.Info
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got != buildinfo.Current() {
			t.Fatalf("build metadata changed in transport: %+v", got)
		}
	}
	status(t, e.req(t, 1, "/api/build", "POST", nil), http.StatusMethodNotAllowed)
}

func TestApplicationUpdateAuthorizationAndLocalBuild(t *testing.T) {
	e := setup(t)
	anonymous := httptest.NewRecorder()
	e.h.ServeHTTP(anonymous, httptest.NewRequest("GET", "/api/build/update", nil))
	status(t, anonymous, http.StatusUnauthorized)
	for _, user := range []int{1, 2, 3} {
		for _, method := range []string{"GET", "POST"} {
			response := e.req(t, user, "/api/build/update", method, nil)
			status(t, response, http.StatusOK)
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("update response cached")
			}
			var fields map[string]any
			if err := json.Unmarshal(response.Body.Bytes(), &fields); err != nil {
				t.Fatal(err)
			}
			if fields["state"] != "unsupported" {
				t.Fatalf("local build should not request GitHub: %v", fields)
			}
			for key := range fields {
				if key != "state" && key != "message" && key != "stale" {
					t.Fatalf("unexpected field %s", key)
				}
			}
		}
	}
}
