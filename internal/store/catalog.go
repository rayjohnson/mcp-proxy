package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogEntry struct {
	ID          string
	ServerType  string
	ServerURL   string
	DisplayName string
	Description *string
	AddedBy     string
	Active      bool
}

type CatalogStore struct {
	pool *pgxpool.Pool
}

func NewCatalogStore(pool *pgxpool.Pool) *CatalogStore {
	return &CatalogStore{pool: pool}
}

func (s *CatalogStore) AddCatalogEntry(ctx context.Context, serverType, serverURL, displayName, description, addedBy string) (*CatalogEntry, error) {
	var e CatalogEntry
	var desc *string
	if description != "" {
		desc = &description
	}
	err := s.pool.QueryRow(ctx, `
		INSERT INTO default_catalog (server_type, server_url, display_name, description, added_by)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, server_type, server_url, display_name, description, added_by, active`,
		serverType, serverURL, displayName, desc, addedBy,
	).Scan(&e.ID, &e.ServerType, &e.ServerURL, &e.DisplayName, &e.Description, &e.AddedBy, &e.Active)
	if err != nil {
		return nil, fmt.Errorf("add catalog entry: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) ListActiveCatalogEntries(ctx context.Context) ([]*CatalogEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, server_type, server_url, display_name, description, added_by, active
		FROM default_catalog WHERE active = true ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list catalog entries: %w", err)
	}
	defer rows.Close()

	var entries []*CatalogEntry
	for rows.Next() {
		var e CatalogEntry
		if err := rows.Scan(&e.ID, &e.ServerType, &e.ServerURL, &e.DisplayName,
			&e.Description, &e.AddedBy, &e.Active); err != nil {
			return nil, fmt.Errorf("scan catalog entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *CatalogStore) DeactivateCatalogEntry(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE default_catalog SET active = false, updated_at = now() WHERE id = $1`, id)
	return err
}

func (s *CatalogStore) GetCatalogEntryByServerType(ctx context.Context, serverType string) (*CatalogEntry, error) {
	var e CatalogEntry
	err := s.pool.QueryRow(ctx, `
		SELECT id, server_type, server_url, display_name, description, added_by, active
		FROM default_catalog WHERE server_type = $1 AND active = true`, serverType,
	).Scan(&e.ID, &e.ServerType, &e.ServerURL, &e.DisplayName, &e.Description, &e.AddedBy, &e.Active)
	if err != nil {
		return nil, fmt.Errorf("get catalog entry: %w", err)
	}
	return &e, nil
}
