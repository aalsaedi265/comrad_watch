# Comrad Watch - Project Status

Last updated: 2026-02-26

## What This Project Is

A mobile app for social justice activists. One tap opens the camera, starts recording, and streams video to a remote server in real-time via RTMP. If the phone is destroyed or seized, the server already has the footage and will automatically save it to Google Drive and post it as an Instagram Story.

## What's Built

### Phase 1: Go Backend (COMPLETE)

Everything in `backend/`. A single Go binary that:

1. **RTMP ingest server** (port 1935) — accepts live video streams from phones
   - Authenticates streams via stream key (tied to a session in the database)
   - Writes incoming video to a single FLV file per session with `fsync` every 2 seconds (crash-resilient)
   - On disconnect (intentional or phone destroyed): closes the FLV, converts to MP4 via FFmpeg, updates the database
   - Files: `internal/rtmp/server.go`, `internal/rtmp/handler.go`

2. **REST API** (port 8080) — serves the mobile app
   - `POST /api/register` — create account (email + password, bcrypt hashed)
   - `POST /api/login` — get JWT token (30-day expiry)
   - `POST /api/sessions/start` — create a streaming session, returns RTMP URL + stream key
   - `GET /api/sessions` — list user's sessions
   - `GET /api/health` — health check
   - `GET /api/config` — public server config (Instagram App ID)
   - `POST /api/instagram/connect` — exchange Instagram OAuth code for long-lived token
   - `GET /api/instagram/status` — check if user has connected Instagram
   - `DELETE /api/instagram/disconnect` — remove stored Instagram connection
   - `GET /api/sessions/{id}/video` — serve recorded MP4 (auth required)
   - `GET /api/video/{key}` — serve recorded MP4 by stream key (public, for Instagram API)
   - Files: `internal/api/router.go`, `internal/api/auth.go`, `internal/api/sessions.go`, `internal/api/instagram.go`

3. **PostgreSQL database** — users, sessions, segments tables
   - Auto-migration on startup (no manual SQL needed)
   - Files: `internal/db/db.go`, `internal/db/queries.go`, `internal/db/migrate.go`, `migrations/001_initial.sql`

4. **Docker support** — `docker-compose.yml` runs PostgreSQL + backend with one command

### Phase 2: Android App (COMPLETE)

Everything in `mobile/`. A Kotlin Multiplatform project with:

1. **Shared KMP module** (`mobile/shared/`) — API client and data models shared between Android and future iOS app
   - Ktor HTTP client, kotlinx.serialization
   - Files: `shared/src/commonMain/.../api/ComradApi.kt`, `shared/src/commonMain/.../model/Models.kt`

2. **Android app** (`mobile/androidApp/`) — Jetpack Compose UI
   - **Main screen**: giant pulsing red "TAP TO RECORD" button, dark background, no distractions
   - **Recording screen**: full-screen camera viewfinder, pulsing red border, timer, LIVE indicator, slide-up menu with STOP & SAVE / STOP & DISCARD
   - **Setup screen**: one-time server URL + login/register + Instagram connection
   - **RootEncoder** for RTMP streaming (camera + mic capture built-in)
   - **Foreground service** keeps recording alive when app is backgrounded
   - Builds to a ~15MB APK

### Phase 4: Instagram Story Posting (COMPLETE)

Automatic Instagram Story posting when a recording finishes.

1. **Instagram Graph API client** (`internal/instagram/client.go`)
   - OAuth code exchange → short-lived token → long-lived token (~60 days)
   - Story publishing: create container → poll for processing → publish
   - User info retrieval (account ID, username)

2. **Token encryption** (`internal/crypto/encrypt.go`)
   - AES-256-GCM encryption for Instagram tokens stored in the database
   - Key derived from JWT secret via SHA-256

3. **Automatic story posting** in `postProcess()` (`internal/rtmp/server.go`)
   - After FLV → MP4 conversion, checks if user has Instagram connected
   - Builds a public video URL using the stream key as a secret token
   - Creates a story container via Instagram API, polls until processed, publishes
   - Records the story ID in the sessions table

4. **Android Instagram OAuth flow**
   - Setup screen shows "CONNECT INSTAGRAM" button after login
   - Opens Instagram OAuth in browser, redirects back via `comradwatch://instagram-callback`
   - Deep link handled in `MainActivity`, code exchanged via backend
   - Shows connected/disconnected status with disconnect option

### What's NOT Built Yet

