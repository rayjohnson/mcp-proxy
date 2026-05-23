package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Suggestion struct {
	ID         string
	UserID     string
	CatalogID  string
	Status     string
	ResolvedAt *time.Time

	// Joined from default_catalog
	ServerType  string
	DisplayName string
	ServerURL   string
	Description *string
}

type SuggestionStore struct {
	pool *pgxpool.Pool
}

func NewSuggestionStore(pool *pgxpool.Pool) *SuggestionStore {
	return &SuggestionStore{pool: pool}
}

// CreateSuggestionForAllUsers inserts a pending suggestion for every existing developer.
func (s *SuggestionStore) CreateSuggestionForAllUsers(ctx context.Context, catalogID string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO catalog_suggestions (user_id, catalog_id)
		SELECT id, $1 FROM users WHERE role = 'developer'
		ON CONFLICT (user_id, catalog_id) DO NOTHING`, catalogID)
	if err != nil {
		return fmt.Errorf("create suggestions for all users: %w", err)
	}
	return nil
}

func (s *SuggestionStore) ListPendingSuggestionsForUser(ctx context.Context, userID string) ([]*Suggestion, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT cs.id, cs.user_id, cs.catalog_id, cs.status, cs.resolved_at,
		       dc.server_type, dc.display_name, dc.server_url, dc.description
		FROM catalog_suggestions cs
		JOIN default_catalog dc ON cs.catalog_id = dc.id
		WHERE cs.user_id = $1 AND cs.status = 'pending' AND dc.active = true
		ORDER BY cs.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list suggestions: %w", err)
	}
	defer rows.Close()

	var suggestions []*Suggestion
	for rows.Next() {
		var sg Suggestion
		if err := rows.Scan(&sg.ID, &sg.UserID, &sg.CatalogID, &sg.Status, &sg.ResolvedAt,
			&sg.ServerType, &sg.DisplayName, &sg.ServerURL, &sg.Description); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		suggestions = append(suggestions, &sg)
	}
	return suggestions, rows.Err()
}

func (s *SuggestionStore) ResolveSuggestion(ctx context.Context, id, userID, status string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE catalog_suggestions
		SET status = $1, resolved_at = now()
		WHERE id = $2 AND user_id = $3 AND status = 'pending'`,
		status, id, userID)
	if err != nil {
		return fmt.Errorf("resolve suggestion: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("suggestion not found or already resolved")
	}
	return nil
}

func (s *SuggestionStore) GetSuggestion(ctx context.Context, id string) (*Suggestion, error) {
	var sg Suggestion
	err := s.pool.QueryRow(ctx, `
		SELECT cs.id, cs.user_id, cs.catalog_id, cs.status, cs.resolved_at,
		       dc.server_type, dc.display_name, dc.server_url, dc.description
		FROM catalog_suggestions cs
		JOIN default_catalog dc ON cs.catalog_id = dc.id
		WHERE cs.id = $1`, id,
	).Scan(&sg.ID, &sg.UserID, &sg.CatalogID, &sg.Status, &sg.ResolvedAt,
		&sg.ServerType, &sg.DisplayName, &sg.ServerURL, &sg.Description)
	if err != nil {
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return &sg, nil
}
