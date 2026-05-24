package upstream

import (
	"fmt"

	"golang.org/x/oauth2"
)

func init() { Register("cloudflare", &CloudflareAdapter{}) }

// CloudflareAdapter uses API key authentication only.
type CloudflareAdapter struct{}

func (a *CloudflareAdapter) AuthHeader(decryptedCreds []byte) (string, error) {
	token := string(decryptedCreds)
	if token == "" {
		return "", fmt.Errorf("cloudflare: empty credentials")
	}
	return "Bearer " + token, nil
}

// OAuth2Config returns nil because Cloudflare MCP uses API keys, not OAuth2.
func (a *CloudflareAdapter) OAuth2Config(_, _, _ string) *oauth2.Config { return nil }

func (a *CloudflareAdapter) AuthType() string { return "api_key" }
