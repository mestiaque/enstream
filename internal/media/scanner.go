package media

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FoundFile is a candidate video discovered by Walk.
type FoundFile struct {
	Path     string // real path (symlinks resolved), inside the root
	RelPath  string
	Size     int64
	Modified time.Time
}

// WalkOptions control discovery.
type WalkOptions struct {
	Extensions    map[string]bool // lower-case, with dot
	IncludeHidden bool
	Excluded      []string // directory names to skip (case-insensitive)
}

// tempFile: partial downloads and editor/OS litter.
func tempFile(name string) bool {
	n := strings.ToLower(name)
	return strings.HasPrefix(n, "~$") || strings.HasPrefix(n, "._") ||
		strings.HasSuffix(n, ".part") || strings.HasSuffix(n, ".crdownload") || strings.HasSuffix(n, ".tmp") || strings.HasSuffix(n, ".download")
}

// Walk lists video files under root recursively. Hidden and excluded directories are
// skipped; directory symlinks are not followed; file symlinks are kept only when they
// resolve inside root. Errors on single entries are collected, not fatal.
func Walk(ctx context.Context, root string, o WalkOptions) ([]FoundFile, []error) {
	excluded := map[string]bool{}
	for _, e := range o.Excluded {
		excluded[strings.ToLower(strings.TrimSpace(e))] = true
	}
	var files []FoundFile
	var errs []error
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err != nil {
			errs = append(errs, err)
			if d != nil && d.IsDir() && path != root {
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if path == root {
				return nil
			}
			if (!o.IncludeHidden && strings.HasPrefix(name, ".")) || excluded[strings.ToLower(name)] || name == "$RECYCLE.BIN" || name == "System Volume Information" {
				return fs.SkipDir
			}
			return nil
		}
		if (!o.IncludeHidden && strings.HasPrefix(name, ".")) || tempFile(name) || !o.Extensions[strings.ToLower(filepath.Ext(name))] {
			return nil
		}
		real := path
		if d.Type()&fs.ModeSymlink != 0 {
			r, err := filepath.EvalSymlinks(path)
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			rel, err := filepath.Rel(root, r)
			if err != nil || strings.HasPrefix(rel, "..") {
				return nil // points outside the media folder
			}
			real = r
		}
		fi, err := os.Stat(real)
		if err != nil || !fi.Mode().IsRegular() {
			if err != nil {
				errs = append(errs, err)
			}
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		files = append(files, FoundFile{Path: real, RelPath: filepath.ToSlash(rel), Size: fi.Size(), Modified: fi.ModTime()})
		return nil
	})
	return files, errs
}

// Title makes a readable title from a file name: "the.big_movie (2020).mkv" → "the big movie (2020)".
func Title(fileName string) string {
	t := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	t = strings.NewReplacer("_", " ", ".", " ").Replace(t)
	return strings.Join(strings.Fields(t), " ")
}
