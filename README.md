# Comrad Watch

One-tap video recording for activists. Phone streams to a remote server in real-time — if the phone is destroyed, the server has the footage.

## Prerequisites

- **Docker + Docker Compose** (for PostgreSQL + backend)
- **FFmpeg** (for FLV-to-MP4 conversion, included in Docker image)
- **Android Studio** (for building the mobile app)
- **Go 1.24+** (only if running backend outside Docker)

## Quick Start

### 1. Start the backend

```bash
cd comrad_watch

# Copy env template
cp backend/.env.example backend/.env
# Edit backend/.env — set a real JWT_SECRET

# Start PostgreSQL + backend server
docker compose up --build
```

This starts:
- PostgreSQL on port **5432**
- REST API on port **8080**
- RTMP ingest on port **1935**

The database schema is applied automatically on first run.

### 2. Verify the backend

```bash
curl http://localhost:8080/api/health
# → {"status":"ok"}

# Register a test user
curl -X POST http://localhost:8080/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123"}'

# Login
curl -X POST http://localhost:8080/api/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"testpass123"}'
# → {"token":"eyJ...","user":{...}}
```

### 3. Build the Android app

```bash
cd mobile

# Set JAVA_HOME to Android Studio's bundled JDK
export JAVA_HOME="C:\Program Files\Android\Android Studio\jbr"

# Build debug APK
./gradlew assembleDebug
```

The APK is at: `mobile/androidApp/build/outputs/apk/debug/androidApp-debug.apk`

### 4. Run on Android device/emulator

Install the APK on a device or emulator. On first launch:

1. Enter the server URL:
   - **Emulator**: `http://10.0.2.2:8080` (routes to host machine)
   - **Physical device**: `http://<your-computer-ip>:8080` (same WiFi network)
2. Register or log in with email + password
3. Grant camera + microphone permissions
4. Tap the big red button to start recording + streaming

### Running backend without Docker

If you prefer running Go directly:

```bash
# Start PostgreSQL separately (or use an existing instance)
# Then:
cd backend
cp .env.example .env
# Edit .env with your DATABASE_URL and JWT_SECRET

go run ./cmd/server
```

## Testing the RTMP stream

You can test the RTMP ingest without the mobile app using FFmpeg:

```bash
# First, create a session via the API to get a stream key:
TOKEN="your-jwt-token-here"
curl -X POST http://localhost:8080/api/sessions/start \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json"
# → {"session_id":"...","rtmp_url":"rtmp://localhost:1935/live","stream_key":"abc123..."}

# Stream a test video:
ffmpeg -f lavfi -i testsrc=size=640x480:rate=30 \
       -f lavfi -i sine=frequency=440:sample_rate=44100 \
       -c:v libx264 -preset ultrafast -tune zerolatency \
       -c:a aac -b:a 128k \
       -f flv "rtmp://localhost:1935/live/YOUR_STREAM_KEY"
```

After stopping the FFmpeg stream (Ctrl+C), the server will convert the FLV to MP4 automatically. Check `backend/segments/<session-id>/` for the output files.

## Project Status

See [docs/STATUS.md](docs/STATUS.md) for what's built, what's remaining, and implementation context for each phase.
