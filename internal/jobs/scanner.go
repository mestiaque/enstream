// Package jobs runs background work: library scans with a bounded pool of FFmpeg workers.
package jobs

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"enstream/internal/config"
	repo "enstream/internal/database/repositories"
	"enstream/internal/media"
)

var ErrBusy = errors.New("a scan is already running")

// ScanRequest: which folders, and whether to retry only the files that failed last time.
type ScanRequest struct {
	FolderID    int64 // 0 = all enabled folders
	RetryFailed bool
}

// Scanner indexes media folders. One scan runs at a time (which also prevents two scans of
// the same folder); inside a scan at most WorkerCount files are probed at once.
type Scanner struct {
	store   *repo.Store
	cfg     *config.Live
	paths   config.Paths
	ffmpeg  *media.Tool
	ffprobe *media.Tool
	log     *slog.Logger

	mu      sync.Mutex
	current *run
	done    chan struct{} // closed when the current run ends
}

func NewScanner(store *repo.Store, cfg *config.Live, paths config.Paths, ffmpeg, ffprobe *media.Tool, log *slog.Logger) *Scanner {
	return &Scanner{store: store, cfg: cfg, paths: paths, ffmpeg: ffmpeg, ffprobe: ffprobe, log: log}
}

type run struct {
	cfg    config.Config // snapshot: settings changed mid-scan apply to the next scan
	job    repo.ScanJob
	cancel context.CancelFunc
	// Counters updated by workers.
	total, processed, skipped, failed, added, updated, removed atomic.Int64
	mu                                                         sync.Mutex
	currentFile                                                string
}

func (r *run) snapshot() repo.ScanJob {
	j := r.job
	j.TotalFiles, j.ProcessedFiles, j.SkippedFiles = int(r.total.Load()), int(r.processed.Load()), int(r.skipped.Load())
	j.FailedFiles, j.NewFiles, j.UpdatedFiles, j.RemovedFiles = int(r.failed.Load()), int(r.added.Load()), int(r.updated.Load()), int(r.removed.Load())
	r.mu.Lock()
	j.CurrentFile = r.currentFile
	r.mu.Unlock()
	return j
}

func (r *run) setCurrent(rel string) {
	r.mu.Lock()
	r.currentFile = rel
	r.mu.Unlock()
}

// Start begins a scan in the background and returns its job id.
func (s *Scanner) Start(req ScanRequest) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		return 0, ErrBusy
	}
	var folderID *int64
	if req.FolderID > 0 {
		if _, err := s.store.Folder(context.Background(), req.FolderID); err != nil {
			return 0, err
		}
		folderID = &req.FolderID
	}
	id, err := s.store.CreateScanJob(context.Background(), folderID)
	if err != nil {
		return 0, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	r := &run{cfg: s.cfg.Get(), job: repo.ScanJob{ID: id, FolderID: folderID, Status: "running", StartedAt: time.Now().Unix()}, cancel: cancel}
	s.current, s.done = r, make(chan struct{})
	go s.execute(ctx, r, req)
	return id, nil
}

// Cancel stops the running scan; returns false when nothing runs.
func (s *Scanner) Cancel() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return false
	}
	s.current.cancel()
	return true
}

// Current is the running job's live progress, or nil.
func (s *Scanner) Current() *repo.ScanJob {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current == nil {
		return nil
	}
	j := s.current.snapshot()
	return &j
}

// Wait blocks until the running scan (if any) ends.
func (s *Scanner) Wait() {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done != nil {
		<-done
	}
}

// Shutdown cancels a running scan and waits for its workers (and FFmpeg processes) to stop.
func (s *Scanner) Shutdown() {
	s.Cancel()
	s.Wait()
}

func (s *Scanner) execute(ctx context.Context, r *run, req ScanRequest) {
	defer func() {
		s.mu.Lock()
		close(s.done)
		s.current = nil
		s.mu.Unlock()
	}()
	// Progress reaches the database every second, so status survives a crash.
	stopTicker := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				_ = s.store.SaveScanJob(context.Background(), r.snapshot())
			case <-stopTicker:
				return
			}
		}
	}()

	err := s.scanAll(ctx, r, req)
	close(stopTicker)
	j := r.snapshot()
	done := time.Now().Unix()
	j.CompletedAt, j.CurrentFile = &done, ""
	switch {
	case ctx.Err() != nil:
		j.Status = "cancelled"
	case err != nil:
		j.Status, j.ErrorMessage = "failed", err.Error()
	default:
		j.Status = "completed"
	}
	if err := s.store.SaveScanJob(context.Background(), j); err != nil {
		s.log.Error("save scan job", "error", err)
	}
	s.log.Info("scan finished", "job", j.ID, "status", j.Status, "total", j.TotalFiles, "new", j.NewFiles, "updated", j.UpdatedFiles,
		"skipped", j.SkippedFiles, "removed", j.RemovedFiles, "failed", j.FailedFiles, "seconds", done-j.StartedAt)
}

