package upstream

import (
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

func init() { Register("googlecloud", &GoogleCloudAdapter{}) }

// GoogleCloudAdapter uses Google OAuth2.
// Credentials are stored as JSON {"access_token":"...","refresh_token":"..."}.
type GoogleCloudAdapter struct{}

func (a *GoogleCloudAdapter) AuthHeader(decryptedCreds []byte) (string, error) {
	var creds struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(decryptedCreds, &creds); err != nil {
		return "", fmt.Errorf("googlecloud: invalid credentials JSON: %w", err)
	}
	if creds.AccessToken == "" {
		return "", fmt.Errorf("googlecloud: missing access_token")
	}
	return "Bearer " + creds.AccessToken, nil
}

func (a *GoogleCloudAdapter) OAuth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		Scopes:      []string{"https://www.googleapis.com/auth/cloud-platform"},
		Endpoint:    google.Endpoint,
		RedirectURL: redirectURL,
	}
}

func (a *GoogleCloudAdapter) AuthType() string { return "oauth2" }
