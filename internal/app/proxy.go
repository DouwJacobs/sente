package app

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// ConfigureTrustedProxies must run before serving. No proxy is trusted by default.
// PUBLIC_URL alone controls cookie security and allowed origins.
func (a *App) ConfigureTrustedProxies(value string) error {
	var prefixes []netip.Prefix
	if strings.TrimSpace(value) != "" {
		for _, item := range strings.Split(value, ",") {
			item = strings.TrimSpace(item)
			prefix, err := netip.ParsePrefix(item)
			if err != nil {
				ip, ipErr := netip.ParseAddr(item)
				if ipErr != nil || ip.Zone() != "" {
					return fmt.Errorf("TRUSTED_PROXIES must contain comma-separated IP addresses or CIDRs")
				}
				ip = ip.Unmap()
				prefix = netip.PrefixFrom(ip, ip.BitLen())
			}
			if prefix.Addr().Is4In6() {
				if prefix.Bits() < 96 {
					return fmt.Errorf("TRUSTED_PROXIES contains an invalid IPv4-mapped network")
				}
				prefix = netip.PrefixFrom(prefix.Addr().Unmap(), prefix.Bits()-96)
			}
			if prefix.Bits() == 0 {
				return fmt.Errorf("TRUSTED_PROXIES must not trust every address")
			}
			prefixes = append(prefixes, prefix.Masked())
		}
	}
	a.trustedProxies = prefixes
	return nil
}
func (a *App) trustedProxy(ip netip.Addr) bool {
	for _, prefix := range a.trustedProxies {
		if prefix.Contains(ip) {
			return true
		}
	}
	return false
}
func (a *App) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return r.RemoteAddr
	}
	peer = peer.Unmap()
	if !a.trustedProxy(peer) {
		return peer.String()
	}
	// Walk from the socket peer towards the client. Stop at the first untrusted
	// hop, so a client-supplied prefix cannot change the rate-limit identity.
	value := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if value == "" || len(value) > 4096 {
		return peer.String()
	}
	hops := strings.Split(value, ",")
	if len(hops) > 32 {
		return peer.String()
	}
	chain := make([]netip.Addr, len(hops))
	for i, hop := range hops {
		ip, err := netip.ParseAddr(strings.TrimSpace(hop))
		if err != nil || ip.Zone() != "" {
			return peer.String()
		}
		chain[i] = ip.Unmap()
	}
	for i := len(chain) - 1; i >= 0 && a.trustedProxy(peer); i-- {
		peer = chain[i]
	}
	return peer.String()
}
