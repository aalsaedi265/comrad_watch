-- Comrad Watch: Initial Schema
-- Run with: psql -d comradwatch -f migrations/001_initial.sql

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    google_token_encrypted TEXT,
    instagram_token_encrypted TEXT,
    instagram_account_id VARCHAR(100),
    default_camera VARCHAR(10) DEFAULT 'front',
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW()
);

-- Stream sessions
CREATE TABLE IF NOT EXISTS sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    stream_key VARCHAR(64) UNIQUE NOT NULL,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    end_reason VARCHAR(50),
    status VARCHAR(20) DEFAULT 'active',
    total_segments INT DEFAULT 0,
    total_duration_seconds INT,
    google_drive_file_id VARCHAR(255),
    instagram_story_id VARCHAR(255),
    metadata JSONB,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Video segments (buffered on server)
CREATE TABLE IF NOT EXISTS segments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id UUID NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
    segment_number INT NOT NULL,
    file_path TEXT NOT NULL,
    duration_ms INT,
    size_bytes BIGINT,
    received_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(session_id, segment_number)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_stream_key ON sessions(stream_key) WHERE status = 'active';
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);
CREATE INDEX IF NOT EXISTS idx_segments_session_id ON segments(session_id);
