package oauth2client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"golang.org/x/oauth2"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// tokenCreds is the JSON structure stored in encrypted_creds for OAuth2 configs.
type tokenCreds struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	Expiry       time.Time `json:"expiry,omitempty"`
}

// RefreshIfExpired checks whether the stored OAuth2 token is within 5 minutes
// of expiry, and if so, performs a token refresh. It re-encrypts and stores the
// updated token pair. On invalid_grant it sets status to reauth_required.
func (s *Service) RefreshIfExpired(ctx context.Context, cfg *store.UpstreamConfig) error {
	if cfg.AuthType != "oauth2" {
		return nil
	}

	plainCreds, err := s.kmsClient.Decrypt(ctx, cfg.EncryptedCreds)
	if err != nil {
		return fmt.Errorf("decrypt creds: %w", err)
	}

	var creds tokenCreds
	if err := json.Unmarshal(plainCreds, &creds); err != nil {
		return fmt.Errorf("unmarshal creds: %w", err)
	}

	// Only refresh if expiry is known and within 5 minutes.
	if creds.Expiry.IsZero() || time.Until(creds.Expiry) > 5*time.Minute {
		return nil
	}

	oauth2cfg, _, err := s.oauthConfig(ctx, cfg.ServerType)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}

	oldToken := &oauth2.Token{
		AccessToken:  creds.AccessToken,
		RefreshToken: creds.RefreshToken,
		Expiry:       creds.Expiry,
	}

	// TokenSource will call the refresh endpoint when the token is expired.
	ts := oauth2cfg.TokenSource(ctx, oldToken)
	newToken, err := ts.Token()
	if err != nil {
		if isInvalidGrant(err) {
			_ = s.upstreamStore.UpdateUpstreamStatus(ctx, cfg.ID, "reauth_required")
		}
		return fmt.Errorf("refresh token: %w", err)
	}

	updatedCreds := tokenCreds{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		Expiry:       newToken.Expiry,
	}

	credsJSON, err := json.Marshal(updatedCreds) //nolint:gosec // intentionally marshaling token for KMS encryption
	if err != nil {
		return fmt.Errorf("marshal refreshed creds: %w", err)
	}

	encrypted, err := s.kmsClient.Encrypt(ctx, credsJSON)
	if err != nil {
		return fmt.Errorf("encrypt refreshed creds: %w", err)
	}

	return s.upstreamStore.UpdateEncryptedCreds(ctx, cfg.ID, encrypted)
}

func isInvalidGrant(err error) bool {
	return err != nil && strings.Contains(err.Error(), "invalid_grant")
}
