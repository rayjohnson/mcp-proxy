package upstream

import (
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"
)

func init() { Register("notion", &NotionAdapter{}) }

// NotionAdapter uses OAuth2 (Bearer token stored as JSON {"access_token":"..."}).
type NotionAdapter struct{}

func (a *NotionAdapter) AuthHeader(decryptedCreds []byte) (string, error) {
	var creds struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(decryptedCreds, &creds); err != nil {
		// Treat as raw bearer token.
		token := string(decryptedCreds)
		if token == "" {
			return "", fmt.Errorf("notion: empty credentials")
		}
		return "Bearer " + token, nil
	}
	if creds.AccessToken == "" {
		return "", fmt.Errorf("notion: missing access_token")
	}
	return "Bearer " + creds.AccessToken, nil
}

func (a *NotionAdapter) OAuth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		Scopes:      []string{},
		RedirectURL: redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://api.notion.com/v1/oauth/authorize",
			TokenURL: "https://api.notion.com/v1/oauth/token",
		},
	}
}

func (a *NotionAdapter) AuthType() string { return "oauth2" }
