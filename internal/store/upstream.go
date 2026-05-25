package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type UpstreamConfig struct {
	ID                string
	UserID            string
	ServerType        string
	ServerURL         string
	AuthType          string
	EncryptedCreds    []byte
	DetectedTransport *string
	Status            string
	StatusCheckedAt   *time.Time
	Enabled           bool
}

type UpstreamStore struct {
	pool *pgxpool.Pool
}

func NewUpstreamStore(pool *pgxpool.Pool) *UpstreamStore {
	return &UpstreamStore{pool: pool}
}

func (s *UpstreamStore) CreateUpstreamConfig(ctx context.Context, userID, serverType, serverURL, authType string, encryptedCreds []byte) (*UpstreamConfig, error) {
	var c UpstreamConfig
	err := s.pool.QueryRow(ctx, `
		INSERT INTO upstream_configs (user_id, server_type, server_url, auth_type, encrypted_creds)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, server_type, server_url, auth_type, encrypted_creds,
		          detected_transport, status, status_checked_at`,
		userID, serverType, serverURL, authType, encryptedCreds,
	).Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
		&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt)
	if err != nil {
		return nil, fmt.Errorf("create upstream config: %w", err)
	}
	return &c, nil
}

func (s *UpstreamStore) GetUpstreamConfigsByUserID(ctx context.Context, userID string) ([]*UpstreamConfig, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, user_id, server_type, server_url, auth_type, encrypted_creds,
		       detected_transport, status, status_checked_at
		FROM upstream_configs WHERE user_id = $1`, userID)
	if err != nil {
		return nil, fmt.Errorf("list upstream configs: %w", err)
	}
	defer rows.Close()

	var configs []*UpstreamConfig
	for rows.Next() {
		var c UpstreamConfig
		if err := rows.Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
			&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt); err != nil {
			return nil, fmt.Errorf("scan upstream config: %w", err)
		}
		configs = append(configs, &c)
	}
	return configs, rows.Err()
}

func (s *UpstreamStore) GetUpstreamConfigByID(ctx context.Context, id string) (*UpstreamConfig, error) {
	var c UpstreamConfig
	err := s.pool.QueryRow(ctx, `
		SELECT id, user_id, server_type, server_url, auth_type, encrypted_creds,
		       detected_transport, status, status_checked_at
		FROM upstream_configs WHERE id = $1`, id,
	).Scan(&c.ID, &c.UserID, &c.ServerType, &c.ServerURL, &c.AuthType,
		&c.EncryptedCreds, &c.DetectedTransport, &c.Status, &c.StatusCheckedAt)
	if err != nil {
		return nil, fmt.Errorf("get upstream config: %w", err)
	}
	return &c, nil
}

func (s *UpstreamStore) UpdateUpstreamStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE upstream_configs
		SET status = $1, status_checked_at = now(), updated_at = now()
		WHERE id = $2`, status, id)
	return err
}

func (s *UpstreamStore) UpdateDetectedTransport(ctx context.Context, id, transport string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE upstream_configs SET detected_transport = $1, updated_at = now()
		WHERE id = $2`, transport, id)
	return err
}

func (s *UpstreamStore) UpdateEncryptedCreds(ctx context.Context, id string, encryptedCreds []byte) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE upstream_configs
		SET encrypted_creds = $1, status = 'unreachable', updated_at = now()
		WHERE id = $2`, encryptedCreds, id)
	return err
}

func (s *UpstreamStore) DeleteUpstreamConfig(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM upstream_configs WHERE id = $1`, id)
	return err
}
