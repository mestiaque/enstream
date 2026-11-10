package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	repo "enstream/internal/database/repositories"
	"enstream/internal/jobs"
	"enstream/internal/server"
)

type Scanner struct {
	Store   *repo.Store
	Scanner *jobs.Scanner
	Log     *slog.Logger
}

func (h *Scanner) start(w http.ResponseWriter, r *http.Request, req jobs.ScanRequest) {
	id, err := h.Scanner.Start(req)
	switch {
	case errors.Is(err, jobs.ErrBusy):
		server.Fail(w, http.StatusConflict, "SCAN_RUNNING", "A scan is already running")
	case errors.Is(err, repo.ErrNotFound):
		server.Fail(w, http.StatusNotFound, "FOLDER_NOT_FOUND", "The folder was not found")
	case err != nil:
		internal(w, r, h.Log, err)
	default:
		server.WriteJSON(w, http.StatusAccepted, server.Envelope{Success: true, Data: map[string]int64{"job_id": id}, Message: "Scan started"})
	}
}

// Scan: {"folder_id": 3} for one folder, {} for all.
func (h *Scanner) Scan(w http.ResponseWriter, r *http.Request) {
	var body struct {
		FolderID int64 `json:"folder_id"`
	}
	if r.ContentLength != 0 {
		if err := server.DecodeJSON(r, &body); err != nil || body.FolderID < 0 {
			server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {} or {\"folder_id\": 1}")
			return
		}
	}
	h.start(w, r, jobs.ScanRequest{FolderID: body.FolderID})
}

func (h *Scanner) RetryFailed(w http.ResponseWriter, r *http.Request) {
	h.start(w, r, jobs.ScanRequest{RetryFailed: true})
}

func (h *Scanner) Cancel(w http.ResponseWriter, r *http.Request) {
	if !h.Scanner.Cancel() {
		server.Fail(w, http.StatusConflict, "NO_SCAN", "No scan is running")
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Cancelling the scan"})
}

// Status: the running scan (live counters) or the last finished one.
func (h *Scanner) Status(w http.ResponseWriter, r *http.Request) {
	failed, _ := h.Store.FailureCount(r.Context())
	if cur := h.Scanner.Current(); cur != nil {
		server.OK(w, map[string]any{"running": true, "job": cur, "failed_items": failed})
		return
	}
	jobsList, err := h.Store.ScanJobs(r.Context(), 1)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	var last any
	if len(jobsList) > 0 {
		last = jobsList[0]
	}
	server.OK(w, map[string]any{"running": false, "job": last, "failed_items": failed})
}

func (h *Scanner) Jobs(w http.ResponseWriter, r *http.Request) {
	limit, err := intParam(r, "limit", 20, 1, 100)
	if err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_PARAMETER", "limit must be between 1 and 100")
		return
	}
	list, err := h.Store.ScanJobs(r.Context(), limit)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	if cur := h.Scanner.Current(); cur != nil && len(list) > 0 && list[0].ID == cur.ID {
		list[0] = *cur
	}
	server.OK(w, list)
}

func (h *Scanner) Job(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if cur := h.Scanner.Current(); cur != nil && cur.ID == id {
		server.OK(w, cur)
		return
	}
	j, err := h.Store.ScanJob(r.Context(), id)
	if errors.Is(err, repo.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "JOB_NOT_FOUND", "The scan job was not found")
		return
	}
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OK(w, j)
}
