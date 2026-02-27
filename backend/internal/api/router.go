package api

import (
	"net/http"

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

	// Public routes
	mux.HandleFunc("POST /api/register", auth.Register)
	mux.HandleFunc("POST /api/login", auth.Login)

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
	mux.HandleFunc("GET /api/video/{key}", ig.ServePublicVideo)

	// PWA chunk upload routes (protected)
	chunks := &chunkHandler{cfg: cfg, queries: queries, rtmpSrv: rtmpSrv}
	mux.HandleFunc("POST /api/sessions/{id}/chunk", requireAuth(cfg, chunks.ReceiveChunk))
	mux.HandleFunc("POST /api/sessions/{id}/end", requireAuth(cfg, chunks.EndWebSession))

	// Serve PWA static files from web/ directory
	// API routes take priority (more specific patterns win in Go 1.22+ mux)
	mux.Handle("/", http.FileServer(http.Dir("web")))

	return withCORS(mux)
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
