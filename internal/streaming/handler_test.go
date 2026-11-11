package streaming

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/go-chi/chi/v5"

	"enstream/internal/database"
	repo "enstream/internal/database/repositories"
)

type fixture struct {
	srv     *httptest.Server
	content []byte
	path    string
	id      int64
	store   *repo.Store
}

func setup(t *testing.T) *fixture {
	t.Helper()
	db, err := database.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	store := repo.New(db)
	media := t.TempDir()
	content := make([]byte, 3<<20+123) // 3 MB, not a multiple of any buffer size
	rand.Read(content)
	path := filepath.Join(media, "movie.mp4")
	os.WriteFile(path, content, 0o644)
	fid, _ := store.AddFolder(context.Background(), media, "m")
	id, _, err := store.UpsertVideo(context.Background(), repo.VideoRecord{FolderID: fid, Title: "movie", FileName: "movie.mp4", FilePath: path, RelativePath: "movie.mp4", Extension: ".mp4", MimeType: "video/mp4", FileSize: int64(len(content))})
	if err != nil {
		t.Fatal(err)
	}
	h := &Handler{Store: store, Log: slog.New(slog.NewTextHandler(io.Discard, nil))}
	r := chi.NewRouter()
	r.Get("/videos/{id}/stream", h.Stream)
	r.Head("/videos/{id}/stream", h.Stream)
	r.Get("/videos/{id}/check", h.Check)
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return &fixture{srv: srv, content: content, path: path, id: id, store: store}
}

func (f *fixture) get(t *testing.T, method, rng string) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, fmt.Sprintf("%s/videos/%d/stream", f.srv.URL, f.id), nil)
	if rng != "" {
		req.Header.Set("Range", rng)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res, b
}

func TestRanges(t *testing.T) {
	f := setup(t)
	n := len(f.content)

	res, body := f.get(t, "GET", "")
	if res.StatusCode != 200 || !bytes.Equal(body, f.content) || res.Header.Get("Content-Type") != "video/mp4" || res.Header.Get("Accept-Ranges") != "bytes" {
		t.Fatalf("full: %d %s", res.StatusCode, res.Header)
	}
	if res.ContentLength != int64(n) {
		t.Fatalf("content-length %d", res.ContentLength)
	}
	if strings.Contains(res.Header.Get("Content-Disposition"), "attachment") {
		t.Fatal("must not force a download")
	}

	cases := []struct {
		rng        string
		start, end int // inclusive
	}{
		{"bytes=0-0", 0, 0},
		{"bytes=100-199", 100, 199},
		{fmt.Sprintf("bytes=%d-", n-10), n - 10, n - 1},         // open-ended
		{"bytes=-500", n - 500, n - 1},                          // suffix
		{fmt.Sprintf("bytes=%d-%d", n-5, n+1000), n - 5, n - 1}, // end clamped
	}
	for _, c := range cases {
		res, body := f.get(t, "GET", c.rng)
		want := f.content[c.start : c.end+1]
		cr := fmt.Sprintf("bytes %d-%d/%d", c.start, c.end, n)
		if res.StatusCode != 206 || !bytes.Equal(body, want) || res.Header.Get("Content-Range") != cr || res.ContentLength != int64(len(want)) {
			t.Errorf("%s: %d %q len=%d", c.rng, res.StatusCode, res.Header.Get("Content-Range"), len(body))
		}
	}

	// Unsatisfiable → 416 with the size.
	res, _ = f.get(t, "GET", fmt.Sprintf("bytes=%d-", n+10))
	if res.StatusCode != 416 || res.Header.Get("Content-Range") != fmt.Sprintf("bytes */%d", n) {
		t.Fatalf("416: %d %q", res.StatusCode, res.Header.Get("Content-Range"))
	}
	// Malformed → ignored per RFC 9110 (full content) or rejected; never a broken partial.
	for _, bad := range []string{"bytes=abc", "items=0-10", "bytes=10-5"} {
		res, body := f.get(t, "GET", bad)
		if res.StatusCode == 206 || (res.StatusCode == 200 && !bytes.Equal(body, f.content)) {
			t.Errorf("%s: %d len=%d", bad, res.StatusCode, len(body))
		}
	}
	// Multiple ranges: a well-formed multipart/byteranges response.
	res, body = f.get(t, "GET", "bytes=0-9,20-29")
	if res.StatusCode != 206 || !strings.HasPrefix(res.Header.Get("Content-Type"), "multipart/byteranges") || !bytes.Contains(body, f.content[20:30]) {
		t.Fatalf("multi-range: %d %s", res.StatusCode, res.Header.Get("Content-Type"))
	}

	// HEAD: headers only, correct length.
	res, body = f.get(t, "HEAD", "")
	if res.StatusCode != 200 || len(body) != 0 || res.ContentLength != int64(n) {
		t.Fatalf("HEAD: %d %d %d", res.StatusCode, len(body), res.ContentLength)
	}

	// If-Range with a stale ETag sends the whole file.
	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/videos/%d/stream", f.srv.URL, f.id), nil)
	req.Header.Set("Range", "bytes=0-9")
	req.Header.Set("If-Range", `"old"`)
	r2, _ := http.DefaultClient.Do(req)
	r2.Body.Close()
	if r2.StatusCode != 200 {
		t.Fatalf("If-Range mismatch: %d", r2.StatusCode)
	}
}

