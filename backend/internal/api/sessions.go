package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
)

type sessionHandler struct {
	cfg     *config.Config
	queries *db.Queries
}

type startSessionResponse struct {
	SessionID string `json:"session_id"`
	StreamKey string `json:"stream_key"`
	RTMPUrl   string `json:"rtmp_url"`
}

type sessionListItem struct {
	ID           string  `json:"id"`
	StartedAt    string  `json:"started_at"`
	EndedAt      *string `json:"ended_at"`
	EndReason    *string `json:"end_reason"`
	Status       string  `json:"status"`
	TotalSegments int    `json:"total_segments"`
}

// StartSession creates a new streaming session and returns a stream key.
// The mobile app uses this stream key to connect via RTMP.
func (h *sessionHandler) StartSession(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	// Generate a unique stream key
	streamKey, err := generateStreamKey()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate stream key")
		return
	}

	session, err := h.queries.CreateSession(r.Context(), userID, streamKey)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session")
		return
	}

	writeJSON(w, http.StatusCreated, startSessionResponse{
		SessionID: session.ID.String(),
		StreamKey: streamKey,
		RTMPUrl:   "rtmp://YOUR_SERVER_IP:1935/live/" + streamKey,
	})
}

// ListSessions returns the user's recent streaming sessions.
func (h *sessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	// For now, return empty list - will implement with pagination later
	writeJSON(w, http.StatusOK, []sessionListItem{})
}

// generateStreamKey creates a cryptographically random stream key.
func generateStreamKey() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}
