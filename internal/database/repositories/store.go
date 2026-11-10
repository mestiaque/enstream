// Package repositories holds every SQL statement ENStream runs. Queries take parameters only;
// user input never becomes SQL text.
package repositories

import (
	"context"
	"database/sql"
	"errors"
	"time"
)

var ErrNotFound = errors.New("not found")

type Store struct{ DB *sql.DB }

func New(db *sql.DB) *Store { return &Store{DB: db} }

func now() int64 { return time.Now().Unix() }

// --------------------------------------------------------------- folders

type Folder struct {
	ID            int64  `json:"id"`
	Path          string `json:"path"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	LastScannedAt *int64 `json:"last_scanned_at"`
	CreatedAt     int64  `json:"created_at"`
	VideoCount    int    `json:"video_count"`
	TotalSize     int64  `json:"total_size"`
}

const folderCols = `f.id, f.path, f.name, f.enabled, f.last_scanned_at, f.created_at,
	(SELECT COUNT(*) FROM videos v WHERE v.folder_id = f.id), (SELECT COALESCE(SUM(file_size), 0) FROM videos v WHERE v.folder_id = f.id)`

func scanFolder(row interface{ Scan(...any) error }) (Folder, error) {
	var f Folder
	var last sql.NullInt64
	err := row.Scan(&f.ID, &f.Path, &f.Name, &f.Enabled, &last, &f.CreatedAt, &f.VideoCount, &f.TotalSize)
	if last.Valid {
		f.LastScannedAt = &last.Int64
	}
	return f, err
}

func (s *Store) Folders(ctx context.Context) ([]Folder, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+folderCols+` FROM media_folders f ORDER BY f.name COLLATE NOCASE`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Folder{}
	for rows.Next() {
		f, err := scanFolder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

func (s *Store) Folder(ctx context.Context, id int64) (Folder, error) {
	f, err := scanFolder(s.DB.QueryRowContext(ctx, `SELECT `+folderCols+` FROM media_folders f WHERE f.id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func (s *Store) AddFolder(ctx context.Context, path, name string) (int64, error) {
	t := now()
	res, err := s.DB.ExecContext(ctx, `INSERT INTO media_folders (path, name, created_at, updated_at) VALUES (?, ?, ?, ?)`, path, name, t, t)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// DeleteFolder removes the folder and its index entries (never files on disk).
func (s *Store) DeleteFolder(ctx context.Context, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM media_folders WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) TouchFolderScanned(ctx context.Context, id int64) error {
	t := now()
	_, err := s.DB.ExecContext(ctx, `UPDATE media_folders SET last_scanned_at = ?, updated_at = ? WHERE id = ?`, t, t, id)
	return err
}

// FolderThumbnails lists thumbnail files of a folder's videos (to delete with the folder).
func (s *Store) FolderThumbnails(ctx context.Context, id int64) ([]string, error) {
	return s.strings(ctx, `SELECT thumbnail_path FROM videos WHERE folder_id = ? AND thumbnail_path != ''`, id)
}

func (s *Store) strings(ctx context.Context, q string, args ...any) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// --------------------------------------------------------------- videos (index state)

// Known is what the scanner compares to decide whether a file changed.
type Known struct {
	ID            int64
	Size          int64
	Modified      int64
	ThumbnailPath string
	ProbeError    string
	Compatibility string
}

func (s *Store) KnownVideos(ctx context.Context, folderID int64) (map[string]Known, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, file_path, file_size, file_modified_at, thumbnail_path, probe_error, compatibility FROM videos WHERE folder_id = ?`, folderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Known{}
	for rows.Next() {
		var k Known
		var path string
		if err := rows.Scan(&k.ID, &path, &k.Size, &k.Modified, &k.ThumbnailPath, &k.ProbeError, &k.Compatibility); err != nil {
			return nil, err
		}
		out[path] = k
	}
	return out, rows.Err()
}

// VideoRecord is everything the scanner writes for one file.
type VideoRecord struct {
	FolderID      int64
	Title         string
	FileName      string
	FilePath      string
	RelativePath  string
	Extension     string
	MimeType      string
	Container     string
	FileSize      int64
	Duration      float64
	Width, Height int
	FPS           float64
	VideoCodec    string
	VideoProfile  string
	PixelFormat   string
	AudioCodec    string
	AudioChannels int
	Bitrate       int64
	Compatibility string
	ProbeError    string
	Modified      int64
}

// UpsertVideo inserts or updates by file path and returns the id and whether it was new.
func (s *Store) UpsertVideo(ctx context.Context, v VideoRecord) (int64, bool, error) {
	t := now()
	var id int64
	err := s.DB.QueryRowContext(ctx, `SELECT id FROM videos WHERE file_path = ?`, v.FilePath).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO videos (folder_id, title, file_name, file_path, relative_path, extension, mime_type, container_format, file_size,
			duration, width, height, fps, video_codec, video_profile, pixel_format, audio_codec, audio_channels, bitrate, compatibility, probe_error,
			file_modified_at, scanned_at, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			v.FolderID, v.Title, v.FileName, v.FilePath, v.RelativePath, v.Extension, v.MimeType, v.Container, v.FileSize,
			v.Duration, v.Width, v.Height, v.FPS, v.VideoCodec, v.VideoProfile, v.PixelFormat, v.AudioCodec, v.AudioChannels, v.Bitrate, v.Compatibility, v.ProbeError,
			v.Modified, t, t, t)
		if err != nil {
			return 0, false, err
		}
		id, err = res.LastInsertId()
		return id, true, err
	}
	if err != nil {
		return 0, false, err
	}
	_, err = s.DB.ExecContext(ctx, `UPDATE videos SET folder_id=?, title=?, file_name=?, relative_path=?, extension=?, mime_type=?, container_format=?, file_size=?,
		duration=?, width=?, height=?, fps=?, video_codec=?, video_profile=?, pixel_format=?, audio_codec=?, audio_channels=?, bitrate=?, compatibility=?, probe_error=?,
		file_modified_at=?, scanned_at=?, updated_at=? WHERE id=?`,
		v.FolderID, v.Title, v.FileName, v.RelativePath, v.Extension, v.MimeType, v.Container, v.FileSize,
		v.Duration, v.Width, v.Height, v.FPS, v.VideoCodec, v.VideoProfile, v.PixelFormat, v.AudioCodec, v.AudioChannels, v.Bitrate, v.Compatibility, v.ProbeError,
		v.Modified, t, t, id)
	return id, false, err
}

func (s *Store) SetThumbnail(ctx context.Context, id int64, path, errMsg string) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE videos SET thumbnail_path = ?, thumbnail_error = ? WHERE id = ?`, path, errMsg, id)
	return err
}

// DeleteVideos removes index entries (files that disappeared from disk).
func (s *Store) DeleteVideos(ctx context.Context, ids []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `DELETE FROM videos WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, id := range ids {
		if _, err := stmt.ExecContext(ctx, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// --------------------------------------------------------------- subtitles

type Subtitle struct {
	ID       int64  `json:"id"`
	VideoID  int64  `json:"-"`
	FilePath string `json:"-"`
	Language string `json:"language"`
	Format   string `json:"format"`
	Label    string `json:"label"`
}

// ReplaceSubtitles sets a video's subtitle list (one transaction).
func (s *Store) ReplaceSubtitles(ctx context.Context, videoID int64, subs []Subtitle) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `DELETE FROM subtitles WHERE video_id = ?`, videoID); err != nil {
		return err
	}
	for _, sb := range subs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO subtitles (video_id, file_path, language, format, label, created_at) VALUES (?, ?, ?, ?, ?, ?)
			ON CONFLICT(file_path) DO UPDATE SET video_id = excluded.video_id, language = excluded.language, format = excluded.format, label = excluded.label`,
			videoID, sb.FilePath, sb.Language, sb.Format, sb.Label, now()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Subtitles(ctx context.Context, videoID int64) ([]Subtitle, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, video_id, file_path, language, format, label FROM subtitles WHERE video_id = ? ORDER BY label`, videoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Subtitle{}
	for rows.Next() {
		var sb Subtitle
		if err := rows.Scan(&sb.ID, &sb.VideoID, &sb.FilePath, &sb.Language, &sb.Format, &sb.Label); err != nil {
			return nil, err
		}
		out = append(out, sb)
	}
	return out, rows.Err()
}

func (s *Store) Subtitle(ctx context.Context, videoID, id int64) (Subtitle, error) {
	var sb Subtitle
	err := s.DB.QueryRowContext(ctx, `SELECT id, video_id, file_path, language, format, label FROM subtitles WHERE id = ? AND video_id = ?`, id, videoID).
		Scan(&sb.ID, &sb.VideoID, &sb.FilePath, &sb.Language, &sb.Format, &sb.Label)
	if errors.Is(err, sql.ErrNoRows) {
		return sb, ErrNotFound
	}
	return sb, err
}

// --------------------------------------------------------------- scan failures

func (s *Store) RecordFailure(ctx context.Context, folderID int64, path, msg string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO scan_failures (folder_id, file_path, error, failed_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(file_path) DO UPDATE SET error = excluded.error, failed_at = excluded.failed_at`, folderID, path, msg, now())
	return err
}

func (s *Store) ClearFailure(ctx context.Context, path string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM scan_failures WHERE file_path = ?`, path)
	return err
}

// FailedPaths: files that failed last time, per folder (retried by "Retry failed items").
func (s *Store) FailedPaths(ctx context.Context, folderID int64) (map[string]bool, error) {
	paths, err := s.strings(ctx, `SELECT file_path FROM scan_failures WHERE folder_id = ?`, folderID)
	m := map[string]bool{}
	for _, p := range paths {
		m[p] = true
	}
	return m, err
}

func (s *Store) FailureCount(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM scan_failures`).Scan(&n)
	return n, err
}
