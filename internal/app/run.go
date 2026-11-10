package app

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"

	"enstream/internal/handlers"
	"enstream/internal/network"
	"enstream/internal/playback"
	"enstream/internal/security"
	"enstream/internal/server"
	"enstream/internal/streaming"
	"enstream/web"
)

// Run bootstraps ENStream and serves until ctx ends.
func Run(ctx context.Context, o Options) error {
	a, err := Bootstrap(ctx, o)
	if err != nil {
		return err
	}
	h, err := a.Handler()
	if err != nil {
		a.DB.Close()
		return err
	}
	return a.Serve(ctx, h, a.Scanner.Shutdown)
}

// Handler builds the full HTTP handler (also used by integration tests).
func (a *App) Handler() (http.Handler, error) {
	sys := &handlers.System{
		DB: a.DB, Config: a.Config, FFmpeg: a.FFmpeg, FFprobe: a.FFprobe, StartedAt: a.StartedAt, Version: Version,
		Addresses: func(refresh bool) []network.Address {
			if refresh {
				return a.RefreshAddresses()
			}
			return a.Addresses
		},
	}
	folders := &handlers.Folders{Store: a.Store, Scanner: a.Scanner, Log: a.Log}
	scans := &handlers.Scanner{Store: a.Store, Scanner: a.Scanner, Log: a.Log}
	videos := &handlers.Videos{Store: a.Store, Paths: a.Paths, FFmpeg: a.FFmpeg, Log: a.Log}
	stream := &streaming.Handler{Store: a.Store, Log: a.Log}
	play := &handlers.Playback{Store: a.Store, Service: &playback.Service{Store: a.Store}, Scanner: a.Scanner, Log: a.Log}
	front, err := server.Frontend(web.Files, Version)
	if err != nil {
		return nil, err
	}
	guard := security.NewNetworkGuard(a.Config.AllowedPrefixes(), func(r *http.Request, ip netip.Addr) {
		a.Log.Warn("connection refused: not an allowed network", "remote", r.RemoteAddr, "path", r.URL.Path)
	})
	return server.NewRouter(server.RouterOptions{
		Log:   a.Log,
		Guard: guard.Middleware,
		API: func(r chi.Router) {
			r.Get("/health", sys.Health)
			r.Get("/system/info", sys.Info)

			r.Get("/folders", folders.List)
			r.Post("/folders", folders.Add)
			r.Get("/folders/{id}", folders.Get)
			r.Delete("/folders/{id}", folders.Delete)

			r.Post("/scanner/scan", scans.Scan)
			r.Get("/scanner/status", scans.Status)
			r.Get("/scanner/jobs", scans.Jobs)
			r.Get("/scanner/jobs/{id}", scans.Job)
			r.Post("/scanner/cancel", scans.Cancel)
			r.Post("/scanner/retry-failed", scans.RetryFailed)

			r.Get("/videos", videos.List)
			r.Get("/videos/extensions", videos.Extensions)
			r.Get("/videos/{id}", videos.Get)
			r.Get("/videos/{id}/thumbnail", videos.Thumbnail)
			r.Get("/videos/{id}/stream", stream.Stream)
			r.Head("/videos/{id}/stream", stream.Stream)
			r.Get("/videos/{id}/subtitles/{sid}", videos.Subtitle)
			r.Put("/videos/{id}/progress", play.Progress)
			r.Post("/videos/{id}/progress", play.Progress) // navigator.sendBeacon
			r.Post("/videos/{id}/completed", play.Completed)

			r.Get("/dashboard", play.Dashboard)
			r.Get("/continue-watching", play.ContinueWatching)
			r.Get("/history", play.History)
			r.Delete("/history", play.ClearHistory)
			r.Delete("/history/{id}", play.DeleteHistory)
			r.Get("/devices", play.Devices)
			r.Put("/devices/current", play.NameDevice)
			r.Delete("/devices/{id}", play.DeleteDevice)
		},
		Frontend: front,
	}), nil
}
