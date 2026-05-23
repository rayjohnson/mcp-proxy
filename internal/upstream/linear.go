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

func (a *LinearAdapter) OAuth2Config(redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		Scopes:      []string{"read", "write"},
		RedirectURL: redirectURL,
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://linear.app/oauth/authorize",
			TokenURL: "https://api.linear.app/oauth/token",
		},
	}
}

func (a *LinearAdapter) AuthType() string { return "oauth2" }
