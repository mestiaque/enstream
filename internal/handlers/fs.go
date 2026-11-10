package handlers

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"enstream/internal/server"
)

// FS lets the folder picker browse folders on the ENStream computer. Only folder names are
// returned (never files), and hidden folders are left out.
type FS struct{}

type dirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
	Kind string `json:"kind,omitempty"` // places only: home | videos | drive | removable
}

const maxDirEntries = 1000

func isDir(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// places: good starting points (home, Videos, drives, USB disks).
func places() []dirEntry {
	out := []dirEntry{}
	add := func(name, p, kind string) {
		if p != "" && isDir(p) && !slices.ContainsFunc(out, func(d dirEntry) bool { return d.Path == p }) {
			out = append(out, dirEntry{Name: name, Path: p, Kind: kind})
		}
	}
	home, _ := os.UserHomeDir()
	for _, n := range []string{"Videos", "Movies", "Downloads", "Desktop"} {
		add(n, filepath.Join(home, n), "videos")
	}
	add("Home", home, "home")
	if runtime.GOOS == "windows" {
		for c := 'C'; c <= 'Z'; c++ {
			d := string(c) + `:\`
			add(string(c)+":", d, "drive")
		}
		return out
	}
	user := filepath.Base(home)
	for _, base := range []string{filepath.Join("/media", user), filepath.Join("/run/media", user), "/media", "/mnt", "/Volumes"} {
		entries, _ := os.ReadDir(base)
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") && e.Name() != user {
				add(e.Name(), filepath.Join(base, e.Name()), "removable")
			}
		}
	}
	add("Computer (/)", "/", "drive")
	return out
}

// Dirs: GET /fs/dirs (places) or /fs/dirs?path=/abs/folder (its subfolders).
func (FS) Dirs(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Query().Get("path")
	if p == "" {
		server.OK(w, map[string]any{"path": "", "parent": nil, "dirs": places()})
		return
	}
	if len(p) > 4096 || !filepath.IsAbs(p) {
		server.Fail(w, http.StatusBadRequest, "INVALID_PATH", "Use a full path")
		return
	}
	p = filepath.Clean(p)
	entries, err := os.ReadDir(p)
	if err != nil {
		server.Fail(w, http.StatusNotFound, "FOLDER_UNREADABLE", "This folder can't be opened")
		return
	}
	dirs := []dirEntry{}
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "$") {
			continue
		}
		full := filepath.Join(p, name)
		if e.IsDir() || (e.Type()&os.ModeSymlink != 0 && isDir(full)) {
			dirs = append(dirs, dirEntry{Name: name, Path: full})
			if len(dirs) >= maxDirEntries {
				break
			}
		}
	}
	slices.SortFunc(dirs, func(a, b dirEntry) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	var parent any
	if up := filepath.Dir(p); up != p {
		parent = up
	}
	server.OK(w, map[string]any{"path": p, "parent": parent, "dirs": dirs})
}
