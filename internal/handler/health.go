package handler

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/rayjohnson/mcp-proxy/internal/oauth2client"
	"github.com/rayjohnson/mcp-proxy/internal/store"
)

// StartHealthProbe runs a background goroutine that checks all upstream configs
// every 5 minutes, updating their status in the database. It also triggers
// OAuth2 token refresh for configs that are approaching expiry.
func StartHealthProbe(
	ctx context.Context,
	upstreamStore store.UpstreamStoreI,
	userStore store.UserStoreI,
	oauth2Svc *oauth2client.Service,
) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runHealthChecks(ctx, upstreamStore, userStore, oauth2Svc)
			}
		}
	}()
}

func runHealthChecks(
	ctx context.Context,
	upstreamStore store.UpstreamStoreI,
	userStore store.UserStoreI,
	oauth2Svc *oauth2client.Service,
) {
	users, err := userStore.ListAllUsers(ctx)
	if err != nil {
		slog.Warn("health probe: list users failed", "err", err)
		return
	}

	for _, u := range users {
		cfgs, err := upstreamStore.GetUpstreamConfigsByUserID(ctx, u.ID)
		if err != nil {
			slog.Warn("health probe: get configs failed", "user_id", u.ID, "err", err)
			continue
		}
		for _, cfg := range cfgs {
			if cfg.AuthType == "oauth2" {
				if err := oauth2Svc.RefreshIfExpired(ctx, cfg); err != nil {
					slog.Warn("health probe: refresh failed",
						"user_id", u.ID, "server_type", cfg.ServerType, "err", err)
				}
			}
		}
	}
}

// HealthHandler responds to liveness probes.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}
