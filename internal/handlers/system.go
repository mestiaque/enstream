// Package handlers turns HTTP requests into service calls and service results into JSON.
package handlers

import (
	qrcode "github.com/skip2/go-qrcode"

	"context"
	"database/sql"
	"net/http"
	"runtime"
	"time"

	"enstream/internal/config"
	"enstream/internal/database"
	"enstream/internal/media"
	"enstream/internal/network"
	"enstream/internal/server"
)

type System struct {
	DB        *sql.DB
	Config    *config.Config
	FFmpeg    *media.Tool
	FFprobe   *media.Tool
	Addresses func(refresh bool) []network.Address
	StartedAt time.Time
	Version   string
}

func (s *System) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.DB.PingContext(ctx); err != nil {
		server.Fail(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "The database is not responding")
		return
	}
	server.OK(w, map[string]any{"status": "ok"})
}

type toolInfo struct {
	Available bool   `json:"available"`
	Version   string `json:"version,omitempty"`
	Message   string `json:"message,omitempty"`
}

func tool(t *media.Tool) toolInfo {
	// The executable path stays on the server: it is an internal detail.
	return toolInfo{Available: t.Available(), Version: t.Version(), Message: t.Err()}
}

// Info: version, uptime, dependencies and the URLs to open on other devices.
func (s *System) Info(w http.ResponseWriter, r *http.Request) {
	v, _ := database.SchemaVersion(r.Context(), s.DB)
	server.OK(w, map[string]any{
		"name":           "ENStream",
		"version":        s.Version,
		"go_version":     runtime.Version(),
		"platform":       runtime.GOOS + "/" + runtime.GOARCH,
		"uptime_seconds": int(time.Since(s.StartedAt).Seconds()),
		"started_at":     s.StartedAt.Unix(),
		"port":           s.Config.Server.Port,
		"addresses":      s.Addresses(r.URL.Query().Get("refresh") == "1"),
		"schema_version": v,
		"ffmpeg":         tool(s.FFmpeg),
		"ffprobe":        tool(s.FFprobe),
	})
}

// QR: a PNG QR code of one of this server's own addresses (for phones to scan).
func (s *System) QR(w http.ResponseWriter, r *http.Request) {
	u := r.URL.Query().Get("url")
	ok := false
	for _, a := range s.Addresses(false) {
		if a.URL == u {
			ok = true
		}
	}
	if !ok {
		server.Fail(w, http.StatusBadRequest, "INVALID_URL", "Only this server's own addresses can be shown as a QR code")
		return
	}
	png, err := qrcode.Encode(u, qrcode.Medium, 320)
	if err != nil {
		server.Fail(w, http.StatusInternalServerError, "QR_FAILED", "Could not make the QR code")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "private, max-age=3600")
	w.Write(png)
}
