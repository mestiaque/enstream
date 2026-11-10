package repositories

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// Device identifies a browser by a random token it keeps in local storage.
type Device struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	UserAgent  string `json:"user_agent"`
	LastSeenAt int64  `json:"last_seen_at"`
	CreatedAt  int64  `json:"created_at"`
	Current    bool   `json:"current"`
}

// TouchDevice returns the device id for a token, creating the device on first use.
func (s *Store) TouchDevice(ctx context.Context, token, userAgent string) (int64, error) {
	t := now()
	if len(userAgent) > 300 {
		userAgent = userAgent[:300]
	}
	var id int64
	err := s.DB.QueryRowContext(ctx, `INSERT INTO devices (device_token, user_agent, last_seen_at, created_at) VALUES (?, ?, ?, ?)
		ON CONFLICT(device_token) DO UPDATE SET last_seen_at = excluded.last_seen_at, user_agent = excluded.user_agent RETURNING id`, token, userAgent, t, t).Scan(&id)
	return id, err
}

func (s *Store) Devices(ctx context.Context, currentToken string) ([]Device, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, device_name, user_agent, last_seen_at, created_at, device_token = ? FROM devices ORDER BY last_seen_at DESC, id DESC`, currentToken)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Device{}
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.UserAgent, &d.LastSeenAt, &d.CreatedAt, &d.Current); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (s *Store) RenameDevice(ctx context.Context, token, name string) error {
	res, err := s.DB.ExecContext(ctx, `UPDATE devices SET device_name = ? WHERE device_token = ?`, strings.TrimSpace(name), token)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) DeleteDevice(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM devices WHERE id = ?`, id)
	return err
}

// SaveProgress stores the shared position of a video (one row per video).
func (s *Store) SaveProgress(ctx context.Context, videoID int64, deviceID *int64, position, duration float64, completed bool) error {
	t := now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO playback_history (video_id, device_id, position, duration, completed, last_played_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(video_id) DO UPDATE SET device_id = excluded.device_id, position = excluded.position, duration = excluded.duration,
		completed = excluded.completed, last_played_at = excluded.last_played_at, updated_at = excluded.updated_at`,
		videoID, deviceID, position, duration, completed, t, t, t)
	return err
}

func (s *Store) DeleteHistory(ctx context.Context, videoID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM playback_history WHERE video_id = ?`, videoID)
	return err
}

func (s *Store) ClearHistory(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM playback_history`)
	return err
}

// HistoryEntry: a played video with its device.
type HistoryEntry struct {
	Video
	DeviceName string `json:"device_name"`
}

// History lists played videos, newest first. inProgress = only unfinished ones (Continue Watching).
func (s *Store) History(ctx context.Context, inProgress bool, limit, offset int) ([]HistoryEntry, int, error) {
	where := ` WHERE h.id IS NOT NULL`
	if inProgress {
		where += ` AND h.completed = 0 AND h.position > 0`
	}
	var total int
	if err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*)`+videoFrom+where).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT `+videoCols+`, COALESCE(NULLIF(d.device_name, ''), '')`+videoFrom+` LEFT JOIN devices d ON d.id = h.device_id`+where+
		` ORDER BY h.last_played_at DESC, v.id DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []HistoryEntry{}
	for rows.Next() {
		var e HistoryEntry
		var last sql.NullInt64
		v := &e.Video
		if err := rows.Scan(&v.ID, &v.FolderID, &v.FolderName, &v.Title, &v.FileName, &v.RelativePath, &v.Extension, &v.MimeType, &v.Container, &v.FileSize, &v.Duration,
			&v.Width, &v.Height, &v.FPS, &v.VideoCodec, &v.VideoProfile, &v.AudioCodec, &v.AudioChannels, &v.Bitrate, &v.Compatibility, &v.HasThumbnail,
			&v.SubtitleCount, &v.CreatedAt, &v.UpdatedAt, &v.Position, &v.Completed, &last, &e.DeviceName); err != nil {
			return nil, 0, err
		}
		if last.Valid {
			v.LastPlayedAt = &last.Int64
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}

func (s *Store) VideoExists(ctx context.Context, id int64) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM videos WHERE id = ?`, id).Scan(&n)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return n > 0, err
}
