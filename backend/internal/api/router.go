package api

import (
	"net/http"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
)

func NewRouter(cfg *config.Config, queries *db.Queries) http.Handler {
	mux := http.NewServeMux()

	auth := &authHandler{cfg: cfg, queries: queries}
	sessions := &sessionHandler{cfg: cfg, queries: queries}
	google := &googleHandler{cfg: cfg, queries: queries}

	// Public routes
	mux.HandleFunc("POST /api/register", auth.Register)
	mux.HandleFunc("POST /api/login", auth.Login)

	// Health check
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	// Protected routes (require JWT)
	mux.HandleFunc("POST /api/sessions/start", requireAuth(cfg, sessions.StartSession))
	mux.HandleFunc("GET /api/sessions", requireAuth(cfg, sessions.ListSessions))

	// Google Drive OAuth (Phase 3)
	mux.HandleFunc("GET /api/google/auth-url", requireAuth(cfg, google.AuthURL))
	mux.HandleFunc("GET /api/google/callback", google.Callback) // public: Google redirects here
	mux.HandleFunc("GET /api/google/status", requireAuth(cfg, google.Status))

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
