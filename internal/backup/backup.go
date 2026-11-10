// Package backup makes consistent copies of the SQLite database and restores them.
//
// A restore is staged (copied next to the live database) and applied on the next start,
// before the database is opened, so no running query ever sees the file change. The live
// database is first copied to a "pre-restore" backup, so a restore can always be undone.
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"enstream/internal/database"
)

// Info describes one backup file.
type Info struct {
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	CreatedAt int64  `json:"created_at"`
	Kind      string `json:"kind"` // manual | auto | pre-restore
}

var nameRE = regexp.MustCompile(`^enstream-(\d{8}-\d{6})(-auto|-pre-restore)?(-\d{1,3})?\.db$`)

var (
	ErrNotFound = errors.New("backup not found")
	ErrInvalid  = errors.New("this file is not a usable ENStream database")
)

type Manager struct {
	DB     *sql.DB
	Dir    string // backups folder
	DBPath string // live database
	now    func() time.Time
}

func (m *Manager) clock() time.Time {
	if m.now != nil {
		return m.now()
	}
	return time.Now()
}

func kindOf(name string) string {
	switch {
	case strings.Contains(name, "-auto"):
		return "auto"
	case strings.Contains(name, "-pre-restore"):
		return "pre-restore"
	}
	return "manual"
}

func uniqueName(dir, stamp, suffix string) string {
	name := fmt.Sprintf("enstream-%s%s.db", stamp, suffix)
	for i := 2; i < 1000; i++ {
		if _, err := os.Stat(filepath.Join(dir, name)); errors.Is(err, os.ErrNotExist) {
			break
		}
		name = fmt.Sprintf("enstream-%s%s-%d.db", stamp, suffix, i)
	}
	return name
}

// Create writes a backup ("manual" or "auto") with VACUUM INTO: consistent even while in use.
func (m *Manager) Create(ctx context.Context, kind string) (Info, error) {
	if err := os.MkdirAll(m.Dir, 0o755); err != nil {
		return Info{}, err
	}
	suffix := ""
	if kind == "auto" {
		suffix = "-auto"
	}
	name := uniqueName(m.Dir, m.clock().Format("20060102-150405"), suffix)
	if err := database.Backup(ctx, m.DB, filepath.Join(m.Dir, name)); err != nil {
		return Info{}, err
	}
	return m.info(name)
}

func (m *Manager) info(name string) (Info, error) {
	fi, err := os.Stat(filepath.Join(m.Dir, name))
	if err != nil {
		return Info{}, ErrNotFound
	}
	created := fi.ModTime().Unix()
	if p := nameRE.FindStringSubmatch(name); p != nil {
		if t, err := time.ParseInLocation("20060102-150405", p[1], time.Local); err == nil {
			created = t.Unix()
		}
	}
	return Info{Name: name, Size: fi.Size(), CreatedAt: created, Kind: kindOf(name)}, nil
}

// List returns backups, newest first. Files that don't follow the naming scheme are ignored.
func (m *Manager) List() ([]Info, error) {
	entries, err := os.ReadDir(m.Dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Info{}, nil
	}
	if err != nil {
		return nil, err
	}
	out := []Info{}
	for _, e := range entries {
		if e.Type().IsRegular() && nameRE.MatchString(e.Name()) {
			if in, err := m.info(e.Name()); err == nil {
				out = append(out, in)
			}
		}
	}
	slices.SortFunc(out, func(a, b Info) int { return int(b.CreatedAt - a.CreatedAt) })
	return out, nil
}

// Path validates name (no traversal: it must match the naming scheme) and returns the file.
func (m *Manager) Path(name string) (string, error) {
	if !nameRE.MatchString(name) {
		return "", ErrNotFound
	}
	p := filepath.Join(m.Dir, name)
	if fi, err := os.Lstat(p); err != nil || !fi.Mode().IsRegular() {
		return "", ErrNotFound
	}
	return p, nil
}

func (m *Manager) Delete(name string) error {
	p, err := m.Path(name)
	if err != nil {
		return err
	}
	return os.Remove(p)
}

// Prune keeps the newest `keep` automatic backups.
func (m *Manager) Prune(keep int) error {
	list, err := m.List()
	if err != nil {
		return err
	}
	n := 0
	for _, b := range list {
		if b.Kind != "auto" {
			continue
		}
		if n++; n > keep {
			_ = os.Remove(filepath.Join(m.Dir, b.Name))
		}
	}
	return nil
}

