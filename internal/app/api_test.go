package app

import (
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type envelope struct {
	Success    bool            `json:"success"`
	Data       json.RawMessage `json:"data"`
	Message    string          `json:"message"`
	Pagination *struct {
		Page, Limit, Total int
		TotalPages         int `json:"total_pages"`
	} `json:"pagination"`
	Error *struct{ Code, Message string } `json:"error"`
}

func call(t *testing.T, h http.Handler, method, path, body string) (int, envelope) {
	t.Helper()
	var rd *strings.Reader
	if body != "" {
		rd = strings.NewReader(body)
	} else {
		rd = strings.NewReader("")
	}
	w := do(h, method, path, "127.0.0.1:4000", rd) // the host: library management is host-only
	var e envelope
	if strings.HasPrefix(w.Header().Get("Content-Type"), "application/json") {
		if err := json.Unmarshal(w.Body.Bytes(), &e); err != nil {
			t.Fatalf("%s %s: bad JSON %s", method, path, w.Body)
		}
	}
	return w.Code, e
}

func clip(t *testing.T, path string) {
	t.Helper()
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	os.MkdirAll(filepath.Dir(path), 0o755)
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=2",
		"-c:v", "libx264", "-pix_fmt", "yuv420p", path).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
}

func TestFoldersScanAndLibraryAPI(t *testing.T) {
	a, h := newTestApp(t)
	media := t.TempDir()
	clip(t, filepath.Join(media, "Beach Day.mp4"))
	clip(t, filepath.Join(media, "trips", "Mountain.mp4"))
	js, _ := json.Marshal(map[string]string{"path": media})

	// Invalid folders are refused with a clear code.
	for _, body := range []string{`{"path":"relative/dir"}`, `{"path":"/does/not/exist"}`, `{"nope":1}`, `not json`} {
		if code, e := call(t, h, "POST", "/api/v1/folders", body); code != 400 || e.Success {
			t.Errorf("%s: %d", body, code)
		}
	}
	code, e := call(t, h, "POST", "/api/v1/folders", string(js))
	if code != 201 || !e.Success {
		t.Fatalf("add folder: %d %+v", code, e)
	}
	var folder struct {
		ID        int64
		Available bool
	}
	json.Unmarshal(e.Data, &folder)
	a.Scanner.Wait() // adding starts a scan

	// Overlapping folders are refused.
	sub, _ := json.Marshal(map[string]string{"path": filepath.Join(media, "trips")})
	if code, e := call(t, h, "POST", "/api/v1/folders", string(sub)); code != 409 || e.Error.Code != "FOLDER_OVERLAPS" {
		t.Fatalf("nested folder: %d %+v", code, e)
	}

	code, e = call(t, h, "GET", "/api/v1/scanner/status", "")
	if code != 200 || !strings.Contains(string(e.Data), `"status":"completed"`) || !strings.Contains(string(e.Data), `"new_files":2`) {
		t.Fatalf("status: %s", e.Data)
	}

	code, e = call(t, h, "GET", "/api/v1/videos?sort=title&order=asc&limit=1", "")
	if code != 200 || e.Pagination == nil || e.Pagination.Total != 2 || e.Pagination.TotalPages != 2 {
		t.Fatalf("list: %d %+v", code, e.Pagination)
	}
	var vids []struct {
		ID           int64
		Title        string
		RelativePath string `json:"relative_path"`
		HasThumbnail bool   `json:"has_thumbnail"`
		Height       int
	}
	json.Unmarshal(e.Data, &vids)
	if len(vids) != 1 || vids[0].Title != "Beach Day" || !vids[0].HasThumbnail || vids[0].Height != 360 {
		t.Fatalf("first page: %+v", vids)
	}
	if strings.Contains(string(e.Data), media) {
		t.Fatal("absolute media paths must not be exposed")
	}
	if _, e := call(t, h, "GET", "/api/v1/videos?search=mount", ""); e.Pagination.Total != 1 {
		t.Fatal("search")
	}
	if _, e := call(t, h, "GET", "/api/v1/videos?resolution=1080p", ""); e.Pagination.Total != 0 {
		t.Fatal("resolution filter")
	}

	// Parameters are validated; SQL never comes from the request.
	for _, q := range []string{"sort=title DROP TABLE videos", "order=sideways", "limit=1000", "page=0", "resolution=8k", "watched=maybe", "folder_id=-1", "extension=../x"} {
		if code, e := call(t, h, "GET", "/api/v1/videos?"+strings.ReplaceAll(q, " ", "%20"), ""); code != 400 || e.Error.Code != "INVALID_PARAMETER" {
			t.Errorf("%s: %d", q, code)
		}
	}
	if _, e := call(t, h, "GET", "/api/v1/videos?search="+url.QueryEscape("%' OR 1=1 --"), ""); e.Pagination.Total != 0 {
		t.Fatal("search must be a plain substring match")
	}

	// Details, thumbnail, unknown ids.
	if code, e := call(t, h, "GET", "/api/v1/videos/"+itoa64(vids[0].ID), ""); code != 200 || !strings.Contains(string(e.Data), `"subtitles":[]`) {
		t.Fatalf("details: %d %s", code, e.Data)
	}
	w := do(h, "GET", "/api/v1/videos/"+itoa64(vids[0].ID)+"/thumbnail", "192.168.1.30:1", nil)
	if w.Code != 200 || w.Header().Get("Content-Type") != "image/webp" || w.Body.Len() == 0 {
		t.Fatalf("thumbnail: %d %s", w.Code, w.Header().Get("Content-Type"))
	}
	if code, e := call(t, h, "GET", "/api/v1/videos/999", ""); code != 404 || e.Error.Code != "VIDEO_NOT_FOUND" {
		t.Fatal("unknown video")
	}
	if code, _ := call(t, h, "GET", "/api/v1/videos/abc", ""); code != 400 {
		t.Fatal("bad id")
	}

	// Removing the folder clears the index and thumbnails but never the files.
	if code, _ := call(t, h, "DELETE", "/api/v1/folders/"+itoa64(folder.ID), ""); code != 200 {
		t.Fatal("delete folder")
	}
	if _, e := call(t, h, "GET", "/api/v1/videos", ""); e.Pagination.Total != 0 {
		t.Fatal("index not cleared")
	}
	if _, err := os.Stat(filepath.Join(media, "Beach Day.mp4")); err != nil {
		t.Fatal("a video file was touched")
	}
	entries, _ := os.ReadDir(a.Paths.Thumbnails)
	if len(entries) != 0 {
		t.Fatalf("thumbnails left: %d", len(entries))
	}
}

