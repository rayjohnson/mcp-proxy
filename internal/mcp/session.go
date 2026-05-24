package mcp

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// ProxySession holds live upstream client connections for one developer session.
type ProxySession struct {
	mu      sync.RWMutex
	clients map[string]*UpstreamClient // keyed by server_type
	configs []*store.UpstreamConfig
	UserID  string
}

type SessionDeps struct {
	UpstreamStore store.UpstreamStoreI
	CatalogStore  store.CatalogStoreI // for stdio entries, which are catalog-level (no per-user creds)
	KMSDecrypt    func(ctx context.Context, ciphertext []byte) ([]byte, error)
	AuthHeader    func(cfg *store.UpstreamConfig, plainCreds []byte) (string, error)
	UpdateTransport func(ctx context.Context, id, transport string) error
}

// OpenSession connects to all reachable upstream servers for the given user.
// Failures on individual upstreams are logged but do not abort the session.
func OpenSession(ctx context.Context, userID string, deps SessionDeps) (*ProxySession, error) {
	configs, err := deps.UpstreamStore.GetUpstreamConfigsByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("load upstream configs: %w", err)
	}

	ps := &ProxySession{
		clients: make(map[string]*UpstreamClient),
		configs: configs,
		UserID:  userID,
	}

	var wg sync.WaitGroup
	for _, cfg := range configs {
		if cfg.Status == "reauth_required" {
			continue
		}
		wg.Add(1)
		go func(c *store.UpstreamConfig) {
			defer wg.Done()

			plainCreds, err := deps.KMSDecrypt(ctx, c.EncryptedCreds)
			if err != nil {
				slog.Warn("failed to decrypt creds", "server_type", c.ServerType, "err", err)
				return
			}

			authHeader, err := deps.AuthHeader(c, plainCreds)
			if err != nil {
				slog.Warn("failed to build auth header", "server_type", c.ServerType, "err", err)
				return
			}

			var cached string
			if c.DetectedTransport != nil {
				cached = *c.DetectedTransport
			}

			client, err := Connect(ctx, c.ServerURL, authHeader, cached)
			if err != nil {
				slog.Warn("upstream connect failed", "server_type", c.ServerType, "err", err)
				return
			}

			// Cache detected transport if it changed.
			if c.DetectedTransport == nil || *c.DetectedTransport != client.DetectedTransport {
				if err := deps.UpdateTransport(ctx, c.ID, client.DetectedTransport); err != nil {
					slog.Warn("update transport cache failed", "err", err)
				}
			}

			ps.mu.Lock()
			ps.clients[c.ServerType] = client
			ps.mu.Unlock()
		}(cfg)
	}
	wg.Wait()

	// Also connect to stdio catalog entries (available to all users; no per-user creds).
	if deps.CatalogStore != nil {
		if err := connectStdioEntries(ctx, ps, deps.CatalogStore); err != nil {
			slog.Warn("stdio catalog connect error", "err", err)
		}
	}

	return ps, nil
}

// connectStdioEntries connects to all active stdio catalog entries.
func connectStdioEntries(ctx context.Context, ps *ProxySession, cs store.CatalogStoreI) error {
	entries, err := cs.ListActiveCatalogEntries(ctx)
	if err != nil {
		return fmt.Errorf("list catalog entries for stdio: %w", err)
	}
	for _, e := range entries {
		if e.Transport != "stdio" {
			continue
		}
		e := e // capture
		client, err := ConnectStdio(ctx, e)
		if err != nil {
			slog.Warn("stdio server connect failed", "server_type", e.ServerType, "err", err)
			continue
		}
		ps.mu.Lock()
		ps.clients[e.ServerType] = client
		ps.mu.Unlock()
	}
	return nil
}

// GetClient returns the upstream client for the given server type, or nil.
func (ps *ProxySession) GetClient(serverType string) *sdkmcp.ClientSession {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	if c, ok := ps.clients[serverType]; ok {
		return c.Session
	}
	return nil
}

// AllClients returns a snapshot of all live upstream clients.
func (ps *ProxySession) AllClients() map[string]*sdkmcp.ClientSession {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	out := make(map[string]*sdkmcp.ClientSession, len(ps.clients))
	for k, v := range ps.clients {
		out[k] = v.Session
	}
	return out
}

// Close closes all upstream client sessions.
func (ps *ProxySession) Close() {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, c := range ps.clients {
		c.Session.Close() //nolint:errcheck,gosec
	}
}
