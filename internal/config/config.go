package config

import (
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	Port       string
	DBDSN      string
	KMSKeyName string
	BaseURL    string

	// LocalMode enables single-user local deployment with SQLite storage.
	LocalMode bool
	// DataDir is the directory used for mcp-proxy.db in local mode.
	DataDir string

	// JumpCloud OIDC — empty until infra team provisions it
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
}

func Load() (*Config, error) {
	localMode := os.Getenv("LOCAL_MODE") == "true"
	dataDir := getenv("DATA_DIR", ".")

	dbDSN := os.Getenv("DB_DSN")
	if dbDSN == "" && localMode {
		dbDSN = "file:" + filepath.Join(dataDir, "mcp-proxy.db")
	}

	cfg := &Config{
		Port:             getenv("PORT", "8080"),
		DBDSN:            dbDSN,
		KMSKeyName:       os.Getenv("KMS_KEY_NAME"),
		BaseURL:          os.Getenv("BASE_URL"),
		LocalMode:        localMode,
		DataDir:          dataDir,
		OIDCIssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
	}

	if !localMode && cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required in hosted mode")
	}
	if cfg.KMSKeyName == "" {
		return nil, fmt.Errorf("KMS_KEY_NAME is required")
	}
	if cfg.BaseURL == "" {
		return nil, fmt.Errorf("BASE_URL is required")
	}

	return cfg, nil
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
