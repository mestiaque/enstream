# STREAMVAULT
# COMPLETE MASTER DEVELOPMENT PROMPT
## Personal LAN Video Streaming Server

---

# 1. ROLE AND RESPONSIBILITIES

Act as a Principal Software Architect, Senior Go Engineer, Network Engineer, Video Streaming Specialist, Database Architect, Security Engineer, and Premium UI/UX Designer.

You are responsible for designing, developing, testing, debugging, optimizing, and documenting a complete cross-platform personal video streaming application.

Your objective is to build a reliable, lightweight, production-quality, self-hosted media streaming platform called:

**StreamVault — Your Personal Cinema**

This is a real software development project, not a prototype, tutorial, or architecture-only exercise.

You must create actual working source code, implement the required features, execute tests, fix errors, and deliver a fully runnable application.

Do not stop after generating a project structure or documentation.

---

# 2. PROJECT VISION

I have more than 500 video files stored on my personal computer.

I want to transform my PC into a private video streaming server.

The server will run on my PC, and every device connected to the same local network will be able to access the video library through an ordinary web browser.

Example:

http://192.168.1.100:8080

Supported client devices:

- Windows Desktop
- Windows Laptop
- Ubuntu/Linux
- Android Phones
- iPhones
- iPads
- Tablets
- Compatible Smart TVs

No client-side application installation should be required.

Users should simply open the server URL in their browser and browse or stream available videos.

The original video files must remain on the host PC.

Client devices must stream video data over the local network without downloading the complete video file.

The system must not depend on:
- Cloud hosting
- External media services
- External accounts
- Internet connectivity during normal operation
- Third-party streaming platforms

The application is exclusively designed for trusted local network usage.

---

# 3. FINAL TECHNOLOGY STACK

These are the selected technologies.

Do not replace them without identifying a compelling technical limitation.

## Backend

- Go (Golang)
- Go standard library
- net/http
- Chi HTTP router
- Structured logging with log/slog
- Context-based cancellation

## Frontend

- HTML5
- CSS3
- Vanilla JavaScript ES6+
- Bootstrap 5
- Custom CSS components
- Lucide Icons
- HTML5 Video API

## Database

- SQLite
- modernc.org/sqlite
- Database migrations
- Prepared statements
- Indexed queries
- Transaction management

Prefer a pure-Go SQLite driver to simplify cross-platform builds. Verify driver compatibility and SQLite concurrency behavior.

## Media Processing

- FFmpeg
- FFprobe
- Background worker pool
- WebP thumbnail generation
- Subtitle processing

## Streaming

- HTTP Range Requests
- HTTP 206 Partial Content
- HTTP 416 Range Not Satisfiable
- Direct Play
- Browser compatibility detection
- Efficient filesystem streaming

## Deployment

- Single native Go executable
- Windows 10+
- Windows 11
- Ubuntu/Linux
- Embedded frontend assets
- embed.FS

Production deployment must not require Node.js, npm, a separate frontend server, or Docker.

---

# 4. CORE PRODUCT REQUIREMENTS

The application must provide:

1. Automatic video discovery.
2. Multiple media folder registration.
3. Recursive folder scanning.
4. Video metadata extraction.
5. Automatic thumbnail generation.
6. Premium video library interface.
7. Video search.
8. Video sorting and filtering.
9. Video details.
10. Browser-based video playback.
11. Direct streaming.
12. HTTP Range seeking.
13. Playback progress tracking.
14. Continue Watching.
15. Playback history.
16. External subtitles.
17. Multiple simultaneous clients.
18. LAN network discovery.
19. Server administration.
20. Application configuration.
21. Cross-platform deployment.
22. Secure filesystem access.

---

# 5. APPLICATION ARCHITECTURE

Build a modular architecture with clear separation of concerns.

The application must contain these layers:

## 5.1 Application Layer

Responsibilities:
- Bootstrap
- Dependency initialization
- Configuration loading
- Database initialization
- Service initialization
- HTTP server lifecycle
- Graceful shutdown

## 5.2 HTTP Layer

Responsibilities:
- API routing
- Middleware
- Request validation
- Response serialization
- Error handling
- Static frontend delivery

## 5.3 Service Layer

Responsibilities:
- Video library operations
- Folder management
- Metadata extraction
- Thumbnail generation
- Playback history
- Scan orchestration
- Settings management

## 5.4 Repository Layer

Responsibilities:
- SQLite access
- CRUD operations
- Queries
- Pagination
- Transactions
- Database migrations

## 5.5 Media Layer

Responsibilities:
- File discovery
- File inspection
- FFprobe integration
- FFmpeg integration
- Thumbnail generation
- Subtitle detection
- Media compatibility analysis

## 5.6 Streaming Layer

Responsibilities:
- Original video file delivery
- HTTP Range processing
- Seeking
- MIME type detection
- Concurrent streaming
- Safe filesystem access

## 5.7 Job Layer

Responsibilities:
- Background scanning
- Thumbnail jobs
- Metadata jobs
- Cancellation
- Worker management
- Job progress
- Error reporting

## 5.8 Network Layer

Responsibilities:
- LAN IPv4 discovery
- Interface detection
- Access URL generation
- Port validation
- Network status

