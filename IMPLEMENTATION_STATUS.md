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

## Phase 2: Media engine ⬜
Folders API, recursive incremental scanner (bounded workers, cancel, progress, history), FFprobe metadata, WebP thumbnails, subtitles discovery, compatibility rating.

## Phase 3: Streaming ⬜
`/videos/{id}/stream` with GET/HEAD, Range 206/416 via `http.ServeContent`, path containment and symlink checks, MIME types, concurrent clients.

## Phase 4: Frontend ⬜
Dashboard, library (grid/list, search, sort, filters, pagination, lazy thumbnails), details, custom player, mobile layout.

## Phase 5: Advanced ⬜
Continue Watching, history, subtitles in the player (SRT→VTT), devices, settings UI, scanner controls, optional PIN authentication.

## Phase 6: Release ⬜
README and docs (architecture, API, database, security, deployment, development), release builds.
