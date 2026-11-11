package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	repo "enstream/internal/database/repositories"
	"enstream/internal/media"
)

// Previewer makes the hover-preview clips in the background, one video at a time, so scans
// stay fast. It runs after start-up, after every scan and every few minutes; a preview that
// a browser asked for (Want) goes first. It also deletes clips of videos that left the index.
type Previewer struct {
	Store  *repo.Store
	FFmpeg *media.Tool
	Dir    string
	Log    *slog.Logger
	// Busy reports a running scan; previews wait so the scan gets the CPU.
	Busy func() bool

	wake   chan struct{}
	urgent chan int64
	mu     sync.Mutex
	failed map[int64]bool // not retried until the next start (e.g. unreadable files)
}

func NewPreviewer(store *repo.Store, ffmpeg *media.Tool, dir string, log *slog.Logger) *Previewer {
	return &Previewer{Store: store, FFmpeg: ffmpeg, Dir: dir, Log: log, wake: make(chan struct{}, 1), urgent: make(chan int64, 64), failed: map[int64]bool{}}
}

// Path is where the preview of a video is stored.
func (p *Previewer) Path(id int64) string { return filepath.Join(p.Dir, fmt.Sprintf("%d.mp4", id)) }

// Wake asks for a pass soon (after a scan).
func (p *Previewer) Wake() {
	select {
	case p.wake <- struct{}{}:
	default:
	}
}

// Want puts one video at the front of the queue (a browser wanted its preview).
func (p *Previewer) Want(id int64) {
	select {
	case p.urgent <- id:
	default:
	}
}

// Run works until ctx ends.
func (p *Previewer) Run(ctx context.Context) {
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case id := <-p.urgent:
			p.one(ctx, id)
			continue
		case <-p.wake:
		case <-timer.C:
		}
		p.pass(ctx)
		timer.Reset(5 * time.Minute)
	}
}

func (p *Previewer) pass(ctx context.Context) {
	items, err := p.Store.PreviewItems(ctx)
	if err != nil {
		p.Log.Warn("previews: list videos", "error", err)
		return
	}
	p.removeOrphans(items)
	if !p.FFmpeg.Available() {
		return
	}
	made := 0
	for _, it := range items {
		// Requests from browsers first, then the queue; pause while a scan runs.
		for {
			select {
			case <-ctx.Done():
				return
			case id := <-p.urgent:
				p.one(ctx, id)
				continue
			default:
			}
			break
		}
		for p.Busy != nil && p.Busy() {
			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
		if p.make(ctx, it) {
			made++
		}
	}
	if made > 0 {
		p.Log.Info("previews made", "count", made)
	}
}

func (p *Previewer) one(ctx context.Context, id int64) {
	f, err := p.Store.Video(ctx, id, 0)
	if err != nil {
		return
	}
	vf, err := p.Store.VideoFile(ctx, id)
	if err != nil {
		return
	}
	p.make(ctx, repo.PreviewItem{ID: id, Path: vf.Path, Duration: f.Duration, Enabled: true})
}

// make creates one preview if it is missing; reports whether it made one.
func (p *Previewer) make(ctx context.Context, it repo.PreviewItem) bool {
	if !it.Enabled || it.Duration < media.PreviewMinDuration || !p.FFmpeg.Available() {
		return false
	}
	dest := p.Path(it.ID)
	if _, err := os.Stat(dest); err == nil {
		return false
	}
	p.mu.Lock()
	failed := p.failed[it.ID]
	p.mu.Unlock()
	if failed {
		return false
	}
	start := time.Now()
	if err := media.Preview(ctx, p.FFmpeg, it.Path, dest, it.Duration); err != nil {
		// A file that is gone right now (an external drive was unplugged or dropped out) is
		// tried again on a later pass; only files that exist but don't convert are skipped.
		if _, statErr := os.Stat(it.Path); statErr != nil {
			return false
		}
		if ctx.Err() == nil {
			p.mu.Lock()
			p.failed[it.ID] = true
			p.mu.Unlock()
			p.Log.Warn("preview failed", "video", it.ID, "error", err)
		}
		return false
	}
	p.Log.Debug("preview made", "video", it.ID, "seconds", time.Since(start).Seconds())
	return true
}

// Forget drops a preview so it is made again (the video file changed).
func (p *Previewer) Forget(id int64) {
	os.Remove(p.Path(id))
	p.mu.Lock()
	delete(p.failed, id)
	p.mu.Unlock()
}

// removeOrphans deletes clips whose video is no longer indexed (removed files or folders).
func (p *Previewer) removeOrphans(items []repo.PreviewItem) {
	keep := make(map[int64]bool, len(items))
	for _, it := range items {
		keep[it.ID] = true
	}
	entries, err := os.ReadDir(p.Dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".tmp.mp4") {
			os.Remove(filepath.Join(p.Dir, name)) // left by an interrupted run
			continue
		}
		id, err := strconv.ParseInt(strings.TrimSuffix(name, ".mp4"), 10, 64)
		if err == nil && strings.HasSuffix(name, ".mp4") && !keep[id] {
			os.Remove(filepath.Join(p.Dir, name))
		}
	}
}