// LastAuto is when the newest automatic backup was made (zero if none).
func (m *Manager) LastAuto() time.Time {
	list, _ := m.List()
	for _, b := range list {
		if b.Kind == "auto" {
			return time.Unix(b.CreatedAt, 0)
		}
	}
	return time.Time{}
}

// ------------------------------------------------------------------ restore

func (m *Manager) stagedPath() string { return m.DBPath + ".restore" }
func (m *Manager) markerPath() string { return m.DBPath + ".restore.json" }

// Verify opens a copy read-only and checks it is an intact ENStream database that this
// version understands.
func Verify(ctx context.Context, path string) error {
	db, err := sql.Open("sqlite", "file:"+filepath.ToSlash(path)+"?mode=ro")
	if err != nil {
		return ErrInvalid
	}
	defer db.Close()
	var check string
	if err := db.QueryRowContext(ctx, `PRAGMA quick_check`).Scan(&check); err != nil || check != "ok" {
		return ErrInvalid
	}
	var version int
	if err := db.QueryRowContext(ctx, `SELECT COALESCE(MAX(version), 0) FROM schema_migrations`).Scan(&version); err != nil || version < 1 {
		return ErrInvalid
	}
	if version > database.LatestVersion() {
		return fmt.Errorf("this backup comes from a newer ENStream (schema %d); update ENStream first", version)
	}
	var n int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name IN ('videos', 'media_folders')`).Scan(&n); err != nil || n != 2 {
		return ErrInvalid
	}
	return nil
}

// Stage prepares name to replace the database on the next start.
func (m *Manager) Stage(ctx context.Context, name string) error {
	src, err := m.Path(name)
	if err != nil {
		return err
	}
	if err := Verify(ctx, src); err != nil {
		return err
	}
	if err := copyFile(src, m.stagedPath()); err != nil {
		return err
	}
	b, _ := json.Marshal(map[string]any{"name": name, "staged_at": m.clock().Unix()})
	return os.WriteFile(m.markerPath(), b, 0o600)
}

// Pending returns the backup that will be restored on the next start ("" if none).
func (m *Manager) Pending() string {
	if _, err := os.Stat(m.stagedPath()); err != nil {
		return ""
	}
	var mk struct{ Name string }
	if b, err := os.ReadFile(m.markerPath()); err == nil {
		_ = json.Unmarshal(b, &mk)
	}
	if mk.Name == "" {
		mk.Name = "uploaded backup"
	}
	return mk.Name
}

func (m *Manager) CancelRestore() error {
	err1 := os.Remove(m.stagedPath())
	_ = os.Remove(m.markerPath())
	if errors.Is(err1, os.ErrNotExist) {
		return ErrNotFound
	}
	return err1
}

// ApplyPending runs at start-up before the database is opened. It returns the restored
// backup's name ("" when nothing was staged).
func ApplyPending(ctx context.Context, dbPath, backupsDir string) (string, error) {
	m := &Manager{Dir: backupsDir, DBPath: dbPath}
	name := m.Pending()
	if name == "" {
		return "", nil
	}
	staged := m.stagedPath()
	if err := Verify(ctx, staged); err != nil {
		os.Rename(staged, staged+".rejected")
		os.Remove(m.markerPath())
		return "", fmt.Errorf("staged restore rejected: %w", err)
	}
	// Keep the current database (including anything still in its WAL) as a backup.
	if _, err := os.Stat(dbPath); err == nil {
		cur, err := database.Open(dbPath)
		if err != nil {
			return "", fmt.Errorf("open current database before restore: %w", err)
		}
		if err := os.MkdirAll(backupsDir, 0o755); err != nil {
			cur.Close()
			return "", err
		}
		pre := filepath.Join(backupsDir, uniqueName(backupsDir, time.Now().Format("20060102-150405"), "-pre-restore"))
		err = database.Backup(ctx, cur, pre)
		cur.Close()
		if err != nil {
			return "", fmt.Errorf("back up current database before restore: %w", err)
		}
	}
	for _, suffix := range []string{"-wal", "-shm"} {
		_ = os.Remove(dbPath + suffix)
	}
	if err := os.Rename(staged, dbPath); err != nil {
		return "", err
	}
	os.Remove(m.markerPath())
	return name, nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return err
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}
