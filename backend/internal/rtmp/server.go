package rtmp

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
	"github.com/google/uuid"
	"github.com/yutopp/go-flv"
	flvtag "github.com/yutopp/go-flv/tag"
	"github.com/yutopp/go-rtmp"
	rtmpmsg "github.com/yutopp/go-rtmp/message"
)

// Server handles RTMP ingest from mobile clients.
// Each incoming stream is authenticated by stream key, then segmented
// into small FLV files on disk for resilience.
type Server struct {
	cfg      *config.Config
	queries  *db.Queries
	listener net.Listener
	srv      *rtmp.Server
	mu       sync.Mutex
	streams  map[string]*activeStream // streamKey -> activeStream
}

type activeStream struct {
	sessionID     uuid.UUID
	userID        uuid.UUID
	segmentNumber int
	currentFile   *os.File
	currentWriter *flv.Encoder
	segmentStart  time.Time
	totalBytes    int64
	cancel        context.CancelFunc
}

func NewServer(cfg *config.Config, queries *db.Queries) *Server {
	return &Server{
		cfg:     cfg,
		queries: queries,
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
				Logger:  log.New(os.Stdout, "[rtmp] ", log.LstdFlags),
			}
		},
	})
	s.srv = srv

	return srv.Serve(ln)
}

func (s *Server) Stop() {
	s.mu.Lock()
	// Finalize all active streams
	for key, stream := range s.streams {
		log.Printf("finalizing stream %s on shutdown", key)
		s.finalizeStream(key, stream, "server_shutdown")
	}
	s.mu.Unlock()

	if s.listener != nil {
		s.listener.Close()
	}
}

// registerStream authenticates a stream key and sets up buffering.
func (s *Server) registerStream(streamKey string) error {
	session, err := s.queries.GetSessionByStreamKey(context.Background(), streamKey)
	if err != nil {
		return fmt.Errorf("query session: %w", err)
	}
	if session == nil {
		return fmt.Errorf("invalid stream key: %s", streamKey)
	}

	ctx, cancel := context.WithCancel(context.Background())
	_ = ctx // used for future background tasks per stream

	stream := &activeStream{
		sessionID:     session.ID,
		userID:        session.UserID,
		segmentNumber: 0,
		cancel:        cancel,
	}

	// Create session segment directory
	sessionDir := filepath.Join(s.cfg.SegmentDir, session.ID.String())
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		cancel()
		return fmt.Errorf("create session dir: %w", err)
	}

	// Open first segment
	if err := s.rotateSegment(stream, session.ID); err != nil {
		cancel()
		return fmt.Errorf("open first segment: %w", err)
	}

	s.mu.Lock()
	s.streams[streamKey] = stream
	s.mu.Unlock()

	log.Printf("stream registered: key=%s session=%s", streamKey, session.ID)
	return nil
}

// writeData writes a FLV tag to the current segment, rotating if needed.
func (s *Server) writeData(streamKey string, tag *flvtag.FlvTag) error {
	s.mu.Lock()
	stream, ok := s.streams[streamKey]
	s.mu.Unlock()

	if !ok {
		return fmt.Errorf("stream not found: %s", streamKey)
	}

	// Rotate segment every ~5 seconds
	if time.Since(stream.segmentStart) >= 5*time.Second {
		if err := s.rotateSegment(stream, stream.sessionID); err != nil {
			return fmt.Errorf("rotate segment: %w", err)
		}
	}

	if stream.currentWriter != nil {
		if err := stream.currentWriter.Encode(tag); err != nil {
			return fmt.Errorf("encode tag: %w", err)
		}
	}

	return nil
}

