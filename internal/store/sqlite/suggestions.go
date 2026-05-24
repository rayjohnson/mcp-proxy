package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type SuggestionStore struct {
	db *sql.DB
}

func NewSuggestionStore(db *sql.DB) *SuggestionStore {
	return &SuggestionStore{db: db}
}

func (s *SuggestionStore) CreateSuggestionForAllUsers(ctx context.Context, catalogID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO catalog_suggestions (user_id, catalog_id)
		SELECT id, ? FROM users WHERE role = 'developer'
		ON CONFLICT (user_id, catalog_id) DO NOTHING`, catalogID)
	if err != nil {
		return fmt.Errorf("create suggestions for all users: %w", err)
	}
	return nil
}

func (s *SuggestionStore) ListPendingSuggestionsForUser(ctx context.Context, userID string) ([]*store.Suggestion, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT cs.id, cs.user_id, cs.catalog_id, cs.status, cs.resolved_at,
		       dc.server_type, dc.display_name, dc.server_url, dc.description
		FROM catalog_suggestions cs
		JOIN default_catalog dc ON cs.catalog_id = dc.id
		WHERE cs.user_id = ? AND cs.status = 'pending' AND dc.active = 1
		ORDER BY cs.created_at`, userID)
	if err != nil {
		return nil, fmt.Errorf("list suggestions: %w", err)
	}
	defer rows.Close() //nolint:errcheck

	var suggestions []*store.Suggestion
	for rows.Next() {
		var sg store.Suggestion
		if err := rows.Scan(&sg.ID, &sg.UserID, &sg.CatalogID, &sg.Status, &sg.ResolvedAt,
			&sg.ServerType, &sg.DisplayName, &sg.ServerURL, &sg.Description); err != nil {
			return nil, fmt.Errorf("scan suggestion: %w", err)
		}
		suggestions = append(suggestions, &sg)
	}
	return suggestions, rows.Err()
}

func (s *SuggestionStore) ResolveSuggestion(ctx context.Context, id, userID, status string) error {
	result, err := s.db.ExecContext(ctx, `
		UPDATE catalog_suggestions
		SET status = ?, resolved_at = CURRENT_TIMESTAMP
		WHERE id = ? AND user_id = ? AND status = 'pending'`,
		status, id, userID)
	if err != nil {
		return fmt.Errorf("resolve suggestion: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("suggestion not found or already resolved")
	}
	return nil
}

func (s *SuggestionStore) GetSuggestion(ctx context.Context, id string) (*store.Suggestion, error) {
	var sg store.Suggestion
	err := s.db.QueryRowContext(ctx, `
		SELECT cs.id, cs.user_id, cs.catalog_id, cs.status, cs.resolved_at,
		       dc.server_type, dc.display_name, dc.server_url, dc.description
		FROM catalog_suggestions cs
		JOIN default_catalog dc ON cs.catalog_id = dc.id
		WHERE cs.id = ?`, id,
	).Scan(&sg.ID, &sg.UserID, &sg.CatalogID, &sg.Status, &sg.ResolvedAt,
		&sg.ServerType, &sg.DisplayName, &sg.ServerURL, &sg.Description)
	if err != nil {
		return nil, fmt.Errorf("get suggestion: %w", err)
	}
	return &sg, nil
}
