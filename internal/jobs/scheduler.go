package jobs

import (
	"context"
	"log/slog"
	"time"

	"enstream/internal/config"
	repo "enstream/internal/database/repositories"
)

// Scheduler runs periodic work: automatic library scans, daily database backups and
// housekeeping. It checks once a minute and never starts a scan while one is running.
type Scheduler struct {
	Scanner  *Scanner
	Store    *repo.Store
	Settings *config.Live
	Log      *slog.Logger
	// Optional hooks.
	LastBackup func() time.Time
	Backup     func(ctx context.Context) error
	Daily      func(ctx context.Context)

	Tick time.Duration // default one minute
	now  func() time.Time
}

func (s *Scheduler) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// Run blocks until ctx ends.
func (s *Scheduler) Run(ctx context.Context) {
	tick := s.Tick
	if tick <= 0 {
		tick = time.Minute
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	var lastDaily time.Time
	for {
		s.Step(ctx)
		if s.Daily != nil && s.clock().Sub(lastDaily) >= 24*time.Hour {
			lastDaily = s.clock()
			s.Daily(ctx)
		}
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

// Step does one round of checks (exported for tests).
func (s *Scheduler) Step(ctx context.Context) {
	cfg := s.Settings.Get()
	if cfg.Scanner.AutoScan {
		s.maybeScan(ctx, time.Duration(cfg.Scanner.ScanIntervalMinutes)*time.Minute)
	}
	if cfg.Backup.Auto && s.Backup != nil && s.LastBackup != nil && s.clock().Sub(s.LastBackup()) >= 24*time.Hour {
		if err := s.Backup(ctx); err != nil {
			s.Log.Warn("automatic backup failed", "error", err)
		} else {
			s.Log.Info("automatic backup created")
		}
	}
}

func (s *Scheduler) maybeScan(ctx context.Context, every time.Duration) {
	if s.Scanner.Current() != nil {
		return
	}
	folders, err := s.Store.Folders(ctx)
	if err != nil || len(folders) == 0 {
		return
	}
	if jobs, err := s.Store.ScanJobs(ctx, 1); err == nil && len(jobs) > 0 {
		last := jobs[0].StartedAt
		if jobs[0].CompletedAt != nil {
			last = *jobs[0].CompletedAt
		}
		if s.clock().Sub(time.Unix(last, 0)) < every {
			return
		}
	}
	if _, err := s.Scanner.Start(ScanRequest{}); err == nil {
		s.Log.Info("automatic scan started", "every_minutes", int(every.Minutes()))
	}
}
