package catalog

import (
	"context"
	"fmt"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// Service orchestrates catalog management and suggestion fan-out.
type Service struct {
	catalog     *store.CatalogStore
	suggestions *store.SuggestionStore
}

func NewService(catalog *store.CatalogStore, suggestions *store.SuggestionStore) *Service {
	return &Service{catalog: catalog, suggestions: suggestions}
}

// AddToCatalog inserts a new catalog entry and fans out a pending suggestion
// to every existing developer user in a single operation.
func (s *Service) AddToCatalog(ctx context.Context, serverType, serverURL, displayName, description, addedBy string) (*store.CatalogEntry, error) {
	entry, err := s.catalog.AddCatalogEntry(ctx, serverType, serverURL, displayName, description, addedBy)
	if err != nil {
		return nil, fmt.Errorf("add to catalog: %w", err)
	}

	if err := s.suggestions.CreateSuggestionForAllUsers(ctx, entry.ID); err != nil {
		// Log but don't fail — the catalog entry itself was created successfully.
		// A background job can retry suggestion fan-out if needed.
		return entry, fmt.Errorf("catalog entry created but suggestion fan-out failed: %w", err)
	}

	return entry, nil
}

// RemoveFromCatalog soft-deletes a catalog entry by marking it inactive.
func (s *Service) RemoveFromCatalog(ctx context.Context, id string) error {
	return s.catalog.DeactivateCatalogEntry(ctx, id)
}
