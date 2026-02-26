package rtmp

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/crypto"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/gdrive"
	"github.com/comradwatch/backend/internal/instagram"
	"github.com/google/uuid"
	"github.com/yutopp/go-flv"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
)

// Server handles RTMP ingest from mobile clients.
// Each stream is authenticated by stream key, recorded to a single FLV
// file on disk, and post-processed (FFmpeg → MP4) when the stream ends.
type Server struct {
	cfg      *config.Config
	queries  *db.Queries
	ig       *instagram.Client
	listener net.Listener
	mu       sync.Mutex
	streams  map[string]*activeStream // streamKey → activeStream
}

// activeStream tracks one active recording session.
type activeStream struct {
	sessionID  uuid.UUID
	userID     uuid.UUID
	streamKey  string
	flvFile    *os.File
	flvEncoder *flv.Encoder
	startedAt  time.Time
	lastSync   time.Time
	cancel     context.CancelFunc
}

func NewServer(cfg *config.Config, queries *db.Queries) *Server {
	return &Server{
		cfg:     cfg,
		queries: queries,
		ig:      instagram.NewClient(cfg.InstagramAppID, cfg.InstagramAppSecret),
		streams: make(map[string]*activeStream),
	}
}

func (s *Server) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", s.cfg.RTMPPort))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	s.listener = ln

	srv := rtmp.NewServer(&rtmp.ServerConfig{
		OnConnect: func(conn net.Conn) (io.ReadWriteCloser, *rtmp.ConnConfig) {
			return conn, &rtmp.ConnConfig{
				Handler: &connHandler{server: s},
				ControlState: rtmp.StreamControlStateConfig{
					DefaultBandwidthWindowSize: 6 * 1024 * 1024 / 8,
				},
				Logger: log.StandardLogger(),
			}
		},
	})

	return srv.Serve(ln)
}

func (s *Server) Stop() {
	s.mu.Lock()
	// Grab all active streams and clear the map while holding the lock.
	active := make(map[string]*activeStream, len(s.streams))
	for k, v := range s.streams {
		active[k] = v
		delete(s.streams, k)
	}
	s.mu.Unlock()

	// Finalize each stream synchronously so all data is flushed before exit.
	for key, stream := range active {
		log.Printf("finalizing stream %s on shutdown", key)
		s.finalizeStream(key, stream, "server_shutdown")
	}

	if s.listener != nil {
		s.listener.Close()
	}
}

// registerStream authenticates a stream key and opens an FLV file for recording.
func (s *Server) registerStream(streamKey string) error {
	session, err := s.queries.GetSessionByStreamKey(context.Background(), streamKey)
	if err != nil {
		return fmt.Errorf("query session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("invalid stream key: %s", streamKey)
	}

	// Create session directory
	sessionDir := filepath.Join(s.cfg.SegmentDir, session.ID.String())
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return fmt.Errorf("create session dir: %w", err)
	}

	// Open FLV file for this session
	flvPath := filepath.Join(sessionDir, "recording.flv")
	f, err := os.Create(flvPath)
	if err != nil {
		return fmt.Errorf("create FLV file: %w", err)
	}

	enc, err := flv.NewEncoder(f, flv.FlagsAudio|flv.FlagsVideo)
	if err != nil {
		f.Close()
		return fmt.Errorf("create FLV encoder: %w", err)
	}

	_, cancel := context.WithCancel(context.Background())

	stream := &activeStream{
		sessionID:  session.ID,
		userID:     session.UserID,
		streamKey:  streamKey,
		flvFile:    f,
		flvEncoder: enc,
		startedAt:  time.Now(),
		lastSync:   time.Now(),
		cancel:     cancel,
	}

	s.mu.Lock()
	s.streams[streamKey] = stream
	s.mu.Unlock()

	log.Printf("stream registered: key=%s session=%s path=%s", streamKey, session.ID, flvPath)
	return nil
}

