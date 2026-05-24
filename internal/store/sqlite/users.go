package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type UserStore struct {
	db *sql.DB
}

func NewUserStore(db *sql.DB) *UserStore {
	return &UserStore{db: db}
}

func (s *UserStore) CreateUser(ctx context.Context, email, passwordHash, role string) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, role)
		VALUES (?, ?, ?)
		RETURNING id, email, password_hash, role, proxy_token`,
		email, passwordHash, role,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.ProxyToken)
	if err != nil {
		if isSQLiteUnique(err) {
			return nil, store.ErrDuplicateEmail
		}
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &u, nil
}

func (s *UserStore) GetUserByEmail(ctx context.Context, email string) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, role, proxy_token
		FROM users WHERE email = ?`, email,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.ProxyToken)
	if err != nil {
		return nil, fmt.Errorf("get user by email: %w", err)
	}
	return &u, nil
}

func (s *UserStore) GetUserByID(ctx context.Context, id string) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, role, proxy_token
		FROM users WHERE id = ?`, id,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.ProxyToken)
	if err != nil {
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &u, nil
}

func (s *UserStore) GetUserByProxyToken(ctx context.Context, token string) (*store.User, error) {
	var u store.User
	err := s.db.QueryRowContext(ctx, `
		SELECT id, email, password_hash, role, proxy_token
		FROM users WHERE proxy_token = ?`, token,
	).Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.ProxyToken)
	if err != nil {
		return nil, fmt.Errorf("get user by proxy token: %w", err)
	}
	return &u, nil
}

func (s *UserStore) ListAllUsers(ctx context.Context) ([]*store.User, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, email, password_hash, role, proxy_token FROM users`)
	if err != nil {
		return nil, fmt.Errorf("list all users: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var users []*store.User
	for rows.Next() {
		var u store.User
		if err := rows.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.Role, &u.ProxyToken); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, &u)
	}
	return users, rows.Err()
}

func (s *UserStore) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, err
}

func (s *UserStore) UpdateUserRole(ctx context.Context, id, role string) error {
	result, err := s.db.ExecContext(ctx,
		`UPDATE users SET role=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`, role, id)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("user not found")
	}
	return nil
}

func isSQLiteUnique(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errors.New("UNIQUE constraint failed")) ||
		containsStr(err.Error(), "UNIQUE constraint failed")
}

func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
