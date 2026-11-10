package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"enstream/internal/config"
	"enstream/internal/database"
	repo "enstream/internal/database/repositories"
	"enstream/internal/jobs"
	"enstream/internal/media"
	"enstream/internal/network"
)

// Options come from the command line.
type Options struct {
	DataDir string
	Host    string // overrides config when set
	Port    int    // overrides config when > 0
	Debug   bool
}

// Bootstrap loads configuration, opens the database, finds FFmpeg and the LAN addresses.
func Bootstrap(ctx context.Context, o Options) (*App, error) {
	dataDir := o.DataDir
	if dataDir == "" {
		dataDir = config.DataDir()
	}
	paths, err := config.NewPaths(dataDir)
	if err != nil {
		return nil, fmt.Errorf("create data directory %s: %w", dataDir, err)
	}
	log := newLogger(paths, o.Debug)

	cfg, err := config.Load(paths.Config())
	if err != nil {
		return nil, err
	}
	if o.Host != "" {
		cfg.Server.Host = o.Host
	}
	if o.Port > 0 {
		cfg.Server.Port = o.Port
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	db, err := database.Open(paths.Database())
	if err != nil {
		return nil, err
	}
	a := &App{Config: cfg, Paths: paths, DB: db, Log: log, StartedAt: time.Now()}
	a.FFmpeg = media.FindTool(ctx, "ffmpeg", cfg.Media.FFmpegPath)
	a.FFprobe = media.FindTool(ctx, "ffprobe", cfg.Media.FFprobePath)
	for _, t := range []*media.Tool{a.FFmpeg, a.FFprobe} {
		if !t.Available() {
			log.Warn("dependency missing", "tool", t.Name, "error", t.Error)
		}
	}
	a.Store = repo.New(db)
	if err := a.Store.MarkInterruptedScans(ctx); err != nil {
		log.Warn("could not close interrupted scans", "error", err)
	}
	a.Scanner = jobs.NewScanner(a.Store, cfg, paths, a.FFmpeg, a.FFprobe, log)
	a.RefreshAddresses()
	log.Info("ENStream starting", "version", Version, "data", paths.Data)
	return a, nil
}

// RefreshAddresses re-reads the network interfaces (Wi-Fi may have changed).
func (a *App) RefreshAddresses() []network.Address {
	addrs, err := network.Discover(a.Config.Server.Port)
	if err != nil {
		a.Log.Warn("network discovery failed", "error", err)
	}
	a.Addresses = addrs
	return addrs
}

// newLogger writes structured logs to the console (text) and to logs/enstream.log (JSON).
func newLogger(p config.Paths, debug bool) *slog.Logger {
	level := slog.LevelInfo
	if debug {
		level = slog.LevelDebug
	}
	var w io.Writer = os.Stderr
	if f, err := os.OpenFile(filepath.Join(p.Logs, "enstream.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600); err == nil {
		return slog.New(fanout{
			slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}),
			slog.NewJSONHandler(f, &slog.HandlerOptions{Level: level}),
		})
	}
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: level}))
}

// fanout sends each record to several handlers.
type fanout []slog.Handler

func (f fanout) Enabled(ctx context.Context, l slog.Level) bool {
	for _, h := range f {
		if h.Enabled(ctx, l) {
			return true
		}
	}
	return false
}

func (f fanout) Handle(ctx context.Context, r slog.Record) error {
	for _, h := range f {
		if h.Enabled(ctx, r.Level) {
			_ = h.Handle(ctx, r.Clone())
		}
	}
	return nil
}

func (f fanout) WithAttrs(as []slog.Attr) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithAttrs(as)
	}
	return out
}

func (f fanout) WithGroup(n string) slog.Handler {
	out := make(fanout, len(f))
	for i, h := range f {
		out[i] = h.WithGroup(n)
	}
	return out
}
