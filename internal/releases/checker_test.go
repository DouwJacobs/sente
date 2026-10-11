package releases

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

type transport func(*http.Request) (*http.Response, error)

func (f transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func fixture(tags ...string) string {
	values := []string{}
	for _, tag := range tags {
		values = append(values, fmt.Sprintf(`{"tag_name":%q,"draft":false,"prerelease":%t,"published_at":"2026-10-09T00:00:00Z"}`, tag, strings.Contains(tag, "-")))
	}
	return "[" + strings.Join(values, ",") + "]"
}
func TestStrictSemVer(t *testing.T) {
	ordered := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.10", "1.0.0-beta.999999999999999999999999", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.10.0", "2.0.0"}
	for i, a := range ordered {
		for j, b := range ordered {
			av, ok := parse(a)
			bv, bok := parse(b)
			if !ok || !bok {
				t.Fatal(a, b)
			}
			got := compare(av, bv)
			if (i < j && got >= 0) || (i > j && got <= 0) || (i == j && got != 0) {
				t.Fatalf("%s vs %s = %d", a, b, got)
			}
		}
	}
	for _, bad := range []string{"v1.0.0", "01.0.0", "1.0", "1.0.0-beta.01", "1.0.0-", "1.0.0+", "1.0.0\n", "1.0.0_beta", "1.0.0+hello..world"} {
		if _, ok := parse(bad); ok {
			t.Fatal("accepted", bad)
		}
	}
	a, _ := parse("1.0.0+abc")
	b, _ := parse("1.0.0+def")
	if compare(a, b) != 0 {
		t.Fatal("metadata affects precedence")
	}
}
func TestChannelsAndNoDowngrades(t *testing.T) {
	for _, tc := range []struct{ installed, body, state, available string }{
		{"1.0.0", fixture("v1.1.0-beta.10", "v1.0.1", "v1.0.0"), "available", "1.0.1"},
		{"1.0.1", fixture("v1.0.1"), "current", ""},
		{"2.0.0", fixture("v1.0.1"), "current", ""},
		{"1.0.0-beta.9", fixture("v1.0.0-beta.10", "v1.0.0-beta.2", "v2.0.0"), "available", "1.0.0-beta.10"},
		{"2.0.0-beta.1", fixture("v1.9.0-beta.999"), "current", ""},
		{"1.0.0-beta.2", fixture("v1.0.0"), "unknown", ""},
		{"1.0.0", "[]", "unknown", ""},
	} {
		t.Run(tc.installed+tc.state, func(t *testing.T) {
			c := Checker{client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "" || r.Header.Get("Cookie") != "" || r.Body != nil {
					t.Fatal("unsafe upstream request")
				}
				if r.URL.Path != "/repos/DouwJacobs/sente/releases" {
					t.Fatal(r.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			got := c.Check(context.Background(), tc.installed, false, false)
			if got.State != tc.state || got.AvailableVersion != tc.available {
				t.Fatalf("%+v", got)
			}
			if got.AvailableVersion != "" && got.ReleaseURL != "https://github.com/DouwJacobs/sente/releases/tag/v"+tc.available {
				t.Fatal(got.ReleaseURL)
			}
		})
	}
}
func TestUnsupportedNeverRequests(t *testing.T) {
	c := Checker{client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { t.Fatal("unexpected request"); return nil, nil })}}
	for _, v := range []string{"dev", "unknown", "1.0.0-dev.0+gabc", "1.0.0-rc.1", "1.0.0+local"} {
		if c.Check(context.Background(), v, false, true).State != "unsupported" {
			t.Fatal(v)
		}
	}
	if c.Check(context.Background(), "1.0.0", true, true).State != "unsupported" {
		t.Fatal("modified build")
	}
}
func TestCacheStalenessFailuresAndConcurrency(t *testing.T) {
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	calls := 0
	broken := false
	c := Checker{now: func() time.Time { return now }, client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		calls++
		code, body := 200, fixture("v1.0.1")
		if broken {
			code = 429
			body = `{"message":"secret provider diagnostic"}`
		}
		return &http.Response{StatusCode: code, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if c.Check(context.Background(), "1.0.0", false, false).State != "available" {
				t.Error("missing available")
			}
		}()
	}
	wg.Wait()
	if calls != 1 {
		t.Fatal(calls)
	}
	c.Check(context.Background(), "1.0.0", false, true)
	if calls != 1 {
		t.Fatal("manual cooldown")
	}
	now = now.Add(2 * time.Minute)
	c.Check(context.Background(), "1.0.0", false, true)
	if calls != 2 {
		t.Fatal("deliberate retry")
	}
	now = now.Add(successTTL)
	broken = true
	stale := c.Check(context.Background(), "1.0.0", false, false)
	if stale.State != "unavailable" || !stale.Stale || stale.AvailableVersion != "1.0.1" || strings.Contains(stale.Message, "secret") {
		t.Fatalf("%+v", stale)
	}
	c.Check(context.Background(), "1.0.0", false, false)
	if calls != 3 {
		t.Fatal("failure cache")
	}
	now = now.Add(failureTTL)
	broken = false
	got := c.Check(context.Background(), "1.0.0", false, false)
	if got.Stale || got.State != "available" || calls != 4 {
		t.Fatalf("%+v calls=%d", got, calls)
	}
}
func TestProviderFailuresAndBounds(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		code       int
	}{
		{"offline", "", 503}, {"rate limit", "", 429}, {"forbidden", "", 403}, {"redirect", "", 302},
		{"invalid json", "oops", 200}, {"null", "null", 200}, {"missing fields", `[{"tag_name":"v1.0.1"}]`, 200},
		{"invalid semver", fixture("v1.0.0-beta.01"), 200},
		{"wrong channel", `[{"tag_name":"v1.0.0-beta.1","draft":false,"prerelease":false,"published_at":"2026-10-09T00:00:00Z"}]`, 200},
		{"body bound", strings.Repeat(" ", maxBody+1), 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Checker{client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: tc.code, Body: io.NopCloser(strings.NewReader(tc.body))}, nil
			})}}
			got := c.Check(context.Background(), "1.0.0", false, false)
			if got.State != "unavailable" || got.CheckedAt != "" || got.Stale {
				t.Fatalf("%+v", got)
			}
		})
	}
	calls := 0
	tags := make([]string, 100)
	for i := range tags {
		tags[i] = "v1.0.0"
	}
	c := Checker{client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(fixture(tags...)))}, nil
	})}}
	if c.Check(context.Background(), "1.0.0", false, false).State != "unavailable" || calls != 3 {
		t.Fatal("pagination bound", calls)
	}
	c = Checker{client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) { <-r.Context().Done(); return nil, r.Context().Err() })}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if c.Check(ctx, "1.0.0", false, false).State != "unavailable" {
		t.Fatal("cancellation")
	}
}

