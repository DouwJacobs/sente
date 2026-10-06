package app

import (
	"database/sql"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type networkValues struct {
	PublicURL      string `json:"public_url"`
	TrustedProxies string `json:"trusted_proxies"`
	Source         string `json:"source,omitempty"`
}
type networkSaved struct {
	Enabled        bool   `json:"enabled"`
	PublicURL      string `json:"public_url"`
	TrustedProxies string `json:"trusted_proxies"`
	Version        int64  `json:"version"`
}

func validateNetworkOrigin(value string) (string, error) {
	value = strings.TrimSpace(value)
	u, err := url.Parse(value)
	if err == nil {
		u.Scheme = strings.ToLower(u.Scheme)
		u.Host = strings.ToLower(u.Host)
	}
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || len(value) > 2048 {
		return "", fail(400, "Use an absolute http or https public URL without a path, credentials, query or fragment")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return "", fail(400, "Use a valid port in the public URL")
		}
		if (u.Scheme == "https" && port == "443") || (u.Scheme == "http" && port == "80") {
			u.Host = u.Hostname()
			if strings.Contains(u.Host, ":") {
				u.Host = "[" + u.Host + "]"
			}
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return "", fail(400, "Use a valid port in the public URL")
	}
	u.Path = ""
	u.RawPath = ""
	return u.String(), nil
}
func (a *App) networkSaved() (networkSaved, error) {
	var value networkSaved
	err := a.DB.QueryRow("SELECT enabled,public_url,trusted_proxies,version FROM network_settings WHERE id=1").Scan(&value.Enabled, &value.PublicURL, &value.TrustedProxies, &value.Version)
	return value, err
}

// ApplyNetworkConfig runs once before serving; saves never mutate active policy.
func (a *App) ApplyNetworkConfig(trusted string) error {
	environmentURL, err := validateNetworkOrigin(a.PublicURL)
	if err != nil {
		return err
	}
	a.networkEnvironment = networkValues{PublicURL: environmentURL, TrustedProxies: strings.TrimSpace(trusted), Source: "environment"}
	desired := a.networkEnvironment
	saved, err := a.networkSaved()
	if err != nil {
		return err
	}
	if saved.Enabled {
		desired = networkValues{PublicURL: saved.PublicURL, TrustedProxies: saved.TrustedProxies, Source: "settings"}
	}
	origin, err := validateNetworkOrigin(desired.PublicURL)
	if err != nil {
		return err
	}
	if err := a.ConfigureTrustedProxies(desired.TrustedProxies); err != nil {
		return err
	}
	a.activeNetworkTrusted = strings.TrimSpace(desired.TrustedProxies)
	a.PublicURL = origin
	a.Secure = strings.HasPrefix(origin, "https://")
	a.networkSource = desired.Source
	return nil
}
func (a *App) networkStatus(w http.ResponseWriter, r *http.Request) error {
	if err := requireAdmin(Current(r)); err != nil {
		return err
	}
	saved, err := a.networkSaved()
	if err != nil {
		return err
	}
	environment := a.networkEnvironment
	if environment.PublicURL == "" {
		environment = networkValues{PublicURL: a.PublicURL, Source: "environment"}
	}
	active := networkValues{PublicURL: a.PublicURL, TrustedProxies: environment.TrustedProxies, Source: a.networkSource}
	if active.Source == "" {
		active.Source = "environment"
	}
	// Active trusted policy is immutable; retain its canonical configured text.
	active.TrustedProxies = a.activeNetworkTrusted
	desired := environment
	if saved.Enabled {
		desired = networkValues{PublicURL: saved.PublicURL, TrustedProxies: saved.TrustedProxies, Source: "settings"}
	}
	send(w, map[string]any{"active": active, "environment": environment, "saved": saved, "restart_required": active != desired, "restart_available": a.RequestRestart != nil})
	return nil
}
func (a *App) saveNetwork(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	var b networkSaved
	if err := decode(r, &b); err != nil {
		return err
	}
	if b.Enabled {
		origin, err := validateNetworkOrigin(b.PublicURL)
		if err != nil {
			return err
		}
		b.PublicURL = origin
		b.TrustedProxies = strings.TrimSpace(b.TrustedProxies)
		if len(b.TrustedProxies) > 4096 {
			return fail(400, "Trusted proxies must be at most 4096 characters")
		}
		probe := &App{}
		if err := probe.ConfigureTrustedProxies(b.TrustedProxies); err != nil {
			return fail(400, err.Error())
		}
	} else {
		b.PublicURL = ""
		b.TrustedProxies = ""
	}
	if b.Version < 1 {
		return fail(400, "Reload network settings before saving")
	}
	if err := a.write(func(tx *sql.Tx) error {
		result, err := tx.Exec("UPDATE network_settings SET enabled=?,public_url=?,trusted_proxies=?,version=version+1 WHERE id=1 AND version=?", b.Enabled, b.PublicURL, b.TrustedProxies, b.Version)
		if err != nil {
			return err
		}
		if err := affected(result); err != nil {
			return err
		}
		return audit(tx, u, nil, "network_settings", 1, "updated", b)
	}); err != nil {
		return err
	}
	return a.networkStatus(w, r)
}
func (a *App) ResetNetworkSettings() error {
	return a.write(func(tx *sql.Tx) error {
		if _, err := tx.Exec("UPDATE network_settings SET enabled=0,public_url='',trusted_proxies='',version=version+1 WHERE id=1"); err != nil {
			return err
		}
		_, err := tx.Exec("INSERT INTO audit(entity,entity_id,action,details) VALUES('network_settings',1,'offline_reset','{}')")
		return err
	})
}

func (a *App) restartNetwork(w http.ResponseWriter, r *http.Request) error {
	u := Current(r)
	if err := requireAdmin(u); err != nil {
		return err
	}
	if a.RequestRestart == nil {
		return fail(503, "Restart is unavailable in this runtime")
	}
	var b struct {
		Version int64 `json:"version"`
	}
	if err := decode(r, &b); err != nil {
		return err
	}
	if err := a.write(func(tx *sql.Tx) error {
		var version int64
		if err := tx.QueryRow("SELECT version FROM network_settings WHERE id=1").Scan(&version); err != nil {
			return err
		}
		if version != b.Version {
			return fail(409, "Network settings changed; reload before restarting")
		}
		return audit(tx, u, nil, "network_settings", 1, "restart_requested", nil)
	}); err != nil {
		return err
	}
	send(w, map[string]bool{"restarting": true})
	time.AfterFunc(500*time.Millisecond, a.RequestRestart)
	return nil
}
