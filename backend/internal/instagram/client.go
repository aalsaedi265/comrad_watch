package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

const graphAPIBase = "https://graph.instagram.com"
const graphFBBase = "https://graph.facebook.com/v21.0"

// Client communicates with the Instagram Graph API to publish Stories.
type Client struct {
	appID     string
	appSecret string
	httpClient *http.Client
}

// NewClient creates an Instagram API client.
func NewClient(appID, appSecret string) *Client {
	return &Client{
		appID:     appID,
		appSecret: appSecret,
		httpClient: &http.Client{Timeout: 120 * time.Second},
	}
}

// --- OAuth Token Exchange ---

// TokenResponse is returned by the Facebook token exchange endpoint.
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

// ExchangeCode exchanges a short-lived authorization code for a short-lived access token.
func (c *Client) ExchangeCode(ctx context.Context, code, redirectURI string) (*TokenResponse, error) {
	params := url.Values{
		"client_id":     {c.appID},
		"client_secret": {c.appSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {redirectURI},
		"code":          {code},
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.instagram.com/oauth/access_token",
		strings.NewReader(params.Encode()))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokenResp, nil
}

// ExchangeLongLived exchanges a short-lived token for a long-lived token (~60 days).
func (c *Client) ExchangeLongLived(ctx context.Context, shortToken string) (*TokenResponse, error) {
	u := fmt.Sprintf("%s/access_token?grant_type=ig_exchange_token&client_secret=%s&access_token=%s",
		graphAPIBase, url.QueryEscape(c.appSecret), url.QueryEscape(shortToken))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("long-lived token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("long-lived token exchange failed (status %d): %s", resp.StatusCode, string(body))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokenResp, nil
}

// --- Instagram User Info ---

// UserInfo holds basic Instagram account details.
type UserInfo struct {
	ID       string `json:"id"`
	Username string `json:"username"`
}

// GetUserInfo fetches the authenticated user's Instagram ID and username.
func (c *Client) GetUserInfo(ctx context.Context, accessToken string) (*UserInfo, error) {
	u := fmt.Sprintf("%s/me?fields=id,username&access_token=%s",
		graphAPIBase, url.QueryEscape(accessToken))

	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("user info request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("user info failed (status %d): %s", resp.StatusCode, string(body))
	}

	var info UserInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("decode user info: %w", err)
	}
	return &info, nil
}

// --- Story Publishing ---

// containerResponse is the response from the media container creation endpoint.
type containerResponse struct {
	ID string `json:"id"`
}

// statusResponse is the response from the container status check endpoint.
type statusResponse struct {
	StatusCode string `json:"status_code"` // EXPIRED, ERROR, FINISHED, IN_PROGRESS, PUBLISHED
}

// publishResponse is the response from the media_publish endpoint.
type publishResponse struct {
	ID string `json:"id"`
}

// PostStory uploads a video as an Instagram Story.
// videoURL must be a publicly accessible URL pointing to the MP4 file.
// Returns the published story media ID.
func (c *Client) PostStory(ctx context.Context, accessToken, igUserID, videoURL string) (string, error) {
	// Step 1: Create a media container for the story
	containerID, err := c.createStoryContainer(ctx, accessToken, igUserID, videoURL)
	if err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	log.Printf("instagram: created story container %s, waiting for processing", containerID)

	// Step 2: Poll until the container is ready (Instagram processes the video)
	if err := c.waitForProcessing(ctx, accessToken, containerID); err != nil {
		return "", fmt.Errorf("wait for processing: %w", err)
	}

	// Step 3: Publish the container
	storyID, err := c.publishContainer(ctx, accessToken, igUserID, containerID)
	if err != nil {
		return "", fmt.Errorf("publish container: %w", err)
	}

	log.Printf("instagram: story published with ID %s", storyID)
	return storyID, nil
}

// createStoryContainer creates an unpublished story container with a video.
func (c *Client) createStoryContainer(ctx context.Context, accessToken, igUserID, videoURL string) (string, error) {
	params := url.Values{
		"media_type":   {"STORIES"},
		"video_url":    {videoURL},
		"access_token": {accessToken},
	}

	u := fmt.Sprintf("%s/%s/media", graphFBBase, igUserID)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("create container request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("create container failed (status %d): %s", resp.StatusCode, string(body))
	}

	var cr containerResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		return "", fmt.Errorf("decode container response: %w", err)
	}
	return cr.ID, nil
}

// waitForProcessing polls the container status until it's FINISHED or errors out.
func (c *Client) waitForProcessing(ctx context.Context, accessToken, containerID string) error {
	u := fmt.Sprintf("%s/%s?fields=status_code&access_token=%s",
		graphFBBase, containerID, url.QueryEscape(accessToken))

	for i := 0; i < 30; i++ { // max ~5 minutes of polling
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
		if err != nil {
			return fmt.Errorf("create request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("status check request: %w", err)
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var sr statusResponse
		if err := json.Unmarshal(body, &sr); err != nil {
			return fmt.Errorf("decode status response: %w", err)
		}

		switch sr.StatusCode {
		case "FINISHED":
			return nil
		case "ERROR", "EXPIRED":
			return fmt.Errorf("container processing failed: %s", sr.StatusCode)
		case "IN_PROGRESS":
			log.Printf("instagram: container %s still processing...", containerID)
			time.Sleep(10 * time.Second)
		default:
			time.Sleep(10 * time.Second)
		}
	}

	return fmt.Errorf("container processing timed out after 5 minutes")
}

// publishContainer publishes a fully-processed media container as a story.
func (c *Client) publishContainer(ctx context.Context, accessToken, igUserID, containerID string) (string, error) {
	params := url.Values{
		"creation_id":  {containerID},
		"access_token": {accessToken},
	}

	u := fmt.Sprintf("%s/%s/media_publish", graphFBBase, igUserID)
	req, err := http.NewRequestWithContext(ctx, "POST", u, strings.NewReader(params.Encode()))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("publish request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("publish failed (status %d): %s", resp.StatusCode, string(body))
	}

	var pr publishResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return "", fmt.Errorf("decode publish response: %w", err)
	}
	return pr.ID, nil
}
