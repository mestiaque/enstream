-- ENStream initial schema. Timestamps are Unix seconds (UTC). Paths are filesystem paths,
-- never URLs; video files and thumbnails are never stored in the database.

CREATE TABLE media_folders (
    id              INTEGER PRIMARY KEY,
    path            TEXT    NOT NULL UNIQUE,
    name            TEXT    NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    last_scanned_at INTEGER,
    created_at      INTEGER NOT NULL,
    updated_at      INTEGER NOT NULL
);

CREATE TABLE videos (
    id               INTEGER PRIMARY KEY,
    folder_id        INTEGER NOT NULL REFERENCES media_folders(id) ON DELETE CASCADE,
    title            TEXT    NOT NULL,
    file_name        TEXT    NOT NULL,
    file_path        TEXT    NOT NULL UNIQUE,
    relative_path    TEXT    NOT NULL,          -- inside its folder, for display and grouping
    extension        TEXT    NOT NULL,
    mime_type        TEXT    NOT NULL,
    container_format TEXT    NOT NULL DEFAULT '',
    file_size        INTEGER NOT NULL,
    duration         REAL    NOT NULL DEFAULT 0, -- seconds; 0 = unknown
    width            INTEGER NOT NULL DEFAULT 0,
    height           INTEGER NOT NULL DEFAULT 0,
    fps              REAL    NOT NULL DEFAULT 0,
    video_codec      TEXT    NOT NULL DEFAULT '',
    video_profile    TEXT    NOT NULL DEFAULT '',
    pixel_format     TEXT    NOT NULL DEFAULT '',
    audio_codec      TEXT    NOT NULL DEFAULT '',
    audio_channels   INTEGER NOT NULL DEFAULT 0,
    bitrate          INTEGER NOT NULL DEFAULT 0,
    compatibility    TEXT    NOT NULL DEFAULT 'unknown', -- likely | possibly | transcode | unknown
    probe_error      TEXT    NOT NULL DEFAULT '',
    thumbnail_path   TEXT    NOT NULL DEFAULT '',
    thumbnail_error  TEXT    NOT NULL DEFAULT '',
    file_modified_at INTEGER NOT NULL,
    scanned_at       INTEGER NOT NULL,
    created_at       INTEGER NOT NULL,
    updated_at       INTEGER NOT NULL
);
CREATE INDEX videos_folder ON videos(folder_id);
CREATE INDEX videos_title ON videos(title COLLATE NOCASE);
CREATE INDEX videos_created ON videos(created_at DESC);
CREATE INDEX videos_size ON videos(file_size);
CREATE INDEX videos_duration ON videos(duration);
CREATE INDEX videos_height ON videos(height);
CREATE INDEX videos_extension ON videos(extension);

CREATE TABLE devices (
    id           INTEGER PRIMARY KEY,
    device_token TEXT    NOT NULL UNIQUE,
    device_name  TEXT    NOT NULL DEFAULT '',
    user_agent   TEXT    NOT NULL DEFAULT '',
    last_seen_at INTEGER NOT NULL,
    created_at   INTEGER NOT NULL
);

-- One row per video: the shared position lets playback resume on another device.
-- device_id records which device played it last.
CREATE TABLE playback_history (
    id             INTEGER PRIMARY KEY,
    video_id       INTEGER NOT NULL UNIQUE REFERENCES videos(id) ON DELETE CASCADE,
    device_id      INTEGER REFERENCES devices(id) ON DELETE SET NULL,
    position       REAL    NOT NULL DEFAULT 0,
    duration       REAL    NOT NULL DEFAULT 0,
    completed      INTEGER NOT NULL DEFAULT 0,
    last_played_at INTEGER NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL
);
CREATE INDEX playback_last_played ON playback_history(last_played_at DESC);

CREATE TABLE subtitles (
    id         INTEGER PRIMARY KEY,
    video_id   INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    file_path  TEXT    NOT NULL UNIQUE,
    language   TEXT    NOT NULL DEFAULT '',
    format     TEXT    NOT NULL,
    label      TEXT    NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE INDEX subtitles_video ON subtitles(video_id);

CREATE TABLE scan_jobs (
    id              INTEGER PRIMARY KEY,
    folder_id       INTEGER REFERENCES media_folders(id) ON DELETE SET NULL, -- NULL = all folders
    status          TEXT    NOT NULL, -- running | completed | cancelled | failed
    total_files     INTEGER NOT NULL DEFAULT 0,
    processed_files INTEGER NOT NULL DEFAULT 0,
    skipped_files   INTEGER NOT NULL DEFAULT 0,
    failed_files    INTEGER NOT NULL DEFAULT 0,
    new_files       INTEGER NOT NULL DEFAULT 0,
    updated_files   INTEGER NOT NULL DEFAULT 0,
    removed_files   INTEGER NOT NULL DEFAULT 0,
    current_file    TEXT    NOT NULL DEFAULT '',
    started_at      INTEGER NOT NULL,
    completed_at    INTEGER,
    error_message   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX scan_jobs_started ON scan_jobs(started_at DESC);

-- Files that failed during a scan, for "Retry failed items".
CREATE TABLE scan_failures (
    id         INTEGER PRIMARY KEY,
    folder_id  INTEGER NOT NULL REFERENCES media_folders(id) ON DELETE CASCADE,
    file_path  TEXT    NOT NULL UNIQUE,
    error      TEXT    NOT NULL,
    failed_at  INTEGER NOT NULL
);

CREATE TABLE settings (
    id         INTEGER PRIMARY KEY,
    key        TEXT    NOT NULL UNIQUE,
    value      TEXT    NOT NULL,
    updated_at INTEGER NOT NULL
);
