package app

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestApp boots a real App on a temporary data directory.
func newTestApp(t *testing.T) (*App, http.Handler) {
	t.Helper()
	a, err := Bootstrap(context.Background(), Options{DataDir: t.TempDir(), Port: 18080})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { a.DB.Close() })
	h, err := a.Handler()
	if err != nil {
		t.Fatal(err)
	}
	return a, h
}

func do(h http.Handler, method, path, remote string, body io.Reader) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, body)
	r.RemoteAddr = remote
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSystemEndpointsAndFrontend(t *testing.T) {
	_, h := newTestApp(t)
	const lan = "192.168.1.20:5555"

	w := do(h, "GET", "/api/v1/health", lan, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"success":true`) {
		t.Fatalf("health: %d %s", w.Code, w.Body)
	}
	w = do(h, "GET", "/api/v1/system/info", lan, nil)
	var info struct {
		Success bool
		Data    map[string]any
	}
	if err := json.Unmarshal(w.Body.Bytes(), &info); err != nil || !info.Success {
		t.Fatalf("info: %s", w.Body)
	}
	if info.Data["name"] != "ENStream" || info.Data["schema_version"].(float64) != 1 {
		t.Fatalf("info data: %v", info.Data)
	}
	if strings.Contains(w.Body.String(), `"path"`) {
		t.Fatal("tool paths must not be exposed")
	}

	w = do(h, "GET", "/api/v1/nope", lan, nil)
	if w.Code != 404 || !strings.Contains(w.Body.String(), `"NOT_FOUND"`) {
		t.Fatalf("unknown API: %d %s", w.Code, w.Body)
	}
	w = do(h, "POST", "/api/v1/health", lan, nil)
	if w.Code != 405 {
		t.Fatalf("method: %d", w.Code)
	}

	// The UI: index for client routes, versioned assets, no directory listing.
	w = do(h, "GET", "/library", lan, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "/static/css/app.css?v="+Version) {
		t.Fatalf("index: %d", w.Code)
	}
	if w.Header().Get("Content-Security-Policy") == "" || w.Header().Get("X-Frame-Options") != "DENY" {
		t.Fatal("security headers")
	}
	w = do(h, "GET", "/static/css/app.css?v=1", lan, nil)
	if w.Code != 200 || !strings.Contains(w.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("asset: %d %s", w.Code, w.Header().Get("Cache-Control"))
	}
	if w := do(h, "GET", "/static/css/", lan, nil); w.Code != 404 {
		t.Fatalf("directory listing: %d", w.Code)
	}
	if w := do(h, "GET", "/missing.js", lan, nil); w.Code != 404 {
		t.Fatalf("missing asset: %d", w.Code)
	}
}

func TestOnlyLocalNetworksAndBodyLimit(t *testing.T) {
	_, h := newTestApp(t)
	if w := do(h, "GET", "/api/v1/health", "8.8.8.8:1234", nil); w.Code != 403 {
		t.Fatalf("public address: %d", w.Code)
	}
	if w := do(h, "GET", "/", "203.0.113.1:1", nil); w.Code != 403 {
		t.Fatalf("public address on UI: %d", w.Code)
	}
	big := strings.NewReader(strings.Repeat("x", 2<<20))
	if w := do(h, "POST", "/api/v1/health", "127.0.0.1:1", big); w.Code != 413 && w.Code != 405 {
		t.Fatalf("oversized body: %d", w.Code)
	}
}

func TestBannerShowsURLs(t *testing.T) {
	a, _ := newTestApp(t)
	b := a.Banner()
	for _, want := range []string{"ENSTREAM", "http://localhost:18080", "Database: READY", "CTRL+C"} {
		if !strings.Contains(b, want) {
			t.Errorf("banner lacks %q", want)
		}
	}
}