---

# 6. PROJECT DIRECTORY STRUCTURE

Use the following structure as the architectural baseline.

streamvault/

├── cmd/
│   └── streamvault/
│       └── main.go
│
├── internal/
│   ├── app/
│   │   ├── app.go
│   │   ├── bootstrap.go
│   │   └── lifecycle.go
│   │
│   ├── config/
│   │   ├── config.go
│   │   ├── defaults.go
│   │   └── validation.go
│   │
│   ├── server/
│   │   ├── server.go
│   │   ├── router.go
│   │   ├── middleware.go
│   │   ├── response.go
│   │   └── errors.go
│   │
│   ├── network/
│   │   ├── discovery.go
│   │   ├── interfaces.go
│   │   ├── addresses.go
│   │   └── status.go
│   │
│   ├── database/
│   │   ├── sqlite.go
│   │   ├── migrate.go
│   │   ├── migrations/
│   │   └── repositories/
│   │       ├── video_repository.go
│   │       ├── folder_repository.go
│   │       ├── history_repository.go
│   │       ├── subtitle_repository.go
│   │       ├── settings_repository.go
│   │       └── scan_repository.go
│   │
│   ├── media/
│   │   ├── scanner.go
│   │   ├── metadata.go
│   │   ├── ffmpeg.go
│   │   ├── ffprobe.go
│   │   ├── thumbnail.go
│   │   ├── subtitle.go
│   │   ├── compatibility.go
│   │   ├── fileinfo.go
│   │   └── validation.go
│   │
│   ├── streaming/
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── mime.go
│   │   ├── range.go
│   │   ├── headers.go
│   │   └── transcoding.go
│   │
│   ├── playback/
│   │   ├── progress.go
│   │   ├── history.go
│   │   └── service.go
│   │
│   ├── jobs/
│   │   ├── manager.go
│   │   ├── worker.go
│   │   ├── queue.go
│   │   ├── scanner_job.go
│   │   └── thumbnail_job.go
│   │
│   ├── handlers/
│   │   ├── dashboard.go
│   │   ├── videos.go
│   │   ├── folders.go
│   │   ├── scanner.go
│   │   ├── playback.go
│   │   ├── subtitles.go
│   │   ├── settings.go
│   │   └── system.go
│   │
│   └── security/
│       ├── auth.go
│       ├── paths.go
│       ├── access.go
│       ├── middleware.go
│       └── validation.go
│
├── web/
│   ├── embed.go
│   ├── templates/
│   │   ├── index.html
│   │   └── player.html
│   │
│   └── static/
│       ├── css/
│       │   ├── variables.css
│       │   ├── base.css
│       │   ├── layout.css
│       │   ├── components.css
│       │   ├── dashboard.css
│       │   ├── library.css
│       │   ├── player.css
│       │   ├── settings.css
│       │   └── responsive.css
│       │
│       ├── js/
│       │   ├── app.js
│       │   ├── api.js
│       │   ├── router.js
│       │   ├── dashboard.js
│       │   ├── library.js
│       │   ├── video-card.js
│       │   ├── player.js
│       │   ├── playback.js
│       │   ├── scanner.js
│       │   ├── folders.js
│       │   ├── settings.js
│       │   ├── notifications.js
│       │   └── utils.js
│       │
│       └── assets/
│
├── migrations/
├── scripts/
│   ├── build.sh
│   ├── build.bat
│   ├── dev.sh
│   └── release.sh
│
├── tests/
│   ├── unit/
│   ├── integration/
│   ├── fixtures/
│   └── testdata/
│
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
├── README.md
└── LICENSE

You may refine this structure when technically justified.

Do not put all functionality in main.go.

---

# 7. SERVER STARTUP AND CONFIGURATION

Create a reliable startup process.

When the application starts:

1. Load configuration.
2. Validate configuration.
3. Create required application directories.
4. Initialize SQLite.
5. Run migrations.
6. Detect FFmpeg.
7. Detect FFprobe.
8. Discover available LAN addresses.
9. Initialize services.
10. Initialize background workers.
11. Start HTTP server.
12. Display server URLs.

Example:

====================================
        STREAMVAULT SERVER
====================================

Status: ONLINE

Local URL:
http://localhost:8080

Network URL:
http://192.168.1.100:8080

Database:
READY

FFmpeg:
AVAILABLE

FFprobe:
AVAILABLE

Press CTRL+C to stop.

====================================

Do not hardcode LAN IP addresses.

Support custom host and port configuration.

Default:

HOST=0.0.0.0
PORT=8080

Use graceful shutdown when the user exits the application.

Do not lose database writes or leave FFmpeg processes running during shutdown.

---

# 8. NETWORK DISCOVERY AND LAN ACCESS

Implement automatic network interface detection.

Requirements:

- Detect active network interfaces.
- Identify usable IPv4 addresses.
- Exclude loopback addresses.
- Exclude unusable interfaces.
- Generate browser access URLs.
- Display multiple available LAN addresses when applicable.
- Refresh network information when needed.
- Validate port availability.

Do not assume every router uses 192.168.1.x.

Do not assume the server is always connected through Wi-Fi. Ethernet must also work.

