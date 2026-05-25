package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

type CatalogStore struct {
	db *sql.DB
}

func NewCatalogStore(db *sql.DB) *CatalogStore {
	return &CatalogStore{db: db}
}

const catalogCols = `id, server_type, server_url, display_name, description, added_by, active,
	auth_type, oauth_client_id, encrypted_oauth_secret, transport, command, args, env`

func scanCatalog(row interface{ Scan(...any) error }, e *store.CatalogEntry) error {
	var argsJSON, envJSON string
	var active int
	err := row.Scan(&e.ID, &e.ServerType, &e.ServerURL, &e.DisplayName,
		&e.Description, &e.AddedBy, &active,
		&e.AuthType, &e.OAuthClientID, &e.EncryptedOAuthSecret,
		&e.Transport, &e.Command, &argsJSON, &envJSON)
	if err != nil {
		return err
	}
	e.Active = active != 0
	if argsJSON != "" {
		_ = json.Unmarshal([]byte(argsJSON), &e.Args)
	}
	if e.Args == nil {
		e.Args = []string{}
	}
	if envJSON != "" {
		_ = json.Unmarshal([]byte(envJSON), &e.Env)
	}
	if e.Env == nil {
		e.Env = map[string]string{}
	}
	return nil
}

func (s *CatalogStore) AddCatalogEntry(ctx context.Context,
	serverType, serverURL, displayName, description, addedBy, authType, transport string,
	command *string, args []string, env map[string]string,
	oauthClientID *string, encryptedOAuthSecret []byte,
) (*store.CatalogEntry, error) {
	var desc *string
	if description != "" {
		desc = &description
	}
	if transport == "" {
		transport = "http"
	}
	if args == nil {
		args = []string{}
	}
	if env == nil {
		env = map[string]string{}
	}
	argsJSON, _ := json.Marshal(args)
	envJSON, _ := json.Marshal(env)

	var e store.CatalogEntry
	err := scanCatalog(s.db.QueryRowContext(ctx, `
		INSERT INTO default_catalog
		  (server_type, server_url, display_name, description, added_by, auth_type,
		   oauth_client_id, encrypted_oauth_secret, transport, command, args, env)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?)
		RETURNING `+catalogCols,
		serverType, serverURL, displayName, desc, addedBy, authType,
		oauthClientID, encryptedOAuthSecret, transport, command, string(argsJSON), string(envJSON),
	), &e)
	if err != nil {
		return nil, fmt.Errorf("add catalog entry: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) ListActiveCatalogEntries(ctx context.Context) ([]*store.CatalogEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE active = 1 ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list catalog entries: %w", err)
	}
	defer rows.Close() //nolint:errcheck
	var entries []*store.CatalogEntry
	for rows.Next() {
		var e store.CatalogEntry
		if err := scanCatalog(rows, &e); err != nil {
			return nil, fmt.Errorf("scan catalog entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *CatalogStore) GetCatalogEntryByServerType(ctx context.Context, serverType string) (*store.CatalogEntry, error) {
	var e store.CatalogEntry
	err := scanCatalog(s.db.QueryRowContext(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE server_type=? AND active=1`, serverType,
	), &e)
	if err != nil {
		return nil, fmt.Errorf("get catalog entry by type: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) GetCatalogEntryByID(ctx context.Context, id string) (*store.CatalogEntry, error) {
	var e store.CatalogEntry
	err := scanCatalog(s.db.QueryRowContext(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE id=? AND active=1`, id,
	), &e)
	if err != nil {
		return nil, fmt.Errorf("get catalog entry by id: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) DeactivateCatalogEntry(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE default_catalog SET active=0, updated_at=CURRENT_TIMESTAMP WHERE id=?`, id)
	return err
}

func (s *CatalogStore) UpdateCatalogEntry(ctx context.Context, id, serverURL, authType, displayName string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE default_catalog SET server_url=?, auth_type=?, display_name=?, updated_at=CURRENT_TIMESTAMP WHERE id=? AND active=1`,
		serverURL, authType, displayName, id)
	return err
}
