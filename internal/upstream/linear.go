package upstream

import (
	"fmt"

	"golang.org/x/oauth2"
)

func init() { Register("linear", &LinearAdapter{}) }

// LinearAdapter supports API key (Bearer) and OAuth2.
type LinearAdapter struct{}

func (a *LinearAdapter) AuthHeader(decryptedCreds []byte) (string, error) {
	token := string(decryptedCreds)
	if token == "" {
		return "", fmt.Errorf("linear: empty credentials")
	}
	return "Bearer " + token, nil
}

func (a *LinearAdapter) OAuth2Config(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"read", "write"},
		RedirectURL:  redirectURL,
		Endpoint: oauth2.Endpoint{ //nolint:gosec // well-known OAuth2 endpoint URLs
			AuthURL:  "https://linear.app/oauth/authorize",
			TokenURL: "https://api.linear.app/oauth/token",
		},
	}
}

func (a *LinearAdapter) AuthType() string { return "oauth2" }
