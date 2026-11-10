package repositories

import (
	"context"
	"database/sql"
	"errors"
)

type ScanJob struct {
	ID             int64  `json:"id"`
	FolderID       *int64 `json:"folder_id"`
	Status         string `json:"status"` // running | completed | cancelled | failed
	TotalFiles     int    `json:"total_files"`
	ProcessedFiles int    `json:"processed_files"`
	SkippedFiles   int    `json:"skipped_files"`
	FailedFiles    int    `json:"failed_files"`
	NewFiles       int    `json:"new_files"`
	UpdatedFiles   int    `json:"updated_files"`
	RemovedFiles   int    `json:"removed_files"`
	CurrentFile    string `json:"current_file"`
	StartedAt      int64  `json:"started_at"`
	CompletedAt    *int64 `json:"completed_at"`
	ErrorMessage   string `json:"error_message"`
}

const scanCols = `id, folder_id, status, total_files, processed_files, skipped_files, failed_files, new_files, updated_files, removed_files, current_file, started_at, completed_at, error_message`

func scanJob(row interface{ Scan(...any) error }) (ScanJob, error) {
	var j ScanJob
	var folder, done sql.NullInt64
	err := row.Scan(&j.ID, &folder, &j.Status, &j.TotalFiles, &j.ProcessedFiles, &j.SkippedFiles, &j.FailedFiles, &j.NewFiles, &j.UpdatedFiles, &j.RemovedFiles, &j.CurrentFile, &j.StartedAt, &done, &j.ErrorMessage)
	if folder.Valid {
		j.FolderID = &folder.Int64
	}
	if done.Valid {
		j.CompletedAt = &done.Int64
	}
	return j, err
}

func (s *Store) CreateScanJob(ctx context.Context, folderID *int64) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `INSERT INTO scan_jobs (folder_id, status, started_at) VALUES (?, 'running', ?)`, folderID, now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) SaveScanJob(ctx context.Context, j ScanJob) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE scan_jobs SET status=?, total_files=?, processed_files=?, skipped_files=?, failed_files=?, new_files=?, updated_files=?,
		removed_files=?, current_file=?, completed_at=?, error_message=? WHERE id=?`,
		j.Status, j.TotalFiles, j.ProcessedFiles, j.SkippedFiles, j.FailedFiles, j.NewFiles, j.UpdatedFiles, j.RemovedFiles, j.CurrentFile, j.CompletedAt, j.ErrorMessage, j.ID)
	return err
}

func (s *Store) ScanJobs(ctx context.Context, limit int) ([]ScanJob, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+scanCols+` FROM scan_jobs ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ScanJob{}
	for rows.Next() {
		j, err := scanJob(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, j)
	}
	return out, rows.Err()
}

func (s *Store) ScanJob(ctx context.Context, id int64) (ScanJob, error) {
	j, err := scanJob(s.DB.QueryRowContext(ctx, `SELECT `+scanCols+` FROM scan_jobs WHERE id = ?`, id))
	if errors.Is(err, sql.ErrNoRows) {
		return j, ErrNotFound
	}
	return j, err
}

// MarkInterruptedScans: jobs still "running" after a restart were cut off by the shutdown.
func (s *Store) MarkInterruptedScans(ctx context.Context) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE scan_jobs SET status = 'cancelled', completed_at = ?, error_message = 'ENStream was stopped during the scan' WHERE status = 'running'`, now())
	return err
}