func (s *Scanner) scanAll(ctx context.Context, r *run, req ScanRequest) error {
	folders, err := s.store.Folders(ctx)
	if err != nil {
		return err
	}
	var problems []string
	for _, f := range folders {
		if (req.FolderID > 0 && f.ID != req.FolderID) || !f.Enabled {
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := s.scanFolder(ctx, r, f, req.RetryFailed); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.log.Warn("folder not scanned", "folder", f.Name, "error", err)
			problems = append(problems, fmt.Sprintf("%s: %v", f.Name, err))
		}
	}
	if len(problems) > 0 {
		return errors.New(strings.Join(problems, "; "))
	}
	return nil
}

type workItem struct {
	file  media.FoundFile
	known *repo.Known
}

func (s *Scanner) scanFolder(ctx context.Context, r *run, f repo.Folder, retryOnly bool) error {
	// A missing root (unplugged drive, renamed folder) must not wipe its index.
	if fi, err := os.Stat(f.Path); err != nil || !fi.IsDir() {
		return fmt.Errorf("the folder is not available (unplugged drive or moved folder?)")
	}
	s.log.Info("scan started", "folder", f.Name, "job", r.job.ID)
	found, walkErrs := media.Walk(ctx, f.Path, media.WalkOptions{
		Extensions: r.cfg.ExtensionSet(), IncludeHidden: r.cfg.Scanner.IncludeHidden, Excluded: r.cfg.Scanner.ExcludedDirectories,
	})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	for _, e := range walkErrs {
		s.log.Debug("walk", "folder", f.Name, "error", e)
	}
	known, err := s.store.KnownVideos(ctx, f.ID)
	if err != nil {
		return err
	}
	failed, err := s.store.FailedPaths(ctx, f.ID)
	if err != nil {
		return err
	}

	seen := map[string]bool{}
	var items []workItem
	dirCache := map[string][]string{}
	for _, ff := range found {
		if seen[ff.Path] { // a symlink to a file already listed
			continue
		}
		seen[ff.Path] = true
		r.total.Add(1)
		k, ok := known[ff.Path]
		var kp *repo.Known
		if ok {
			kp = &k
		}
		if s.needsWork(ff, kp, failed[ff.Path], retryOnly) {
			items = append(items, workItem{file: ff, known: kp})
			continue
		}
		r.skipped.Add(1)
		r.processed.Add(1)
		if kp != nil {
			s.syncSubtitles(ctx, kp.ID, ff.Path, dirCache)
		}
	}

	// Files gone from disk: drop their index entries and thumbnails. Not in retry mode,
	// where only part of the folder is looked at.
	if !retryOnly {
		var gone []int64
		for path, k := range known {
			if !seen[path] {
				gone = append(gone, k.ID)
				if k.ThumbnailPath != "" {
					os.Remove(k.ThumbnailPath)
				}
			}
		}
		if len(gone) > 0 {
			if err := s.store.DeleteVideos(ctx, gone); err != nil {
				return err
			}
			r.removed.Add(int64(len(gone)))
		}
	}

	// Bounded worker pool: at most WorkerCount FFprobe/FFmpeg processes at a time.
	queue := make(chan workItem)
	var wg sync.WaitGroup
	for i := 0; i < r.cfg.Scanner.WorkerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cache := map[string][]string{}
			for it := range queue {
				s.process(ctx, r, f, it, cache)
			}
		}()
	}
feed:
	for _, it := range items {
		select {
		case queue <- it:
		case <-ctx.Done():
			break feed
		}
	}
	close(queue)
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return s.store.TouchFolderScanned(ctx, f.ID)
}

