package sqlite

import (
	"context"
	"database/sql"
	"fmt"
)

type ToggleStore struct {
	db *sql.DB
}

func NewToggleStore(db *sql.DB) *ToggleStore {
	return &ToggleStore{db: db}
}

func (s *ToggleStore) ToggleCatalogEntry(ctx context.Context, userID, catalogID string) (bool, error) {
	var newEnabled int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO server_toggles (user_id, catalog_id, enabled)
		VALUES (?, ?, 0)
		ON CONFLICT(user_id, catalog_id) DO UPDATE
		SET enabled = 1 - enabled, updated_at = CURRENT_TIMESTAMP
		RETURNING enabled`, userID, catalogID,
	).Scan(&newEnabled)
	if err != nil {
		return false, fmt.Errorf("toggle catalog entry: %w", err)
	}
	return newEnabled != 0, nil
}

func (s *ToggleStore) DisabledCatalogIDs(ctx context.Context, userID string) (map[string]struct{}, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT catalog_id FROM server_toggles
		WHERE user_id = ? AND enabled = 0`, userID)
	if err != nil {
		return nil, fmt.Errorf("list disabled catalog ids: %w", err)
	}
	defer rows.Close() //nolint:errcheck

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
