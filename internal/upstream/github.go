package upstream

import (
	"fmt"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/github"
)

func init() { Register("github", &GitHubAdapter{}) }

// GitHubAdapter supports both API-key (Personal Access Token) and OAuth2 flows.
type GitHubAdapter struct{}

func (a *GitHubAdapter) AuthHeader(decryptedCreds []byte) (string, error) {
	token := string(decryptedCreds)
	if token == "" {
		return "", fmt.Errorf("github: empty credentials")
	}
	return "Bearer " + token, nil
}

func (a *GitHubAdapter) OAuth2Config(clientID, clientSecret, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       []string{"repo", "read:user"},
		Endpoint:     github.Endpoint,
		RedirectURL:  redirectURL,
	}
}

func (a *GitHubAdapter) AuthType() string { return "oauth2" }
