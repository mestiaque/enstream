package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	repo "enstream/internal/database/repositories"
	"enstream/internal/jobs"
	"enstream/internal/security"
	"enstream/internal/server"
)

type Folders struct {
	Store   *repo.Store
	Scanner *jobs.Scanner
	Log     *slog.Logger
}

type folderView struct {
	repo.Folder
	Available bool `json:"available"`
}

func view(f repo.Folder) folderView {
	fi, err := os.Stat(f.Path)
	return folderView{Folder: f, Available: err == nil && fi.IsDir()}
}

func (h *Folders) List(w http.ResponseWriter, r *http.Request) {
	fs, err := h.Store.Folders(r.Context())
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	out := make([]folderView, len(fs))
	for i, f := range fs {
		out[i] = view(f)
	}
	server.OK(w, out)
}

func (h *Folders) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	f, err := h.Store.Folder(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "FOLDER_NOT_FOUND", "The folder was not found")
		return
	}
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OK(w, view(f))
}

// Add registers a folder (and starts scanning it unless another scan runs).
func (h *Folders) Add(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
		Name string `json:"name"`
		Scan *bool  `json:"scan"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send JSON like {\"path\": \"/home/you/Videos\"}")
		return
	}
	path, err := security.CanonicalFolder(body.Path)
	if err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_FOLDER", err.Error())
		return
	}
	existing, err := h.Store.Folders(r.Context())
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	// Nested folders would index the same videos twice.
	for _, f := range existing {
		switch {
		case security.Within(f.Path, path):
			server.Fail(w, http.StatusConflict, "FOLDER_OVERLAPS", "This folder is already included in \""+f.Name+"\"")
			return
		case security.Within(path, f.Path):
			server.Fail(w, http.StatusConflict, "FOLDER_OVERLAPS", "This folder contains \""+f.Name+"\", which is already added; remove that one first")
			return
		}
	}
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = filepath.Base(path)
	}
	if len(name) > 120 {
		name = name[:120]
	}
	id, err := h.Store.AddFolder(r.Context(), path, name)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	h.Log.Info("folder added", "id", id, "name", name)
	f, _ := h.Store.Folder(r.Context(), id)
	msg := "Folder added"
	if body.Scan == nil || *body.Scan {
		if _, err := h.Scanner.Start(jobs.ScanRequest{FolderID: id}); err == nil {
			msg = "Folder added; scanning started"
		} else {
			msg = "Folder added; scan it when the current scan finishes"
		}
	}
	server.Created(w, view(f), msg)
}

// Delete forgets a folder: its index entries and thumbnails are removed. The video files
// themselves are never touched.
func (h *Folders) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if cur := h.Scanner.Current(); cur != nil && (cur.FolderID == nil || *cur.FolderID == id) {
		server.Fail(w, http.StatusConflict, "SCAN_RUNNING", "Wait for the scan to finish (or cancel it) before removing this folder")
		return
	}
	thumbs, _ := h.Store.FolderThumbnails(r.Context(), id)
	if err := h.Store.DeleteFolder(r.Context(), id); err != nil {
		if errors.Is(err, repo.ErrNotFound) {
			server.Fail(w, http.StatusNotFound, "FOLDER_NOT_FOUND", "The folder was not found")
			return
		}
		internal(w, r, h.Log, err)
		return
	}
	for _, t := range thumbs {
		os.Remove(t)
	}
	h.Log.Info("folder removed", "id", id)
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Folder removed from the library; your files were not touched"})
}

// internal logs the real error with the request id and sends a generic message.
func internal(w http.ResponseWriter, r *http.Request, log *slog.Logger, err error) {
	log.Error("request failed", "id", server.RequestID(r), "path", r.URL.Path, "error", err)
	server.Fail(w, http.StatusInternalServerError, "INTERNAL", "Something went wrong (request "+server.RequestID(r)+")")
}
