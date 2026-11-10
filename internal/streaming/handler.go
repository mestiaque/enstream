// Package streaming delivers original video files to browsers with HTTP range support.
package streaming

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/go-chi/chi/v5"

	repo "enstream/internal/database/repositories"
	"enstream/internal/security"
	"enstream/internal/server"
)

// Handler serves GET and HEAD /videos/{id}/stream.
//
// http.ServeContent implements the HTTP semantics browsers rely on for seeking: Range
// (single, open-ended, suffix and multiple ranges as multipart/byteranges), 206 Partial
// Content, 416 Range Not Satisfiable, If-Range with the ETag below, HEAD, and
// Content-Length. The file is read in chunks straight from disk; nothing is buffered whole.
// When a client disconnects the copy stops and the file is closed.
type Handler struct {
	Store *repo.Store
	Log   *slog.Logger
}

func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		server.Fail(w, http.StatusBadRequest, "INVALID_ID", "The id must be a positive number")
		return
	}
	vf, err := h.Store.VideoFile(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "VIDEO_NOT_FOUND", "The requested video was not found")
		return
	}
	if err != nil {
		h.Log.Error("stream lookup", "id", id, "error", err)
		server.Fail(w, http.StatusInternalServerError, "INTERNAL", "Something went wrong")
		return
	}
	// The stored path is checked again on every request: the file may have been replaced
	// by a symlink pointing outside the folder since it was scanned.
	real, err := security.ResolveInside(vf.Path, vf.FolderPath)
	if err != nil {
		if errors.Is(err, security.ErrOutsideRoot) {
			h.Log.Warn("blocked file outside its media folder", "video", id)
		}
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available; rescan the library")
		return
	}
	f, err := os.Open(real)
	if err != nil {
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available; rescan the library")
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available")
		return
	}
	hd := w.Header()
	hd.Set("Content-Type", vf.MimeType) // set first so ServeContent does not sniff
	hd.Set("Accept-Ranges", "bytes")
	hd.Set("Cache-Control", "private, no-transform")
	hd.Set("Content-Disposition", "inline")
	hd.Set("ETag", fmt.Sprintf(`"%x-%x"`, fi.Size(), fi.ModTime().UnixNano()))
	http.ServeContent(w, r, filepath.Base(real), fi.ModTime(), f)
}