Default deployment must be LAN-oriented.

Security requirements:
- No router port forwarding.
- No public cloud relay.
- No external access service.
- Firewall guidance.
- Private network firewall rules.

IMPORTANT:

Binding to 0.0.0.0 makes the service listen on all available interfaces. This does not itself enforce LAN-only access.

Document and implement safe network exposure defaults. Restrict access to trusted private networks where practical, and provide explicit configuration for allowed networks.

Do not blindly block all private subnets because legitimate LANs may use different private address ranges.

---

# 9. MEDIA FOLDER MANAGEMENT

Users must be able to register multiple directories.

Example:

/home/user/Videos
/home/user/Movies
/media/storage/Media

Windows examples:

D:\Videos
E:\Movies

Features:

- Add folder
- Remove folder
- List folders
- Validate folder existence
- Validate read permissions
- Display folder path
- Display indexed video count
- Display folder size where practical
- Scan individual folders
- Scan all folders
- Exclude subdirectories
- Hidden directory handling
- Detect inaccessible folders

Removing a folder must never delete original media files.

The application should preserve indexed data according to a clear removal policy and provide an option to remove stale database records.

Do not allow arbitrary filesystem browsing outside registered media directories.

---

# 10. VIDEO FILE DISCOVERY

Support these file extensions:

.mp4
.mkv
.avi
.mov
.webm
.m4v
.mpeg
.mpg
.wmv
.flv
.3gp
.ts
.mts
.m2ts
.ogv
.vob

Use case-insensitive extension matching.

Implement recursive scanning.

Ignore:
- Hidden directories by default
- Excluded directories
- Temporary files
- Invalid media files
- Inaccessible files

Never assume that an extension guarantees a playable file.

Use FFprobe to inspect actual media content.

---

# 11. INCREMENTAL MEDIA SCANNER

This module is a high priority.

Build a reliable scanner capable of handling 500+ files and much larger collections.

For each discovered file:

1. Resolve and validate the path.
2. Obtain file size.
3. Obtain modification timestamp.
4. Compare against existing database record.
5. Skip unchanged files.
6. Extract metadata when new or changed.
7. Generate thumbnail when required.
8. Save metadata.
9. Associate subtitles.
10. Record errors without terminating the complete scan.

Support:

- Full scan
- Incremental scan
- Folder-specific scan
- Manual rescan
- Failed item retry
- Scan cancellation
- Scan progress
- Scan statistics
- Scan history
- Deleted file detection
- Missing directory detection

Use a configurable bounded worker pool.

Default worker count: 3.

Do not start hundreds of FFmpeg processes.

Prevent duplicate concurrent scans for the same folder.

Use context.Context for cancellation.

Track:
- Total discovered
- Processed
- Skipped
- Failed
- New files
- Updated files
- Removed files
- Current filename
- Scan start time
- Scan duration

Scanner progress must be available through the API.

A scan must not block normal video playback.

---

# 12. FFMPEG AND FFPROBE INTEGRATION

Create dedicated services for FFmpeg and FFprobe.

Do not mix command execution logic throughout the codebase.

Requirements:

- Configurable executable paths
- Automatic executable discovery
- Version detection
- Availability checks
- Context-aware execution
- Execution timeout
- Standard output capture
- Standard error capture
- Safe argument handling
- Process cancellation
- Error reporting

Use os/exec with explicit arguments.

Never concatenate user input into shell commands.

Do not use shell interpolation.

FFprobe must extract:

- Container format
- Duration
- File size
- Width
- Height
- Video codec
- Audio codec
- Frame rate
- Pixel format where available
- Bitrate where available
- Audio channels
- Subtitle streams
- Codec profile where available

Handle missing metadata gracefully.

Do not assume every video contains audio.

Do not assume every file has a valid duration.

---

# 13. THUMBNAIL GENERATION

Generate thumbnails using FFmpeg.

Default:
- Format: WebP
- Width: 480px
- Height: proportional
- Quality: 80

Requirements:

- Generate thumbnail from a suitable timestamp.
- Avoid selecting a blank opening frame when practical.
- Handle very short videos.
- Handle failed extraction.
- Cache generated thumbnails.
- Avoid regenerating unchanged thumbnails.
- Support manual regeneration.
- Support configurable quality.
- Generate thumbnails asynchronously.
- Use atomic file replacement.

Store thumbnails in application data directories.

Do not place generated thumbnails inside original media folders.

Use stable internal identifiers for thumbnail paths.

Do not construct filesystem paths directly from untrusted video filenames.

---

# 14. DATABASE DESIGN

Use SQLite.

Create versioned migrations.

Required tables:

## media_folders

- id
- path
- name
- enabled
- last_scanned_at
- created_at
- updated_at

## videos

- id
- folder_id
- title
- file_name
- file_path
- extension
- mime_type
- container_format
- file_size
- duration
- width
- height
- fps
- video_codec
- audio_codec
- bitrate
- thumbnail_path
- file_modified_at
- scanned_at
- created_at
- updated_at

## playback_history

- id
- video_id
- device_id
- position
- duration
- completed
- last_played_at
- created_at
- updated_at

## subtitles

- id
- video_id
- file_path
- language
- format
- label
- created_at

