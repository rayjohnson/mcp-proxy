package store

// Postgres migration required before using this store:
//
//   ALTER TABLE upstream_configs ADD COLUMN IF NOT EXISTS enabled BOOLEAN NOT NULL DEFAULT true;
//
//   CREATE TABLE IF NOT EXISTS server_toggles (
//     id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
//     user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
//     catalog_id UUID NOT NULL REFERENCES default_catalog(id) ON DELETE CASCADE,
//     enabled    BOOLEAN NOT NULL DEFAULT true,
//     created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
//     updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
//     UNIQUE(user_id, catalog_id)
//   );

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ToggleStore struct {
	pool *pgxpool.Pool
}

func NewToggleStore(pool *pgxpool.Pool) *ToggleStore {
	return &ToggleStore{pool: pool}
}

func (s *ToggleStore) ToggleCatalogEntry(ctx context.Context, userID, catalogID string) (bool, error) {
	var newEnabled bool
	err := s.pool.QueryRow(ctx, `
		INSERT INTO server_toggles (user_id, catalog_id, enabled)
		VALUES ($1, $2, false)
		ON CONFLICT(user_id, catalog_id) DO UPDATE
		SET enabled = NOT server_toggles.enabled, updated_at = now()
		RETURNING enabled`, userID, catalogID,
	).Scan(&newEnabled)
	if err != nil {
		return false, fmt.Errorf("toggle catalog entry: %w", err)
	}
	return newEnabled, nil
}

func (s *ToggleStore) DisabledCatalogIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT catalog_id FROM server_toggles
		WHERE user_id = $1 AND enabled = false`, userID)
	if err != nil {
		return nil, fmt.Errorf("list disabled catalog ids: %w", err)
	}
	defer rows.Close()

	disabled := make(map[string]struct{})
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan catalog id: %w", err)
		}
		disabled[id] = struct{}{}
	}
	return disabled, rows.Err()
}
