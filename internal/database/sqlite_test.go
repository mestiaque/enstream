package database

import (
	"context"
	"path/filepath"
	"testing"
)

func TestOpenMigratesAndEnforcesForeignKeys(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "t.db")
	db, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if v, err := SchemaVersion(ctx, db); err != nil || v != 3 {
		t.Fatalf("schema version %d %v", v, err)
	}
	var mode string
	if err := db.QueryRow(`PRAGMA journal_mode`).Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal mode %q %v", mode, err)
	}
	// A video needs an existing folder.
	_, err = db.Exec(`INSERT INTO videos (folder_id, title, file_name, file_path, relative_path, extension, mime_type, file_size, file_modified_at, scanned_at, created_at, updated_at)
		VALUES (99, 't', 'f', '/x', 'x', '.mp4', 'video/mp4', 1, 0, 0, 0, 0)`)
	if err == nil {
		t.Fatal("foreign keys are not enforced")
	}
	// Deleting a folder removes its videos (index only; files are untouched elsewhere).
	if _, err := db.Exec(`INSERT INTO media_folders (id, path, name, created_at, updated_at) VALUES (1, '/m', 'm', 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO videos (folder_id, title, file_name, file_path, relative_path, extension, mime_type, file_size, file_modified_at, scanned_at, created_at, updated_at)
		VALUES (1, 't', 'f', '/m/f.mp4', 'f.mp4', '.mp4', 'video/mp4', 1, 0, 0, 0, 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`DELETE FROM media_folders WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM videos`).Scan(&n)
	if n != 0 {
		t.Fatal("cascade delete")
	}
	db.Close()

	// Reopening keeps data and does not re-run migrations.
	db, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.QueryRow(`SELECT COUNT(*) FROM schema_migrations`).Scan(&n)
	if n != 3 {
		t.Fatalf("migrations recorded %d times", n)
	}
	if err := Backup(ctx, db, filepath.Join(t.TempDir(), "backup.db")); err != nil {
		t.Fatal(err)
	}
}
