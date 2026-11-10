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
	w := do(h, method, path, "192.168.1.30:4000", rd)
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
