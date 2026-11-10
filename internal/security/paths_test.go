package security

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCanonicalFolder(t *testing.T) {
	dir := t.TempDir()
	if _, err := CanonicalFolder("relative/videos"); err == nil {
		t.Fatal("relative path accepted")
	}
	if _, err := CanonicalFolder(filepath.Join(dir, "missing")); err == nil {
		t.Fatal("missing folder accepted")
	}
	file := filepath.Join(dir, "f.txt")
	os.WriteFile(file, []byte("x"), 0o644)
	if _, err := CanonicalFolder(file); !errors.Is(err, ErrNotFolder) {
		t.Fatalf("file accepted: %v", err)
	}
	link := filepath.Join(dir, "link")
	real := filepath.Join(dir, "real")
	os.Mkdir(real, 0o755)
	os.Symlink(real, link)
	got, err := CanonicalFolder(link + string(filepath.Separator))
	want, _ := filepath.EvalSymlinks(real)
	if err != nil || got != want {
		t.Fatalf("symlinks resolved: %q %v", got, err)
	}
}

func TestWithinAndResolveInside(t *testing.T) {
	sep := string(filepath.Separator)
	root := filepath.Join(sep, "media", "videos")
	for path, want := range map[string]bool{
		root:                                         true,
		filepath.Join(root, "a", "b.mp4"):             true,
		filepath.Join(sep, "media", "videos2", "x"):   false, // prefix trick
		filepath.Join(root, "..", "secret"):           false,
		filepath.Join(sep, "etc", "passwd"):           false,
	} {
		if Within(root, filepath.Clean(path)) != want {
			t.Errorf("Within(%s) != %v", path, want)
		}
	}

	dir := t.TempDir()
	media := filepath.Join(dir, "media")
	outside := filepath.Join(dir, "outside")
	os.MkdirAll(media, 0o755)
	os.MkdirAll(outside, 0o755)
	os.WriteFile(filepath.Join(media, "ok.mp4"), []byte("v"), 0o644)
	os.WriteFile(filepath.Join(outside, "secret.mp4"), []byte("s"), 0o644)
	os.Symlink(filepath.Join(outside, "secret.mp4"), filepath.Join(media, "escape.mp4"))
	if _, err := ResolveInside(filepath.Join(media, "ok.mp4"), media); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveInside(filepath.Join(media, "escape.mp4"), media); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("symlink escape not refused: %v", err)
	}
	if _, err := ResolveInside(filepath.Join(media, "..", "outside", "secret.mp4"), media); !errors.Is(err, ErrOutsideRoot) {
		t.Fatalf("traversal not refused: %v", err)
	}
}
