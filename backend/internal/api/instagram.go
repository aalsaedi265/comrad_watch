package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/crypto"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/instagram"
	"github.com/google/uuid"
)

type instagramHandler struct {
	cfg     *config.Config
	queries *db.Queries
	ig      *instagram.Client
}

// --- Connect Instagram (OAuth code exchange) ---

type connectInstagramRequest struct {
	Code        string `json:"code"`
	RedirectURI string `json:"redirect_uri"`
}

type connectInstagramResponse struct {
	Username  string `json:"username"`
	AccountID string `json:"account_id"`
}

// ConnectInstagram exchanges an Instagram authorization code for a long-lived token
// and stores it encrypted in the database.
func (h *instagramHandler) ConnectInstagram(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var req connectInstagramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" {
		writeError(w, http.StatusBadRequest, "code is required")
		return
	}

	ctx := r.Context()

	// Build redirect URI server-side — never trust client-provided values
	var redirectURI string
	if h.cfg.PublicHost == "localhost" {
		redirectURI = fmt.Sprintf("http://localhost:%d/?ig_callback=1", h.cfg.HTTPPort)
	} else {
		redirectURI = fmt.Sprintf("https://%s/?ig_callback=1", h.cfg.PublicHost)
	}

	// Step 1: Exchange auth code for short-lived token
	shortToken, err := h.ig.ExchangeCode(ctx, req.Code, redirectURI)
	if err != nil {
		log.Printf("instagram: failed to exchange code: %v", err)
		writeError(w, http.StatusBadGateway, "failed to connect Instagram")
		return
	}

	// Step 2: Exchange for long-lived token (~60 days)
	longToken, err := h.ig.ExchangeLongLived(ctx, shortToken.AccessToken)
	if err != nil {
		log.Printf("instagram: failed to get long-lived token: %v", err)
		writeError(w, http.StatusBadGateway, "failed to connect Instagram")
		return
	}

	// Step 3: Get the user's Instagram account info
	userInfo, err := h.ig.GetUserInfo(ctx, longToken.AccessToken)
	if err != nil {
		log.Printf("instagram: failed to get user info: %v", err)
		writeError(w, http.StatusBadGateway, "failed to connect Instagram")
		return
	}

	// Step 4: Encrypt the token and store in DB
	encrypted, err := crypto.Encrypt(longToken.AccessToken, h.cfg.EncryptionKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to encrypt token")
		return
	}

	if err := h.queries.SetUserInstagramToken(ctx, userID, encrypted, userInfo.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save token")
		return
	}

	writeJSON(w, http.StatusOK, connectInstagramResponse{
		Username:  userInfo.Username,
		AccountID: userInfo.ID,
	})
}

// --- Instagram connection status ---

type instagramStatusResponse struct {
	Connected bool    `json:"connected"`
	AccountID *string `json:"account_id,omitempty"`
}

// InstagramStatus returns whether the user has connected Instagram.
func (h *instagramHandler) InstagramStatus(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	token, accountID, err := h.queries.GetUserInstagramToken(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check status")
		return
	}

	connected := token != nil && *token != ""
	writeJSON(w, http.StatusOK, instagramStatusResponse{
		Connected: connected,
		AccountID: accountID,
	})
}

// --- Disconnect Instagram ---

// DisconnectInstagram removes the stored Instagram token.
func (h *instagramHandler) DisconnectInstagram(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	if err := h.queries.ClearUserInstagramToken(r.Context(), userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to disconnect")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

// --- Serve video file (for Instagram to fetch) ---

// ServeSessionVideo serves the MP4 recording for a given session.
// The session must belong to the requesting user.
func (h *instagramHandler) ServeSessionVideo(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := r.PathValue("id")
	if sessionIDStr == "" {
		writeError(w, http.StatusBadRequest, "session id required")
		return
	}

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	session, err := h.queries.GetSessionByID(r.Context(), sessionID)
	if err != nil || session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Only the session owner can access the video
	userID := getUserID(r)
	if session.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	mp4Path := filepath.Join(h.cfg.SegmentDir, sessionID.String(), "recording.mp4")
	if _, err := os.Stat(mp4Path); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, mp4Path)
}

// --- Public video endpoint (for Instagram API to fetch) ---

// publicVideoExpiry is how long a video remains accessible via the public URL
// after the session ends. After this window, the video returns 410 Gone.
// Instagram typically fetches within minutes, so 2 hours is generous.
const publicVideoExpiry = 2 * time.Hour

// ServePublicVideo serves a session video without auth, keyed by stream key.
// This is needed because the Instagram API fetches the video_url server-side
// and cannot provide our JWT token. The stream key acts as a secret URL token.
//
// SECURITY: This URL expires 2 hours after the session ends. After that,
// the video is only accessible via authenticated endpoints or Google Drive.
func (h *instagramHandler) ServePublicVideo(w http.ResponseWriter, r *http.Request) {
	streamKey := r.PathValue("key")
	if streamKey == "" {
		writeError(w, http.StatusBadRequest, "stream key required")
		return
	}

	// Look up session by stream key and check expiry
	var sessionID uuid.UUID
	var endedAt *time.Time
	err := h.queries.Pool().QueryRow(r.Context(),
		`SELECT id, ended_at FROM sessions WHERE stream_key = $1`, streamKey,
	).Scan(&sessionID, &endedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	// Enforce expiry: video only accessible for a limited window after session ends
	if endedAt != nil && time.Since(*endedAt) > publicVideoExpiry {
		writeError(w, http.StatusGone, "video link expired")
		return
	}

	mp4Path := filepath.Join(h.cfg.SegmentDir, sessionID.String(), "recording.mp4")
	if _, err := os.Stat(mp4Path); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "video not found")
		return
	}

	w.Header().Set("Content-Type", "video/mp4")
	http.ServeFile(w, r, mp4Path)
}
