package handlers

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"

	repo "enstream/internal/database/repositories"
	"enstream/internal/jobs"
	"enstream/internal/playback"
	"enstream/internal/server"
)

type Playback struct {
	Store   *repo.Store
	Service *playback.Service
	Scanner *jobs.Scanner
	Log     *slog.Logger
}

// deviceToken comes from the X-Device-Token header (or the body for sendBeacon).
func deviceToken(r *http.Request, body string) string {
	if t := r.Header.Get("X-Device-Token"); t != "" {
		return t
	}
	return body
}

// Progress: PUT (or POST from navigator.sendBeacon) {"position": 61.5, "duration": 3600}.
func (h *Playback) Progress(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Position    float64 `json:"position"`
		Duration    float64 `json:"duration"`
		DeviceToken string  `json:"device_token"`
	}
	if err := server.DecodeJSON(r, &body); err != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {\"position\": seconds, \"duration\": seconds}")
		return
	}
	done, err := h.Service.Update(r.Context(), id, deviceToken(r, body.DeviceToken), r.UserAgent(), body.Position, body.Duration)
	switch {
	case errors.Is(err, playback.ErrInvalid):
		server.Fail(w, http.StatusBadRequest, "INVALID_PROGRESS", "position and duration must be valid numbers of seconds")
	case errors.Is(err, playback.ErrNotFound):
		server.Fail(w, http.StatusNotFound, "VIDEO_NOT_FOUND", "The requested video was not found")
	case err != nil:
		internal(w, r, h.Log, err)
	default:
		server.OK(w, map[string]bool{"completed": done})
	}
}

func (h *Playback) Completed(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	err := h.Service.MarkCompleted(r.Context(), id, deviceToken(r, ""), r.UserAgent())
	if errors.Is(err, playback.ErrNotFound) {
		server.Fail(w, http.StatusNotFound, "VIDEO_NOT_FOUND", "The requested video was not found")
		return
	}
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Marked as watched"})
}

func (h *Playback) list(w http.ResponseWriter, r *http.Request, inProgress bool) {
	page, err1 := intParam(r, "page", 1, 1, 1_000_000)
	limit, err2 := intParam(r, "limit", 30, 1, maxPageSize)
	if err1 != nil || err2 != nil {
		server.Fail(w, http.StatusBadRequest, "INVALID_PARAMETER", "page must be 1 or more and limit between 1 and 100")
		return
	}
	list, total, err := h.Store.History(r.Context(), inProgress, limit, (page-1)*limit)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OKPage(w, list, server.NewPagination(page, limit, total))
}

func (h *Playback) ContinueWatching(w http.ResponseWriter, r *http.Request) { h.list(w, r, true) }
func (h *Playback) History(w http.ResponseWriter, r *http.Request)          { h.list(w, r, false) }

func (h *Playback) DeleteHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteHistory(r.Context(), id); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Removed from history"})
}

func (h *Playback) ClearHistory(w http.ResponseWriter, r *http.Request) {
	if err := h.Store.ClearHistory(r.Context()); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "History cleared"})
}

// ------------------------------------------------------------------ devices

func (h *Playback) Devices(w http.ResponseWriter, r *http.Request) {
	list, err := h.Store.Devices(r.Context(), deviceToken(r, ""))
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.OK(w, list)
}

// NameDevice: PUT /devices/current {"name": "Living room TV"} (token in X-Device-Token).
func (h *Playback) NameDevice(w http.ResponseWriter, r *http.Request) {
	token := deviceToken(r, "")
	var body struct {
		Name string `json:"name"`
	}
	if err := server.DecodeJSON(r, &body); err != nil || !playback.ValidToken(token) {
		server.Fail(w, http.StatusBadRequest, "INVALID_BODY", "Send {\"name\": \"...\"} with the X-Device-Token header")
		return
	}
	name := strings.TrimSpace(body.Name)
	if len(name) > 60 {
		server.Fail(w, http.StatusBadRequest, "INVALID_NAME", "Use 60 characters or fewer")
		return
	}
	if _, err := h.Store.TouchDevice(r.Context(), token, r.UserAgent()); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	if err := h.Store.RenameDevice(r.Context(), token, name); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Device name saved"})
}

func (h *Playback) DeleteDevice(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r, "id")
	if !ok {
		return
	}
	if err := h.Store.DeleteDevice(r.Context(), id); err != nil {
		internal(w, r, h.Log, err)
		return
	}
	server.WriteJSON(w, http.StatusOK, server.Envelope{Success: true, Message: "Device removed"})
}

// ------------------------------------------------------------------ dashboard

// Dashboard: library totals, recently added, continue watching, recent plays, last scan.
func (h *Playback) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	stats, err := h.Store.Stats(ctx)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	recent, _, err := h.Store.Videos(ctx, repo.VideoQuery{Page: 1, Limit: 12, Sort: "created_at", Desc: true})
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	cont, _, err := h.Store.History(ctx, true, 12, 0)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	played, _, err := h.Store.History(ctx, false, 8, 0)
	if err != nil {
		internal(w, r, h.Log, err)
		return
	}
	var lastScan any
	if cur := h.Scanner.Current(); cur != nil {
		lastScan = cur
	} else if js, err := h.Store.ScanJobs(ctx, 1); err == nil && len(js) > 0 {
		lastScan = js[0]
	}
	server.OK(w, map[string]any{
		"stats": stats, "recently_added": recent, "continue_watching": cont, "recently_played": played,
		"last_scan": lastScan, "scanning": h.Scanner.Current() != nil,
	})
}
