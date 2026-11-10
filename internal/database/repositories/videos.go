package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Video is the public view of an indexed file. The absolute path is not part of it.
type Video struct {
	ID            int64   `json:"id"`
	FolderID      int64   `json:"folder_id"`
	FolderName    string  `json:"folder_name"`
	Title         string  `json:"title"`
	FileName      string  `json:"file_name"`
	RelativePath  string  `json:"relative_path"`
	Extension     string  `json:"extension"`
	MimeType      string  `json:"mime_type"`
	Container     string  `json:"container_format"`
	FileSize      int64   `json:"file_size"`
	Duration      float64 `json:"duration"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	FPS           float64 `json:"fps"`
	VideoCodec    string  `json:"video_codec"`
	VideoProfile  string  `json:"video_profile"`
	AudioCodec    string  `json:"audio_codec"`
	AudioChannels int     `json:"audio_channels"`
	Bitrate       int64   `json:"bitrate"`
	Compatibility string  `json:"compatibility"`
	HasThumbnail  bool    `json:"has_thumbnail"`
	SubtitleCount int     `json:"subtitle_count"`
	CreatedAt     int64   `json:"created_at"`
	UpdatedAt     int64   `json:"updated_at"`
	// Playback (shared across devices)
	Position     float64 `json:"position"`
	Completed    bool    `json:"completed"`
	LastPlayedAt *int64  `json:"last_played_at"`
}

const videoCols = `v.id, v.folder_id, f.name, v.title, v.file_name, v.relative_path, v.extension, v.mime_type, v.container_format, v.file_size, v.duration,
	v.width, v.height, v.fps, v.video_codec, v.video_profile, v.audio_codec, v.audio_channels, v.bitrate, v.compatibility, v.thumbnail_path != '',
	(SELECT COUNT(*) FROM subtitles s WHERE s.video_id = v.id), v.created_at, v.updated_at,
	COALESCE(h.position, 0), COALESCE(h.completed, 0), h.last_played_at`

// videoFrom joins the history of one device: its first placeholder is that device's id
// (0 = an unknown browser, which then has no history).
const videoFrom = ` FROM videos v JOIN media_folders f ON f.id = v.folder_id LEFT JOIN playback_history h ON h.video_id = v.id AND h.device_id = ?`

func scanVideo(row interface{ Scan(...any) error }) (Video, error) {
	var v Video
	var last sql.NullInt64
	err := row.Scan(&v.ID, &v.FolderID, &v.FolderName, &v.Title, &v.FileName, &v.RelativePath, &v.Extension, &v.MimeType, &v.Container, &v.FileSize, &v.Duration,
		&v.Width, &v.Height, &v.FPS, &v.VideoCodec, &v.VideoProfile, &v.AudioCodec, &v.AudioChannels, &v.Bitrate, &v.Compatibility, &v.HasThumbnail,
		&v.SubtitleCount, &v.CreatedAt, &v.UpdatedAt, &v.Position, &v.Completed, &last)
	if last.Valid {
		v.LastPlayedAt = &last.Int64
	}
	return v, err
}

// VideoQuery: the library filters. Zero values mean "any".
type VideoQuery struct {
	// DeviceID selects whose history (positions, watched marks, filters) is shown.
	DeviceID    int64
	Page, Limit int
	Search      string
	FolderID    int64
	Extension   string // ".mkv"
	Resolution  string // "4k" | "1080p" | "720p" | "sd"
	Watched     string // "watched" | "unwatched" | "in_progress"
	RecentDays  int    // added in the last N days
	Sort        string // allowlisted below
	Desc        bool
}

// Sort keys map to fixed SQL; anything else falls back to the title.
var sortColumns = map[string]string{
	"title":          "v.title COLLATE NOCASE",
	"created_at":     "v.created_at",
	"updated_at":     "v.updated_at",
	"file_size":      "v.file_size",
	"duration":       "v.duration",
	"last_played_at": "COALESCE(h.last_played_at, 0)",
}

func ValidSort(s string) bool { _, ok := sortColumns[s]; return ok }

func (q VideoQuery) where() (string, []any) {
	var conds []string
	var args []any
	if s := strings.TrimSpace(q.Search); s != "" {
		like := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s) + "%"
		conds = append(conds, `(v.title LIKE ? ESCAPE '\' OR v.relative_path LIKE ? ESCAPE '\')`)
		args = append(args, like, like)
	}
	if q.FolderID > 0 {
		conds = append(conds, "v.folder_id = ?")
		args = append(args, q.FolderID)
	}
	if q.Extension != "" {
		conds = append(conds, "v.extension = ?")
		args = append(args, strings.ToLower(q.Extension))
	}
	switch q.Resolution {
	case "4k":
		conds = append(conds, "v.height >= 2000")
	case "1080p":
		conds = append(conds, "v.height >= 1000 AND v.height < 2000")
	case "720p":
		conds = append(conds, "v.height >= 700 AND v.height < 1000")
	case "sd":
		conds = append(conds, "v.height > 0 AND v.height < 700")
	}
	switch q.Watched {
	case "watched":
		conds = append(conds, "h.completed = 1")
	case "unwatched":
		conds = append(conds, "h.id IS NULL")
	case "in_progress":
		conds = append(conds, "h.completed = 0 AND h.position > 0")
	}
	if q.RecentDays > 0 {
		conds = append(conds, "v.created_at >= ?")
		args = append(args, now()-int64(q.RecentDays)*86400)
	}
	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

// Videos returns one page and the total count.
func (s *Store) Videos(ctx context.Context, q VideoQuery) ([]Video, int, error) {
	where, args := q.where()
	args = append([]any{q.DeviceID}, args...)
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+videoFrom+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	col, ok := sortColumns[q.Sort]
	if !ok {
		col = sortColumns["title"]
	}
	dir := "ASC"
	if q.Desc {
		dir = "DESC"
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+videoCols+videoFrom+where+` ORDER BY `+col+` `+dir+`, v.id `+dir+` LIMIT ? OFFSET ?`,
		append(args, q.Limit, (q.Page-1)*q.Limit)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []Video{}
	for rows.Next() {
		v, err := scanVideo(rows)
		if err != nil {
			return nil, 0, err
		}
		out = append(out, v)
	}
	return out, total, rows.Err()
}

// Video returns one video with the given device's position and watched mark.
func (s *Store) Video(ctx context.Context, id, deviceID int64) (Video, error) {
	v, err := scanVideo(s.DB.QueryRowContext(ctx, `SELECT `+videoCols+videoFrom+` WHERE v.id = ?`, deviceID, id))
	if errors.Is(err, sql.ErrNoRows) {
		return v, ErrNotFound
	}
	return v, err
}

// VideoFile is what streaming needs: the stored path and its folder root.
type VideoFile struct {
	Path, FolderPath, MimeType, ThumbnailPath string
}

func (s *Store) VideoFile(ctx context.Context, id int64) (VideoFile, error) {
	var f VideoFile
	err := s.DB.QueryRowContext(ctx, `SELECT v.file_path, mf.path, v.mime_type, v.thumbnail_path FROM videos v JOIN media_folders mf ON mf.id = v.folder_id WHERE v.id = ? AND mf.enabled = 1`, id).
		Scan(&f.Path, &f.FolderPath, &f.MimeType, &f.ThumbnailPath)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

// Extensions present in the library (for the format filter).
func (s *Store) Extensions(ctx context.Context) ([]string, error) {
	return s.strings(ctx, `SELECT DISTINCT extension FROM videos ORDER BY extension`)
}

// LibraryStats for the dashboard.
type LibraryStats struct {
	Videos   int   `json:"videos"`
	Size     int64 `json:"total_size"`
	Duration int64 `json:"total_duration"`
	Folders  int   `json:"folders"`
}

func (s *Store) Stats(ctx context.Context) (LibraryStats, error) {
	var st LibraryStats
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(SUM(file_size), 0), CAST(COALESCE(SUM(duration), 0) AS INTEGER), (SELECT COUNT(*) FROM media_folders) FROM videos`).
		Scan(&st.Videos, &st.Size, &st.Duration, &st.Folders)
	return st, err
}