## scan_jobs

- id
- folder_id
- status
- total_files
- processed_files
- skipped_files
- failed_files
- new_files
- updated_files
- removed_files
- current_file
- started_at
- completed_at
- error_message

## settings

- id
- key
- value
- updated_at

## devices (optional)

- id
- device_token
- device_name
- last_seen_at
- created_at

Requirements:

- Foreign keys
- Proper indexes
- Unique constraints
- Prepared statements
- Transactions
- Safe migrations
- Database backup
- Database restore
- Data consistency

Use SQLite WAL mode when appropriate.

Configure a sensible busy timeout and connection pool compatible with SQLite concurrency.

Use database constraints to prevent duplicate records.

Store video paths as filesystem paths, not URLs.

Store thumbnail paths separately.

Never store complete video files in SQLite.

---

# 15. VIDEO LIBRARY API

Implement:

GET /api/v1/videos

Query parameters:

- page
- limit
- search
- folder_id
- extension
- resolution
- sort
- order
- watched
- recently_added

Default pagination:

page=1
limit=30

Maximum limit should be configurable or capped at a sensible value.

Support sorting:

- title
- created_at
- updated_at
- file_size
- duration
- last_played_at

Support filtering:

- Folder
- File format
- Resolution
- Watched / Unwatched
- Recently added

Return consistent JSON.

Example:

{
  "success": true,
  "data": [],
  "pagination": {
    "page": 1,
    "limit": 30,
    "total": 500,
    "total_pages": 17
  }
}

Validate every user-controlled query parameter.

Do not interpolate query strings into SQL.

---

# 16. VIDEO STREAMING ENGINE

This is the most important technical component.

Required endpoint:

GET /api/v1/videos/{id}/stream

Support:

- GET
- HEAD
- Range requests
- HTTP 206
- HTTP 416
- Accept-Ranges
- Content-Range
- Content-Length
- Correct Content-Type
- Browser seeking
- Multiple concurrent clients
- Large files
- Efficient memory use
- Client disconnect handling

Use http.ServeContent or a carefully tested equivalent.

Do not read entire files into memory.

Do not implement a naive custom Range handler unless all relevant HTTP semantics are correctly handled.

Validate video ID against the database.

Resolve the actual file path.

Verify that the file remains inside an authorized registered media directory.

Prevent:
- Directory traversal
- Arbitrary file access
- Symlink escape outside permitted roots
- Unauthorized access
- Path confusion between Windows and Linux

Use appropriate MIME types.

Support HEAD requests.

Ensure browsers can request video byte ranges for seeking.

Do not force attachment downloads.

Do not return incorrect Content-Length values.

Test normal, suffix, open-ended, invalid, and unsatisfiable byte ranges.

If multiple ranges are unsupported by the chosen implementation, reject them correctly rather than returning malformed responses.

---

# 17. BROWSER VIDEO COMPATIBILITY

Different browsers support different codecs.

Do not claim universal support for MP4, MKV, AVI, MOV, or any other container.

Use FFprobe information to identify:

- Container
- Video codec
- Audio codec
- Codec profile
- Resolution
- Audio channels

Implement a compatibility status:

- Likely Compatible
- Possibly Compatible
- Requires Transcoding

Use browser-friendly defaults.

MP4 with H.264 video and AAC audio should be prioritized.

Do not automatically transcode every video.

Display compatibility warnings when appropriate.

---

# 18. OPTIONAL TRANSCODING ARCHITECTURE

Keep the streaming architecture extensible.

Transcoding is not required for the first MVP.

Design a separate transcoding service that can later support:

- FFmpeg process management
- Browser-compatible output
- HLS or fragmented MP4
- Session tracking
- Cancellation
- Temporary file cleanup
- Concurrent session limits
- Resource monitoring

If implemented:

- Do not use unbounded FFmpeg processes.
- Cancel work when clients disconnect.
- Clean temporary data.
- Do not modify original files.
- Avoid creating permanent duplicate video files.
- Make resource usage visible.

Direct Play must remain the default.

---

# 19. SUBTITLE MANAGEMENT

Support external subtitle files:

.srt
.vtt
.ass
.ssa

Match subtitles to video filenames.

Example:

movie.mp4
movie.en.srt
movie.bn.srt

Display:
- Language
- Label
- Subtitle format

Use WebVTT for browser playback.

Convert SRT to VTT in a cache directory where required.

Do not overwrite original subtitle files.

Do not expose arbitrary subtitle paths through the API.

Use HTML5 track elements.

Provide subtitle selection in the player.

---

# 20. PLAYBACK PROGRESS AND HISTORY

Implement a shared playback history system.

A user may start watching on an iPhone and resume on a Laptop.

Store:
- Video ID
- Device ID
- Current position
- Duration
- Completion status
- Last played timestamp

Progress update frequency:

Approximately every 10–15 seconds.

Also update on:
- Pause
- Seek
- Video ended
- Page visibility change
- Page unload using best-effort delivery

Do not rely solely on beforeunload because browser termination can prevent requests from completing.

Use sendBeacon where appropriate.

Prevent excessive database writes.

