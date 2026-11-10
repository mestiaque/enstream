package app

import (
	"encoding/json"
	"strings"
	"testing"
)

// Library management is offered only on the computer ENStream runs on; other devices watch.
func TestHostOnlyManagement(t *testing.T) {
	_, h := newTestApp(t)
	const phone = "192.168.1.40:5555"

	for _, c := range []struct{ method, path, body string }{
		{"GET", "/api/v1/fs/dirs", ""},
		{"POST", "/api/v1/folders", `{"path":"/tmp"}`},
		{"DELETE", "/api/v1/folders/1", ""},
		{"POST", "/api/v1/scanner/scan", `{}`},
		{"POST", "/api/v1/scanner/cancel", ``},
		{"POST", "/api/v1/scanner/retry-failed", ``},
		{"POST", "/api/v1/system/ffmpeg/install", ``},
		{"GET", "/api/v1/settings", ""},
		{"PUT", "/api/v1/settings", `{"scanner":{"auto_scan":false}}`},
		{"PUT", "/api/v1/auth/pin", `{"new_pin":"2468"}`},
		{"POST", "/api/v1/auth/pin/disable", `{}`},
		{"GET", "/api/v1/auth/sessions", ""},
		{"POST", "/api/v1/auth/sessions/sign-out-others", ``},
		{"GET", "/api/v1/backups", ""},
		{"POST", "/api/v1/backups", ``},
		{"DELETE", "/api/v1/devices/1", ""},
	} {
		w := do(h, c.method, c.path, phone, strings.NewReader(c.body))
		if w.Code != 403 || !strings.Contains(w.Body.String(), "HOST_ONLY") {
			t.Errorf("%s %s from a phone: %d %s", c.method, c.path, w.Code, w.Body)
		}
	}

	// Watching works from any device on the network.
	for _, path := range []string{"/api/v1/videos", "/api/v1/folders", "/api/v1/dashboard", "/api/v1/history", "/api/v1/scanner/status"} {
		if w := do(h, "GET", path, phone, nil); w.Code != 200 {
			t.Errorf("GET %s from a phone: %d", path, w.Code)
		}
	}
	if w := doWith(h, "PUT", "/api/v1/devices/current", phone, strings.NewReader(`{"name":"Living room TV"}`), map[string]string{"X-Device-Token": "tv-ABCDEFGHIJKLMN"}); w.Code != 200 {
		t.Errorf("a phone could not name itself: %d", w.Code)
	}

	// The client learns which kind it is.
	host := func(remote string) bool {
		w := do(h, "GET", "/api/v1/system/info", remote, nil)
		var e struct{ Data struct{ Host bool } }
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		return e.Data.Host
	}
	if host(phone) || !host("127.0.0.1:1") || !host("[::1]:1") {
		t.Error("system/info host flag is wrong")
	}
}
