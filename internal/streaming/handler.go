// Package streaming delivers original video files to browsers with HTTP range support.
package streaming

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

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

// open finds the video's file and answers the error itself when it can't.
func (h *Handler) open(w http.ResponseWriter, r *http.Request) (*os.File, os.FileInfo, string, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		server.Fail(w, http.StatusBadRequest, "INVALID_ID", "The id must be a positive number")
		return nil, nil, "", false
	}
	vf, err := h.Store.VideoFile(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "VIDEO_NOT_FOUND", "The requested video was not found")
		return nil, nil, "", false
	}
	if err != nil {
		h.Log.Error("stream lookup", "id", id, "error", err)
		server.Fail(w, http.StatusInternalServerError, "INTERNAL", "Something went wrong")
		return nil, nil, "", false
	}
	// The whole media folder is gone (most often an external drive that is not mounted):
	// say so, instead of "rescan the library", which would empty it.
	if st, err := os.Stat(vf.FolderPath); err != nil || !st.IsDir() {
		server.Fail(w, http.StatusNotFound, "FOLDER_OFFLINE",
			fmt.Sprintf("The folder %q is not reachable. If it is on an external drive, connect and open the drive, then try again.", filepath.Base(vf.FolderPath)))
		return nil, nil, "", false
	}
	// The stored path is checked again on every request: the file may have been replaced
	// by a symlink pointing outside the folder since it was scanned.
	real, err := security.ResolveInside(vf.Path, vf.FolderPath)
	if err != nil {
		if errors.Is(err, security.ErrOutsideRoot) {
			h.Log.Warn("blocked file outside its media folder", "video", id)
		}
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available; rescan the library")
		return nil, nil, "", false
	}
	f, err := os.Open(real)
	if err != nil {
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available; rescan the library")
		return nil, nil, "", false
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() {
		f.Close()
		server.Fail(w, http.StatusNotFound, "FILE_MISSING", "The video file is no longer available")
		return nil, nil, "", false
	}
	return f, fi, vf.MimeType, true
}

func (h *Handler) Stream(w http.ResponseWriter, r *http.Request) {
	f, fi, mime, ok := h.open(w, r)
	if !ok {
		return
	}
	defer f.Close()
	hd := w.Header()
	hd.Set("Content-Type", mime) // set first so ServeContent does not sniff
	hd.Set("Accept-Ranges", "bytes")
	hd.Set("Cache-Control", "private, no-transform")
	hd.Set("Content-Disposition", "inline")
	hd.Set("ETag", fmt.Sprintf(`"%x-%x"`, fi.Size(), fi.ModTime().UnixNano()))
	http.ServeContent(w, r, filepath.Base(f.Name()), fi.ModTime(), f)
}

// Check answers whether the file can actually be read: a little from the start, the middle
// and the end, where MP4 files often keep their index. The player asks when the browser
// reports an error, because a browser can't tell "unsupported format" from "the drive
// stopped answering" and would otherwise blame the format.
func (h *Handler) Check(w http.ResponseWriter, r *http.Request) {
	f, fi, _, ok := h.open(w, r)
	if !ok {
		return
	}
	done := make(chan error, 1)
	go func() {
		defer f.Close()
		buf := make([]byte, 64<<10)
		size := fi.Size()
		for _, at := range []int64{0, size / 2, size - int64(len(buf))} {
			if at < 0 {
				at = 0
			}
			if _, err := f.ReadAt(buf, at); err != nil && !errors.Is(err, io.EOF) {
				done <- err
				return
			}
		}
		done <- nil
	}()
	select {
	case err := <-done:
		if err != nil {
			h.Log.Warn("video file unreadable", "path", f.Name(), "error", err)
			server.Fail(w, http.StatusServiceUnavailable, "READ_ERROR", "The drive could not read this file. It may have been disconnected or be failing: reconnect it and try again.")
			return
		}
		server.OK(w, map[string]any{"readable": true})
	case <-time.After(8 * time.Second):
		server.Fail(w, http.StatusServiceUnavailable, "READ_ERROR", "The drive is not responding. It may have been disconnected: reconnect it and try again.")
	}
}