Resume rules:
- Resume from last known position.
- If playback is almost complete, treat it as completed.
- Provide a replay option.
- Avoid resuming from invalid positions.
- Allow clearing history.

Implement:
- Continue Watching
- Recently Played
- Watch History
- Clear History
- Mark Completed

---

# 21. DEVICE MANAGEMENT

Provide lightweight browser device identification.

Do not rely on IP address as the only device identifier because DHCP addresses can change.

Optionally assign a persistent random device token through local browser storage.

Allow users to:
- Register a friendly device name.
- View known devices.
- Identify current device.
- Remove old device records.

Do not collect unnecessary personal information.

If browser storage is unavailable, playback must still work using a temporary device identity.

---

# 22. PREMIUM UI/UX DESIGN

Create a premium cinematic dark interface.

The product must look like a modern personal streaming platform.

Reference design quality:
- Plex
- Jellyfin
- Modern streaming dashboards
- Premium media libraries

Do not copy their branding or exact layouts.

## Color System

Use a carefully defined CSS token system.

Background:
Deep charcoal / near-black.

Surface:
Elevated dark slate.

Primary:
Electric blue.

Secondary:
Soft violet.

Text:
Warm white.

Muted:
Cool gray.

Border:
Subtle neutral gray.

Use CSS variables for all major design tokens.

## Design Principles

- Minimal
- Cinematic
- Premium
- Modern
- Responsive
- Readable
- Accessible
- Consistent
- Fast

Avoid:
- Generic admin dashboard appearance
- Excessive gradients
- Excessive glass effects
- Excessive animations
- Oversized statistics
- Cluttered cards
- Unnecessary decorative elements

## Main Navigation

Desktop:
- Left sidebar
- Brand logo
- Dashboard
- All Videos
- Continue Watching
- Recently Added
- History
- Settings
- Server status

Top bar:
- Search
- Scan status
- Server status
- Settings shortcut

Mobile:
- Compact header
- Search button
- Responsive video grid
- Bottom navigation
- Mobile player controls

---

# 23. REQUIRED FRONTEND PAGES

## A. Dashboard

Display:

- Total videos
- Total storage
- Registered folders
- Recently added
- Continue watching
- Recent playback
- Library status
- Last scan
- Server status

Do not overcrowd the dashboard.

The main visual priority must be video content.

## B. All Videos

Implement:

- Responsive grid
- List view
- Search
- Sorting
- Filtering
- Folder selection
- Format selection
- Resolution selection
- Watched/unwatched filter
- Pagination
- Lazy thumbnail loading

Each card should show:
- Thumbnail
- Title
- Duration
- Resolution
- File size
- Progress indicator

Use real scanned data.

## C. Continue Watching

Display incomplete videos with:
- Thumbnail
- Title
- Progress bar
- Last watched information
- Resume button

## D. Recently Added

Display videos sorted by latest addition or indexing timestamp.

## E. Watch History

Display:
- Video
- Last played
- Playback progress
- Completion state

Allow history clearing.

## F. Video Details

Show:
- Large thumbnail
- Video title
- Filename
- Duration
- Resolution
- File size
- Format
- Video codec
- Audio codec
- FPS
- Subtitle availability
- File location

Provide:
- Play
- Resume
- Restart
- Copy filename

Do not expose unnecessary filesystem details to unauthorized clients.

## G. Video Player

Build a custom responsive HTML5 player.

Controls:
- Play/Pause
- Timeline
- Seek
- Volume
- Mute
- Fullscreen
- Playback speed
- Subtitle selector
- Picture-in-Picture
- Resume
- Auto next
- Next video
- Previous video

Keyboard:
- Space: Play/Pause
- Left/Right: Seek
- Up/Down: Volume
- F: Fullscreen
- M: Mute

Do not trigger player shortcuts while typing into an input.

Support touch gestures only where they are reliable and discoverable.

Display loading and buffering states.

Handle playback errors gracefully.

Do not promise playback support for unsupported codecs.

## H. Folder Management

Allow:
- Adding folders
- Removing folders
- Viewing registered folders
- Starting folder scans
- Viewing scan status
- Viewing video counts
- Handling inaccessible directories

## I. Scanner Management

Show:
- Current job
- Scan progress
- Total files
- Processed files
- Failed files
- Skipped files
- Current filename
- Scan duration

Actions:
- Scan all
- Scan folder
- Cancel scan
- Retry failed items

## J. Settings

Include:

Server:
- Host
- Port
- Server status
- LAN URLs

Media:
- Registered folders
- Excluded folders

Processing:
- FFmpeg path
- FFprobe path
- Worker count
- Thumbnail settings

Library:
- Auto scan
- Scan interval
- Default sorting

Security:
- Optional PIN/password
- Authentication status
- Allowed network configuration

Storage:
- Database location
- Cache location
- Thumbnail size
- Clear cache
- Backup database

About:
- Application name
- Version
- Go version
- Server uptime
- Dependency status

---

# 24. FRONTEND ENGINEERING RULES

Use modular JavaScript.

Avoid one giant JavaScript file.

Create a centralized API client.

Implement:
- Loading states
- Empty states
- Error states
- Toast notifications
- Modal dialogs
- Confirmation dialogs
- Search debounce
- Lazy loading
- Pagination
- Responsive behavior