| Phase | What | Context for Implementation |
|-------|------|--------------------------|
| **Phase 3** | **Google Drive upload** | Backend needs Google Drive API integration (`google.golang.org/api/drive/v3`). On stream finalization, upload the MP4 to the user's Drive under `ComradWatch/YYYY-MM-DD/` folder. Mobile app needs Google OAuth flow in the KMP shared module. The `postProcess()` function in `server.go` has a `TODO Phase 3` comment marking exactly where upload code goes. DB already has `google_drive_file_id` column and `SetSessionDriveFileID()` query. Config already has `GoogleClientID` / `GoogleClientSecret` fields. |
| **Phase 5** | **iOS app** | SwiftUI UI layer + AVFoundation camera + HaishinKit for RTMP streaming. The KMP shared module already compiles for iOS targets (iosX64, iosArm64, iosSimulatorArm64). The shared API client and models will be reused. Only the UI layer and camera/streaming code need to be written natively in Swift. |
| **Phase 6** | **Polish & launch** | Reconnection logic for dropped RTMP streams, local recording gap-fill, error UX, app store submissions. |

## Key Libraries & Gotchas

### Go Backend
- **yutopp/go-rtmp + go-flv**: RTMP handler's `ConnConfig.Logger` must be `logrus.StandardLogger()` (not stdlib log)
- Audio/video data readers are consumed on decode — MUST buffer to `bytes.Buffer` first (see handler.go)
- `OnSetDataFrame` handler is required to capture stream metadata

### Instagram API
- Instagram Content Publishing API requires a Business or Creator account
- Instagram Live is NOT possible programmatically — only Stories (pre-recorded video)
- Video URL passed to Instagram must be publicly accessible (our `/api/video/{key}` endpoint)
- Story container processing takes ~10-60 seconds; we poll every 10 seconds up to 5 minutes
- Long-lived tokens last ~60 days; token refresh is not yet implemented (Phase 6)
- `PUBLIC_HOST` env var must be set to the server's externally-reachable hostname

### Android / RootEncoder v2.5.3
- Interface is `ConnectChecker` from `com.pedro.common` (NOT `ConnectCheckerRtmp`)
- All callbacks have NO "Rtmp" suffix: `onConnectionSuccess()`, `onDisconnect()`, etc.
- Constructor: `RtmpCamera2(OpenGlView, ConnectChecker)` — NOT SurfaceView
- `prepareVideo` takes 5 params: `(width, height, fps, bitrate, rotation)`

### Gradle / KMP
- AGP 8.7.3, Kotlin 2.1.0, Gradle 8.10.2, compileSdk 36
- JAVA_HOME must point to Android Studio's bundled JDK: `C:\Program Files\Android\Android Studio\jbr`

## File Map

```
comrad_watch/
  backend/
    cmd/server/main.go          # Entry point
    internal/
      api/router.go             # HTTP routes + CORS
      api/auth.go               # Register, login, JWT, middleware
      api/sessions.go           # Start session, list sessions
      api/instagram.go          # Instagram OAuth + video serving
      config/config.go          # Env-based config
      crypto/encrypt.go         # AES-256-GCM token encryption
      db/db.go                  # PostgreSQL pool
      db/queries.go             # All SQL queries (including Instagram)
      db/migrate.go             # Auto-migration runner
      instagram/client.go       # Instagram Graph API client
      rtmp/server.go            # RTMP server, stream lifecycle, FFmpeg, Instagram posting
      rtmp/handler.go           # RTMP protocol handler (audio/video/metadata)
    migrations/001_initial.sql  # Schema: users, sessions, segments
    Dockerfile                  # Multi-stage build (Go → Alpine + FFmpeg)
    .env.example                # Environment variable template
  mobile/
    shared/src/commonMain/.../
      api/ComradApi.kt          # Shared HTTP client (Ktor) — includes Instagram methods
      model/Models.kt           # Data classes for API — includes Instagram types
    androidApp/src/main/kotlin/.../
      MainActivity.kt           # Entry point + Instagram deep link handler
      ComradApp.kt              # App singleton with API client + Instagram App ID
      ui/MainScreen.kt          # "TAP TO RECORD" button
      ui/RecordingScreen.kt     # Camera viewfinder + recording controls
      ui/SetupScreen.kt         # Login/register + Instagram connect
      ui/Navigation.kt          # Nav graph (setup → main → recording)
      ui/theme/Theme.kt         # High-contrast dark theme
      camera/CameraPreviewView.kt  # RootEncoder camera + RTMP wrapper
      service/StreamingService.kt   # Foreground service
    gradle/libs.versions.toml   # Version catalog
  docker-compose.yml            # PostgreSQL + backend (with Instagram env vars)
```
