package security

import (
	"net/http"
	"net/http/httptest"
	"net/netip"
	"testing"

	"enstream/internal/config"
)

func TestNetworkGuard(t *testing.T) {
	c := config.Default()
	denied := 0
	g := NewNetworkGuard(c.AllowedPrefixes(), func(*http.Request, netip.Addr) { denied++ })
	h := g.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(204) }))
	for addr, want := range map[string]int{
		"127.0.0.1:5000":         204,
		"192.168.0.44:5000":      204,
		"10.1.2.3:1":             204,
		"172.31.255.1:1":         204,
		"[::1]:1":                204,
		"[::ffff:192.168.1.9]:1": 204, // IPv4-mapped
		"8.8.8.8:443":            403,
		"203.0.113.9:1":          403,
		"garbage":                403,
	} {
		r := httptest.NewRequest("GET", "/", nil)
		r.RemoteAddr = addr
		r.Header.Set("X-Forwarded-For", "192.168.1.1") // must be ignored
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != want {
			t.Errorf("%s: %d, want %d", addr, w.Code, want)
		}
	}
	if denied != 3 {
		t.Fatalf("denied callback %d", denied)
	}
	// An explicit list replaces the defaults.
	c.Security.AllowedNetworks = []string{"192.168.50.0/24"}
	g = NewNetworkGuard(c.AllowedPrefixes(), nil)
	if g.Allowed(netip.MustParseAddr("192.168.1.2")) || !g.Allowed(netip.MustParseAddr("192.168.50.7")) {
		t.Fatal("explicit networks")
	}
}
