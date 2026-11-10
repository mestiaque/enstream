package jobs

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"enstream/internal/config"
	"enstream/internal/database"
	repo "enstream/internal/database/repositories"
	"enstream/internal/media"
)

// Integration test with real FFmpeg: generated clips, no personal media.
func requireFFmpeg(t *testing.T) (*media.Tool, *media.Tool) {
	t.Helper()
	ff := media.FindTool(context.Background(), "ffmpeg", "")
	fp := media.FindTool(context.Background(), "ffprobe", "")
	if !ff.Available() || !fp.Available() {
		t.Skip("ffmpeg/ffprobe not installed")
	}
	return ff, fp
}

func makeClip(t *testing.T, path string, seconds int, vcodec, acodec string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o755)
	args := []string{"-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=320x240:rate=25:duration=" + itoa(seconds)}
	if acodec != "" {
		args = append(args, "-f", "lavfi", "-i", "sine=frequency=440:duration="+itoa(seconds), "-c:a", acodec)
	}
	args = append(args, "-c:v", vcodec, "-pix_fmt", "yuv420p", path)
	if out, err := exec.Command("ffmpeg", args...).CombinedOutput(); err != nil {
		t.Fatalf("make clip: %v %s", err, out)
	}
}

func itoa(n int) string { return string(rune('0'+n/10)) + string(rune('0'+n%10)) }

type env struct {
	store *repo.Store
	sc    *Scanner
	media string
	paths config.Paths
}

func newEnv(t *testing.T, ffmpeg, ffprobe *media.Tool) *env {
	t.Helper()
	data := t.TempDir()
	paths, _ := config.NewPaths(data)
	db, err := database.Open(paths.Database())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	cfg := config.Default()
	store := repo.New(db)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	return &env{store: store, sc: NewScanner(store, &cfg, paths, ffmpeg, ffprobe, log), media: t.TempDir(), paths: paths}
}

