// Package app wires ENStream together: configuration, database, tools, services and the
// HTTP server, and runs them until the process is asked to stop.
package app

import (
	"database/sql"
	"log/slog"
	"time"

	"enstream/internal/auth"
	"enstream/internal/backup"
	"enstream/internal/config"
	repo "enstream/internal/database/repositories"
	"enstream/internal/jobs"
	"enstream/internal/media"
	"enstream/internal/network"
)

// Version is set at build time: -ldflags "-X enstream/internal/app.Version=1.2.3".
var Version = "0.1.0-dev"

type App struct {
	Config    *config.Config // startup values (server, networks, tool paths)
	Settings  *config.Live   // values the settings page can change
	Paths     config.Paths
	DB        *sql.DB
	Log       *slog.Logger
	FFmpeg    *media.Tool
	FFprobe   *media.Tool
	Addresses []network.Address
	StartedAt time.Time
	Store     *repo.Store
	Scanner   *jobs.Scanner
	Auth      *auth.Service
	Desktop   bool // started by the desktop app (see Options.Desktop)
	Backups   *backup.Manager
}
