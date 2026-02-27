package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/rtmp"
	"github.com/google/uuid"
)

type chunkHandler struct {
	cfg     *config.Config
	queries *db.Queries
	rtmpSrv *rtmp.Server
}

// ReceiveChunk accepts a raw video blob from the browser's MediaRecorder
// and appends it to the session's recording file on disk.
// Called every ~2 seconds during a PWA recording session.
func (h *chunkHandler) ReceiveChunk(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	session, err := h.queries.GetSessionByID(r.Context(), sessionID)
	if err != nil || session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if session.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if session.Status != "active" {
		writeError(w, http.StatusConflict, "session is not active")
		return
	}

	// Append chunk to the recording file (streaming, no memory buffering)
	recordingPath := filepath.Join(h.cfg.SegmentDir, sessionID.String(), "recording.webm")

	// Ensure session directory exists
	if err := os.MkdirAll(filepath.Dir(recordingPath), 0755); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create session directory")
		return
	}

	f, err := os.OpenFile(recordingPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to open recording file")
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, r.Body); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write chunk")
		return
	}

	f.Sync()

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// EndWebSession signals that the PWA recording is done.
// Triggers post-processing (FFmpeg → MP4 → Google Drive → Instagram).
type endSessionRequest struct {
	MimeType string `json:"mime_type"`
}

func (h *chunkHandler) EndWebSession(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)

	sessionID, err := uuid.Parse(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session id")
		return
	}

	session, err := h.queries.GetSessionByID(r.Context(), sessionID)
	if err != nil || session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	if session.UserID != userID {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	if session.Status != "active" {
		writeError(w, http.StatusConflict, "session is not active")
		return
	}

	// Parse optional mime_type from body
	var req endSessionRequest
	json.NewDecoder(r.Body).Decode(&req) // ignore errors — mime_type is optional

	// Calculate duration
	duration := int(time.Since(session.StartedAt).Seconds())

	// End session in DB
	if err := h.queries.EndSession(r.Context(), sessionID, "user_stopped", 1, duration); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to end session")
		return
	}

	// Trigger async post-processing for web recordings
	rawPath := filepath.Join(h.cfg.SegmentDir, sessionID.String(), "recording.webm")
	h.rtmpSrv.PostProcessWebSession(sessionID, userID, session.StreamKey, rawPath, req.MimeType)

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