Avoid unnecessary full-page reloads.

Use proper semantic HTML.

Provide accessible labels and keyboard navigation.

Prevent XSS:
- Never insert untrusted media metadata using unsafe innerHTML.
- Use textContent or safe DOM construction.
- Sanitize any intentionally supported rich text.

Do not trust client-side validation alone.

---

# 25. API SPECIFICATION

Base path:

/api/v1

## System

GET /health
GET /system/info
GET /dashboard

## Videos

GET /videos
GET /videos/{id}
GET /videos/{id}/stream
HEAD /videos/{id}/stream
GET /videos/{id}/thumbnail
GET /videos/{id}/subtitles

## Folders

GET /folders
POST /folders
GET /folders/{id}
DELETE /folders/{id}

## Scanner

POST /scanner/scan
GET /scanner/status
GET /scanner/jobs
GET /scanner/jobs/{id}
POST /scanner/cancel
POST /scanner/retry-failed

## Playback

GET /history
GET /continue-watching
PUT /videos/{id}/progress
POST /videos/{id}/completed
DELETE /history/{id}
DELETE /history

## Subtitles

GET /videos/{id}/subtitles
GET /videos/{id}/subtitles/{subtitle_id}

## Settings

GET /settings
PUT /settings

Use consistent response structures.

Example success:

{
  "success": true,
  "data": {},
  "message": "Operation completed successfully"
}

Example error:

{
  "success": false,
  "error": {
    "code": "VIDEO_NOT_FOUND",
    "message": "The requested video was not found"
  }
}

Implement correct HTTP status codes.

Validate request bodies and query parameters.

Apply pagination to collection endpoints.

Do not expose database errors, stack traces, or absolute internal paths to normal browser clients.

---

# 26. OPTIONAL AUTHENTICATION

The default product is a trusted private LAN application.

Authentication should be optional and disabled by default.

When enabled:

- Provide a PIN/password setup.
- Protect administrative operations.
- Protect playback access if configured.
- Use secure session management.
- Hash passwords with a suitable password hashing algorithm.
- Implement login throttling.
- Use CSRF protection for cookie-authenticated state-changing requests.
- Provide session expiration.
- Provide logout.
- Avoid storing plaintext credentials.

Do not implement weak authentication using a hardcoded PIN.

Do not expose secrets in browser JavaScript.

---

# 27. SECURITY REQUIREMENTS

Implement defense in depth.

Critical requirements:

1. Prevent directory traversal.
2. Restrict file access to registered media roots.
3. Resolve canonical filesystem paths.
4. Prevent symlink escapes.
5. Use safe file-opening practices.
6. Validate video IDs.
7. Validate folder paths.
8. Use prepared SQL statements.
9. Limit request body size.
10. Avoid shell command injection.
11. Protect administrative endpoints.
12. Validate Host and Origin where appropriate.
13. Use restrictive CORS defaults.
14. Apply sensible timeouts.
15. Protect against unbounded jobs.
16. Clean up temporary files.
17. Avoid leaking internal filesystem paths.
18. Protect settings changes.
19. Log security-relevant failures.
20. Never expose arbitrary filesystem browsing.

Use authentication or another explicit trust mechanism for administrative actions.

Provide clear documentation for trusted LAN operation.

---

# 28. PERFORMANCE AND RESOURCE MANAGEMENT

Optimize for:

- 500+ videos
- Thousands of videos in future
- Large video files
- Multiple simultaneous clients
- Limited system memory
- Background media processing

Mandatory:

- Bounded worker pools
- Context cancellation
- SQLite indexes
- Pagination
- Lazy-loaded images
- Incremental scanning
- Efficient filesystem access
- No complete video buffering
- No uncontrolled goroutines
- Proper file handle cleanup
- Efficient HTTP responses

Avoid unnecessary metadata processing.

Avoid unnecessary thumbnail regeneration.

Do not block normal playback because of scanning.

Do not load every video card into the browser DOM simultaneously.

Measure actual performance instead of inventing benchmark claims.

---

# 29. CONFIGURATION MANAGEMENT

Provide a configuration file.

Example:

{
  "server": {
    "host": "0.0.0.0",
    "port": 8080
  },
  "media": {
    "folders": [],
    "extensions": [
      ".mp4",
      ".mkv",
      ".avi",
      ".mov",
      ".webm",
      ".m4v",
      ".mpeg",
      ".mpg"
    ]
  },
  "scanner": {
    "worker_count": 3,
    "auto_scan": false,
    "excluded_directories": []
  },
  "thumbnail": {
    "format": "webp",
    "width": 480,
    "quality": 80
  },
  "security": {
    "authentication_enabled": false,
    "allowed_networks": []
  }
}

Configuration rules:

- Provide sensible defaults.
- Validate all configuration values.
- Preserve existing configuration.
- Never overwrite user settings on restart.
- Support Windows and Linux paths.
- Keep sensitive settings out of logs.

The actual schema may be extended as necessary.

---

# 30. APPLICATION DATA DIRECTORY

Use an operating-system-appropriate user data directory.

Do not assume that the application executable's directory is writable.

Suggested layout:

StreamVault Data/

