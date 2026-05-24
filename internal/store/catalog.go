package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type CatalogEntry struct {
	ID                   string
	ServerType           string
	ServerURL            string
	DisplayName          string
	Description          *string
	AddedBy              string
	Active               bool
	AuthType             string
	OAuthClientID        *string
	EncryptedOAuthSecret []byte
	Transport            string            // "http" (default) or "stdio"
	Command              *string           // stdio only
	Args                 []string          // stdio only
	Env                  map[string]string // stdio only
}

type CatalogStore struct {
	pool *pgxpool.Pool
}

func NewCatalogStore(pool *pgxpool.Pool) *CatalogStore {
	return &CatalogStore{pool: pool}
}

const catalogCols = `id, server_type, server_url, display_name, description, added_by, active,
	auth_type, oauth_client_id, encrypted_oauth_secret, transport, command, args, env`

func scanCatalog(row interface{ Scan(...any) error }, e *CatalogEntry) error {
	var argsJSON, envJSON []byte
	err := row.Scan(&e.ID, &e.ServerType, &e.ServerURL, &e.DisplayName,
		&e.Description, &e.AddedBy, &e.Active,
		&e.AuthType, &e.OAuthClientID, &e.EncryptedOAuthSecret,
		&e.Transport, &e.Command, &argsJSON, &envJSON)
	if err != nil {
		return err
	}
	if len(argsJSON) > 0 {
		_ = json.Unmarshal(argsJSON, &e.Args)
	}
	if e.Args == nil {
		e.Args = []string{}
	}
	if len(envJSON) > 0 {
		_ = json.Unmarshal(envJSON, &e.Env)
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
) (*CatalogEntry, error) {
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

	var e CatalogEntry
	err := scanCatalog(s.pool.QueryRow(ctx, `
		INSERT INTO default_catalog
		  (server_type, server_url, display_name, description, added_by, auth_type,
		   oauth_client_id, encrypted_oauth_secret, transport, command, args, env)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING `+catalogCols,
		serverType, serverURL, displayName, desc, addedBy, authType,
		oauthClientID, encryptedOAuthSecret, transport, command, argsJSON, envJSON,
	), &e)
	if err != nil {
		return nil, fmt.Errorf("add catalog entry: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) ListActiveCatalogEntries(ctx context.Context) ([]*CatalogEntry, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE active = true ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list catalog entries: %w", err)
	}
	defer rows.Close()
	var entries []*CatalogEntry
	for rows.Next() {
		var e CatalogEntry
		if err := scanCatalog(rows, &e); err != nil {
			return nil, fmt.Errorf("scan catalog entry: %w", err)
		}
		entries = append(entries, &e)
	}
	return entries, rows.Err()
}

func (s *CatalogStore) GetCatalogEntryByServerType(ctx context.Context, serverType string) (*CatalogEntry, error) {
	var e CatalogEntry
	err := scanCatalog(s.pool.QueryRow(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE server_type=$1 AND active=true`, serverType,
	), &e)
	if err != nil {
		return nil, fmt.Errorf("get catalog entry by type: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) GetCatalogEntryByID(ctx context.Context, id string) (*CatalogEntry, error) {
	var e CatalogEntry
	err := scanCatalog(s.pool.QueryRow(ctx,
		`SELECT `+catalogCols+` FROM default_catalog WHERE id=$1 AND active=true`, id,
	), &e)
	if err != nil {
		return nil, fmt.Errorf("get catalog entry by id: %w", err)
	}
	return &e, nil
}

func (s *CatalogStore) DeactivateCatalogEntry(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE default_catalog SET active=false, updated_at=now() WHERE id=$1`, id)
	return err
}
