package media

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPreviewPoints(t *testing.T) {
	if PreviewPoints(10) != nil {
		t.Fatal("short videos get no preview")
	}
	pts := PreviewPoints(100)
	if len(pts) != PreviewPieces || pts[0] < 1 || pts[len(pts)-1]+PreviewPieceLength > 100 {
		t.Fatalf("points: %v", pts)
	}
	for i := 1; i < len(pts); i++ {
		if pts[i] <= pts[i-1]+PreviewPieceLength {
			t.Fatalf("pieces overlap: %v", pts)
		}
	}
}

func TestPreviewClip(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "long.mp4")
	out, err := exec.Command("ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi", "-i", "testsrc2=size=640x360:rate=25:duration=40",
		"-f", "lavfi", "-i", "sine=duration=40", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-c:a", "aac", src).CombinedOutput()
	if err != nil {
		t.Fatalf("%v %s", err, out)
	}
	ctx := context.Background()
	ff := FindTool(ctx, "ffmpeg", "", dir)
	probe := FindTool(ctx, "ffprobe", "", dir)
	dest := filepath.Join(dir, "previews", "7.mp4")
	os.MkdirAll(filepath.Dir(dest), 0o755)
	if err := Preview(ctx, ff, src, dest, 40); err != nil {
		t.Fatal(err)
	}
	m, err := Probe(ctx, probe, dest)
	if err != nil {
		t.Fatal(err)
	}
	if m.Width != PreviewWidth || m.AudioCodec != "" || m.VideoCodec != "h264" || m.Duration < 9 || m.Duration > 11 {
		t.Fatalf("preview: %+v", m)
	}
	if left, _ := filepath.Glob(filepath.Join(dir, "previews", "*.tmp*")); len(left) != 0 {
		t.Fatalf("temporary files left: %v", left)
	}
	if err := Preview(ctx, ff, filepath.Join(dir, "missing.mp4"), filepath.Join(dir, "previews", "8.mp4"), 40); err == nil {
		t.Fatal("a broken source must fail")
	}
	if _, err := os.Stat(filepath.Join(dir, "previews", "8.mp4")); err == nil {
		t.Fatal("no clip for a failed run")
	}
}