// writeData writes a FLV tag to the session's recording file.
func (s *Server) writeData(streamKey string, tag *flvtag.FlvTag) error {
	s.mu.Lock()
	stream, ok := s.streams[streamKey]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("stream not found: %s", streamKey)
	}

	if stream.flvEncoder == nil {
		return fmt.Errorf("encoder not initialized for stream: %s", streamKey)
	}

	if err := stream.flvEncoder.Encode(tag); err != nil {
		return fmt.Errorf("encode FLV tag: %w", err)
	}

	// Flush to disk every 2 seconds so data survives a server crash.
	// FLV is append-friendly — a truncated file is still partially playable.
	if time.Since(stream.lastSync) >= 2*time.Second {
		stream.flvFile.Sync()
		stream.lastSync = time.Now()
	}

	return nil
}

// onDisconnect handles stream disconnection (intentional or unexpected).
func (s *Server) onDisconnect(streamKey string, reason string) {
	s.mu.Lock()
	stream, ok := s.streams[streamKey]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.streams, streamKey)
	s.mu.Unlock()

	s.finalizeStream(streamKey, stream, reason)
}

// finalizeStream closes the FLV file, updates DB, and starts post-processing.
func (s *Server) finalizeStream(streamKey string, stream *activeStream, reason string) {
	duration := int(time.Since(stream.startedAt).Seconds())
	log.Printf("finalizing stream: key=%s reason=%s duration=%ds", streamKey, reason, duration)

	// Close FLV file
	if stream.flvFile != nil {
		stream.flvFile.Sync()
		stream.flvFile.Close()
	}

	stream.cancel()

	// Update session in database
	if err := s.queries.EndSession(
		context.Background(),
		stream.sessionID,
		reason,
		1, // single file, 1 "segment"
		duration,
	); err != nil {
		log.Printf("error ending session: %v", err)
	}

	// Record the FLV file as a segment for tracking
	flvPath := filepath.Join(s.cfg.SegmentDir, stream.sessionID.String(), "recording.flv")
	if info, err := os.Stat(flvPath); err == nil {
		s.queries.CreateSegment(
			context.Background(),
			stream.sessionID,
			0,
			flvPath,
			info.Size(),
		)
	}

	// Post-process asynchronously
	go s.postProcess(stream.sessionID, stream.userID, stream.streamKey)
}

// postProcess converts FLV → MP4 via FFmpeg, then uploads.
func (s *Server) postProcess(sessionID, userID uuid.UUID, streamKey string) {
	log.Printf("post-processing session %s", sessionID)

	sessionDir := filepath.Join(s.cfg.SegmentDir, sessionID.String())
	flvPath := filepath.Join(sessionDir, "recording.flv")
	mp4Path := filepath.Join(sessionDir, "recording.mp4")

	// Check FLV file exists
	if _, err := os.Stat(flvPath); os.IsNotExist(err) {
		log.Printf("no FLV file found for session %s", sessionID)
		s.queries.UpdateSessionStatus(context.Background(), sessionID, "failed")
		return
	}

	// Convert FLV → MP4 using FFmpeg
	// -movflags +faststart: moves the moov atom to the beginning for streaming
	// -c copy: no re-encoding, just remux (fast)
	cmd := exec.CommandContext(
		context.Background(),
		"ffmpeg",
		"-i", flvPath,
		"-c", "copy",
		"-movflags", "+faststart",
		"-y", // overwrite if exists
		mp4Path,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Printf("FFmpeg conversion failed for session %s: %v", sessionID, err)
		// Still mark as "finalized" — the FLV is still usable
		s.queries.UpdateSessionStatus(context.Background(), sessionID, "finalized_flv")
		return
	}

	log.Printf("MP4 ready: %s", mp4Path)

	// Upload to Google Drive (if user has connected)
	s.uploadToGoogleDrive(sessionID, userID, mp4Path)

	// Post to Instagram Story (if user has connected Instagram)
	s.postToInstagram(sessionID, userID, streamKey)

	s.queries.UpdateSessionStatus(context.Background(), sessionID, "uploaded")
	log.Printf("post-processing complete for session %s", sessionID)
}

