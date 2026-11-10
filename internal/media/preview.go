package media

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Hover previews: a short, silent clip of one-second pieces from across a video (like the
// previews on video sites), shown when the mouse rests on a card.
const (
	PreviewPieces      = 10
	PreviewPieceLength = 1.0  // seconds per piece
	PreviewMinDuration = 20.0 // shorter videos are previewed by playing them from the start
	PreviewWidth       = 480
)

// PreviewPoints returns where each piece starts: evenly from 5 % to 86 % of the video.
func PreviewPoints(duration float64) []float64 {
	if duration < PreviewMinDuration {
		return nil
	}
	out := make([]float64, PreviewPieces)
	for i := range out {
		out[i] = duration * (0.05 + 0.09*float64(i))
	}
	return out
}

// Preview writes the preview clip of src to dest atomically: H.264 in MP4 with the index at
// the front (plays as soon as the first bytes arrive), no audio, 480 px wide, 24 fps.
// Each piece is read with input seeking, so even long or 4K files are quick.
func Preview(ctx context.Context, ffmpeg *Tool, src, dest string, duration float64) error {
	points := PreviewPoints(duration)
	if points == nil {
		return fmt.Errorf("too short for a preview")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	tmp := dest + ".tmp.mp4"
	defer os.Remove(tmp)
	args := []string{"-hide_banner", "-loglevel", "error", "-y"}
	var filter strings.Builder
	for i, at := range points {
		args = append(args, "-ss", fmt.Sprintf("%.2f", at), "-t", fmt.Sprintf("%.2f", PreviewPieceLength), "-i", src)
		fmt.Fprintf(&filter, "[%d:v:0]scale=%d:-2:flags=bicubic,fps=24,setsar=1,format=yuv420p[v%d];", i, PreviewWidth, i)
	}
	for i := range points {
		fmt.Fprintf(&filter, "[v%d]", i)
	}
	fmt.Fprintf(&filter, "concat=n=%d:v=1:a=0[out]", len(points))
	args = append(args, "-filter_complex", filter.String(), "-map", "[out]", "-an", "-sn", "-dn",
		"-c:v", "libx264", "-preset", "veryfast", "-crf", "30", "-profile:v", "main", "-movflags", "+faststart", "-f", "mp4", tmp)
	if _, err := ffmpeg.Run(ctx, 3*time.Minute, args...); err != nil {
		return err
	}
	if fi, err := os.Stat(tmp); err != nil || fi.Size() == 0 {
		return fmt.Errorf("ffmpeg produced no preview")
	}
	return os.Rename(tmp, dest)
}
