package gdrive

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

// Uploader handles uploading files to a user's Google Drive.
type Uploader struct {
	oauthCfg *oauth2.Config
}

// NewUploader creates a new Google Drive uploader.
func NewUploader(oauthCfg *oauth2.Config) *Uploader {
	return &Uploader{oauthCfg: oauthCfg}
}

// Upload uploads an MP4 file to the user's Google Drive.
// Files are placed under ComradWatch/YYYY-MM-DD/ with a timestamped name.
// Returns the Google Drive file ID.
func (u *Uploader) Upload(ctx context.Context, token *oauth2.Token, mp4Path string) (string, error) {
	client := u.oauthCfg.Client(ctx, token)

	srv, err := drive.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return "", fmt.Errorf("create drive service: %w", err)
	}

	// Find or create the folder hierarchy: ComradWatch / YYYY-MM-DD
	rootFolderID, err := findOrCreateFolder(ctx, srv, "root", "ComradWatch")
	if err != nil {
		return "", fmt.Errorf("create root folder: %w", err)
	}

	dateFolder := time.Now().Format("2006-01-02")
	dateFolderID, err := findOrCreateFolder(ctx, srv, rootFolderID, dateFolder)
	if err != nil {
		return "", fmt.Errorf("create date folder: %w", err)
	}

	// Open the file for upload
	f, err := os.Open(mp4Path)
	if err != nil {
		return "", fmt.Errorf("open file: %w", err)
	}
	defer f.Close()

	// Build filename: recording_HH-MM-SS.mp4
	timestamp := time.Now().Format("15-04-05")
	baseName := filepath.Base(mp4Path)
	ext := filepath.Ext(baseName)
	fileName := fmt.Sprintf("recording_%s%s", timestamp, ext)

	driveFile := &drive.File{
		Name:    fileName,
		Parents: []string{dateFolderID},
	}

	created, err := srv.Files.Create(driveFile).Media(f).Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("upload file: %w", err)
	}

	return created.Id, nil
}

// findOrCreateFolder finds a folder by name under parentID, or creates it.
func findOrCreateFolder(ctx context.Context, srv *drive.Service, parentID, name string) (string, error) {
	query := fmt.Sprintf(
		"name = '%s' and '%s' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false",
		name, parentID,
	)

	result, err := srv.Files.List().Q(query).Fields("files(id)").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("search folder: %w", err)
	}

	if len(result.Files) > 0 {
		return result.Files[0].Id, nil
	}

	// Create the folder
	folder := &drive.File{
		Name:     name,
		MimeType: "application/vnd.google-apps.folder",
		Parents:  []string{parentID},
	}

	created, err := srv.Files.Create(folder).Fields("id").Context(ctx).Do()
	if err != nil {
		return "", fmt.Errorf("create folder: %w", err)
	}

	return created.Id, nil
}
