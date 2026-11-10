package server

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// Frontend serves the embedded UI: files under /static/ (cached for a year when the URL
// carries ?v=<version>), and index.html for every other path so client-side routes work.
// "{{VERSION}}" in index.html is replaced with the version, giving versioned asset URLs.
func Frontend(files fs.FS, version string) (http.Handler, error) {
	static, err := fs.Sub(files, "static")
	if err != nil {
		return nil, err
	}
	raw, err := fs.ReadFile(files, "templates/index.html")
	if err != nil {
		return nil, err
	}
	index := bytes.ReplaceAll(raw, []byte("{{VERSION}}"), []byte(version))
	fileServer := http.StripPrefix("/static/", http.FileServer(http.FS(static)))
	started := time.Now()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			if strings.HasSuffix(r.URL.Path, "/") {
				http.NotFound(w, r) // no directory listings
				return
			}
			if r.URL.Query().Get("v") != "" {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			} else {
				w.Header().Set("Cache-Control", "no-cache")
			}
			fileServer.ServeHTTP(w, r)
			return
		}
		if path.Ext(r.URL.Path) != "" && r.URL.Path != "/" {
			http.NotFound(w, r) // a missing asset, not a page
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeContent(w, r, "index.html", started, bytes.NewReader(index))
	}), nil
}
