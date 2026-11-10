package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"enstream/internal/media"
	"enstream/internal/server"
)

// Tools installs FFmpeg in the background (Windows) and reports progress.
type Tools struct {
	FFmpeg, FFprobe *media.Tool
	// Install downloads FFmpeg and re-detects both tools.
	Install func(ctx context.Context, progress media.Progress) error
	Log     *slog.Logger

	mu          sync.Mutex
	running     bool
	done, total int64
	lastErr     string
}

func (h *Tools) state() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]any{
		"ffmpeg":      tool(h.FFmpeg),
		"ffprobe":     tool(h.FFprobe),
		"can_install": media.CanInstallFFmpeg(),
		"hint":        media.InstallHint(),
		"installing":  h.running,
		"done":        h.done,
		"total":       h.total,
		"error":       h.lastErr,
	}
}

func (h *Tools) Status(w http.ResponseWriter, r *http.Request) { server.OK(w, h.state()) }

func (h *Tools) InstallFFmpeg(w http.ResponseWriter, r *http.Request) {
	if !media.CanInstallFFmpeg() {
		server.Fail(w, http.StatusBadRequest, "NOT_SUPPORTED", "ENStream can't install FFmpeg on this system. "+media.InstallHint())
		return
	}
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		server.Fail(w, http.StatusConflict, "INSTALL_RUNNING", "FFmpeg is already being installed")
		return
	}
	h.running, h.done, h.total, h.lastErr = true, 0, 0, ""
	h.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()
		h.Log.Info("installing FFmpeg")
		err := h.Install(ctx, func(done, total int64) {
			h.mu.Lock()
			h.done, h.total = done, total
			h.mu.Unlock()
		})
		h.mu.Lock()
		h.running = false
		if err != nil {
			h.lastErr = err.Error()
		}
		h.mu.Unlock()
		if err != nil {
			h.Log.Warn("FFmpeg install failed", "error", err)
		} else {
			h.Log.Info("FFmpeg installed", "version", h.FFmpeg.Version())
		}
	}()
	server.WriteJSON(w, http.StatusAccepted, server.Envelope{Success: true, Data: h.state(), Message: "Installing FFmpeg"})
}
