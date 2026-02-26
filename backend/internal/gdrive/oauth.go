package gdrive

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/drive/v3"
)

// OAuthConfig builds an oauth2.Config for Google Drive access.
func OAuthConfig(clientID, clientSecret, redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{drive.DriveFileScope},
		Endpoint:     google.Endpoint,
	}
}

// AuthURL generates the Google OAuth consent URL.
// The state parameter should contain an encrypted user identifier.
func AuthURL(cfg *oauth2.Config, state string) string {
	return cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
}

// ExchangeCode exchanges an authorization code for OAuth tokens.
func ExchangeCode(ctx context.Context, cfg *oauth2.Config, code string) (*oauth2.Token, error) {
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("exchange code: %w", err)
	}
	return token, nil
}

// MarshalToken serializes an OAuth token to JSON.
func MarshalToken(token *oauth2.Token) (string, error) {
	data, err := json.Marshal(token)
	if err != nil {
		return "", fmt.Errorf("marshal token: %w", err)
	}
	return string(data), nil
}

// UnmarshalToken deserializes an OAuth token from JSON.
func UnmarshalToken(data string) (*oauth2.Token, error) {
	var token oauth2.Token
	if err := json.Unmarshal([]byte(data), &token); err != nil {
		return nil, fmt.Errorf("unmarshal token: %w", err)
	}
	return &token, nil
}
