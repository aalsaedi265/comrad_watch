package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/comradwatch/backend/internal/api"
	"github.com/comradwatch/backend/internal/config"
	"github.com/comradwatch/backend/internal/db"
	"github.com/comradwatch/backend/internal/rtmp"
)

func main() {
	// Load .env file if present (dev convenience; not required in production)
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Connect to PostgreSQL
	pool, err := db.Connect(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Run database migrations
	if err := db.RunMigrations(pool, "migrations"); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	queries := db.New(pool)

	// Ensure segment storage directory exists
	if err := os.MkdirAll(cfg.SegmentDir, 0755); err != nil {
		log.Fatalf("failed to create segment directory: %v", err)
	}

	// Start RTMP ingest server
	rtmpServer := rtmp.NewServer(cfg, queries)
	go func() {
		log.Printf("RTMP server listening on :%d", cfg.RTMPPort)
		if err := rtmpServer.Start(); err != nil {
			log.Fatalf("RTMP server error: %v", err)
		}
	}()

	// Start HTTP API server (serves both REST API and PWA static files)
	router := api.NewRouter(cfg, queries, rtmpServer)
	httpServer := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      router,
		ReadTimeout:  60 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("HTTP API server listening on :%d", cfg.HTTPPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("shutting down servers...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rtmpServer.Stop()

	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("HTTP server forced shutdown: %v", err)
	}

	log.Println("server stopped")
}
