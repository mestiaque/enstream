package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ThumbnailOptions control the generated WebP image.
type ThumbnailOptions struct {
	Width   int
	Quality int
}

// ThumbnailTime picks where to grab a frame: about 10% in (past black intros and logos),
// at most 5 minutes in, and the start for very short or unknown-length videos.
func ThumbnailTime(duration float64) float64 {
	switch {
	case duration <= 0:
		return 0
	case duration < 4:
		return duration / 2
	}
	t := duration * 0.1
	if t < 2 {
		t = 2
	}
	if t > 300 {
		t = 300
	}
	return t
}

// Thumbnail writes a WebP frame of src to dest atomically (a temporary file, then rename).
// FFmpeg's "thumbnail" filter picks the most representative of the next frames, which
// avoids black or blurred frames. Falls back to the first frame when seeking fails.
func Thumbnail(ctx context.Context, ffmpeg *Tool, src, dest string, duration float64, o ThumbnailOptions) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp.webp"
	defer os.Remove(tmp)
	grab := func(at float64) error {
		args := []string{"-hide_banner", "-loglevel", "error", "-y"}
		if at > 0 {
			args = append(args, "-ss", fmt.Sprintf("%.2f", at))
		}
		args = append(args, "-i", src, "-an", "-sn", "-dn",
			"-vf", fmt.Sprintf("thumbnail=40,scale=%d:-2:flags=lanczos", o.Width),
			"-frames:v", "1", "-c:v", "libwebp", "-quality", fmt.Sprint(o.Quality), "-f", "webp", tmp)
		_, err := ffmpeg.Run(ctx, 90*time.Second, args...)
		if err == nil {
			if fi, e := os.Stat(tmp); e != nil || fi.Size() == 0 {
				return fmt.Errorf("ffmpeg produced no image")
			}
		}
		return err
	}
	err := grab(ThumbnailTime(duration))
	if err != nil && ctx.Err() == nil && duration > 0 {
		err = grab(0)
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}
