package app

import (
	"context"
	"net/http"
	"net/netip"

	"github.com/go-chi/chi/v5"

	"enstream/internal/handlers"
	"enstream/internal/network"
	"enstream/internal/security"
	"enstream/internal/server"
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
	return a.Serve(ctx, h, nil)
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
		},
		Frontend: front,
	}), nil
}
