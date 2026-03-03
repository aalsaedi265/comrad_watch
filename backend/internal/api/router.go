package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/instagram"
	"github.com/comradwatch/backend/internal/rtmp"
)

func NewRouter(cfg *config.Config, queries *db.Queries, rtmpSrv *rtmp.Server) http.Handler {
	mux := http.NewServeMux()

	auth := &authHandler{cfg: cfg, queries: queries}
	sessions := &sessionHandler{cfg: cfg, queries: queries}
	google := &googleHandler{cfg: cfg, queries: queries}
	ig := &instagramHandler{
		cfg:     cfg,
		queries: queries,
		ig:      instagram.NewClient(cfg.InstagramAppID, cfg.InstagramAppSecret),
	}

	// Public routes (rate limited: 10 attempts per minute per IP)
	authLimiter := newRateLimiter(10, time.Minute)
	mux.HandleFunc("POST /api/register", authLimiter.wrap(auth.Register))
	mux.HandleFunc("POST /api/login", authLimiter.wrap(auth.Login))

	// Health check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Public config (non-secret values the mobile app needs)
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"instagram_app_id": cfg.InstagramAppID,
		})
	})

	// Protected routes (require JWT)
	mux.HandleFunc("POST /api/sessions/start", requireAuth(cfg, sessions.StartSession))
	mux.HandleFunc("GET /api/sessions", requireAuth(cfg, sessions.ListSessions))

	// Google Drive routes
	mux.HandleFunc("GET /api/google/auth-url", requireAuth(cfg, google.AuthURL))
	mux.HandleFunc("GET /api/google/callback", google.Callback) // Public — browser redirect from Google
	mux.HandleFunc("GET /api/google/status", requireAuth(cfg, google.Status))

	// Instagram routes (protected)
	mux.HandleFunc("POST /api/instagram/connect", requireAuth(cfg, ig.ConnectInstagram))
	mux.HandleFunc("GET /api/instagram/status", requireAuth(cfg, ig.InstagramStatus))
	mux.HandleFunc("DELETE /api/instagram/disconnect", requireAuth(cfg, ig.DisconnectInstagram))
	mux.HandleFunc("GET /api/sessions/{id}/video", requireAuth(cfg, ig.ServeSessionVideo))

	// Public video endpoint (Instagram API fetches this server-side)
	// Rate limited: 20 requests per minute per IP to prevent brute-force key guessing
	videoLimiter := newRateLimiter(20, time.Minute)
	mux.HandleFunc("GET /api/video/{key}", videoLimiter.wrap(ig.ServePublicVideo))

	// PWA chunk upload routes (protected)
	chunks := &chunkHandler{cfg: cfg, queries: queries, rtmpSrv: rtmpSrv}
	mux.HandleFunc("POST /api/sessions/{id}/chunk", requireAuth(cfg, chunks.ReceiveChunk))
	mux.HandleFunc("POST /api/sessions/{id}/end", requireAuth(cfg, chunks.EndWebSession))

	// Serve PWA static files from web/ directory
	// API routes take priority (more specific patterns win in Go 1.22+ mux)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return withLogging(withSecurityHeaders(cfg, mux))
}

// --- Rate Limiter (in-memory, per-IP) ---

type rateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientRate
	limit   int
	window  time.Duration
}

type clientRate struct {
	count   int
	resetAt time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		clients: make(map[string]*clientRate),
		limit:   limit,
		window:  window,
	}
}

func (rl *rateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Evict expired entries on every check to bound memory usage.
	for k, v := range rl.clients {
		if now.After(v.resetAt) {
			delete(rl.clients, k)
		}
	}

	client, ok := rl.clients[ip]
	if !ok {
		rl.clients[ip] = &clientRate{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	client.count++
	return client.count <= rl.limit
}

func (rl *rateLimiter) wrap(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := clientIP(r)
		if !rl.allow(ip) {
			writeError(w, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		next(w, r)
	}
}

// clientIP extracts the real client IP, respecting X-Forwarded-For when
// behind a reverse proxy (Caddy, nginx, etc.). Falls back to RemoteAddr.
func clientIP(r *http.Request) string {
	// X-Forwarded-For: client, proxy1, proxy2 — first entry is the real client
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if comma := strings.Index(xff, ","); comma > 0 {
			return strings.TrimSpace(xff[:comma])
		}
		return strings.TrimSpace(xff)
	}

	// Strip port from RemoteAddr
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	return ip
}

// --- Request Logging ---

type statusWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.status = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

func withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

// --- Security Headers ---

func withSecurityHeaders(cfg *config.Config, next http.Handler) http.Handler {
	// Build allowed origin from config (same-origin only)
	var allowedOrigin string
	if cfg.PublicHost == "localhost" {
		allowedOrigin = fmt.Sprintf("http://localhost:%d", cfg.HTTPPort)
	} else {
		allowedOrigin = "https://" + cfg.PublicHost
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// CORS — only allow requests from the configured origin
		origin := r.Header.Get("Origin")
		if origin != "" && origin == allowedOrigin {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		}

		// Content Security Policy
		w.Header().Set("Content-Security-Policy",
			"default-src 'self'; "+
				"script-src 'self'; "+
				"style-src 'self' https://fonts.googleapis.com; "+
				"font-src 'self' https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self'")

		// Prevent MIME-type sniffing
		w.Header().Set("X-Content-Type-Options", "nosniff")

		// Prevent clickjacking
		w.Header().Set("X-Frame-Options", "DENY")

		// Don't leak URLs in referrer headers (stream keys are in URLs)
		w.Header().Set("Referrer-Policy", "no-referrer")

		// Force HTTPS after first visit (1 year)
		if cfg.PublicHost != "localhost" {
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		// Restrict browser features
		w.Header().Set("Permissions-Policy", "camera=(self), microphone=(self), geolocation=()")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
