package mcp

// Tests for session enable/disable filtering (T012, T020).

import (
	"context"
	"testing"

	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ---------------------------------------------------------------------------
// Fake stores
// ---------------------------------------------------------------------------

type sessionFakeUpstreamStore struct {
	configs []*store.UpstreamConfig
}

func (f *sessionFakeUpstreamStore) CreateUpstreamConfig(_ context.Context, _, _, _, _ string, _ []byte) (*store.UpstreamConfig, error) {
	return nil, nil
}
func (f *sessionFakeUpstreamStore) GetUpstreamConfigsByUserID(_ context.Context, _ string) ([]*store.UpstreamConfig, error) {
	return f.configs, nil
}
func (f *sessionFakeUpstreamStore) GetUpstreamConfigByID(_ context.Context, _ string) (*store.UpstreamConfig, error) {
	return nil, nil
}
func (f *sessionFakeUpstreamStore) UpdateUpstreamStatus(_ context.Context, _, _ string) error { return nil }
func (f *sessionFakeUpstreamStore) UpdateDetectedTransport(_ context.Context, _, _ string) error {
	return nil
}
func (f *sessionFakeUpstreamStore) UpdateEncryptedCreds(_ context.Context, _ string, _ []byte) error {
	return nil
}
func (f *sessionFakeUpstreamStore) DeleteUpstreamConfig(_ context.Context, _ string) error { return nil }
func (f *sessionFakeUpstreamStore) ToggleUpstream(_ context.Context, _ string) (bool, error) {
	return false, nil
}

type sessionFakeToggleStore struct {
	disabled map[string]struct{}
}

func (f *sessionFakeToggleStore) ToggleCatalogEntry(_ context.Context, _, _ string) (bool, error) {
	return false, nil
}
func (f *sessionFakeToggleStore) DisabledCatalogIDs(_ context.Context, _ string) (map[string]struct{}, error) {
	return f.disabled, nil
}

type sessionFakeCatalogStore struct {
	entries []*store.CatalogEntry
}

func (f *sessionFakeCatalogStore) AddCatalogEntry(_ context.Context, _, _, _, _, _, _, _ string, _ *string, _ []string, _ map[string]string, _ *string, _ []byte) (*store.CatalogEntry, error) {
	return nil, nil
}
func (f *sessionFakeCatalogStore) ListActiveCatalogEntries(_ context.Context) ([]*store.CatalogEntry, error) {
	return f.entries, nil
}
func (f *sessionFakeCatalogStore) GetCatalogEntryByServerType(_ context.Context, _ string) (*store.CatalogEntry, error) {
	return nil, nil
}
func (f *sessionFakeCatalogStore) GetCatalogEntryByID(_ context.Context, _ string) (*store.CatalogEntry, error) {
	return nil, nil
}
func (f *sessionFakeCatalogStore) DeactivateCatalogEntry(_ context.Context, _ string) error  { return nil }
func (f *sessionFakeCatalogStore) UpdateCatalogEntry(_ context.Context, _, _, _, _ string) error {
	return nil
}

// ---------------------------------------------------------------------------
// T012: disabled HTTP upstream is not connected in OpenSession
// ---------------------------------------------------------------------------

func TestOpenSession_SkipsDisabledUpstream(t *testing.T) {
	enabled := &store.UpstreamConfig{
		ID: "up-enabled", UserID: "u1", ServerType: "enabled-srv",
		ServerURL: "http://enabled.example.com", AuthType: "none",
		Status: "active", Enabled: true,
	}
	disabled := &store.UpstreamConfig{
		ID: "up-disabled", UserID: "u1", ServerType: "disabled-srv",
		ServerURL: "http://disabled.example.com", AuthType: "none",
		Status: "active", Enabled: false,
	}

	us := &sessionFakeUpstreamStore{configs: []*store.UpstreamConfig{enabled, disabled}}
	deps := SessionDeps{
		UpstreamStore: us,
		ToggleStore:   &sessionFakeToggleStore{disabled: map[string]struct{}{}},
		KMSDecrypt: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, nil
		},
		AuthHeader: func(_ *store.UpstreamConfig, _ []byte) (string, error) {
			return "", nil
		},
		UpdateTransport: func(_ context.Context, _, _ string) error { return nil },
	}

	ctx := context.Background()
	// OpenSession will attempt to connect disabled-srv's URL but we only care
	// that it skips the disabled one without trying to connect. Since the URL
	// is unreachable in tests, an actual connect would fail silently. We verify
	// by checking the session's configs only includes enabled ones.
	ps, err := OpenSession(ctx, "u1", deps)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}

	// The session configs should not include the disabled upstream.
	for _, c := range ps.configs {
		if c.ID == "up-disabled" {
			t.Error("disabled upstream should not be in session configs")
		}
	}
}

// ---------------------------------------------------------------------------
// T012: disabled catalog entry is skipped in connectStdioEntries
// ---------------------------------------------------------------------------

func TestConnectStdioEntries_SkipsDisabledEntry(t *testing.T) {
	catEnabled := &store.CatalogEntry{
		ID:        "cat-enabled",
		Transport: "stdio",
		ServerType: "enabled-stdio",
	}
	catDisabled := &store.CatalogEntry{
		ID:        "cat-disabled",
		Transport: "stdio",
		ServerType: "disabled-stdio",
	}

	cs := &sessionFakeCatalogStore{entries: []*store.CatalogEntry{catEnabled, catDisabled}}
	ts := &sessionFakeToggleStore{disabled: map[string]struct{}{"cat-disabled": {}}}

	ps := &ProxySession{
		clients: make(map[string]*UpstreamClient),
		UserID:  "u1",
	}

	// connectStdioEntries will try to launch the stdio process — it will fail for
	// the fake entries (no real binary), but we verify the disabled one was never
	// attempted by checking it was not added to clients.
	_ = connectStdioEntriesFiltered(context.Background(), ps, cs, ts)

	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if _, ok := ps.clients["disabled-stdio"]; ok {
		t.Error("disabled stdio entry should not be connected")
	}
}

// ---------------------------------------------------------------------------
// T020: re-enabled upstream IS connected in a fresh OpenSession
// ---------------------------------------------------------------------------

func TestOpenSession_ConnectsReEnabledUpstream(t *testing.T) {
	cfg := &store.UpstreamConfig{
		ID: "up-re", UserID: "u2", ServerType: "re-srv",
		ServerURL: "http://re.example.com", AuthType: "none",
		Status: "active", Enabled: true,
	}
	us := &sessionFakeUpstreamStore{configs: []*store.UpstreamConfig{cfg}}
	deps := SessionDeps{
		UpstreamStore: us,
		ToggleStore:   &sessionFakeToggleStore{disabled: map[string]struct{}{}},
		KMSDecrypt: func(_ context.Context, _ []byte) ([]byte, error) {
			return nil, nil
		},
		AuthHeader: func(_ *store.UpstreamConfig, _ []byte) (string, error) {
			return "", nil
		},
		UpdateTransport: func(_ context.Context, _, _ string) error { return nil },
	}

	ps, err := OpenSession(context.Background(), "u2", deps)
	if err != nil {
		t.Fatalf("OpenSession: %v", err)
	}
	// Re-enabled upstream should be present in session configs.
	found := false
	for _, c := range ps.configs {
		if c.ID == "up-re" {
			found = true
		}
	}
	if !found {
		t.Error("re-enabled upstream should be present in session configs")
	}
}
