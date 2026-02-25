package db

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Queries struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Queries {
	return &Queries{pool: pool}
}

// --- User operations ---

type User struct {
	ID                    uuid.UUID
	Email                 string
	PasswordHash          string
	GoogleTokenEncrypted  *string
	InstagramTokenEncrypted *string
	InstagramAccountID    *string
	DefaultCamera         string
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

func (q *Queries) CreateUser(ctx context.Context, email, passwordHash string) (*User, error) {
	user := &User{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash)
		 VALUES ($1, $2)
		 RETURNING id, email, password_hash, default_camera, created_at, updated_at`,
		email, passwordHash,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.DefaultCamera, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (q *Queries) GetUserByEmail(ctx context.Context, email string) (*User, error) {
	user := &User{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, google_token_encrypted, instagram_token_encrypted,
		        instagram_account_id, default_camera, created_at, updated_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&user.ID, &user.Email, &user.PasswordHash, &user.GoogleTokenEncrypted,
		&user.InstagramTokenEncrypted, &user.InstagramAccountID, &user.DefaultCamera,
		&user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return user, nil
}

// --- Session operations ---

type Session struct {
	ID                   uuid.UUID
	UserID               uuid.UUID
	StreamKey            string
	StartedAt            time.Time
	EndedAt              *time.Time
	EndReason            *string
	Status               string
	TotalSegments        int
	TotalDurationSeconds *int
	GoogleDriveFileID    *string
	InstagramStoryID     *string
	CreatedAt            time.Time
}

func (q *Queries) CreateSession(ctx context.Context, userID uuid.UUID, streamKey string) (*Session, error) {
	session := &Session{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO sessions (user_id, stream_key, started_at)
		 VALUES ($1, $2, NOW())
		 RETURNING id, user_id, stream_key, started_at, status, total_segments, created_at`,
		userID, streamKey,
	).Scan(&session.ID, &session.UserID, &session.StreamKey, &session.StartedAt,
		&session.Status, &session.TotalSegments, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (q *Queries) GetSessionByStreamKey(ctx context.Context, streamKey string) (*Session, error) {
	session := &Session{}
	err := q.pool.QueryRow(ctx,
		`SELECT id, user_id, stream_key, started_at, ended_at, end_reason, status,
		        total_segments, total_duration_seconds, google_drive_file_id,
		        instagram_story_id, created_at
		 FROM sessions WHERE stream_key = $1 AND status = 'active'`,
		streamKey,
	).Scan(&session.ID, &session.UserID, &session.StreamKey, &session.StartedAt,
		&session.EndedAt, &session.EndReason, &session.Status, &session.TotalSegments,
		&session.TotalDurationSeconds, &session.GoogleDriveFileID,
		&session.InstagramStoryID, &session.CreatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return session, nil
}

func (q *Queries) EndSession(ctx context.Context, sessionID uuid.UUID, endReason string, totalSegments int, durationSeconds int) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sessions
		 SET ended_at = NOW(), end_reason = $2, status = 'finalizing',
		     total_segments = $3, total_duration_seconds = $4
		 WHERE id = $1`,
		sessionID, endReason, totalSegments, durationSeconds,
	)
	return err
}

func (q *Queries) UpdateSessionStatus(ctx context.Context, sessionID uuid.UUID, status string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sessions SET status = $2 WHERE id = $1`,
		sessionID, status,
	)
	return err
}

func (q *Queries) SetSessionDriveFileID(ctx context.Context, sessionID uuid.UUID, fileID string) error {
	_, err := q.pool.Exec(ctx,
		`UPDATE sessions SET google_drive_file_id = $2 WHERE id = $1`,
		sessionID, fileID,
	)
	return err
}

// --- Segment operations ---

type Segment struct {
	ID            uuid.UUID
	SessionID     uuid.UUID
	SegmentNumber int
	FilePath      string
	DurationMS    *int
	SizeBytes     *int64
	ReceivedAt    time.Time
}

func (q *Queries) CreateSegment(ctx context.Context, sessionID uuid.UUID, segmentNumber int, filePath string, sizeBytes int64) (*Segment, error) {
	seg := &Segment{}
	err := q.pool.QueryRow(ctx,
		`INSERT INTO segments (session_id, segment_number, file_path, size_bytes)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, session_id, segment_number, file_path, size_bytes, received_at`,
		sessionID, segmentNumber, filePath, sizeBytes,
	).Scan(&seg.ID, &seg.SessionID, &seg.SegmentNumber, &seg.FilePath, &seg.SizeBytes, &seg.ReceivedAt)
	if err != nil {
		return nil, err
	}
	return seg, nil
}

func (q *Queries) GetSegmentsBySession(ctx context.Context, sessionID uuid.UUID) ([]*Segment, error) {
	rows, err := q.pool.Query(ctx,
		`SELECT id, session_id, segment_number, file_path, duration_ms, size_bytes, received_at
		 FROM segments WHERE session_id = $1
		 ORDER BY segment_number ASC`,
		sessionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var segments []*Segment
	for rows.Next() {
		seg := &Segment{}
		if err := rows.Scan(&seg.ID, &seg.SessionID, &seg.SegmentNumber, &seg.FilePath,
			&seg.DurationMS, &seg.SizeBytes, &seg.ReceivedAt); err != nil {
			return nil, err
		}
		segments = append(segments, seg)
	}
	return segments, rows.Err()
}
