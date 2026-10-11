package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

const endpoint = "https://api.github.com/repos/DouwJacobs/sente/releases"
const repository = "https://github.com/DouwJacobs/sente"
const successTTL = 6 * time.Hour
const failureTTL = 5 * time.Minute
const retryCooldown = time.Minute
const maxBody = 4 << 20

// Status contains only selected, validated public metadata. Failed checks never claim current.
type Status struct {
	State            string `json:"state"`
	Channel          string `json:"channel,omitempty"`
	AvailableVersion string `json:"available_version,omitempty"`
	ReleaseURL       string `json:"release_url,omitempty"`
	CheckedAt        string `json:"checked_at,omitempty"`
	AttemptedAt      string `json:"attempted_at,omitempty"`
	RetryAt          string `json:"retry_at,omitempty"`
	Stale            bool   `json:"stale"`
	Message          string `json:"message,omitempty"`
}
type release struct {
	Tag        *string    `json:"tag_name"`
	Draft      *bool      `json:"draft"`
	Prerelease *bool      `json:"prerelease"`
	Published  *time.Time `json:"published_at"`
}

// Checker is process-local and serializes requests across every signed-in user.
// Its zero value uses a credential-free client and a fixed public repository.
type Checker struct {
	mu        sync.Mutex
	client    *http.Client
	now       func() time.Time
	cached    Status
	attempted time.Time
	expires   time.Time
	installed string
}

func (c *Checker) Check(ctx context.Context, installed string, modified, force bool) Status {
	v, ok := parse(installed)
	if !ok || modified || channel(v) == "" {
		return Status{State: "unsupported", Message: "Automatic release checks are unavailable for local, development or unknown builds."}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	if c.now != nil {
		now = c.now().UTC()
	}
	if c.installed != installed {
		c.cached = Status{}
		c.attempted = time.Time{}
		c.expires = time.Time{}
		c.installed = installed
	}
	if !c.attempted.IsZero() && ((!force && now.Before(c.expires)) || now.Before(retryTime(c.cached.RetryAt, c.attempted.Add(retryCooldown)))) {
		return c.cached
	}
	c.attempted = now
	result := Status{State: "unavailable", Channel: channel(v), AttemptedAt: now.Format(time.RFC3339), RetryAt: now.Add(retryCooldown).Format(time.RFC3339)}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	latest, err := c.latest(ctx, channel(v))
	if err != nil {
		var provider *providerError
		if errors.As(err, &provider) && provider.wait > retryCooldown {
			result.RetryAt = now.Add(provider.wait).Format(time.RFC3339)
		}
		result.Message = "Release check unavailable. GitHub may be offline or limiting requests. Try again shortly."
		if c.cached.CheckedAt != "" {
			result.Stale = true
			result.CheckedAt = c.cached.CheckedAt
			result.AvailableVersion = c.cached.AvailableVersion
			result.ReleaseURL = c.cached.ReleaseURL
		}
		c.expires = now.Add(failureTTL)
	} else {
		result.CheckedAt = now.Format(time.RFC3339)
		if latest == "" {
			result.State = "unknown"
			result.Message = "No published release was found for this channel."
		} else {
			candidate, _ := parse(latest)
			result.State = "current"
			if compare(candidate, v) > 0 {
				result.State = "available"
				result.AvailableVersion = latest
				result.ReleaseURL = repository + "/releases/tag/v" + latest
			}
		}
		c.expires = now.Add(successTTL)
	}
	c.cached = result
	return result
}
func (c *Checker) latest(ctx context.Context, wanted string) (string, error) {
	client := c.client
	if client == nil {
		client = &http.Client{Timeout: 6 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	}
	latest := ""
	// Inspect at most 300 releases; incomplete coverage must not claim up to date.
	for page := 1; page <= 3; page++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s?per_page=100&page=%d", endpoint, page), nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", "application/vnd.github+json")
		req.Header.Set("X-GitHub-Api-Version", "2026-03-10")
		req.Header.Set("User-Agent", "Sente-release-check")
		response, err := client.Do(req)
		if err != nil {
			return "", err
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, maxBody+1))
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK || len(body) > maxBody {
			return "", &providerError{wait: providerWait(response.Header)}
		}
		var items []release
		if err = json.Unmarshal(body, &items); err != nil || items == nil || len(items) > 100 {
			return "", fmt.Errorf("invalid release metadata")
		}
		for _, item := range items {
			if item.Tag == nil || item.Draft == nil || item.Prerelease == nil {
				return "", fmt.Errorf("incomplete release metadata")
			}
			if *item.Draft {
				continue
			}
			if item.Published == nil || item.Published.IsZero() {
				return "", fmt.Errorf("unpublished release metadata")
			}
			tag := *item.Tag
			if !strings.HasPrefix(tag, "v") {
				return "", fmt.Errorf("invalid release tag")
			}
			candidate, valid := parse(tag[1:])
			if !valid {
				return "", fmt.Errorf("invalid release version")
			}
			if *item.Prerelease != (len(candidate.pre) > 0) {
				return "", fmt.Errorf("inconsistent release channel")
			}
			if channel(candidate) != wanted {
				continue
			}
			if latest == "" {
				latest = tag[1:]
			} else {
				previous, _ := parse(latest)
				if compare(candidate, previous) > 0 {
					latest = tag[1:]
				}
			}
		}
		if len(items) < 100 {
			return latest, nil
		}
	}
	return "", fmt.Errorf("release lookup limit reached")
}

// Respect provider backoff without permitting an unbounded remote lockout.
type providerError struct{ wait time.Duration }

func (*providerError) Error() string { return "release provider unavailable" }
func retryTime(value string, fallback time.Time) time.Time {
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return fallback
	}
	return parsed
}
func providerWait(headers http.Header) time.Duration {
	wait := retryCooldown
	if seconds, err := strconv.ParseInt(headers.Get("Retry-After"), 10, 64); err == nil && seconds > 0 {
		if seconds > 3600 {
			seconds = 3600
		}
		wait = time.Duration(seconds) * time.Second
	} else if date, err := http.ParseTime(headers.Get("Retry-After")); err == nil {
		wait = time.Until(date)
	}
	if reset, err := strconv.ParseInt(headers.Get("X-RateLimit-Reset"), 10, 64); err == nil && headers.Get("X-RateLimit-Remaining") == "0" {
		if until := time.Until(time.Unix(reset, 0)); until > wait {
			wait = until
		}
	}
	if wait < retryCooldown {
		wait = retryCooldown
	}
	if wait > time.Hour {
		wait = time.Hour
	}
	return wait
}
