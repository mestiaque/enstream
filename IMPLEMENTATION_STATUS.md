# ENStream implementation status

ENStream is the plan's "StreamVault", named to match the EN apps (ENViewer, ENSave, ENPlayer,
ENPost). Legend: ✅ implemented and tested · 🔶 implemented, not fully tested · ⬜ not started.

## Phase 1: Foundation ✅

- ✅ Go module, single binary (`cmd/enstream`), pure-Go SQLite (no CGO), Linux and Windows builds
- ✅ Configuration: `config.json` in the per-user data folder, defaults, validation, atomic save, invalid files are reported, never replaced
- ✅ Data folder layout: thumbnails, subtitle cache, logs, backups (`ENSTREAM_DATA_DIR` / `-data` override)
- ✅ SQLite: WAL, foreign keys, busy timeout, small pool, embedded versioned migrations, backup (`VACUUM INTO`)
- ✅ Full schema: media_folders, videos, playback_history, subtitles, scan_jobs, scan_failures, settings, devices
- ✅ LAN discovery: active IPv4 on Wi-Fi and Ethernet, private first; skips loopback, link-local, Docker/VM bridges
- ✅ Network guard: only this computer and private networks (configurable); X-Forwarded-For ignored
- ✅ HTTP: chi router, request ids, structured access log (no query strings), panic recovery, security headers + CSP, 1 MB body limit, JSON envelope
- ✅ Embedded UI via `embed.FS`, versioned asset URLs, SPA fallback, no directory listings
- ✅ FFmpeg/FFprobe discovery and version, context + timeout runner, no shell
- ✅ Startup banner with URLs and dependency status; graceful shutdown on Ctrl+C
- ✅ Tests: config, database, network, security guard, app integration (go test, go vet, -race)

## Phase 2: Media engine ✅

- ✅ Folders API: list, add (absolute, existing, readable, symlinks resolved; nested/overlapping folders refused), get, remove (index and thumbnails only, files untouched; refused while that folder scans); adding a folder starts its scan
- ✅ Discovery: recursive, case-insensitive extensions, hidden and excluded folders skipped, temp/partial files ignored, directory symlinks not followed, file symlinks only when they stay inside the folder, duplicates by real path removed
- ✅ Incremental scanner: size + modification time; new, changed, previously failed and not-yet-probed files are processed, the rest skipped; deleted files leave the index (with thumbnails); an unavailable folder (unplugged drive) keeps its index
- ✅ Bounded worker pool (worker_count, default 3), one scan at a time, cancel, live progress (total, processed, skipped, failed, new, updated, removed, current file), persisted every second, interrupted scans marked on restart, scan history, retry failed items
- ✅ FFprobe metadata: container, duration, size, resolution, fps, codecs, profile, pixel format, bitrate, audio channels, embedded subtitles; cover art ignored; missing values tolerated
- ✅ WebP thumbnails: ~10% in (2 s to 5 min), representative frame, fallback to the first frame, atomic write, named by video id, stored in the data folder
- ✅ External subtitles (.srt .vtt .ass .ssa) matched by name with language detection
- ✅ Browser compatibility rating: likely / possibly / transcode / unknown
- ✅ Videos API: pagination (max 100), search (escaped LIKE), folder, format, resolution, watched, recently added filters, allowlisted sort; details with subtitles; thumbnails served only from the thumbnails folder; no absolute paths in video responses
- ✅ Tests: path containment and symlink escape, ffprobe parsing, compatibility, subtitle matching, discovery, real FFmpeg scan integration (generated clips), API integration (validation, SQL injection attempts, overlap, removal)
- ✅ Measured: 520 generated clips scanned in 28.5 s (3 workers, every file thumbnailed); rescan with no changes 0.34 s; first library page 10 ms

## Phase 3: Streaming ⬜
`/videos/{id}/stream` with GET/HEAD, Range 206/416 via `http.ServeContent`, path containment and symlink checks, MIME types, concurrent clients.

## Phase 4: Frontend ⬜
Dashboard, library (grid/list, search, sort, filters, pagination, lazy thumbnails), details, custom player, mobile layout.

## Phase 5: Advanced ⬜
Continue Watching, history, subtitles in the player (SRT→VTT), devices, settings UI, scanner controls, optional PIN authentication.

## Phase 6: Release ⬜
README and docs (architecture, API, database, security, deployment, development), release builds.
