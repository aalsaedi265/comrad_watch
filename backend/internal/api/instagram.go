package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

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

	var req connectInstagramRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Code == "" || req.RedirectURI == "" {
		writeError(w, http.StatusBadRequest, "code and redirect_uri are required")
		return
	}

	ctx := r.Context()

	// Step 1: Exchange auth code for short-lived token
	shortToken, err := h.ig.ExchangeCode(ctx, req.Code, req.RedirectURI)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to exchange code: "+err.Error())
		return
	}

	// Step 2: Exchange for long-lived token (~60 days)
	longToken, err := h.ig.ExchangeLongLived(ctx, shortToken.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to get long-lived token: "+err.Error())
		return
	}

	// Step 3: Get the user's Instagram account info
	userInfo, err := h.ig.GetUserInfo(ctx, longToken.AccessToken)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to get user info: "+err.Error())
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

// ServePublicVideo serves a session video without auth, keyed by stream key.
// This is needed because the Instagram API fetches the video_url server-side
// and cannot provide our JWT token. The stream key acts as a secret URL token.
func (h *instagramHandler) ServePublicVideo(w http.ResponseWriter, r *http.Request) {
	streamKey := r.PathValue("key")
	if streamKey == "" {
		writeError(w, http.StatusBadRequest, "stream key required")
		return
	}

	// Look up session by stream key (any status, not just active)
	var sessionID uuid.UUID
	err := h.queries.Pool().QueryRow(r.Context(),
		`SELECT id FROM sessions WHERE stream_key = $1`, streamKey,
	).Scan(&sessionID)
	if err != nil {
		writeError(w, http.StatusNotFound, "not found")
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
