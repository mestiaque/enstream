package app

import (
	"context"
	"fmt"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"

	"enstream/internal/config"
	"enstream/internal/handlers"
	"enstream/internal/jobs"
	"enstream/internal/media"
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
	sched := &jobs.Scheduler{
		Scanner: a.Scanner, Store: a.Store, Settings: a.Settings, Log: a.Log,
		LastBackup: a.Backups.LastAuto,
		Backup: func(ctx context.Context) error {
			if _, err := a.Backups.Create(ctx, "auto"); err != nil {
				return err
			}
			return a.Backups.Prune(a.Settings.Get().Backup.Keep)
		},
		Daily: func(ctx context.Context) { _ = a.Store.PurgeExpiredSessions(ctx) },
	}
	go sched.Run(ctx)
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
	settings := &handlers.Settings{Live: a.Settings, Auth: a.Auth, Log: a.Log, Server: func() config.Server { return a.Config.Server }}
	authH := &handlers.Auth{Service: a.Auth, Log: a.Log}
	backups := &handlers.Backups{Manager: a.Backups, Log: a.Log}
	tools := &handlers.Tools{FFmpeg: a.FFmpeg, FFprobe: a.FFprobe, Log: a.Log, Install: a.InstallFFmpeg}
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
		Auth:  a.Auth.Middleware,
		API: func(r chi.Router) {
			r.Get("/health", sys.Health)
			r.Get("/system/info", sys.Info)
			r.Get("/system/ffmpeg", tools.Status)
			r.Get("/system/qr", sys.QR)
			r.With(server.HostOnly).Get("/fs/dirs", handlers.FS{}.Dirs)
			r.With(server.HostOnly).Post("/system/ffmpeg/install", tools.InstallFFmpeg)

			r.Get("/folders", folders.List)
			r.With(server.HostOnly).Post("/folders", folders.Add)
			r.Get("/folders/{id}", folders.Get)
			r.With(server.HostOnly).Delete("/folders/{id}", folders.Delete)

			r.With(server.HostOnly).Post("/scanner/scan", scans.Scan)
			r.Get("/scanner/status", scans.Status)
			r.Get("/scanner/jobs", scans.Jobs)
			r.Get("/scanner/jobs/{id}", scans.Job)
			r.With(server.HostOnly).Post("/scanner/cancel", scans.Cancel)
			r.With(server.HostOnly).Post("/scanner/retry-failed", scans.RetryFailed)

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
			r.With(server.HostOnly).Delete("/history", play.ClearHistory)
			r.Delete("/history/{id}", play.DeleteHistory)
			r.Get("/devices", play.Devices)
			r.Put("/devices/current", play.NameDevice)
			r.With(server.HostOnly).Delete("/devices/{id}", play.DeleteDevice)

			r.With(server.HostOnly).Get("/settings", settings.Get)
			r.With(server.HostOnly).Put("/settings", settings.Update)

			r.Get("/auth/status", authH.Status)
			r.Post("/auth/login", authH.Login)
			r.Post("/auth/logout", authH.Logout)
			r.With(server.HostOnly).Put("/auth/pin", authH.SetPIN)
			r.With(server.HostOnly).Post("/auth/pin/disable", authH.DisablePIN)
			r.With(server.HostOnly).Get("/auth/sessions", authH.Sessions)
			r.With(server.HostOnly).Delete("/auth/sessions/{id}", authH.DeleteSession)
			r.With(server.HostOnly).Post("/auth/sessions/sign-out-others", authH.SignOutOthers)

			r.With(server.HostOnly).Get("/backups", backups.List)
			r.With(server.HostOnly).Post("/backups", backups.Create)
			r.With(server.HostOnly).Delete("/backups/restore", backups.CancelRestore)
			r.With(server.HostOnly).Get("/backups/{name}", backups.Download)
			r.With(server.HostOnly).Delete("/backups/{name}", backups.Delete)
			r.With(server.HostOnly).Post("/backups/{name}/restore", backups.Restore)
		},
		Frontend: front,
	}), nil
}

// InstallFFmpeg downloads FFmpeg into the tools folder and switches to it without a restart.
func (a *App) InstallFFmpeg(ctx context.Context, progress media.Progress) error {
	if err := media.InstallFFmpeg(ctx, a.Paths.Tools, progress); err != nil {
		return err
	}
	cfg := a.Settings.Get()
	a.FFmpeg.Replace(media.FindTool(ctx, "ffmpeg", cfg.Media.FFmpegPath, a.Paths.Tools))
	a.FFprobe.Replace(media.FindTool(ctx, "ffprobe", cfg.Media.FFprobePath, a.Paths.Tools))
	if !a.FFmpeg.Available() || !a.FFprobe.Available() {
		return fmt.Errorf("FFmpeg was downloaded but does not run: %s", a.FFmpeg.Err()+a.FFprobe.Err())
	}
	return nil
}