// needsWork: new or changed files, files that failed last time, and files probed without
// FFprobe/FFmpeg that can now get details or a thumbnail.
func (s *Scanner) needsWork(ff media.FoundFile, k *repo.Known, failedBefore, retryOnly bool) bool {
	if retryOnly {
		return failedBefore || (k != nil && k.ProbeError != "")
	}
	if k == nil || failedBefore {
		return true
	}
	if k.Size != ff.Size || k.Modified != ff.Modified.Unix() {
		return true
	}
	if s.ffprobe.Available() && k.Compatibility == string(media.Unknown) && k.ProbeError == "" {
		return true
	}
	return s.ffmpeg.Available() && k.ThumbnailPath == "" && k.ProbeError == ""
}

func (s *Scanner) process(ctx context.Context, r *run, f repo.Folder, it workItem, dirCache map[string][]string) {
	ff := it.file
	r.setCurrent(ff.RelPath)
	defer r.processed.Add(1)
	name := filepath.Base(ff.Path)
	ext := strings.ToLower(filepath.Ext(name))
	rec := repo.VideoRecord{
		FolderID: f.ID, Title: media.Title(name), FileName: name, FilePath: ff.Path, RelativePath: ff.RelPath, Extension: ext,
		MimeType: media.MimeType(ext), FileSize: ff.Size, Modified: ff.Modified.Unix(), Compatibility: string(media.Unknown),
	}
	var meta media.Metadata
	probeFailed := false
	if s.ffprobe.Available() {
		m, err := media.Probe(ctx, s.ffprobe, ff.Path)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			probeFailed = true
			rec.ProbeError = err.Error()
			r.failed.Add(1)
			_ = s.store.RecordFailure(ctx, f.ID, ff.Path, err.Error())
			s.log.Warn("cannot read video", "file", ff.RelPath, "error", err)
		} else {
			meta = m
			rec.Container, rec.Duration, rec.Width, rec.Height, rec.FPS = m.Container, m.Duration, m.Width, m.Height, m.FPS
			rec.VideoCodec, rec.VideoProfile, rec.PixelFormat = m.VideoCodec, m.VideoProfile, m.PixelFormat
			rec.AudioCodec, rec.AudioChannels, rec.Bitrate = m.AudioCodec, m.AudioChannels, m.Bitrate
			rec.Compatibility = string(media.Rate(ext, m))
		}
	}
	id, isNew, err := s.store.UpsertVideo(ctx, rec)
	if err != nil {
		if ctx.Err() == nil {
			r.failed.Add(1)
			s.log.Error("save video", "file", ff.RelPath, "error", err)
		}
		return
	}
	if isNew {
		r.added.Add(1)
	} else {
		r.updated.Add(1)
	}
	if !probeFailed {
		_ = s.store.ClearFailure(ctx, ff.Path)
	}

	// Thumbnail named after the stable id, never after the file name.
	if s.ffmpeg.Available() && !probeFailed {
		dest := filepath.Join(s.paths.Thumbnails, fmt.Sprintf("%d.webp", id))
		err := media.Thumbnail(ctx, s.ffmpeg, ff.Path, dest, meta.Duration, media.ThumbnailOptions{Width: r.cfg.Thumbnail.Width, Quality: r.cfg.Thumbnail.Quality})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.log.Warn("thumbnail failed", "file", ff.RelPath, "error", err)
			_ = s.store.SetThumbnail(ctx, id, "", err.Error())
		} else {
			_ = s.store.SetThumbnail(ctx, id, dest, "")
		}
	}
	s.syncSubtitles(ctx, id, ff.Path, dirCache)
}

// syncSubtitles records the external subtitle files sitting next to a video.
func (s *Scanner) syncSubtitles(ctx context.Context, videoID int64, path string, cache map[string][]string) {
	dir := filepath.Dir(path)
	names, ok := cache[dir]
	if !ok {
		names = media.DirNames(dir)
		cache[dir] = names
	}
	found := media.MatchSubtitles(path, names)
	subs := make([]repo.Subtitle, 0, len(found))
	for _, sf := range found {
		subs = append(subs, repo.Subtitle{VideoID: videoID, FilePath: sf.Path, Language: sf.Language, Format: sf.Format, Label: sf.Label})
	}
	if err := s.store.ReplaceSubtitles(ctx, videoID, subs); err != nil && ctx.Err() == nil {
		s.log.Warn("subtitles", "file", path, "error", err)
	}
}
