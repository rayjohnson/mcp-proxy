package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type OAuth2StateStore struct {
	db *sql.DB
}

func NewOAuth2StateStore(db *sql.DB) *OAuth2StateStore {
	return &OAuth2StateStore{db: db}
}

func (s *OAuth2StateStore) CreateOAuth2State(ctx context.Context, userID, serverType, state string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO oauth2_state (user_id, server_type, state, expires_at)
		VALUES (?, ?, ?, datetime('now', '+10 minutes'))`,
		userID, serverType, state)
	if err != nil {
		return fmt.Errorf("create oauth2 state: %w", err)
	}
	return nil
}

func (s *OAuth2StateStore) ConsumeOAuth2State(ctx context.Context, state string) (*store.OAuth2State, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	var st store.OAuth2State
	err = tx.QueryRowContext(ctx, `
		SELECT id, user_id, server_type, state, expires_at
		FROM oauth2_state WHERE state = ? AND expires_at > datetime('now')`, state,
	).Scan(&st.ID, &st.UserID, &st.ServerType, &st.State, &st.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("oauth2 state not found or expired: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `DELETE FROM oauth2_state WHERE id = ?`, st.ID); err != nil {
		return nil, fmt.Errorf("delete oauth2 state: %w", err)
	}

	return &st, tx.Commit()
}

func (s *OAuth2StateStore) DeleteExpiredStates(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM oauth2_state WHERE expires_at <= datetime('now')`)
	return err
}