func (e *env) scan(t *testing.T, req ScanRequest) repo.ScanJob {
	t.Helper()
	id, err := e.sc.Start(req)
	if err != nil {
		t.Fatal(err)
	}
	e.sc.Wait()
	j, err := e.store.ScanJob(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return j
}

func TestScanIndexesProbesThumbnailsAndIsIncremental(t *testing.T) {
	ffmpeg, ffprobe := requireFFmpeg(t)
	e := newEnv(t, ffmpeg, ffprobe)
	ctx := context.Background()
	makeClip(t, filepath.Join(e.media, "Holiday Trip.mp4"), 3, "libx264", "aac")
	makeClip(t, filepath.Join(e.media, "old", "clip.avi"), 2, "mpeg4", "")
	os.WriteFile(filepath.Join(e.media, "Holiday Trip.en.srt"), []byte("1\n00:00:00,000 --> 00:00:01,000\nHi\n"), 0o644)
	os.WriteFile(filepath.Join(e.media, "broken.mkv"), []byte("not a video"), 0o644)
	fid, _ := e.store.AddFolder(ctx, e.media, "Media")

	j := e.scan(t, ScanRequest{})
	if j.Status != "completed" || j.TotalFiles != 3 || j.NewFiles != 3 || j.FailedFiles != 1 {
		t.Fatalf("first scan: %+v", j)
	}
	vids, total, err := e.store.Videos(ctx, repo.VideoQuery{Page: 1, Limit: 30, Sort: "title"})
	if err != nil || total != 3 {
		t.Fatalf("videos: %d %v", total, err)
	}
	byName := map[string]repo.Video{}
	for _, v := range vids {
		byName[v.FileName] = v
	}
	h := byName["Holiday Trip.mp4"]
	if h.Title != "Holiday Trip" || h.Width != 320 || h.VideoCodec != "h264" || h.AudioCodec != "aac" || h.Compatibility != "likely" || !h.HasThumbnail || h.SubtitleCount != 1 {
		t.Fatalf("mp4 metadata: %+v", h)
	}
	if h.Duration < 2.9 || h.Duration > 3.2 {
		t.Fatalf("duration %v", h.Duration)
	}
	if byName["clip.avi"].Compatibility != "transcode" || byName["clip.avi"].RelativePath != "old/clip.avi" {
		t.Fatalf("avi: %+v", byName["clip.avi"])
	}
	thumb, _ := e.store.VideoFile(ctx, h.ID)
	if fi, err := os.Stat(thumb.ThumbnailPath); err != nil || fi.Size() == 0 || filepath.Dir(thumb.ThumbnailPath) != e.paths.Thumbnails {
		t.Fatalf("thumbnail %s %v", thumb.ThumbnailPath, err)
	}
	if n, _ := e.store.FailureCount(ctx); n != 1 {
		t.Fatalf("failures %d", n)
	}

	// Incremental: unchanged files are skipped; a new file is processed.
	makeClip(t, filepath.Join(e.media, "new.webm"), 2, "libvpx-vp9", "libopus")
	j = e.scan(t, ScanRequest{})
	if j.NewFiles != 1 || j.SkippedFiles != 2 || j.FailedFiles != 1 {
		t.Fatalf("incremental: %+v", j)
	}

	// A deleted file leaves the index (and its thumbnail goes too).
	os.Remove(filepath.Join(e.media, "Holiday Trip.mp4"))
	j = e.scan(t, ScanRequest{FolderID: fid})
	if j.RemovedFiles != 1 {
		t.Fatalf("removal: %+v", j)
	}
	if _, err := os.Stat(thumb.ThumbnailPath); !os.IsNotExist(err) {
		t.Fatal("thumbnail of a removed video must be deleted")
	}

	// Retry failed: only the broken file is looked at.
	j = e.scan(t, ScanRequest{RetryFailed: true})
	if j.FailedFiles != 1 || j.ProcessedFiles != 3 || j.SkippedFiles != 2 {
		t.Fatalf("retry: %+v", j)
	}

	// An unplugged folder keeps its index.
	os.Rename(e.media, e.media+"-gone")
	j = e.scan(t, ScanRequest{})
	if j.Status != "failed" {
		t.Fatalf("missing folder: %+v", j)
	}
	if _, total, _ := e.store.Videos(ctx, repo.VideoQuery{Page: 1, Limit: 30}); total != 3 {
		t.Fatalf("index wiped for a missing folder: %d", total)
	}
}

func TestOneScanAtATimeAndCancel(t *testing.T) {
	ffmpeg, ffprobe := requireFFmpeg(t)
	e := newEnv(t, ffmpeg, ffprobe)
	for i := 0; i < 8; i++ {
		makeClip(t, filepath.Join(e.media, "c"+itoa(i)+".mp4"), 2, "libx264", "")
	}
	e.store.AddFolder(context.Background(), e.media, "Media")
	if _, err := e.sc.Start(ScanRequest{}); err != nil {
		t.Fatal(err)
	}
	if _, err := e.sc.Start(ScanRequest{}); err != ErrBusy {
		t.Fatalf("second scan: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if e.sc.Current() == nil || !e.sc.Cancel() {
		t.Fatal("running scan not visible or not cancellable")
	}
	e.sc.Wait()
	jobs, _ := e.store.ScanJobs(context.Background(), 1)
	if jobs[0].Status != "cancelled" {
		t.Fatalf("status %s", jobs[0].Status)
	}
	if e.sc.Current() != nil || e.sc.Cancel() {
		t.Fatal("nothing should be running")
	}
}

func TestWithoutFFmpegFilesAreStillIndexed(t *testing.T) {
	e := newEnv(t, &media.Tool{Name: "ffmpeg"}, &media.Tool{Name: "ffprobe"})
	os.WriteFile(filepath.Join(e.media, "a.mp4"), []byte("x"), 0o644)
	e.store.AddFolder(context.Background(), e.media, "Media")
	j := e.scan(t, ScanRequest{})
	if j.NewFiles != 1 || j.FailedFiles != 0 {
		t.Fatalf("%+v", j)
	}
	v, _, _ := e.store.Videos(context.Background(), repo.VideoQuery{Page: 1, Limit: 10})
	if v[0].Compatibility != "unknown" || v[0].HasThumbnail {
		t.Fatalf("%+v", v[0])
	}
}
