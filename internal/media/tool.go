// Package media discovers and inspects video files: scanning, FFprobe metadata, FFmpeg
// thumbnails, subtitles and browser compatibility.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Tool is an external program (ffmpeg or ffprobe). All calls go through Run: explicit
// arguments, never a shell, always a context and a timeout.
type Tool struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Version string `json:"version"`
	Error   string `json:"error,omitempty"`
}

func (t *Tool) Available() bool { return t.Path != "" && t.Error == "" }

// FindTool resolves a configured path, or the name on PATH, and reads its version.
func FindTool(ctx context.Context, name, configured string) *Tool {
	t := &Tool{Name: name}
	target := configured
	if target == "" {
		target = name
	}
	p, err := exec.LookPath(target)
	if err != nil {
		t.Error = fmt.Sprintf("%s was not found (install FFmpeg, or set media.%s_path in config.json)", name, name)
		return t
	}
	t.Path = p
	out, err := t.Run(ctx, 10*time.Second, "-version")
	if err != nil {
		t.Error = fmt.Sprintf("%s does not run: %v", name, err)
		return t
	}
	// "ffmpeg version 6.1.1-3ubuntu5 Copyright …" → "6.1.1-3ubuntu5"
	if f := strings.Fields(strings.SplitN(string(out), "\n", 2)[0]); len(f) >= 3 {
		t.Version = f[2]
	}
	return t
}

// ErrToolMissing is returned when the program was not found.
var ErrToolMissing = errors.New("tool not available")

// Run executes the tool and returns stdout. On failure the error carries the last line of
// stderr. The process is killed when ctx ends or the timeout passes.
func (t *Tool) Run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	if t.Path == "" {
		return nil, ErrToolMissing
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, t.Path, args...)
	hideWindow(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.WaitDelay = 2 * time.Second
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("%s timed out after %s", t.Name, timeout)
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("%s failed: %s", t.Name, lastLine(stderr.String(), err))
	}
	return stdout.Bytes(), nil
}

func lastLine(s string, fallback error) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if l := strings.TrimSpace(lines[i]); l != "" {
			if len(l) > 300 {
				l = l[:300]
			}
			return l
		}
	}
	return fallback.Error()
}
