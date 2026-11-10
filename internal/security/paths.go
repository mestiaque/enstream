package security

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	ErrOutsideRoot = errors.New("path is outside the allowed folder")
	ErrNotFolder   = errors.New("not a folder")
)

// CanonicalFolder validates a folder a user wants to register: absolute, existing, a
// directory, readable. It returns the cleaned path with symlinks resolved, so containment
// checks later compare real locations.
func CanonicalFolder(p string) (string, error) {
	p = strings.TrimSpace(p)
	if p == "" {
		return "", errors.New("enter a folder path")
	}
	if !filepath.IsAbs(p) {
		return "", fmt.Errorf("%q is not a full path (like /home/you/Videos or D:\\Videos)", p)
	}
	real, err := filepath.EvalSymlinks(filepath.Clean(p))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("the folder %q does not exist", p)
		}
		return "", fmt.Errorf("cannot open %q: %w", p, err)
	}
	fi, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%q: %w", p, ErrNotFolder)
	}
	f, err := os.Open(real)
	if err != nil {
		return "", fmt.Errorf("cannot read %q (check its permissions)", p)
	}
	f.Close()
	return real, nil
}

// Within reports whether path is root itself or below it (lexically; both must be clean
// and absolute). Case-insensitive on Windows.
func Within(root, path string) bool {
	if runtime.GOOS == "windows" {
		root, path = strings.ToLower(root), strings.ToLower(path)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel))
}

// ResolveInside returns the real location of path (symlinks followed) only when it lies
// inside one of roots. Used before serving any file: a symlink inside a media folder that
// points elsewhere on the disk is refused.
func ResolveInside(path string, roots ...string) (string, error) {
	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", err
	}
	for _, r := range roots {
		if rr, err := filepath.EvalSymlinks(r); err == nil && Within(rr, real) {
			return real, nil
		}
	}
	return "", ErrOutsideRoot
}
