package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"enstream/internal/backup"
	"enstream/internal/server"
)

type Backups struct {
	Manager *backup.Manager
	Log     *slog.Logger
}

func (h *Backups) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.Manager.List()
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	var pending any
	if p := h.Manager.Pending(); p != "" {
		pending = p
	}
	server.OK(w, map[string]any{"backups": list, "pending_restore": pending})
}

func (h *Backups) Create(w http.ResponseWriter, r *http.Request) {
	info, err := h.Manager.Create(r.Context(), "manual")
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	h.Log.Info("backup created", "name", info.Name)
	server.Created(w, info, "Backup created")
}

func (h *Backups) Download(w http.ResponseWriter, r *http.Request) {
	p, err := h.Manager.Path(chi.URLParam(r, "name"))
	if err != nil {
		server.Fail(w, http.StatusNotFound, "BACKUP_NOT_FOUND", "The backup was not found")
		return
	}
	f, err := os.Open(p)
	if err != nil {
		server.Fail(w, http.StatusNotFound, "BACKUP_NOT_FOUND", "The backup was not found")
		return
	}
	defer f.Close()
	fi, _ := f.Stat()
	w.Header().Set("Content-Type", "application/vnd.sqlite3")
	w.Header().Set("Content-Disposition", `attachment; filename="`+fi.Name()+`"`)
	http.ServeContent(w, r, fi.Name(), fi.ModTime(), f)
}

func (h *Backups) Delete(w http.ResponseWriter, r *http.Request) {
	if err := h.Manager.Delete(chi.URLParam(r, "name")); err != nil {
		server.Fail(w, http.StatusNotFound, "BACKUP_NOT_FOUND", "The backup was not found")
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Backup deleted"})
}

func (h *Backups) Restore(w http.ResponseWriter, r *http.Request) {
	name := chi.URLParam(r, "name")
	err := h.Manager.Stage(r.Context(), name)
	switch {
	case errors.Is(err, backup.ErrNotFound):
		server.Fail(w, http.StatusNotFound, "BACKUP_NOT_FOUND", "The backup was not found")
	case errors.Is(err, backup.ErrInvalid):
		server.Fail(w, http.StatusUnprocessableEntity, "BACKUP_INVALID", err.Error())
	case err != nil:
		server.Fail(w, http.StatusUnprocessableEntity, "BACKUP_INVALID", err.Error())
	default:
		h.Log.Info("restore staged", "name", name)
		server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Restart ENStream to finish restoring. The current library is kept as a backup first."})
	}
}

func (h *Backups) CancelRestore(w http.ResponseWriter, r *http.Request) {
	if err := h.Manager.CancelRestore(); err != nil {
		server.Fail(w, http.StatusNotFound, "NO_PENDING_RESTORE", "No restore is waiting")
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Restore cancelled"})
}
