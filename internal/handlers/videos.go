package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"enstream/internal/config"
	repo "enstream/internal/database/repositories"
	"enstream/internal/security"
	"enstream/internal/server"
)

type Videos struct {
	Store *repo.Store
	Paths config.Paths
	Log   *slog.Logger
}

const maxPageSize = 100

func (h *Videos) List(w http.ResponseWriter, r *http.Request) {
	bad := func(msg string) { server.Fail(w, http.StatusBadRequest, "INVALID_PARAMETER", msg) }
	q := repo.VideoQuery{Search: r.URL.Query().Get("search")}
	var err error
	if q.Page, err = intParam(r, "page", 1, 1, 1_000_000); err != nil {
		bad("page must be 1 or more")
		return
	}
	if q.Limit, err = intParam(r, "limit", 30, 1, maxPageSize); err != nil {
		bad("limit must be between 1 and 100")
		return
	}
	folder, err := intParam(r, "folder_id", 0, 0, 1<<31)
	if err != nil {
		bad("folder_id must be a positive number")
		return
	}
	q.FolderID = int64(folder)
	if len(q.Search) > 200 {
		bad("search is too long")
		return
	}
	if ext := strings.ToLower(r.URL.Query().Get("extension")); ext != "" {
		if !strings.HasPrefix(ext, ".") {
			ext = "." + ext
		}
		if len(ext) > 8 || strings.ContainsAny(ext[1:], "./\\ ") {
			bad("extension must look like mp4")
			return
		}
		q.Extension = ext
	}
	switch res := r.URL.Query().Get("resolution"); res {
	case "", "4k", "1080p", "720p", "sd":
		q.Resolution = res
	default:
		bad("resolution must be 4k, 1080p, 720p or sd")
		return
	}
	switch wtc := r.URL.Query().Get("watched"); wtc {
	case "", "watched", "unwatched", "in_progress":
		q.Watched = wtc
	default:
		bad("watched must be watched, unwatched or in_progress")
		return
	}
	if q.RecentDays, err = intParam(r, "recently_added", 0, 0, 3650); err != nil {
		bad("recently_added is a number of days (0-3650)")
		return
	}
	q.Sort = r.URL.Query().Get("sort")
	if q.Sort == "" {
		q.Sort = "title"
	}
	if !repo.ValidSort(q.Sort) {
		bad("sort must be title, created_at, updated_at, file_size, duration or last_played_at")
		return
	}
	switch o := r.URL.Query().Get("order"); o {
	case "", "asc":
		q.Desc = false
	case "desc":
		q.Desc = true
	default:
		bad("order must be asc or desc")
		return
	}
	vids, total, err := h.Store.Videos(r.Context(), q)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OKPage(w, vids, server.NewPagination(q.Page, q.Limit, total))
}

func (h *Videos) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	v, err := h.Store.Video(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "VIDEO_NOT_FOUND", "The requested video was not found")
		return
	}
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	subs, err := h.Store.Subtitles(r.Context(), id)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OK(w, map[string]any{"video": v, "subtitles": subs})
}

// Thumbnail serves the generated WebP. The stored path must lie in the thumbnails folder.
func (h *Videos) Thumbnail(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := h.Store.VideoFile(r.Context(), id)
	if err != nil || f.ThumbnailPath == "" {
		http.NotFound(w, r)
		return
	}
	real, err := security.ResolveInside(f.ThumbnailPath, h.Paths.Thumbnails)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	file, err := os.Open(real)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer file.Close()
	fi, err := file.Stat()
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/webp")
	w.Header().Set("Cache-Control", "private, max-age=86400")
	http.ServeContent(w, r, filepath.Base(real), fi.ModTime(), file)
}

// Extensions: formats present in the library, for the filter menu.
func (h *Videos) Extensions(w http.ResponseWriter, r *http.Request) {
	exts, err := h.Store.Extensions(r.Context())
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	if exts == nil {
		exts = []string{}
	}
	server.OK(w, exts)
}
