-- Playback history per device: each phone, TV and computer keeps its own history, Continue
-- Watching, resume positions and watched marks (before, one row per video was shared by all).
-- Existing rows stay with the device that played them last; rows without a device are dropped.
-- Forgetting a device removes its history.
CREATE TABLE playback_history_new (
    id             INTEGER PRIMARY KEY,
    video_id       INTEGER NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    device_id      INTEGER NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
    position       REAL    NOT NULL DEFAULT 0,
    duration       REAL    NOT NULL DEFAULT 0,
    completed      INTEGER NOT NULL DEFAULT 0,
    last_played_at INTEGER NOT NULL,
    created_at     INTEGER NOT NULL,
    updated_at     INTEGER NOT NULL,
    UNIQUE (device_id, video_id)
);
INSERT INTO playback_history_new (id, video_id, device_id, position, duration, completed, last_played_at, created_at, updated_at)
    SELECT id, video_id, device_id, position, duration, completed, last_played_at, created_at, updated_at
    FROM playback_history WHERE device_id IS NOT NULL;
DROP TABLE playback_history;
ALTER TABLE playback_history_new RENAME TO playback_history;
CREATE INDEX playback_device_last_played ON playback_history(device_id, last_played_at DESC);
CREATE INDEX playback_video ON playback_history(video_id);
