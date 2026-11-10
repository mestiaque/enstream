package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Serve runs the HTTP server until ctx is cancelled (Ctrl+C), then shuts down gracefully:
// in-flight requests get up to 10 s, background work is stopped, the database is closed.
func (a *App) Serve(ctx context.Context, handler http.Handler, onStop func()) error {
	addr := net.JoinHostPort(a.Config.Server.Host, fmt.Sprint(a.Config.Server.Port))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("cannot listen on %s (is another program using port %d?): %w", addr, a.Config.Server.Port, err)
	}
	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
		// No WriteTimeout: a video stream legitimately lasts hours.
	}
	fmt.Println(a.Banner())
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case <-ctx.Done():
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
	}
	a.Log.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
	if onStop != nil {
		onStop()
	}
	return a.DB.Close()
}

// Banner is the startup summary printed to the console.
func (a *App) Banner() string {
	status := func(ok bool) string {
		if ok {
			return "AVAILABLE"
		}
		return "MISSING"
	}
	var b strings.Builder
	line := strings.Repeat("=", 44)
	fmt.Fprintf(&b, "\n%s\n        ENSTREAM %s · Your Personal Cinema\n%s\n\n", line, Version, line)
	fmt.Fprintf(&b, "Status:   ONLINE\n\nOn this computer:\n  http://localhost:%d\n\n", a.Config.Server.Port)
	if len(a.Addresses) == 0 {
		b.WriteString("On your network:\n  (no network address found; connect to Wi-Fi or Ethernet)\n\n")
	} else {
		b.WriteString("On your network (phones, TVs, other computers):\n")
		for _, ad := range a.Addresses {
			fmt.Fprintf(&b, "  %-24s (%s)\n", ad.URL, ad.Interface)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Database: READY\nFFmpeg:   %s\nFFprobe:  %s\nData:     %s\n", status(a.FFmpeg.Available()), status(a.FFprobe.Available()), a.Paths.Data)
	if !a.FFmpeg.Available() || !a.FFprobe.Available() {
		b.WriteString("\nWithout FFmpeg, videos play but have no thumbnails or details.\n  Ubuntu:  sudo apt install ffmpeg\n  Windows: winget install Gyan.FFmpeg\n")
	}
	fmt.Fprintf(&b, "\nPress CTRL+C to stop.\n%s\n", line)
	return b.String()
}
