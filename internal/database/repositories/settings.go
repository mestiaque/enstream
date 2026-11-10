package repositories

import (
	"context"
	"database/sql"
	"errors"
)

// ------------------------------------------------------------------ key/value settings

func (s *Store) Setting(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.DB.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&v)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	return v, err == nil, err
}

func (s *Store) SetSetting(ctx context.Context, key, value string) error {
	_, err := s.DB.ExecContext(ctx, `INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, key, value, now())
	return err
}

func (s *Store) DeleteSetting(ctx context.Context, key string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM settings WHERE key = ?`, key)
	return err
}

// ------------------------------------------------------------------ sessions

type Session struct {
	ID         int64  `json:"id"`
	UserAgent  string `json:"user_agent"`
	IP         string `json:"ip"`
	CreatedAt  int64  `json:"created_at"`
	LastSeenAt int64  `json:"last_seen_at"`
	ExpiresAt  int64  `json:"expires_at"`
	Current    bool   `json:"current"`
}

func (s *Store) CreateSession(ctx context.Context, tokenHash, ua, ip string, expires int64) error {
	t := now()
	_, err := s.DB.ExecContext(ctx, `INSERT INTO sessions (token_hash, user_agent, ip, created_at, last_seen_at, expires_at) VALUES (?, ?, ?, ?, ?, ?)`,
		tokenHash, ua, ip, t, t, expires)
	return err
}

// SessionByHash returns the live session for a token hash (expired ones count as missing).
func (s *Store) SessionByHash(ctx context.Context, tokenHash string) (Session, error) {
	var x Session
	err := s.DB.QueryRowContext(ctx, `SELECT id, user_agent, ip, created_at, last_seen_at, expires_at FROM sessions WHERE token_hash = ? AND expires_at > ?`, tokenHash, now()).
		Scan(&x.ID, &x.UserAgent, &x.IP, &x.CreatedAt, &x.LastSeenAt, &x.ExpiresAt)
	if errors.Is(err, sql.ErrNoRows) {
		return x, ErrNotFound
	}
	return x, err
}

func (s *Store) TouchSession(ctx context.Context, id int64, ip string, expires int64) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE sessions SET last_seen_at = ?, ip = ?, expires_at = ? WHERE id = ?`, now(), ip, expires, id)
	return err
}

func (s *Store) Sessions(ctx context.Context) ([]Session, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id, user_agent, ip, created_at, last_seen_at, expires_at FROM sessions WHERE expires_at > ? ORDER BY last_seen_at DESC`, now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Session{}
	for rows.Next() {
		var x Session
		if err := rows.Scan(&x.ID, &x.UserAgent, &x.IP, &x.CreatedAt, &x.LastSeenAt, &x.ExpiresAt); err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSessionByHash(ctx context.Context, tokenHash string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = ?`, tokenHash)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, id int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteSessionsExcept signs out everyone but keepID (0 = everyone).
func (s *Store) DeleteSessionsExcept(ctx context.Context, keepID int64) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE id != ?`, keepID)
	return err
}

func (s *Store) PurgeExpiredSessions(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at <= ?`, now())
	return err
}
