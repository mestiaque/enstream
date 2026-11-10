package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"enstream/internal/config"
	"enstream/internal/jobs"
)

// client keeps cookies between requests, like a browser.
type client struct {
	t       *testing.T
	h       http.Handler
	ip      string
	cookies []*http.Cookie
}

func (c *client) do(method, path, body string, headers ...string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = c.ip + ":5000"
	for i := 0; i+1 < len(headers); i += 2 {
		r.Header.Set(headers[i], headers[i+1])
	}
	for _, ck := range c.cookies {
		r.AddCookie(ck)
	}
	w := httptest.NewRecorder()
	c.h.ServeHTTP(w, r)
	for _, ck := range w.Result().Cookies() {
		c.cookies = []*http.Cookie{}
		if ck.MaxAge >= 0 && ck.Value != "" {
			c.cookies = append(c.cookies, ck)
		}
	}
	return w
}

func TestPINSessionsAndCSRF(t *testing.T) {
	_, h := newTestApp(t)
	owner := &client{t: t, h: h, ip: "192.168.1.10"}
	guest := &client{t: t, h: h, ip: "192.168.1.20"}

	// No PIN: everything is open.
	if w := guest.do("GET", "/api/v1/videos", ""); w.Code != 200 {
		t.Fatalf("open library: %d", w.Code)
	}
	if w := owner.do("PUT", "/api/v1/auth/pin", `{"new_pin":"12"}`); w.Code != 400 {
		t.Fatalf("short PIN accepted: %d", w.Code)
	}
	w := owner.do("PUT", "/api/v1/auth/pin", `{"new_pin":"2468"}`)
	if w.Code != 200 || len(owner.cookies) != 1 {
		t.Fatalf("set PIN: %d %s", w.Code, w.Body)
	}
	ck := owner.cookies[0]
	if !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Fatal("session cookie must be HttpOnly and SameSite=Strict")
	}
	// The owner stays signed in; the guest is locked out of the API but can load the page.
	if w := owner.do("GET", "/api/v1/videos", ""); w.Code != 200 {
		t.Fatalf("owner after PIN: %d", w.Code)
	}
	if w := guest.do("GET", "/api/v1/videos", ""); w.Code != 401 || !strings.Contains(w.Body.String(), "AUTH_REQUIRED") {
		t.Fatalf("guest without session: %d", w.Code)
	}
	for _, p := range []string{"/api/v1/videos/1/stream", "/api/v1/videos/1/thumbnail", "/api/v1/settings"} {
		if w := guest.do("GET", p, ""); w.Code != 401 {
			t.Fatalf("%s without session: %d", p, w.Code)
		}
	}
	if w := guest.do("GET", "/library", ""); w.Code != 200 {
		t.Fatal("the page itself must load for the sign-in screen")
	}
	if w := guest.do("GET", "/api/v1/auth/status", ""); !strings.Contains(w.Body.String(), `"authenticated":false`) {
		t.Fatalf("status: %s", w.Body)
	}

	// Wrong PINs are throttled per IP.
	for i := 0; i < 5; i++ {
		if w := guest.do("POST", "/api/v1/auth/login", `{"pin":"0000"}`); w.Code != 401 {
			t.Fatalf("wrong PIN %d: %d", i, w.Code)
		}
	}
	w = guest.do("POST", "/api/v1/auth/login", `{"pin":"0000"}`)
	if w.Code != 429 || w.Header().Get("Retry-After") == "" {
		t.Fatalf("lockout: %d", w.Code)
	}
	if w := guest.do("POST", "/api/v1/auth/login", `{"pin":"2468"}`); w.Code != 429 {
		t.Fatal("even the right PIN waits during a lockout")
	}
	other := &client{t: t, h: h, ip: "192.168.1.21"}
	if w := other.do("POST", "/api/v1/auth/login", `{"pin":"2468"}`); w.Code != 200 || len(other.cookies) != 1 {
		t.Fatalf("login from another IP: %d", w.Code)
	}
	if w := other.do("GET", "/api/v1/videos", ""); w.Code != 200 {
		t.Fatal("session works")
	}

	// CSRF: a state change from another website is refused even with a session.
	if w := other.do("POST", "/api/v1/scanner/scan", `{}`, "Origin", "http://evil.example"); w.Code != 403 {
		t.Fatalf("cross-origin POST: %d", w.Code)
	}
	if w := other.do("PUT", "/api/v1/settings", `{}`, "Sec-Fetch-Site", "cross-site"); w.Code != 403 {
		t.Fatalf("cross-site PUT: %d", w.Code)
	}
	if w := other.do("PUT", "/api/v1/settings", `{}`, "Origin", "http://example.com"); w.Code != 200 {
		t.Fatalf("same-origin PUT: %d %s", w.Code, w.Body)
	}

	// Sessions list and sign-out of others.
	w = owner.do("GET", "/api/v1/auth/sessions", "")
	if !strings.Contains(w.Body.String(), `"current":true`) || strings.Count(w.Body.String(), `"id"`) != 2 {
		t.Fatalf("sessions: %s", w.Body)
	}
	owner.do("POST", "/api/v1/auth/sessions/sign-out-others", "")
	if w := other.do("GET", "/api/v1/videos", ""); w.Code != 401 {
		t.Fatal("signed-out session still works")
	}

	// Changing the PIN needs the current one; turning it off too.
	if w := owner.do("PUT", "/api/v1/auth/pin", `{"current_pin":"1111","new_pin":"9999"}`); w.Code != 401 {
		t.Fatalf("change with wrong PIN: %d", w.Code)
	}
	if w := owner.do("PUT", "/api/v1/auth/pin", `{"current_pin":"2468","new_pin":"9999"}`); w.Code != 200 {
		t.Fatalf("change PIN: %d", w.Code)
	}
	if w := owner.do("GET", "/api/v1/videos", ""); w.Code != 200 {
		t.Fatal("owner keeps a session after changing the PIN")
	}
	if w := owner.do("POST", "/api/v1/auth/pin/disable", `{"current_pin":"9999"}`); w.Code != 200 {
		t.Fatalf("disable: %d", w.Code)
	}
	if w := guest.do("GET", "/api/v1/videos", ""); w.Code != 200 {
		t.Fatal("open again after removing the PIN")
	}
}