func TestProviderBackoff(t *testing.T) {
	now := time.Date(2026, 10, 9, 0, 0, 0, 0, time.UTC)
	calls := 0
	c := Checker{now: func() time.Time { return now }, client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 429, Header: http.Header{"Retry-After": []string{"600"}}, Body: io.NopCloser(strings.NewReader("limited"))}, nil
	})}}
	result := c.Check(context.Background(), "1.0.0", false, false)
	if result.RetryAt != now.Add(10*time.Minute).Format(time.RFC3339) {
		t.Fatal(result)
	}
	now = now.Add(6 * time.Minute)
	c.Check(context.Background(), "1.0.0", false, true)
	if calls != 1 {
		t.Fatal("ignored provider backoff")
	}
	now = now.Add(5 * time.Minute)
	c.Check(context.Background(), "1.0.0", false, true)
	if calls != 2 {
		t.Fatal("retry did not resume")
	}
	if providerWait(http.Header{"Retry-After": []string{"999999999999"}}) != time.Hour {
		t.Fatal("unbounded backoff")
	}
}

func TestPublishedReleasesAcrossPages(t *testing.T) {
	calls := 0
	tags := make([]string, 100)
	for i := range tags {
		tags[i] = "v9.0.0-beta.1"
	}
	c := Checker{client: &http.Client{Transport: transport(func(r *http.Request) (*http.Response, error) {
		calls++
		body := fixture(tags...)
		if r.URL.Query().Get("page") == "2" {
			body = fixture("v1.0.2", "v1.0.10", "v1.0.3")
			body = strings.TrimSuffix(body, "]") + `,{"tag_name":"v99.0.0","draft":true,"prerelease":false,"published_at":null}]`
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body))}, nil
	})}}
	got := c.Check(context.Background(), "1.0.0", false, false)
	if calls != 2 || got.AvailableVersion != "1.0.10" {
		t.Fatal(calls, got)
	}
	c = Checker{client: &http.Client{Transport: transport(func(*http.Request) (*http.Response, error) { return nil, fmt.Errorf("private proxy diagnostic") })}}
	got = c.Check(context.Background(), "1.0.0", false, false)
	if got.State != "unavailable" || strings.Contains(got.Message, "private") {
		t.Fatal(got)
	}
}
