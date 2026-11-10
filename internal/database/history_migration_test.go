package database

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"
)

// Upgrading a database from the shared history (schema 2) to per-device history (schema 3)
// keeps every row that has a device and drops rows nobody can own.
func TestHistoryMigratesToPerDevice(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "old.db")
	raw, err := sql.Open("sqlite", "file:"+path+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	ms, err := loadMigrations()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := raw.Exec(`CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, name TEXT NOT NULL, applied_at INTEGER NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	for _, m := range ms[:2] { // the schema as it was before per-device history
		if _, err := raw.Exec(m.sql); err != nil {
			t.Fatal(err)
		}
		raw.Exec(`INSERT INTO schema_migrations VALUES (?, ?, ?)`, m.version, m.name, time.Now().Unix())
	}
	for _, q := range []string{
		`INSERT INTO media_folders (id, path, name, created_at, updated_at) VALUES (1, '/m', 'm', 0, 0)`,
		`INSERT INTO videos (id, folder_id, title, file_name, file_path, relative_path, extension, mime_type, file_size, file_modified_at, scanned_at, created_at, updated_at)
			VALUES (1, 1, 'a', 'a', '/m/a.mp4', 'a.mp4', '.mp4', 'video/mp4', 1, 0, 0, 0, 0), (2, 1, 'b', 'b', '/m/b.mp4', 'b.mp4', '.mp4', 'video/mp4', 1, 0, 0, 0, 0)`,
		`INSERT INTO devices (id, device_token, last_seen_at, created_at) VALUES (7, 'phone-token-0000000', 0, 0)`,
		`INSERT INTO playback_history (video_id, device_id, position, duration, completed, last_played_at, created_at, updated_at)
			VALUES (1, 7, 42, 100, 0, 5, 5, 5), (2, NULL, 10, 100, 0, 6, 6, 6)`,
	} {
		if _, err := raw.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	raw.Close()

	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if v, _ := SchemaVersion(ctx, db); v != 3 {
		t.Fatalf("schema %d", v)
	}
	var n int
	var pos float64
	db.QueryRow(`SELECT COUNT(*) FROM playback_history`).Scan(&n)
	db.QueryRow(`SELECT position FROM playback_history WHERE device_id = 7 AND video_id = 1`).Scan(&pos)
	if n != 1 || pos != 42 {
		t.Fatalf("rows %d, position %v", n, pos)
	}
	// Two devices can now each have the same video in their history.
	if _, err := db.Exec(`INSERT INTO devices (id, device_token, last_seen_at, created_at) VALUES (8, 'tv-token-000000000', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO playback_history (video_id, device_id, position, duration, completed, last_played_at, created_at, updated_at) VALUES (1, 8, 3, 100, 0, 7, 7, 7)`); err != nil {
		t.Fatal(err)
	}
	// Forgetting a device removes its history.
	if _, err := db.Exec(`DELETE FROM devices WHERE id = 7`); err != nil {
		t.Fatal(err)
	}
	db.QueryRow(`SELECT COUNT(*) FROM playback_history`).Scan(&n)
	if n != 1 {
		t.Fatalf("history of a forgotten device stayed: %d rows", n)
	}
}
