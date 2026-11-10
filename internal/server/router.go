package server

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// RouterOptions: everything the router mounts.
type RouterOptions struct {
	Log      *slog.Logger
	Guard    func(http.Handler) http.Handler // network access control, applied to everything
	Auth     func(http.Handler) http.Handler // optional PIN check for the API
	API      func(r chi.Router)              // mounted under /api/v1
	Frontend http.Handler                    // embedded UI
}

func NewRouter(o RouterOptions) http.Handler {
	r := chi.NewRouter()
	r.Use(Recover(o.Log), Logging(o.Log))
	if o.Guard != nil {
		r.Use(o.Guard)
	}
	r.Use(SecurityHeaders)
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(MaxBody(1<<20), SameOrigin)
		if o.Auth != nil {
			api.Use(o.Auth)
		}
		api.NotFound(func(w http.ResponseWriter, _ *http.Request) {
			Fail(w, http.StatusNotFound, "NOT_FOUND", "No such API endpoint")
		})
		api.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
			Fail(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "This method is not allowed here")
		})
		o.API(api)
	})
	if o.Frontend != nil {
		r.Handle("/*", o.Frontend)
	}
	return r
}