├── config.json
├── streamvault.db
├── thumbnails/
├── cache/
│   ├── subtitles/
│   └── transcoding/
├── logs/
└── backups/

Keep original video files outside this directory.

Provide configurable data directory support where practical.

---

# 31. EMBEDDED FRONTEND

Use Go embed.FS.

The production executable must include:
- HTML templates
- CSS
- JavaScript
- Static assets

Use separate development and production asset-serving strategies if necessary.

Do not require an external web server.

Do not require npm to run the application.

Ensure static asset paths work correctly from the embedded filesystem.

Provide correct cache headers and versioned asset URLs.

---

# 32. WINDOWS AND UBUNTU BUILD SUPPORT

Provide scripts for:

Ubuntu:

./scripts/build.sh

Windows:

scripts/build.bat

Support:
- Native Linux build
- Native Windows build
- Cross-compilation where supported

Example targets:

GOOS=linux GOARCH=amd64
GOOS=windows GOARCH=amd64

Verify SQLite driver and dependency compatibility before claiming cross-platform support.

The build must include frontend assets.

External FFmpeg/FFprobe binaries may remain system dependencies.

Provide a dependency check at startup.

If dependencies are missing, explain how to install or configure them.

---

# 33. LOGGING AND ERROR HANDLING

Use structured logging.

Log:
- Application startup
- Shutdown
- Server errors
- Database errors
- Scan start and finish
- Media processing failures
- Thumbnail failures
- Streaming failures
- Authentication failures
- Configuration errors

Do not log:
- Passwords
- Session secrets
- Full sensitive credentials

Use appropriate log levels.

Do not expose raw internal errors to browser clients.

Provide a useful error message and a traceable log entry.

---

# 34. TESTING REQUIREMENTS

Create automated tests.

## Unit Tests

Test:
- Configuration
- Path validation
- MIME detection
- Metadata parsing
- Range handling
- Pagination
- Playback progress rules
- Database repositories
- Subtitle matching

## Integration Tests

Test:
- SQLite migrations
- Folder registration
- Recursive scanning
- FFprobe integration
- Thumbnail generation
- HTTP streaming
- Browser seeking behavior through HTTP requests
- Multiple simultaneous streams
- Scan cancellation
- Database persistence
- Deleted file detection

## Security Tests

Test:
- Directory traversal
- Symlink escape
- Arbitrary file access
- Invalid video ID
- Invalid folder path
- Oversized request body
- Unauthorized administration
- Malformed Range header
- SQL injection attempts
- Unsafe metadata rendering

## Required Commands

go test ./...
go vet ./...
go test -race ./...

Run platform build checks.

Fix failures.

Do not disable failing tests just to claim success.

Use generated fixtures and temporary directories.

Automated tests must not depend on my actual 500+ video collection.

---

# 35. ACCEPTANCE TESTING

The final application must pass these scenarios.

### Scenario 1: First Startup

- Launch application.
- Database initializes.
- LAN IP is detected.
- Access URL is displayed.
- Browser opens the application.

### Scenario 2: Library Setup

- Register a video directory.
- Start scanning.
- Detect 500+ files.
- Extract metadata.
- Generate thumbnails.
- Display the library.

### Scenario 3: Video Playback

- Open a video.
- Play video.
- Pause video.
- Seek forward.
- Seek backward.
- Change volume.
- Enter fullscreen.
- Play on mobile browser.

### Scenario 4: Multi-device Playback

- Open the application on three devices.
- Stream three different videos.
- Verify independent streaming.
- Verify server stability.

### Scenario 5: Cross-device Resume

- Watch a video on an iPhone.
- Stop at a specific position.
- Open the same video on a Laptop.
- Resume near the saved position.

### Scenario 6: Incremental Scan

- Add a new video.
- Run incremental scan.
- Only new or changed files should be processed.

### Scenario 7: Persistence

- Stop server.
- Restart server.
- Verify library and history remain intact.

### Scenario 8: Security

- Attempt directory traversal.
- Attempt arbitrary file access.
- Attempt unauthorized administration.
- Verify all are rejected appropriately.

---

# 36. DOCUMENTATION

Create a comprehensive README.md.

Include:

1. Project overview
2. Main features
3. Architecture overview
4. Requirements
5. Installation
6. Development setup
7. Build instructions
8. Windows usage
9. Ubuntu usage
10. FFmpeg setup
11. First-time configuration
12. Adding media folders
13. Starting the server
14. Accessing from mobile
15. Firewall setup
16. Streaming compatibility
17. Troubleshooting
18. Database backup
19. Database restore
20. Updating the application
21. Security considerations

Also provide:

docs/
├── ARCHITECTURE.md
├── API.md
├── DATABASE.md
├── SECURITY.md
├── DEPLOYMENT.md
└── DEVELOPMENT.md

Documentation must describe the actual implemented behavior.

Do not document features that do not exist.

---

# 37. DEVELOPMENT WORKFLOW

You must work systematically.

## Step 1 — Workspace Inspection

Inspect the current workspace.

If it is empty:
- Initialize the project.

If an existing project exists:
- Inspect its files.
- Preserve useful existing work.
- Do not overwrite unrelated files.

