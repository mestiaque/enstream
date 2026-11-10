// Package security holds ENStream's access rules: which networks may connect, and (in
// paths.go) which files may be served.
package security

import (
	"net"
	"net/http"
	"net/netip"
)

// NetworkGuard allows requests only from the configured networks (by default this computer
// and private LAN ranges). Binding to 0.0.0.0 listens everywhere; this is what keeps
// ENStream LAN-only. X-Forwarded-For is ignored on purpose: ENStream is not meant to sit
// behind a proxy, and the header is trivially forged.
type NetworkGuard struct {
	allowed []netip.Prefix
	onDeny  func(r *http.Request, ip netip.Addr)
}

func NewNetworkGuard(allowed []netip.Prefix, onDeny func(*http.Request, netip.Addr)) *NetworkGuard {
	return &NetworkGuard{allowed: allowed, onDeny: onDeny}
}

func (g *NetworkGuard) Allowed(ip netip.Addr) bool {
	ip = ip.Unmap()
	for _, p := range g.allowed {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

// ClientIP parses the peer address of a request.
func ClientIP(r *http.Request) (netip.Addr, bool) {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}, false
	}
	return ip.Unmap(), true
}

func (g *NetworkGuard) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip, ok := ClientIP(r)
		if !ok || !g.Allowed(ip) {
			if g.onDeny != nil {
				g.onDeny(r, ip)
			}
			http.Error(w, "ENStream only accepts connections from your local network.", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}
