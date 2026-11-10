// Package media discovers and inspects video files: scanning, FFprobe metadata, FFmpeg
// thumbnails, subtitles and browser compatibility.
package media

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Tool is an external program (ffmpeg or ffprobe). All calls go through Run: explicit
// arguments, never a shell, always a context and a timeout.
//
// A Tool can be replaced in place (Replace) after FFmpeg is installed while ENStream runs,
// so its fields are only read through the methods.
type Tool struct {
	Name string

	mu      sync.RWMutex
	path    string
	version string
	err     string
}

func (t *Tool) snapshot() (path, version, errMsg string) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.path, t.version, t.err
}

func (t *Tool) Path() string    { p, _, _ := t.snapshot(); return p }
func (t *Tool) Version() string { _, v, _ := t.snapshot(); return v }
func (t *Tool) Err() string     { _, _, e := t.snapshot(); return e }
func (t *Tool) Available() bool { p, _, e := t.snapshot(); return p != "" && e == "" }

// Replace copies another tool's state into t (callers keep their *Tool).
func (t *Tool) Replace(o *Tool) {
	p, v, e := o.snapshot()
	t.mu.Lock()
	t.path, t.version, t.err = p, v, e
	t.mu.Unlock()
}

func exeName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}

// FindTool resolves a configured path, then the given folders (ENStream's own tools
// folder), then PATH, and reads the version.
func FindTool(ctx context.Context, name, configured string, dirs ...string) *Tool {
	t := &Tool{Name: name}
	var p string
	var err error
	if configured != "" {
		p, err = exec.LookPath(configured)
	} else {
		err = exec.ErrNotFound
		for _, d := range dirs {
			if c := filepath.Join(d, exeName(name)); isExecutable(c) {
				p, err = c, nil
				break
			}
		}
		if err != nil {
			p, err = exec.LookPath(name)
		}
	}
	if err != nil {
		t.err = fmt.Sprintf("%s was not found (install FFmpeg, or set media.%s_path in config.json)", name, name)
		return t
	}
	t.path = p
	out, err := t.Run(ctx, 10*time.Second, "-version")
	if err != nil {
		t.path, t.err = "", fmt.Sprintf("%s does not run: %v", name, err)
		return t
	}
	// "ffmpeg version 6.1.1-3ubuntu5 Copyright …" → "6.1.1-3ubuntu5"
	if f := strings.Fields(strings.SplitN(string(out), "\n", 2)[0]); len(f) >= 3 {
		t.version = f[2]
	}
	return t
}

// ErrToolMissing is returned when the program was not found.
var ErrToolMissing = errors.New("tool not available")

// Run executes the tool and returns stdout. On failure the error carries the last line of
// stderr. The process is killed when ctx ends or the timeout passes.
func (t *Tool) Run(ctx context.Context, timeout time.Duration, args ...string) ([]byte, error) {
	path := t.Path()
	if path == "" {
		return nil, ErrToolMissing
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
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
