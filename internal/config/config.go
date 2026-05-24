package config

import (
	"fmt"
	"os"
)

type Config struct {
	Port       string
	DBDSN      string
	KMSKeyName string
	BaseURL    string

	// JumpCloud OIDC — empty until infra team provisions it
	OIDCIssuerURL    string
	OIDCClientID     string
	OIDCClientSecret string
}

func Load() (*Config, error) {
	cfg := &Config{
		Port:             getenv("PORT", "8080"),
		DBDSN:            os.Getenv("DB_DSN"),
		KMSKeyName:       os.Getenv("KMS_KEY_NAME"),
		BaseURL:          os.Getenv("BASE_URL"),
		OIDCIssuerURL:    os.Getenv("OIDC_ISSUER_URL"),
		OIDCClientID:     os.Getenv("OIDC_CLIENT_ID"),
		OIDCClientSecret: os.Getenv("OIDC_CLIENT_SECRET"),
	}

	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("DB_DSN is required")
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
