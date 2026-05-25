package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type UpstreamStore struct {
	db *sql.DB
}

func NewUpstreamStore(db *sql.DB) *UpstreamStore {
	return &UpstreamStore{db: db}
}

func (s *UpstreamStore) CreateUpstreamConfig(ctx context.Context, userID, serverType, serverURL, authType string, encryptedCreds []byte) (*store.UpstreamConfig, error) {
	var c store.UpstreamConfig
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO upstream_configs (user_id, server_type, server_url, auth_type, encrypted_creds)
		VALUES (?, ?, ?, ?, ?)
		RETURNING id, user_id, server_type, server_url, auth_type, encrypted_creds,
		          detected_transport, status, status_checked_at, enabled`,
		userID, serverType, serverURL, authType, encryptedCreds,
	).Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
		&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt, &enabled)
	if err != nil {
		return nil, fmt.Errorf("create upstream config: %w", err)
	}
	c.Enabled = enabled != 0
	return &c, nil
}

func (s *UpstreamStore) GetUpstreamConfigsByUserID(ctx context.Context, userID string) ([]*store.UpstreamConfig, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_id, server_type, server_url, auth_type, encrypted_creds,
		       detected_transport, status, status_checked_at, enabled
		FROM upstream_configs WHERE user_id = ?`, userID)
	if err != nil {
		return nil, fmt.Errorf("list upstream configs: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var configs []*store.UpstreamConfig
	for rows.Next() {
		var c store.UpstreamConfig
		var enabled int
		if err := rows.Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
			&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt, &enabled); err != nil {
			return nil, fmt.Errorf("scan upstream config: %w", err)
		}
		c.Enabled = enabled != 0
		configs = append(configs, &c)
	}
	return configs, rows.Err()
}

func (s *UpstreamStore) GetUpstreamConfigByID(ctx context.Context, id string) (*store.UpstreamConfig, error) {
	var c store.UpstreamConfig
	var enabled int
	err := s.db.QueryRowContext(ctx, `
		SELECT id, user_id, server_type, server_url, auth_type, encrypted_creds,
		       detected_transport, status, status_checked_at, enabled
		FROM upstream_configs WHERE id = ?`, id,
	).Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
		&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt, &enabled)
	if err != nil {
		return nil, fmt.Errorf("get upstream config: %w", err)
	}
	c.Enabled = enabled != 0
	return &c, nil
}

func (s *UpstreamStore) UpdateUpstreamStatus(ctx context.Context, id, status string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE upstream_configs
		SET status = ?, status_checked_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, id)
	return err
}

func (s *UpstreamStore) UpdateDetectedTransport(ctx context.Context, id, transport string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE upstream_configs SET detected_transport = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, transport, id)
	return err
}

func (s *UpstreamStore) UpdateEncryptedCreds(ctx context.Context, id string, encryptedCreds []byte) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE upstream_configs
		SET encrypted_creds = ?, status = 'unreachable', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, encryptedCreds, id)
	return err
}

func (s *UpstreamStore) DeleteUpstreamConfig(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM upstream_configs WHERE id = ?`, id)
	return err
}

func (s *UpstreamStore) ToggleUpstream(ctx context.Context, id string) (bool, error) {
	var newEnabled int
	err := s.db.QueryRowContext(ctx, `
		UPDATE upstream_configs
		SET enabled = 1 - enabled, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		RETURNING enabled`, id,
	).Scan(&newEnabled)
	if err != nil {
		return false, fmt.Errorf("toggle upstream: %w", err)
	}
	return newEnabled != 0, nil
}
