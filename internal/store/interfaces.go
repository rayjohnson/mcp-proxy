package store

import "context"

// UserStoreI is the interface satisfied by both the Postgres and SQLite user stores.
type UserStoreI interface {
	CreateUser(ctx context.Context, email, passwordHash, role string) (*User, error)
	GetUserByEmail(ctx context.Context, email string) (*User, error)
	GetUserByID(ctx context.Context, id string) (*User, error)
	GetUserByProxyToken(ctx context.Context, token string) (*User, error)
	ListAllUsers(ctx context.Context) ([]*User, error)
	CountUsers(ctx context.Context) (int, error)
	UpdateUserRole(ctx context.Context, id, role string) error
}

// CatalogStoreI is the interface satisfied by both the Postgres and SQLite catalog stores.
type CatalogStoreI interface {
	AddCatalogEntry(ctx context.Context,
		serverType, serverURL, displayName, description, addedBy, authType, transport string,
		command *string, args []string, env map[string]string,
		oauthClientID *string, encryptedOAuthSecret []byte,
	) (*CatalogEntry, error)
	ListActiveCatalogEntries(ctx context.Context) ([]*CatalogEntry, error)
	GetCatalogEntryByServerType(ctx context.Context, serverType string) (*CatalogEntry, error)
	GetCatalogEntryByID(ctx context.Context, id string) (*CatalogEntry, error)
	DeactivateCatalogEntry(ctx context.Context, id string) error
}

// UpstreamStoreI is the interface satisfied by both the Postgres and SQLite upstream stores.
type UpstreamStoreI interface {
	CreateUpstreamConfig(ctx context.Context, userID, serverType, serverURL, authType string, encryptedCreds []byte) (*UpstreamConfig, error)
	GetUpstreamConfigsByUserID(ctx context.Context, userID string) ([]*UpstreamConfig, error)
	GetUpstreamConfigByID(ctx context.Context, id string) (*UpstreamConfig, error)
	UpdateUpstreamStatus(ctx context.Context, id, status string) error
	UpdateDetectedTransport(ctx context.Context, id, transport string) error
	UpdateEncryptedCreds(ctx context.Context, id string, encryptedCreds []byte) error
	DeleteUpstreamConfig(ctx context.Context, id string) error
}

// OAuth2StateStoreI is the interface satisfied by both the Postgres and SQLite OAuth2 state stores.
type OAuth2StateStoreI interface {
	CreateOAuth2State(ctx context.Context, userID, serverType, state string) error
	ConsumeOAuth2State(ctx context.Context, state string) (*OAuth2State, error)
	DeleteExpiredStates(ctx context.Context) error
}

// SuggestionStoreI is the interface satisfied by both the Postgres and SQLite suggestion stores.
type SuggestionStoreI interface {
	CreateSuggestionForAllUsers(ctx context.Context, catalogID string) error
	ListPendingSuggestionsForUser(ctx context.Context, userID string) ([]*Suggestion, error)
	ResolveSuggestion(ctx context.Context, id, userID, status string) error
	GetSuggestion(ctx context.Context, id string) (*Suggestion, error)
}