func TestSettingsAPI(t *testing.T) {
	a, h := newTestApp(t)
	c := &client{t: t, h: h, ip: "192.168.1.10"}
	w := c.do("GET", "/api/v1/settings", "")
	if w.Code != 200 || !strings.Contains(w.Body.String(), `"worker_count":3`) || !strings.Contains(w.Body.String(), `"pin_enabled":false`) {
		t.Fatalf("get: %s", w.Body)
	}
	w = c.do("PUT", "/api/v1/settings", `{"scanner":{"auto_scan":true,"scan_interval_minutes":30,"excluded_directories":[" Samples ","Samples","Trailers"]},"media":{"extensions":["MP4","mkv"]}}`)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "scan your folders") {
		t.Fatalf("put: %d %s", w.Code, w.Body)
	}
	cur := a.Settings.Get()
	if !cur.Scanner.AutoScan || cur.Scanner.ScanIntervalMinutes != 30 || len(cur.Scanner.ExcludedDirectories) != 2 || cur.Media.Extensions[0] != ".mp4" {
		t.Fatalf("applied: %+v %+v", cur.Scanner, cur.Media)
	}
	// Saved to disk, without the -port override of the test.
	disk, err := config.Load(a.Paths.Config())
	if err != nil || !disk.Scanner.AutoScan || disk.Server.Port != 8080 {
		t.Fatalf("on disk: %+v %v", disk.Server, err)
	}
	for _, bad := range []string{`{"scanner":{"worker_count":0}}`, `{"scanner":{"scan_interval_minutes":1}}`, `{"media":{"extensions":[]}}`,
		`{"media":{"extensions":["../x"]}}`, `{"scanner":{"excluded_directories":["/etc"]}}`, `{"server":{"port":80}}`, `{"thumbnail":{"quality":0}}`} {
		if w := c.do("PUT", "/api/v1/settings", bad); w.Code != 400 {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	if a.Settings.Get().Scanner.WorkerCount != 3 {
		t.Fatal("a rejected update must change nothing")
	}
}

func TestBackupAndRestore(t *testing.T) {
	a, h := newTestApp(t)
	c := &client{t: t, h: h, ip: "192.168.1.10"}
	ctx := context.Background()
	media := t.TempDir()
	a.Store.AddFolder(ctx, media, "Before")

	w := c.do("POST", "/api/v1/backups", "")
	if w.Code != 201 {
		t.Fatalf("create: %d %s", w.Code, w.Body)
	}
	var created struct{ Data struct{ Name string } }
	json.Unmarshal(w.Body.Bytes(), &created)
	name := created.Data.Name
	if w := c.do("GET", "/api/v1/backups/"+name, ""); w.Code != 200 || !strings.HasPrefix(w.Body.String(), "SQLite format 3") {
		t.Fatalf("download: %d", w.Code)
	}
	for _, bad := range []string{"..%2Fenstream.db", "enstream.db", "x.db"} {
		if w := c.do("GET", "/api/v1/backups/"+bad, ""); w.Code != 404 {
			t.Errorf("%s: %d", bad, w.Code)
		}
	}
	// A broken file is refused for restore.
	os.WriteFile(a.Paths.Backups+"/enstream-20200101-000000.db", []byte("not sqlite"), 0o600)
	if w := c.do("POST", "/api/v1/backups/enstream-20200101-000000.db/restore", ""); w.Code != 422 {
		t.Fatalf("invalid restore: %d %s", w.Code, w.Body)
	}

	// Change the library after the backup, then restore it.
	a.Store.AddFolder(ctx, t.TempDir(), "After")
	if w := c.do("POST", "/api/v1/backups/"+name+"/restore", ""); w.Code != 200 {
		t.Fatalf("stage: %d %s", w.Code, w.Body)
	}
	if w := c.do("GET", "/api/v1/backups", ""); !strings.Contains(w.Body.String(), `"pending_restore":"`+name+`"`) {
		t.Fatalf("pending: %s", w.Body)
	}
	a.DB.Close()
	b, err := Bootstrap(ctx, Options{DataDir: a.Paths.Data, Port: 18080})
	if err != nil {
		t.Fatal(err)
	}
	defer b.DB.Close()
	folders, _ := b.Store.Folders(ctx)
	if len(folders) != 1 || folders[0].Name != "Before" {
		t.Fatalf("restored folders: %+v", folders)
	}
	list, _ := b.Backups.List()
	pre := 0
	for _, x := range list {
		if x.Kind == "pre-restore" {
			pre++
		}
	}
	if pre != 1 || b.Backups.Pending() != "" {
		t.Fatalf("pre-restore copy %d, pending %q", pre, b.Backups.Pending())
	}
}

func TestSchedulerAutoScanAndBackup(t *testing.T) {
	a, _ := newTestApp(t)
	ctx := context.Background()
	a.Store.AddFolder(ctx, t.TempDir(), "Media")
	backups := 0
	s := &jobs.Scheduler{Scanner: a.Scanner, Store: a.Store, Settings: a.Settings, Log: a.Log,
		LastBackup: a.Backups.LastAuto,
		Backup:     func(ctx context.Context) error { backups++; _, err := a.Backups.Create(ctx, "auto"); return err }}

	s.Step(ctx) // auto scan is off by default
	a.Scanner.Wait()
	if j, _ := a.Store.ScanJobs(ctx, 1); len(j) != 0 {
		t.Fatal("scan started while auto scan is off")
	}
	if backups != 1 {
		t.Fatalf("first automatic backup: %d", backups)
	}
	a.Settings.Update(func(c *config.Config) error {
		c.Scanner.AutoScan = true
		c.Scanner.ScanIntervalMinutes = 60
		return nil
	})
	s.Step(ctx)
	a.Scanner.Wait()
	s.Step(ctx) // just scanned, so no second scan; backup is fresh
	a.Scanner.Wait()
	if j, _ := a.Store.ScanJobs(ctx, 10); len(j) != 1 {
		t.Fatalf("scans: %d", len(j))
	}
	if backups != 1 {
		t.Fatalf("backups: %d", backups)
	}
	_ = time.Second
}

func TestFolderBrowserQRAndTools(t *testing.T) {
	a, h := newTestApp(t)
	c := &client{t: t, h: h, ip: "192.168.1.10"}
	root := t.TempDir()
	os.MkdirAll(root+"/Movies/Old", 0o755)
	os.MkdirAll(root+"/.hidden", 0o755)
	os.WriteFile(root+"/file.mp4", []byte("x"), 0o644)
	w := c.do("GET", "/api/v1/fs/dirs?path="+root, "")
	body := w.Body.String()
	if w.Code != 200 || !strings.Contains(body, `"name":"Movies"`) || strings.Contains(body, "hidden") || strings.Contains(body, "file.mp4") {
		t.Fatalf("dirs: %d %s", w.Code, body)
	}
	if w := c.do("GET", "/api/v1/fs/dirs?path=relative", ""); w.Code != 400 {
		t.Fatalf("relative: %d", w.Code)
	}
	if w := c.do("GET", "/api/v1/fs/dirs", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"kind"`) {
		t.Fatalf("places: %s", w.Body)
	}
	if w := c.do("GET", "/api/v1/system/qr?url=https://evil.example", ""); w.Code != 400 {
		t.Fatal("QR for a foreign URL")
	}
	if len(a.Addresses) > 0 {
		if w := c.do("GET", "/api/v1/system/qr?url="+a.Addresses[0].URL, ""); w.Code != 200 || w.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("QR: %d", w.Code)
		}
	}
	if w := c.do("GET", "/api/v1/system/ffmpeg", ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"can_install":false`) {
		t.Fatalf("ffmpeg status: %s", w.Body)
	}
	if w := c.do("POST", "/api/v1/system/ffmpeg/install", ""); w.Code != 400 {
		t.Fatalf("install on Linux: %d", w.Code)
	}
}
