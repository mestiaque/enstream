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

## Phase 3: Streaming ✅

- ✅ `GET`/`HEAD /api/v1/videos/{id}/stream` through `http.ServeContent`: Range (single, open-ended, suffix, clamped end, multiple as multipart/byteranges), 206, 416 with `bytes */size`, If-Range with an ETag, HEAD, correct Content-Length, `inline` (never forced downloads), MIME type from the extension
- ✅ The stored path is re-checked on every request (symlinks resolved, must stay inside its registered folder); disabled folders, deleted files and escapes give a clean 404
- ✅ Chunked reads from disk, kernel sendfile kept through the logging middleware, no write timeout (long streams), client disconnects end the copy
- ✅ Tests: every range form, malformed ranges, HEAD, If-Range, 20 concurrent clients reading different ranges, invalid ids, symlink escape, deleted file
- ✅ Checked on the real binary over the LAN address: HEAD 200, range 206, out-of-range 416; FFprobe read the video over HTTP by seeking with ranges

## Phase 4: Frontend ✅
Vanilla ES modules embedded in the binary; no build step, no CDN. CSP stays `'self'` only (no inline script/style), and all
metadata goes into the DOM as text nodes.

- **Shell** (`web/static/js/shell.js`): History-API router, sidebar + mobile bottom navigation, global search (`/` shortcut,
  300 ms debounce), one scan-status poller (1 s while scanning, 15 s idle, 60 s in a hidden tab) with a finish toast.
  `app.js` is only the versioned entry so every other module is loaded once.
- **Home**: greeting, Resume/Scan actions, library stats, Continue Watching and Recently Added rails, LAN addresses to copy.
- **All videos / Recently added**: grid or list (remembered), folder/format/resolution/watched filters with removable chips,
  9 sort orders, pagination, lazy WebP thumbnails, URL-synced state (back button works), skeleton loading.
- **Continue watching / History**: progress, device name, per-item remove, clear all (confirm dialog).
- **Details**: poster, resume/start over, mark watched, reset progress, copy stream link (for VLC), browser compatibility
  note, full technical metadata.
- **Player** (`views/player.js`): custom controls (timeline with buffer + hover time, volume, speed, subtitles menu, PiP,
  fullscreen incl. iPhone), resume, previous/next from the list it was opened from, auto-next with countdown, keyboard
  shortcuts (ignored while typing), touch: tap to show controls, double-tap sides to seek, Media Session (lock screen),
  progress every 10 s + on pause/seek/end/tab hide; `sendBeacon` on page close. Unsupported codecs get a clear message.
- **Folders & scan**: add (path + name), per-folder scan, remove (confirm), live progress with cancel, retry failed,
  recent scans table.
- **Settings**: device name, devices list, auto-next, server/FFmpeg status, network addresses, shortcuts, About.

API added for the UI: `GET /dashboard`, `PUT|POST /videos/{id}/progress`, `POST /videos/{id}/completed`,
`GET /continue-watching`, `GET|DELETE /history`, `DELETE /history/{id}`, `GET /devices`, `PUT /devices/current`,
`DELETE /devices/{id}`, `GET /videos/{id}/subtitles/{sid}` (SRT/VTT → WebVTT in Go, ASS/SSA via FFmpeg; the file must
lie inside the video's folder; non-UTF-8 SRT is read as Windows-1252).

Verified: Go tests (incl. `-race`), plus a headless-Chrome run against the real binary with a demo library: resume,
subtitles, speed, seek, pause-saves-progress, Esc, search, list view, 4K filter, next-up overlay, mobile menu, no stray
`null`/`undefined` text on any page, no console errors.

## Phase 5: Advanced ⬜
Done early in phase 4: Continue Watching, history, subtitles, devices, scanner controls.
Remaining: settings API + UI (scan interval, extensions, workers), automatic periodic scans, optional PIN
authentication, database backup/restore from the UI.

## Phase 6: Release ⬜
README and docs (architecture, API, database, security, deployment, development), release builds.
