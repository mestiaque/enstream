package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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
	// Asset URLs carry a hash of the files, not just the version: a rebuilt copy with the
	// same version number must never be served from a browser's year-long cache.
	tag, err := assetTag(static, version)
	if err != nil {
		return nil, err
	}
	index := bytes.ReplaceAll(raw, []byte("{{VERSION}}"), []byte(tag))
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

// assetTag is "<version>-<first 10 hex digits of a SHA-256 over every static file>".
func assetTag(static fs.FS, version string) (string, error) {
	h := sha256.New()
	err := fs.WalkDir(static, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(static, p)
		if err != nil {
			return err
		}
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(b)
		return nil
	})
	if err != nil {
		return "", err
	}
	return version + "-" + hex.EncodeToString(h.Sum(nil))[:10], nil
}