func TestConcurrentClients(t *testing.T) {
	f := setup(t)
	var wg sync.WaitGroup
	errs := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := i * 100_000
			req, _ := http.NewRequest("GET", fmt.Sprintf("%s/videos/%d/stream", f.srv.URL, f.id), nil)
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+99_999))
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				errs <- err.Error()
				return
			}
			b, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != 206 || !bytes.Equal(b, f.content[start:start+100_000]) {
				errs <- fmt.Sprintf("client %d: %d", i, res.StatusCode)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Error(e)
	}
}

func TestSecurity(t *testing.T) {
	f := setup(t)
	for _, path := range []string{"/videos/0/stream", "/videos/-1/stream", "/videos/abc/stream", "/videos/999/stream"} {
		res, err := http.Get(f.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 400 && res.StatusCode != 404 {
			t.Errorf("%s: %d", path, res.StatusCode)
		}
	}
	// The file replaced by a symlink to a file outside the folder: refused.
	secret := filepath.Join(t.TempDir(), "secret.txt")
	os.WriteFile(secret, []byte("password"), 0o600)
	os.Remove(f.path)
	os.Symlink(secret, f.path)
	res, body := f.get(t, "GET", "")
	if res.StatusCode != 404 || bytes.Contains(body, []byte("password")) {
		t.Fatalf("symlink escape: %d %s", res.StatusCode, body)
	}
	// A deleted file is a clean 404.
	os.Remove(f.path)
	if res, _ := f.get(t, "GET", ""); res.StatusCode != 404 {
		t.Fatalf("deleted file: %d", res.StatusCode)
	}
}

func TestMissingFileAndOfflineFolder(t *testing.T) {
	f := setup(t)
	os.Remove(f.path)
	res, body := f.get(t, "GET", "bytes=0-0")
	if res.StatusCode != 404 || !strings.Contains(string(body), "FILE_MISSING") {
		t.Fatalf("missing file: %d %s", res.StatusCode, body)
	}
	// The whole folder disappears, like an unmounted drive.
	os.RemoveAll(filepath.Dir(f.path))
	res, body = f.get(t, "GET", "bytes=0-0")
	if res.StatusCode != 404 || !strings.Contains(string(body), "FOLDER_OFFLINE") || !strings.Contains(string(body), "external drive") {
		t.Fatalf("offline folder: %d %s", res.StatusCode, body)
	}
}

func TestCheck(t *testing.T) {
	f := setup(t)
	check := func() (int, string) {
		res, err := http.Get(fmt.Sprintf("%s/videos/%d/check", f.srv.URL, f.id))
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, string(b)
	}
	if code, body := check(); code != 200 || !strings.Contains(body, `"readable":true`) {
		t.Fatalf("readable file: %d %s", code, body)
	}
	// A file shorter than the probe size is fine too.
	os.WriteFile(f.path, []byte("tiny"), 0o644)
	if code, body := check(); code != 200 {
		t.Fatalf("tiny file: %d %s", code, body)
	}
	os.Remove(f.path)
	if code, body := check(); code != 404 || !strings.Contains(body, "FILE_MISSING") {
		t.Fatalf("missing file: %d %s", code, body)
	}
}