// rotateSegment closes the current segment file and opens a new one.
func (s *Server) rotateSegment(stream *activeStream, sessionID uuid.UUID) error {
	// Close current segment
	if stream.currentFile != nil {
		stream.currentWriter = nil
		stream.currentFile.Close()

		// Record segment in database
		info, err := os.Stat(stream.currentFile.Name())
		if err == nil {
			_, dbErr := s.queries.CreateSegment(
				context.Background(),
				sessionID,
				stream.segmentNumber,
				stream.currentFile.Name(),
				info.Size(),
			)
			if dbErr != nil {
				log.Printf("warning: failed to record segment in db: %v", dbErr)
			}
		}

		stream.segmentNumber++
	}

	// Open new segment file
	sessionDir := filepath.Join(s.cfg.SegmentDir, sessionID.String())
	segPath := filepath.Join(sessionDir, fmt.Sprintf("seg_%04d.flv", stream.segmentNumber))

	f, err := os.Create(segPath)
	if err != nil {
		return fmt.Errorf("create segment file: %w", err)
	}

	enc, err := flv.NewEncoder(f, flv.FlagsAudio|flv.FlagsVideo)
	if err != nil {
		f.Close()
		return fmt.Errorf("create FLV encoder: %w", err)
	}

	stream.currentFile = f
	stream.currentWriter = enc
	stream.segmentStart = time.Now()

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

// finalizeStream closes the current segment, updates the DB, and triggers
// post-processing (Google Drive upload, Instagram Story post).
func (s *Server) finalizeStream(streamKey string, stream *activeStream, reason string) {
	log.Printf("finalizing stream: key=%s reason=%s segments=%d",
		streamKey, reason, stream.segmentNumber+1)

	// Close current segment file
	if stream.currentFile != nil {
		stream.currentWriter = nil
		stream.currentFile.Close()

		info, err := os.Stat(stream.currentFile.Name())
		if err == nil {
			s.queries.CreateSegment(
				context.Background(),
				stream.sessionID,
				stream.segmentNumber,
				stream.currentFile.Name(),
				info.Size(),
			)
		}
	}

	// Update session in database
	totalSegments := stream.segmentNumber + 1
	err := s.queries.EndSession(
		context.Background(),
		stream.sessionID,
		reason,
		totalSegments,
		0, // duration will be calculated during finalization
	)
	if err != nil {
		log.Printf("error ending session: %v", err)
	}

	stream.cancel()

	// Trigger async post-processing (video concatenation, upload, social post)
	go s.postProcess(stream.sessionID)
}

// postProcess concatenates segments and uploads to Google Drive / posts to Instagram.
// This runs asynchronously after stream ends.
func (s *Server) postProcess(sessionID uuid.UUID) {
	log.Printf("starting post-processing for session %s", sessionID)

	segments, err := s.queries.GetSegmentsBySession(context.Background(), sessionID)
	if err != nil {
		log.Printf("error getting segments for post-processing: %v", err)
		s.queries.UpdateSessionStatus(context.Background(), sessionID, "failed")
		return
	}

	if len(segments) == 0 {
		log.Printf("no segments found for session %s", sessionID)
		s.queries.UpdateSessionStatus(context.Background(), sessionID, "failed")
		return
	}

	// Concatenate segments into a single file
	outputDir := filepath.Join(s.cfg.SegmentDir, sessionID.String())
	outputPath := filepath.Join(outputDir, "final.flv")

	if err := concatenateSegments(segments, outputPath); err != nil {
		log.Printf("error concatenating segments: %v", err)
		s.queries.UpdateSessionStatus(context.Background(), sessionID, "failed")
		return
	}

	log.Printf("video finalized: %s (%d segments)", outputPath, len(segments))

	// TODO Phase 3: Upload to Google Drive
	// TODO Phase 4: Post to Instagram Story

	s.queries.UpdateSessionStatus(context.Background(), sessionID, "uploaded")
	log.Printf("post-processing complete for session %s", sessionID)
}

// concatenateSegments joins segment FLV files into a single output file.
func concatenateSegments(segments []*db.Segment, outputPath string) error {
	out, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("create output file: %w", err)
	}
	defer out.Close()

	for _, seg := range segments {
		f, err := os.Open(seg.FilePath)
		if err != nil {
			log.Printf("warning: could not open segment %s: %v", seg.FilePath, err)
			continue
		}
		if _, err := io.Copy(out, f); err != nil {
			f.Close()
			return fmt.Errorf("copy segment %d: %w", seg.SegmentNumber, err)
		}
		f.Close()
	}

	return nil
}
