package media

import (
	"archive/zip"
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// FFmpeg for Windows: the same builds ENSave uses (yt-dlp's FFmpeg-Builds, "shared" variant:
// ffmpeg.exe, ffprobe.exe and their DLLs). Every download is checked against the published
// SHA-256 before anything is written.
const (
	ffmpegBase  = "https://github.com/yt-dlp/FFmpeg-Builds/releases/download/latest"
	ffmpegAsset = "ffmpeg-master-latest-win64-gpl-shared.zip"
)

// CanInstallFFmpeg: ENStream installs FFmpeg itself only on 64-bit Windows. On Linux the
// package manager does it (the .deb depends on ffmpeg).
func CanInstallFFmpeg() bool { return runtime.GOOS == "windows" && runtime.GOARCH == "amd64" }

// InstallHint tells people how to get FFmpeg where ENStream can't install it.
func InstallHint() string {
	switch runtime.GOOS {
	case "windows":
		return "Use Install FFmpeg in Settings → About."
	case "darwin":
		return "Install it with Homebrew: brew install ffmpeg"
	default:
		return "Install it with your package manager, for example: sudo apt install ffmpeg"
	}
}

// Progress reports download progress in bytes (total may be 0 when unknown).
type Progress func(done, total int64)

var httpClient = &http.Client{Timeout: 20 * time.Minute}

func get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ENStream (FFmpeg install)")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}
	return resp, nil
}

// ChecksumFor finds asset in a "sha256  name" listing.
func ChecksumFor(sums, asset string) string {
	sc := bufio.NewScanner(strings.NewReader(sums))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		if len(f) >= 2 && strings.TrimPrefix(f[1], "*") == asset && len(f[0]) == 64 {
			return strings.ToLower(f[0])
		}
	}
	return ""
}

// InstallFFmpeg downloads FFmpeg into dir (Windows only).
func InstallFFmpeg(ctx context.Context, dir string, progress Progress) error {
	if !CanInstallFFmpeg() {
		return errors.New("ENStream can only install FFmpeg on Windows. " + InstallHint())
	}
	resp, err := get(ctx, ffmpegBase+"/checksums.sha256")
	if err != nil {
		return err
	}
	sums, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	resp.Body.Close()
	if err != nil {
		return err
	}
	want := ChecksumFor(string(sums), ffmpegAsset)
	if want == "" {
		return errors.New("no checksum is published for the FFmpeg download; nothing was installed")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "ffmpeg-*.zip.part")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	defer tmp.Close()
	resp, err = get(ctx, ffmpegBase+"/"+ffmpegAsset)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	h := sha256.New()
	pr := &progressReader{r: resp.Body, total: resp.ContentLength, fn: progress}
	if _, err := io.Copy(io.MultiWriter(tmp, h), pr); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return errors.New("the FFmpeg download failed its checksum check; nothing was installed")
	}
	n, err := ExtractBin(tmp.Name(), dir)
	if err != nil {
		return err
	}
	if !isExecutable(filepath.Join(dir, exeName("ffmpeg"))) || !isExecutable(filepath.Join(dir, exeName("ffprobe"))) {
		return fmt.Errorf("the FFmpeg download did not contain ffmpeg and ffprobe (%d files)", n)
	}
	return nil
}

// ExtractBin writes the files of the archive's */bin/ folder into dir. Only plain file
// names are used, so no entry can point outside dir; each file goes through a temporary
// name so a half-written program is never left behind.
func ExtractBin(zipPath, dir string) (int, error) {
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return 0, fmt.Errorf("the FFmpeg archive is damaged: %w", err)
	}
	defer zr.Close()
	n := 0
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		parent, name := path.Split(f.Name)
		if !strings.HasSuffix(strings.TrimSuffix(parent, "/"), "/bin") && strings.TrimSuffix(parent, "/") != "bin" {
			continue
		}
		if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\:`) {
			continue
		}
		if err := writeEntry(f, filepath.Join(dir, name)); err != nil {
			return n, err
		}
		n++
	}
	return n, nil
}

func writeEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	tmp := dest + ".download"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, io.LimitReader(rc, 512<<20)); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

type progressReader struct {
	r           io.Reader
	done, total int64
	last        time.Time
	fn          Progress
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.fn != nil && (time.Since(p.last) > 250*time.Millisecond || err == io.EOF) {
		p.last = time.Now()
		p.fn(p.done, p.total)
	}
	return n, err
}
