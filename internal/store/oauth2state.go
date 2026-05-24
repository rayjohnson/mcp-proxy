package store

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type OAuth2State struct {
	ID         string
	UserID     string
	ServerType string
	State      string
	ExpiresAt  time.Time
}

type OAuth2StateStore struct {
	pool *pgxpool.Pool
}

func NewOAuth2StateStore(pool *pgxpool.Pool) *OAuth2StateStore {
	return &OAuth2StateStore{pool: pool}
}

func (s *OAuth2StateStore) CreateOAuth2State(ctx context.Context, userID, serverType, state string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO oauth2_state (user_id, server_type, state, expires_at)
		VALUES ($1, $2, $3, now() + interval '10 minutes')`,
		userID, serverType, state)
	if err != nil {
		return fmt.Errorf("create oauth2 state: %w", err)
	}
	return nil
}

// ConsumeOAuth2State fetches and deletes the state in one transaction.
// Returns an error if the state does not exist or has expired.
func (s *OAuth2StateStore) ConsumeOAuth2State(ctx context.Context, state string) (*OAuth2State, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var st OAuth2State
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, server_type, state, expires_at
		FROM oauth2_state WHERE state = $1 AND expires_at > now()`, state,
	).Scan(&st.ID, &st.UserID, &st.ServerType, &st.State, &st.ExpiresAt)
	if err != nil {
		return nil, fmt.Errorf("oauth2 state not found or expired: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM oauth2_state WHERE id = $1`, st.ID); err != nil {
		return nil, fmt.Errorf("delete oauth2 state: %w", err)
	}

	return &st, tx.Commit(ctx)
}

func (s *OAuth2StateStore) DeleteExpiredStates(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM oauth2_state WHERE expires_at <= now()`)
	return err
}