// uploadToGoogleDrive uploads the MP4 to the user's Google Drive if connected.
func (s *Server) uploadToGoogleDrive(sessionID, userID uuid.UUID, mp4Path string) {
	ctx := context.Background()

	// Check if Google Drive is configured at the server level
	if s.cfg.GoogleClientID == "" || s.cfg.GoogleClientSecret == "" {
		log.Printf("gdrive: skipping (not configured)")
		return
	}

	// Check if user has connected Google Drive
	encryptedToken, err := s.queries.GetUserGoogleToken(ctx, userID)
	if err != nil {
		log.Printf("gdrive: error checking user token: %v", err)
		return
	}
	if encryptedToken == nil || *encryptedToken == "" {
		log.Printf("gdrive: skipping for session %s (user has no Google Drive connected)", sessionID)
		return
	}

	// Decrypt the token JSON
	tokenJSON, err := crypto.Decrypt(*encryptedToken, s.cfg.EncryptionKey)
	if err != nil {
		log.Printf("gdrive: failed to decrypt token for user %s: %v", userID, err)
		return
	}

	// Unmarshal the OAuth token
	token, err := gdrive.UnmarshalToken(tokenJSON)
	if err != nil {
		log.Printf("gdrive: failed to unmarshal token for user %s: %v", userID, err)
		return
	}

	// Upload
	oauthCfg := gdrive.OAuthConfig(s.cfg.GoogleClientID, s.cfg.GoogleClientSecret, s.cfg.GoogleRedirectURI)
	uploader := gdrive.NewUploader(oauthCfg)

	fileID, err := uploader.Upload(ctx, token, mp4Path)
	if err != nil {
		log.Printf("gdrive: upload failed for session %s: %v", sessionID, err)
		return
	}

	// Record the Drive file ID
	if err := s.queries.SetSessionDriveFileID(ctx, sessionID, fileID); err != nil {
		log.Printf("gdrive: failed to save file ID: %v", err)
	}

	log.Printf("gdrive: uploaded session %s to Google Drive (file ID: %s)", sessionID, fileID)
}

// postToInstagram checks if the user has connected Instagram, and if so
// publishes the session's MP4 as an Instagram Story.
func (s *Server) postToInstagram(sessionID, userID uuid.UUID, streamKey string) {
	ctx := context.Background()

	// Check if Instagram is configured at the server level
	if s.cfg.InstagramAppID == "" || s.cfg.InstagramAppSecret == "" {
		log.Printf("instagram: skipping (app not configured)")
		return
	}

	// Check if user has connected their Instagram account
	encryptedToken, accountID, err := s.queries.GetUserInstagramToken(ctx, userID)
	if err != nil {
		log.Printf("instagram: error checking user token: %v", err)
		return
	}
	if encryptedToken == nil || *encryptedToken == "" || accountID == nil {
		log.Printf("instagram: skipping for session %s (user has no Instagram connected)", sessionID)
		return
	}

	// Decrypt the access token
	encKey := crypto.DeriveKey(s.cfg.JWTSecret)
	accessToken, err := crypto.Decrypt(encKey, *encryptedToken)
	if err != nil {
		log.Printf("instagram: failed to decrypt token for user %s: %v", userID, err)
		return
	}

	// Build the public video URL that Instagram can fetch.
	// Uses the stream key as a secret URL token (no auth header needed).
	videoURL := fmt.Sprintf("http://%s:%d/api/video/%s",
		s.cfg.PublicHost, s.cfg.HTTPPort, streamKey)

	log.Printf("instagram: posting story for session %s, video URL: %s", sessionID, videoURL)

	storyID, err := s.ig.PostStory(ctx, accessToken, *accountID, videoURL)
	if err != nil {
		log.Printf("instagram: failed to post story for session %s: %v", sessionID, err)
		return
	}

	// Record the story ID in the database
	if err := s.queries.SetSessionInstagramStoryID(ctx, sessionID, storyID); err != nil {
		log.Printf("instagram: failed to save story ID: %v", err)
	}

	log.Printf("instagram: story posted successfully for session %s (story ID: %s)", sessionID, storyID)
}
