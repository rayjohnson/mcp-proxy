package upstream

import "golang.org/x/oauth2"

// Adapter provides per-service-type configuration for connecting to an upstream
// MCP server. Each supported service implements this interface.
type Adapter interface {
	// AuthHeader returns the Authorization header value for API-key-based auth.
	AuthHeader(decryptedCreds []byte) (string, error)
	// OAuth2Config returns the oauth2.Config for browser-based authorization flows.
	// Returns nil for API-key-only services.
	OAuth2Config(redirectURL string) *oauth2.Config
	// AuthType returns "api_key" or "oauth2".
	AuthType() string
}
