# Comrad Watch

One-tap video recording for activists. Your phone streams to a remote server in real-time — if the phone is destroyed, the server has the footage.

---

## Quick Start

**Use Docker.** It handles the database, FFmpeg, and all configuration. Running without Docker means installing PostgreSQL, Go, and FFmpeg yourself — it's an uphill battle and highly unrecommended.

```bash
git clone https://github.com/aalsaedi265/comrad_watch.git
cd comrad_watch
docker compose up --build
```

Open **http://localhost:8080** — register an account and start recording. That's it.

---

## What's Running

When you run `docker compose up --build`, three things start:

| Service | Port | What It Does |
|---------|------|-------------|
| **PostgreSQL** | 5432 | Stores user accounts, session metadata |
| **REST API + Web App** | 8080 | Serves the PWA and handles video uploads |
| **RTMP Ingest** | 1935 | Receives live video from the Android app |

The database schema is created automatically on first run. No manual SQL needed.


## Using the Android App

The native Android app streams video via RTMP for better quality and background recording support.

### Build from source:

1. Install **JDK 17+** from [adoptium.net](https://adoptium.net/)
2. Open the `mobile/` folder in Android Studio
3. Wait for Gradle sync to finish
4. Click **Run** to install on a device or emulator

### First launch:

1. Enter the server URL:
   - **Emulator**: `http://10.0.2.2:8080`
   - **Physical device on same WiFi**: `http://your-computer-ip:8080`
2. Register or log in
3. Grant camera + microphone permissions
4. Tap the red button — streaming starts immediately

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
1. Tap the Share button (square with arrow)
2. Tap **Add to Home Screen** → **Add**
3. Launch it from your home screen — runs fullscreen like a native app

**Android (Chrome):**
1. Tap the three-dot menu → **Add to Home screen** (or **Install app**)
2. Launch it from your home screen

### Connect Google Drive (optional):

1. Tap the gear icon → Settings
2. Tap **Connect Google Drive**
3. Sign in and allow access
4. All future recordings are automatically saved to your Drive

### Connect Instagram (optional):

1. In Settings, tap **Connect Instagram**
2. Sign in with your Instagram account
3. All future recordings are automatically posted as Instagram Stories

---


## Deploying to Production

Production requires HTTPS — browsers block camera and microphone access on plain HTTP. The production setup includes [Caddy](https://caddyserver.com/) which handles HTTPS certificates automatically via Let's Encrypt.

### Step 1: Get a server

Any VPS with Docker works (DigitalOcean, Linode, Hetzner, etc.). Minimum: 1 CPU, 1 GB RAM.

### Step 2: Point your domain

Set an **A record** for your domain (e.g., `comradwatch.org`) pointing to your server's IP address. Wait for DNS to propagate.

### Step 3: Clone and configure

```bash
git clone https://github.com/aalsaedi265/comrad_watch.git
cd comrad_watch
./deploy.sh
```

The deploy script will:
- Ask for your domain name
- Generate strong random secrets (JWT, encryption keys, database password)
- Optionally configure Google Drive and Instagram credentials
- Write everything to `backend/.env` and `.env`

### Step 4: Start the server

```bash
docker compose -f docker-compose.prod.yml up -d
```

Caddy automatically gets an HTTPS certificate from Let's Encrypt. Your app is live at `https://your-domain.com`.

### Step 5: Open firewall ports

Make sure these ports are open:

| Port | Protocol | Required For |
|------|----------|-------------|
| 80 | TCP | HTTP → HTTPS redirect |
| 443 | TCP | HTTPS (web app + API) |
| 1935 | TCP | RTMP (Android app streaming) |

### Google Drive setup (optional):

1. Go to [Google Cloud Console](https://console.cloud.google.com/apis/credentials)
2. Create an OAuth 2.0 Client ID (Web application)
3. Set authorized redirect URI to `https://your-domain.com/api/google/callback`
4. Copy the Client ID and Client Secret into `backend/.env`

### Instagram setup (optional):

1. Go to [Meta Developer Console](https://developers.facebook.com/)
2. Create an app with Instagram Basic Display
3. Copy the App ID and App Secret into `backend/.env`

### Monitoring:

```bash
# View logs
docker compose -f docker-compose.prod.yml logs -f

# Check if everything is running
docker compose -f docker-compose.prod.yml ps

# Restart after config changes
docker compose -f docker-compose.prod.yml up -d
```

---

## Running Without Docker (Not Recommended)

> **Seriously, use Docker.** The instructions below require you to install and manage PostgreSQL, Go 1.25+, and FFmpeg yourself. If any of these are missing or misconfigured, things will break in confusing ways. Docker handles all of this for you with a single command.

If you still want to run the Go backend directly:

### Using the setup script:

```bash
# Linux/Mac
./setup.sh
cd backend && go run ./cmd/server

# Windows
setup.bat
cd backend && go run .\cmd\server
```

The setup script generates `backend/.env` with random secrets and starts PostgreSQL via Docker (you still need Docker for the database, at minimum).

### Fully manual setup:

1. Install and start PostgreSQL, then create the database:
   ```bash
   createdb comradwatch
   ```

2. Create and configure the env file:
   ```bash
   cd backend
   cp .env.example .env
   # Edit .env — set DATABASE_URL, JWT_SECRET, ENCRYPTION_KEY
   ```

3. Install FFmpeg:
   - **Mac**: `brew install ffmpeg`
   - **Linux**: `sudo apt install ffmpeg`
   - **Windows**: Download from [ffmpeg.org](https://ffmpeg.org/download.html) and add to PATH

4. Run the server:
   ```bash
   go run ./cmd/server
   ```

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

## Project Structure

```
comrad_watch/
  backend/
    cmd/server/       # Entry point
    internal/
      api/            # REST API handlers + middleware
      rtmp/           # RTMP video ingest + post-processing
      db/             # Database connection + migrations
      config/         # Environment config
      crypto/         # AES-256 encryption for OAuth tokens
      gdrive/         # Google Drive upload
      instagram/      # Instagram Story posting
    web/              # PWA frontend (HTML/CSS/JS)
    migrations/       # SQL schema
  mobile/
    shared/           # Kotlin shared code (API client)
    androidApp/       # Android native app
  setup.sh            # Local dev setup (Linux/Mac)
  setup.bat           # Local dev setup (Windows)
  deploy.sh           # Production setup
  docker-compose.yml          # Dev (local)
  docker-compose.prod.yml     # Production (HTTPS + Caddy)
  Caddyfile                   # Reverse proxy config
```

## Status

| Feature | Status |
|---------|--------|
| Backend (API + video ingest) | Complete |
| Android app | Complete |
| Google Drive auto-upload | Complete |
| Instagram Story auto-post | Complete |
| Web app (PWA for iPhone + desktop) | Complete |