## Step 2 — Architecture

Create:
- Implementation checklist
- Project structure
- Database schema
- API contract
- Service boundaries
- Security plan

## Step 3 — Foundation

Implement:
- Go bootstrap
- Configuration
- SQLite
- Migrations
- HTTP server
- LAN discovery
- Frontend embedding

Verify basic browser access.

## Step 4 — Media Engine

Implement:
- Folder management
- File scanner
- FFprobe
- Thumbnail generator
- Worker pool
- Metadata persistence

Verify real media indexing.

## Step 5 — Streaming

Implement:
- Stream endpoint
- Range requests
- HEAD
- Seeking
- MIME handling
- Concurrent playback
- File security

Verify playback from another device.

## Step 6 — Frontend

Implement:
- Dashboard
- Library
- Search
- Filters
- Video details
- Premium player
- Responsive design

Use real API data.

## Step 7 — Advanced Features

Implement:
- Continue Watching
- Playback history
- Subtitles
- Device identification
- Settings
- Scanner controls
- Optional authentication

## Step 8 — Testing

Execute:
- Unit tests
- Integration tests
- Race detector
- Security tests
- Build tests

Fix errors before release.

## Step 9 — Deployment

Produce:
- Ubuntu executable
- Windows executable
- Configuration example
- Installation instructions
- Complete README

---

# 38. IMPORTANT CLAUDE CODE EXECUTION RULES

This is a real implementation task.

Follow these rules strictly:

1. Create actual project files.
2. Do not return only code snippets.
3. Do not return only architecture documentation.
4. Do not skip essential modules.
5. Do not generate a giant monolithic source file.
6. Do not use fake production data.
7. Do not leave essential features as TODO.
8. Do not silently replace technologies.
9. Do not use unnecessary dependencies.
10. Do not introduce unnecessary frontend frameworks.
11. Do not create uncontrolled background processes.
12. Do not claim untested features work.
13. Do not ignore compiler errors.
14. Do not ignore failed tests.
15. Do not overwrite user media.
16. Do not move or copy original videos unnecessarily.
17. Do not expose private filesystem paths.
18. Do not sacrifice security for convenience.
19. Do not build all modules before verifying the foundation.
20. Maintain a clear implementation checklist.

If a feature cannot be completed within the current phase, document the reason and implement the necessary interface or safe fallback.

Do not silently pretend it is complete.

---

# 39. PROGRESS REPORTING

Maintain:

IMPLEMENTATION_STATUS.md

Track each feature:

- [ ] Not started
- [ ] In progress
- [x] Implemented
- [x] Tested

After every major phase, report:

1. What was implemented.
2. Which files were created.
3. Which files were modified.
4. Which tests were executed.
5. Test results.
6. Known limitations.
7. Remaining tasks.

Keep documentation synchronized with implementation.

When using Claude Code, work directly in the repository.

Do not stop after a plan if you have the tools and permission to implement the next phase.

If context becomes limited, update the project status document with exact completed work and the next steps so development can resume without losing architectural decisions.

---

# 40. FINAL DELIVERY REQUIREMENTS

The final deliverable must include:

- Complete Go source code
- Embedded frontend
- SQLite database migrations
- Media scanning
- FFprobe integration
- FFmpeg thumbnail generation
- Direct video streaming
- HTTP Range support
- Responsive premium UI
- Video search and filtering
- Playback progress
- Continue Watching
- Subtitle support
- Folder management
- Scanner dashboard
- Server settings
- LAN discovery
- Security implementation
- Automated tests
- Windows build
- Ubuntu build
- README
- Architecture documentation
- API documentation
- Deployment documentation

---

# 41. FINAL ACCEPTANCE CRITERIA

StreamVault is considered complete only when:

1. It builds successfully.
2. It runs on Ubuntu.
3. It runs on Windows.
4. It displays LAN access URLs.
5. It works from mobile browsers.
6. It registers multiple media folders.
7. It scans 500+ video files.
8. It generates thumbnails.
9. It extracts video metadata.
10. It displays a responsive library.
11. It streams original video files.
12. HTTP Range seeking works.
13. Multiple clients can stream simultaneously.
14. Playback history is persistent.
15. Cross-device resume works.
16. Subtitles work where supported.
17. The application survives restarts.
18. Original media files remain untouched.
19. Unauthorized filesystem access is prevented.
20. Automated tests pass.
21. Documentation matches the actual implementation.

---

# 42. FINAL EXECUTION INSTRUCTION

Start now.

First inspect the workspace and create the implementation checklist.

Then build StreamVault phase by phase.

Prioritize:

1. Reliability
2. Streaming correctness
3. Security
4. Performance
5. Maintainability
6. User experience

Do not overengineer the initial version.

Do not build optional transcoding before reliable Direct Play is working.

Do not stop after generating an architecture plan.

Implement the actual working software.

The final product should feel like a premium private Netflix running entirely on my own PC and accessible through my local network.

**Project Name: StreamVault**

**Tagline: Your Personal Cinema.**

**Backend: Go**

**Frontend: Premium Responsive Dark UI**

**Database: SQLite**

**Access: Private LAN**

**Deployment: Windows + Ubuntu**

Now begin implementation.