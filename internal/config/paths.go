package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// Paths are the directories ENStream writes to. Original videos are never stored here.
type Paths struct {
	Data       string // config.json, enstream.db
	Thumbnails string
	Previews   string // short hover-preview clips (<id>.mp4)
	Subtitles  string // converted WebVTT cache
	Logs       string
	Backups    string
	Tools      string // FFmpeg installed by ENStream (Windows)
}

// DataDir picks the per-user data directory:
//   - ENSTREAM_DATA_DIR when set (or the -data flag, applied by the caller);
//   - Windows: %LOCALAPPDATA%\ENStream;
//   - Linux and others: $XDG_DATA_HOME/enstream, else ~/.local/share/enstream.
//
// The executable's own folder is not used: it may not be writable.
func DataDir() string {
	if d := os.Getenv("ENSTREAM_DATA_DIR"); d != "" {
		return d
	}
	if runtime.GOOS == "windows" {
		if d := os.Getenv("LOCALAPPDATA"); d != "" {
			return filepath.Join(d, "ENStream")
		}
	}
	if d := os.Getenv("XDG_DATA_HOME"); d != "" {
		return filepath.Join(d, "enstream")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "enstream")
	}
	return filepath.Join(os.TempDir(), "enstream")
}

// NewPaths lays out the data directory and creates every folder.
func NewPaths(data string) (Paths, error) {
	p := Paths{
		Data:       data,
		Thumbnails: filepath.Join(data, "thumbnails"),
		Previews:   filepath.Join(data, "previews"),
		Subtitles:  filepath.Join(data, "cache", "subtitles"),
		Logs:       filepath.Join(data, "logs"),
		Backups:    filepath.Join(data, "backups"),
		Tools:      filepath.Join(data, "tools"),
	}
	for _, d := range []string{p.Data, p.Thumbnails, p.Previews, p.Subtitles, p.Logs, p.Backups} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return p, err
		}
	}
	return p, nil
}

func (p Paths) Config() string   { return filepath.Join(p.Data, "config.json") }
func (p Paths) Database() string { return filepath.Join(p.Data, "enstream.db") }
