# Comrad Watch

One-tap video recording for activists. Your phone streams to a remote server in real-time — if the phone is destroyed, the server has the footage.

---

## What You Need

| Tool | Required For | Install |
|------|-------------|---------|
| **Docker + Docker Compose** | Running everything (server + database) | [docker.com](https://docs.docker.com/get-docker/) |
| **Go 1.24+** | Only if running backend without Docker | [go.dev](https://go.dev/dl/) |
| **Android Studio + JDK 17** | Only if building the Android app | [developer.android.com](https://developer.android.com/studio) + [adoptium.net](https://adoptium.net/) |
| **FFmpeg** | Included in Docker image. Manual install only if running without Docker | [ffmpeg.org](https://ffmpeg.org/download.html) |

---

## Getting Started (Docker — Recommended)

### Step 1: Clone and configure

```bash
git clone https://github.com/aalsaedi265/comrad_watch.git
cd comrad_watch

# Create your env file from the template
cp backend/.env.example backend/.env
```

Open `backend/.env` and set these values:

```env
# REQUIRED — change these from the defaults
JWT_SECRET=some-long-random-string-here
ENCRYPTION_KEY=another-long-random-string-here

# OPTIONAL — only needed if you want Google Drive backup
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret
GOOGLE_REDIRECT_URI=http://localhost:8080/api/google/callback

# OPTIONAL — only needed if you want Instagram Story auto-post
INSTAGRAM_APP_ID=your-ig-app-id
INSTAGRAM_APP_SECRET=your-ig-app-secret
```

### Step 2: Start the server

```bash
docker compose up --build
```

This starts:
- **PostgreSQL** database on port 5432
- **REST API + PWA** on port 8080
- **RTMP ingest** on port 1935 (for Android app streaming)

The database schema is created automatically on first run.

### Step 3: Verify it works

Open your browser to **http://localhost:8080** — you should see the login screen.

Or test from the command line:

```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}
```

---

## Using the Web App (PWA)

The web app works on any phone or computer with a modern browser. This is the fastest way to start recording.

### On your phone:

1. Open **http://your-server-ip:8080** in Chrome (Android) or Safari (iPhone)
2. Tap **Register** — enter your email and a password
3. You'll see the main screen with a big red record button
4. Tap the red button — allow camera and microphone when prompted
5. You're recording! Video chunks upload to the server every 2 seconds
6. Tap stop when done — the server converts it to MP4 and saves it

### Save to home screen (acts like a native app):

**iPhone (Safari):**
1. Open the web app in Safari
2. Tap the Share button (square with arrow)
3. Scroll down and tap **Add to Home Screen**
4. Tap **Add** — the Comrad Watch icon appears on your home screen
5. Now tap the icon to launch it fullscreen, just like a real app

**Android (Chrome):**
1. Open the web app in Chrome
2. Tap the three-dot menu (top right)
3. Tap **Add to Home screen** (or **Install app**)
4. Tap **Add** — the icon appears on your home screen

### Connect Google Drive (optional):

1. Tap the gear icon to open Settings
2. Tap **Connect Google Drive**
3. Sign in with your Google account and allow access
4. All future recordings are automatically saved to your Drive in `ComradWatch/` folder

### Connect Instagram (optional):

1. In Settings, tap **Connect Instagram**
2. Sign in with your Instagram account
3. All future recordings are automatically posted as Instagram Stories

---

## Using the Android App

The native Android app streams video via RTMP for better quality and background recording support.

### Build from source:

1. Install **JDK 17+** from [adoptium.net](https://adoptium.net/)
2. Open the `mobile/` folder in Android Studio
3. Wait for Gradle sync to finish
4. Click **Run** (green play button) to install on a connected device or emulator

### First launch:

1. Enter the server URL:
   - **Emulator**: `http://10.0.2.2:8080`
   - **Physical device on same WiFi**: `http://your-computer-ip:8080`
2. Register or log in
3. Grant camera + microphone permissions
4. Tap the red button — streaming starts immediately

---

## Running Without Docker

If you prefer running the Go backend directly:

### 1. Install and start PostgreSQL

Make sure PostgreSQL is running with a database called `comradwatch`.

### 2. Configure environment

```bash
cd backend
cp .env.example .env
```

Edit `.env` and set `DATABASE_URL` to your PostgreSQL connection string:

```env
DATABASE_URL=postgres://youruser:yourpass@localhost:5432/comradwatch?sslmode=disable
JWT_SECRET=some-long-random-string
ENCRYPTION_KEY=another-long-random-string
```

### 3. Run the server

```bash
cd backend
go run ./cmd/server
```

The server starts on port 8080 (HTTP) and 1935 (RTMP).

### 4. Install FFmpeg

FFmpeg is needed for converting recordings to MP4. Install it for your OS:

- **Windows**: Download from [ffmpeg.org](https://ffmpeg.org/download.html) and add to PATH
- **Mac**: `brew install ffmpeg`
- **Linux**: `sudo apt install ffmpeg`

---

## Testing the RTMP Stream (Without a Phone)

You can simulate a phone stream using FFmpeg from your terminal:

```bash
# 1. Register and login to get a JWT token
TOKEN=$(curl -s -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123"}' | jq -r '.token')

# 2. Start a session to get a stream key
STREAM_KEY=$(curl -s -X POST http://localhost:8080/api/sessions/start \
  -H "Authorization: Bearer $TOKEN" | jq -r '.stream_key')

# 3. Stream a test video
ffmpeg -f lavfi -i testsrc=size=640x480:rate=30 \
       -f lavfi -i sine=frequency=440:sample_rate=44100 \
       -c:v libx264 -preset ultrafast -tune zerolatency \
       -c:a aac -b:a 128k \
       -f flv "rtmp://localhost:1935/live/$STREAM_KEY"
```

Press Ctrl+C to stop. The server automatically converts the FLV to MP4.

---

## Ports

| Port | Service |
|------|---------|
| 8080 | HTTP API + Web App |
| 1935 | RTMP video ingest (Android app) |
| 5432 | PostgreSQL (Docker only) |

## Project Structure

```
comrad_watch/
  backend/
    cmd/server/       # Entry point
    internal/
      api/            # REST API handlers
      rtmp/           # RTMP video ingest + post-processing
      db/             # Database queries + migrations
      config/         # Environment config
      crypto/         # AES-256 encryption
      gdrive/         # Google Drive integration
      instagram/      # Instagram Story integration
    web/              # PWA frontend (HTML/CSS/JS)
    migrations/       # SQL schema files
  mobile/
    shared/           # Kotlin shared code (API client)
    androidApp/       # Android native app
```

## Status

| Feature | Status |
|---------|--------|
| Backend (API + video ingest) | Complete |
| Android app | Complete |
| Google Drive auto-upload | Complete |
| Instagram Story auto-post | Complete |
| Web app (PWA for iPhone + desktop) | Complete |
