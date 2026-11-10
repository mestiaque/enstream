package security

import (
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// The host is the computer ENStream runs on. Library management (folders, scans, settings,
// PIN, backups, forgetting devices) is only offered there; phones, TVs and other computers
// get the viewer. Requests from the host arrive over loopback (the desktop app and
// "Open in browser" use 127.0.0.1/localhost) or, when a browser on the host opens the LAN
// address, from one of this computer's own addresses.

var local struct {
	sync.Mutex
	addrs map[netip.Addr]bool
	at    time.Time
}

// ownAddrs returns this computer's interface addresses, refreshed every 30 s (DHCP changes).
func ownAddrs() map[netip.Addr]bool {
	local.Lock()
	defer local.Unlock()
	if local.addrs != nil && time.Since(local.at) < 30*time.Second {
		return local.addrs
	}
	m := map[netip.Addr]bool{}
	if list, err := net.InterfaceAddrs(); err == nil {
		for _, a := range list {
			if n, ok := a.(*net.IPNet); ok {
				if ip, ok := netip.AddrFromSlice(n.IP); ok {
					m[ip.Unmap()] = true
				}
			}
		}
	}
	local.addrs, local.at = m, time.Now()
	return m
}

// IsHostAddr reports whether ip belongs to the computer ENStream runs on.
func IsHostAddr(ip netip.Addr) bool {
	ip = ip.Unmap()
	return ip.IsLoopback() || ownAddrs()[ip]
}

// IsHost reports whether a request comes from the computer ENStream runs on.
func IsHost(r *http.Request) bool {
	ip, ok := ClientIP(r)
	return ok && IsHostAddr(ip)
}