func itoa64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestPlaybackHistoryDevicesAndSubtitles(t *testing.T) {
	a, h := newTestApp(t)
	media := t.TempDir()
	clip(t, filepath.Join(media, "Talk.mp4"))
	os.WriteFile(filepath.Join(media, "Talk.en.srt"), []byte("1\n00:00:00,000 --> 00:00:01,000\nHi\n"), 0o644)
	js, _ := json.Marshal(map[string]string{"path": media})
	call(t, h, "POST", "/api/v1/folders", string(js))
	a.Scanner.Wait()
	_, e := call(t, h, "GET", "/api/v1/videos", "")
	var vids []struct{ ID int64 }
	json.Unmarshal(e.Data, &vids)
	id := itoa64(vids[0].ID)

	send := func(method, path, body, token string) int {
		req := strings.NewReader(body)
		w := doWith(h, method, path, "192.168.1.30:4000", req, map[string]string{"X-Device-Token": token})
		return w.Code
	}
	tok := "phone-ABCDEFGHIJKL"
	// History is per device: read it the way the phone does, with its token.
	asPhone := func(path string) envelope {
		w := doWith(h, "GET", path, "192.168.1.30:4000", nil, map[string]string{"X-Device-Token": tok})
		var e envelope
		_ = json.Unmarshal(w.Body.Bytes(), &e)
		return e
	}
	// A malformed token is accepted but records nothing: history needs a device.
	if c := send("PUT", "/api/v1/videos/"+id+"/progress", `{"position":1,"duration":2}`, "short"); c != 200 {
		t.Fatalf("progress without a valid token: %d", c)
	}
	if c := send("PUT", "/api/v1/videos/"+id+"/progress", `{"position":-1,"duration":2}`, tok); c != 400 {
		t.Fatalf("negative position: %d", c)
	}
	if c := send("PUT", "/api/v1/videos/999/progress", `{"position":1,"duration":2}`, tok); c != 404 {
		t.Fatalf("unknown video: %d", c)
	}
	if c := send("PUT", "/api/v1/videos/"+id+"/progress", `{"position":0.8,"duration":2}`, tok); c != 200 {
		t.Fatalf("progress: %d", c)
	}
	// sendBeacon: POST with the token in the body.
	w := do(h, "POST", "/api/v1/videos/"+id+"/progress", "192.168.1.30:1", strings.NewReader(`{"position":0.9,"duration":2,"device_token":"`+tok+`"}`))
	if w.Code != 200 {
		t.Fatalf("beacon: %d %s", w.Code, w.Body)
	}
	e = asPhone("/api/v1/continue-watching")
	if e.Pagination.Total != 1 || !strings.Contains(string(e.Data), `"position":0.9`) {
		t.Fatalf("continue watching: %s", e.Data)
	}
	// Another device (here the host) does not see the phone's history.
	if _, e = call(t, h, "GET", "/api/v1/continue-watching", ""); e.Pagination.Total != 0 {
		t.Fatalf("the phone's history leaked to another device: %s", e.Data)
	}
	e = asPhone("/api/v1/dashboard")
	if !strings.Contains(string(e.Data), `"position":0.9`) {
		t.Fatalf("dashboard continue watching: %s", e.Data)
	}
	for _, k := range []string{`"stats"`, `"recently_added"`, `"continue_watching"`, `"last_scan"`} {
		if !strings.Contains(string(e.Data), k) {
			t.Fatalf("dashboard lacks %s: %s", k, e.Data)
		}
	}
	if c := send("POST", "/api/v1/videos/"+id+"/completed", "", tok); c != 200 {
		t.Fatalf("completed: %d", c)
	}
	if e = asPhone("/api/v1/continue-watching"); e.Pagination.Total != 0 {
		t.Fatal("a finished video must leave continue watching")
	}
	if e = asPhone("/api/v1/history"); e.Pagination.Total != 1 {
		t.Fatal("history")
	}

	// Devices: the phone appears and can be named.
	if c := send("PUT", "/api/v1/devices/current", `{"name":"Kitchen phone"}`, tok); c != 200 {
		t.Fatalf("name device: %d", c)
	}
	w = doWith(h, "GET", "/api/v1/devices", "192.168.1.30:1", nil, map[string]string{"X-Device-Token": tok})
	if !strings.Contains(w.Body.String(), `"name":"Kitchen phone"`) || !strings.Contains(w.Body.String(), `"current":true`) {
		t.Fatalf("devices: %s", w.Body)
	}

	// Subtitles come out as WebVTT.
	_, e = call(t, h, "GET", "/api/v1/videos/"+id, "")
	var det struct{ Subtitles []struct{ ID int64 } }
	json.Unmarshal(e.Data, &det)
	if len(det.Subtitles) != 1 {
		t.Fatalf("subtitles: %s", e.Data)
	}
	w = do(h, "GET", "/api/v1/videos/"+id+"/subtitles/"+itoa64(det.Subtitles[0].ID), "192.168.1.30:1", nil)
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "WEBVTT") || !strings.Contains(w.Body.String(), "00:00:00.000 --> 00:00:01.000") {
		t.Fatalf("vtt: %d %q", w.Code, w.Body)
	}
	if w := do(h, "GET", "/api/v1/videos/"+id+"/subtitles/999", "192.168.1.30:1", nil); w.Code != 404 {
		t.Fatal("unknown subtitle")
	}

	// Each device clears its own history.
	if c := send("DELETE", "/api/v1/history", "", tok); c != 200 {
		t.Fatalf("phone could not clear its history: %d", c)
	}
	if e = asPhone("/api/v1/history"); e.Pagination.Total != 0 {
		t.Fatal("history not cleared")
	}
}
