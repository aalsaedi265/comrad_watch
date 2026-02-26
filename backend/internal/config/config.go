package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// Server
	HTTPPort int
	RTMPPort int

	// Database
	DatabaseURL string

	// Storage
	SegmentDir string // local directory for buffering video segments

	// JWT
	JWTSecret string

	// Encryption key for storing OAuth tokens (AES-256)
	EncryptionKey string

	// Google Drive (Phase 3)
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURI  string

	// Instagram (populated later in Phase 4)
	InstagramAppID     string
	InstagramAppSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		HTTPPort:   getEnvInt("HTTP_PORT", 8080),
		RTMPPort:   getEnvInt("RTMP_PORT", 1935),
		SegmentDir: getEnv("SEGMENT_DIR", "./segments"),
		JWTSecret:  getEnv("JWT_SECRET", ""),

		DatabaseURL: getEnv("DATABASE_URL", ""),

		EncryptionKey: getEnv("ENCRYPTION_KEY", ""),

		GoogleClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		GoogleClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		GoogleRedirectURI:  getEnv("GOOGLE_REDIRECT_URI", "http://localhost:8080/api/google/callback"),

		InstagramAppID:     getEnv("INSTAGRAM_APP_ID", ""),
		InstagramAppSecret: getEnv("INSTAGRAM_APP_SECRET", ""),
	}

	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET is required")
	}
	if cfg.GoogleClientID != "" && cfg.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required when Google Drive is configured")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
