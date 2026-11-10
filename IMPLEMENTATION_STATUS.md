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

## Phase 5: Advanced ✅
Continue Watching, history, subtitles, devices and scanner controls were finished in phase 4. Added:

- **Live settings** (`config.Live`): `GET/PUT /settings` changes scanning (auto scan, interval, workers, hidden files,
  excluded folder names), video formats, thumbnails and backups without a restart. Updates are partial, validated as a
  whole and saved atomically; a rejected update changes nothing. `-host`/`-port` overrides are never written to
  config.json. Server address and allowed networks stay file-only (shown read-only). A running scan keeps the
  settings it started with.
- **Scheduler** (`jobs.Scheduler`, checks once a minute): automatic scans counted from the end of the last scan (never
  while one runs, only when folders exist), a daily automatic backup, daily purge of expired sessions.
- **Optional PIN** (`internal/auth`): PBKDF2-HMAC-SHA256 (600 000 iterations, random salt, constant-time compare),
  stored in the database. Sessions: random 256-bit token in an HttpOnly, SameSite=Strict cookie, only its SHA-256 is
  stored, 30-day sliding expiry. Wrong PINs: 5 free attempts per IP, then a lockout doubling from 30 s to 15 min
  (`429` + `Retry-After`). Changing the PIN needs the current one and signs out other devices; the signed-in device list
  can sign out one or all others. With a PIN on, every API call (streams and thumbnails included) needs a session; the
  page itself loads so the sign-in screen can show. Forgotten PIN: start with `-reset-pin`.
- **CSRF**: state-changing API requests carrying a foreign `Origin` (or `Sec-Fetch-Site: cross-site`) are refused.
- **Backups** (`internal/backup`): manual and daily backups with `VACUUM INTO` (consistent while in use), keep N
  automatic ones, download, delete. Names are validated against a strict pattern (no path traversal). Restore checks
  the file (`quick_check`, schema version not newer than this build, required tables), stages it, and applies it on the
  next start before the database opens, after saving the current library (including its WAL) as a "pre-restore" backup.
- **UI**: sign-in screen (lockout countdown), Sign out in the sidebar, Settings split into General / Library /
  Security / Backups / About tabs.

Verified: Go tests incl. `-race` (PIN flow, throttling, CSRF, settings validation, restore across a restart, scheduler),
a headless-Chrome run of every settings tab, lock → wrong PIN → unlock, and `-reset-pin` on the real binary.

## Desktop app ✅
`desktop/`: a Tauri 2 app around the server, so a host PC only needs an installer.

- **Sidecar**: the Go server is bundled as `enstream-server` and started with `-desktop`
  (machine-readable ready line, next free port if 8080 is busy, exits when the app's pipe closes,
  so it never outlives the app). A start-up screen shows progress, or the error with *Try again*.
- **Window**: frameless with the family title bar; it shows the server's own page, so desktop,
  phones and TVs share one interface. Navigation is limited to the bundled page and the local
  server; other links open in the browser. A `server-page` capability grants that loopback origin
  exactly the app's commands (folder picker, desktop settings, updates, window controls).
- **Tray**: open, open in browser, copy the phone address, quit. Closing the window keeps serving.
  Starts with the computer (hidden in the tray) from the first run; both are switches.
- **Updates**: daily check of GitHub releases, verified SHA-256, in-place install (.deb via pkexec,
  AppImage swap, NSIS silent). The server is stopped before its file is replaced.
- **Packaging**: .deb depends on ffmpeg; Windows NSIS (per machine) adds a private-network firewall
  rule; FFmpeg is downloaded on Windows by the server itself (`POST /system/ffmpeg/install`,
  checksum-verified, no restart needed).
- **Web additions**: first-run wizard (folders, optional PIN, QR code for phones), folder browser
  (`GET /fs/dirs`, folders only), `GET /system/qr`, Settings → Desktop.
- **CI**: `.github/workflows/build.yml` tests the server, builds .deb/AppImage/NSIS and the
  stand-alone server binaries, and publishes a release for `v*` tags.

Verified: Go tests; desktop app under Xvfb (wizard → folder → QR → home, Settings → Desktop over
IPC, close to tray with the server still answering, autostart entry, server stops with the app);
.deb installed with apt in a clean Ubuntu 22.04 container.

## Phase 6: Release ⬜
README and docs (architecture, API, database, security, deployment, development), release builds.
