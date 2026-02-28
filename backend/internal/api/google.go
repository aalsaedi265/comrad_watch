package api

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/crypto"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/gdrive"
	"github.com/google/uuid"
)

type googleHandler struct {
	cfg     *config.Config
	queries *db.Queries
}

type googleAuthURLResponse struct {
	URL string `json:"url"`
}

type googleStatusResponse struct {
	Connected bool `json:"connected"`
}

// AuthURL returns the Google OAuth consent URL for the authenticated user.
// The mobile app opens this URL in a browser to start the OAuth flow.
func (h *googleHandler) AuthURL(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	if h.cfg.GoogleClientID == "" || h.cfg.GoogleClientSecret == "" {
		writeError(w, http.StatusServiceUnavailable, "Google Drive not configured")
		return
	}

	oauthCfg := gdrive.OAuthConfig(h.cfg.GoogleClientID, h.cfg.GoogleClientSecret, h.cfg.GoogleRedirectURI)

	// Encrypt the user ID as state parameter so we can identify the user on callback
	state, err := crypto.Encrypt(userID.String(), h.cfg.EncryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate auth state")
		return
	}

	url := gdrive.AuthURL(oauthCfg, state)
	writeJSON(w, http.StatusOK, googleAuthURLResponse{URL: url})
}

// Callback handles the Google OAuth redirect after user authorization.
// Google redirects here with ?code=...&state=...
// This is NOT a JSON endpoint — it returns HTML so the user sees a success message.
func (h *googleHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" || state == "" {
		http.Error(w, "Missing code or state parameter", http.StatusBadRequest)
		return
	}

	// Decrypt the state to get the user ID
	userIDStr, err := crypto.Decrypt(state, h.cfg.EncryptionKey)
	if err != nil {
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID in state", http.StatusBadRequest)
		return
	}

	// Exchange the authorization code for tokens
	oauthCfg := gdrive.OAuthConfig(h.cfg.GoogleClientID, h.cfg.GoogleClientSecret, h.cfg.GoogleRedirectURI)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	token, err := gdrive.ExchangeCode(ctx, oauthCfg, code)
	if err != nil {
		log.Printf("gdrive: failed to exchange code: %v", err)
		http.Error(w, "Failed to connect Google Drive", http.StatusInternalServerError)
		return
	}

	// Serialize and encrypt the token
	tokenJSON, err := gdrive.MarshalToken(token)
	if err != nil {
		http.Error(w, "Failed to serialize token", http.StatusInternalServerError)
		return
	}

	encryptedToken, err := crypto.Encrypt(tokenJSON, h.cfg.EncryptionKey)
	if err != nil {
		http.Error(w, "Failed to encrypt token", http.StatusInternalServerError)
		return
	}

	// Store the encrypted token
	if err := h.queries.SetUserGoogleToken(ctx, userID, encryptedToken); err != nil {
		http.Error(w, "Failed to store token", http.StatusInternalServerError)
		return
	}

	// Return a simple HTML page
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head><title>Comrad Watch</title>
<style>
  body { background: #0a0a0a; color: #fff; font-family: system-ui, sans-serif;
         display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; }
  .card { text-align: center; padding: 40px; }
  h1 { color: #4CAF50; }
  p { color: #aaa; margin-top: 16px; }
</style></head>
<body>
  <div class="card">
    <h1>Google Drive Connected</h1>
    <p>You can close this window and return to the app.</p>
    <p>Your recordings will now be automatically uploaded to Google Drive.</p>
  </div>
</body>
</html>`))
}

// Status returns whether the authenticated user has connected Google Drive.
func (h *googleHandler) Status(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	token, err := h.queries.GetUserGoogleToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check status")
		return
	}

	writeJSON(w, http.StatusOK, googleStatusResponse{
		Connected: token != nil && *token != "",
	})
}
