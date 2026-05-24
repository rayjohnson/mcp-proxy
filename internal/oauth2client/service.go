package oauth2client

import (
	"context"
	"encoding/json"
	"fmt"

	"golang.org/x/oauth2"

	"github.com/rayjohnson/mcp-proxy/internal/auth"
	"github.com/rayjohnson/mcp-proxy/internal/kms"
	"github.com/rayjohnson/mcp-proxy/internal/store"
	"github.com/rayjohnson/mcp-proxy/internal/upstream"
)

// Service handles OAuth2 authorization flows for upstream MCP servers.
type Service struct {
	stateStore    *store.OAuth2StateStore
	upstreamStore *store.UpstreamStore
	catalogStore  *store.CatalogStore
	kmsClient     *kms.Client
	baseURL       string
}

func NewService(
	stateStore *store.OAuth2StateStore,
	upstreamStore *store.UpstreamStore,
	catalogStore *store.CatalogStore,
	kmsClient *kms.Client,
	baseURL string,
) *Service {
	return &Service{
		stateStore:    stateStore,
		upstreamStore: upstreamStore,
		catalogStore:  catalogStore,
		kmsClient:     kmsClient,
		baseURL:       baseURL,
	}
}

// oauthConfig loads OAuth2 configuration from the admin-configured catalog entry.
func (s *Service) oauthConfig(ctx context.Context, serverType string) (*oauth2.Config, *store.CatalogEntry, error) {
	entry, err := s.catalogStore.GetCatalogEntryByServerType(ctx, serverType)
	if err != nil {
		return nil, nil, fmt.Errorf("catalog entry not found for %q: %w", serverType, err)
	}
	if entry.AuthType != "oauth2" || entry.OAuthClientID == nil || len(entry.EncryptedOAuthSecret) == 0 {
		return nil, nil, fmt.Errorf("%s is not configured for OAuth2 in the catalog", serverType)
	}

	plainSecret, err := s.kmsClient.Decrypt(ctx, entry.EncryptedOAuthSecret)
	if err != nil {
		return nil, nil, fmt.Errorf("decrypt oauth secret: %w", err)
	}

	adapter, err := upstream.GetAdapter(serverType)
	if err != nil {
		return nil, nil, err
	}
	cfg := adapter.OAuth2Config(*entry.OAuthClientID, string(plainSecret), s.redirectURL(serverType))
	if cfg == nil {
		return nil, nil, fmt.Errorf("%s does not support OAuth2", serverType)
	}
	return cfg, entry, nil
}

// StartAuthFlow generates a state token, stores it, and returns the upstream
// OAuth2 authorization URL for the user to visit in their browser.
func (s *Service) StartAuthFlow(ctx context.Context, userID, serverType string) (string, error) {
	cfg, _, err := s.oauthConfig(ctx, serverType)
	if err != nil {
		return "", fmt.Errorf("start auth flow: %w", err)
	}

	state, err := auth.GenerateSecureToken()
	if err != nil {
		return "", fmt.Errorf("generate state: %w", err)
	}
	if err := s.stateStore.CreateOAuth2State(ctx, userID, serverType, state); err != nil {
		return "", fmt.Errorf("store oauth2 state: %w", err)
	}
	return cfg.AuthCodeURL(state), nil
}

// HandleCallback validates the OAuth2 state, exchanges the code for tokens,
// encrypts them with KMS, and stores them in upstream_configs.
func (s *Service) HandleCallback(ctx context.Context, serverType, code, state string) error {
	st, err := s.stateStore.ConsumeOAuth2State(ctx, state)
	if err != nil {
		return fmt.Errorf("invalid or expired state: %w", err)
	}
	if st.ServerType != serverType {
		return fmt.Errorf("state server_type mismatch")
	}

	cfg, entry, err := s.oauthConfig(ctx, serverType)
	if err != nil {
		return fmt.Errorf("handle callback: %w", err)
	}

	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}

	credsJSON, err := json.Marshal(token) //nolint:gosec // intentionally marshaling token for KMS encryption
	if err != nil {
		return fmt.Errorf("marshal token: %w", err)
	}
	encrypted, err := s.kmsClient.Encrypt(ctx, credsJSON)
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	// Update existing config or create a new one.
	existing, _ := s.upstreamStore.GetUpstreamConfigsByUserID(ctx, st.UserID)
	for _, c := range existing {
		if c.ServerType == serverType {
			if err := s.upstreamStore.UpdateEncryptedCreds(ctx, c.ID, encrypted); err != nil {
				return fmt.Errorf("update creds: %w", err)
			}
			return s.upstreamStore.UpdateUpstreamStatus(ctx, c.ID, "unreachable")
		}
	}

	_, err = s.upstreamStore.CreateUpstreamConfig(ctx,
		st.UserID, serverType, entry.ServerURL, "oauth2", encrypted)
	return err
}

func (s *Service) redirectURL(serverType string) string {
	return s.baseURL + "/api/oauth2/callback/" + serverType
}
