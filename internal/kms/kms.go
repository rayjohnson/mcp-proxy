package kms

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"

	kmsapi "cloud.google.com/go/kms/apiv1"
	"cloud.google.com/go/kms/apiv1/kmspb"
)

// Client wraps either GCP KMS or a local AES-256-GCM implementation.
// Use KMS_KEY_NAME=local to activate the local shim for development.
type Client struct {
	keyName string
	gcp     *kmsapi.KeyManagementClient // nil in local mode
	local   *localKMS                   // nil in GCP mode
}

// New returns a Client for the given key name.
// When keyName is "local", a local AES-256-GCM shim is used instead of GCP KMS.
// The local key is read from LOCAL_KMS_KEY (hex-encoded 32 bytes); if unset, a
// random ephemeral key is generated (credentials are lost on restart — dev only).
func New(ctx context.Context, keyName string) (*Client, error) {
	if keyName == "local" {
		lk, err := newLocalKMS()
		if err != nil {
			return nil, err
		}
		return &Client{keyName: keyName, local: lk}, nil
	}

	c, err := kmsapi.NewKeyManagementClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("create kms client: %w", err)
	}
	return &Client{keyName: keyName, gcp: c}, nil
}

func (c *Client) Encrypt(ctx context.Context, plaintext []byte) ([]byte, error) {
	if c.local != nil {
		return c.local.encrypt(plaintext)
	}
	resp, err := c.gcp.Encrypt(ctx, &kmspb.EncryptRequest{
		Name:      c.keyName,
		Plaintext: plaintext,
	})
	if err != nil {
		return nil, fmt.Errorf("kms encrypt: %w", err)
	}
	return resp.Ciphertext, nil
}

func (c *Client) Decrypt(ctx context.Context, ciphertext []byte) ([]byte, error) {
	if c.local != nil {
		return c.local.decrypt(ciphertext)
	}
	resp, err := c.gcp.Decrypt(ctx, &kmspb.DecryptRequest{
		Name:       c.keyName,
		Ciphertext: ciphertext,
	})
	if err != nil {
		return nil, fmt.Errorf("kms decrypt: %w", err)
	}
	return resp.Plaintext, nil
}

func (c *Client) Close() error {
	if c.gcp != nil {
		return c.gcp.Close()
	}
	return nil
}

// localKMS implements AES-256-GCM encryption for local development.
// Ciphertext format: [12-byte nonce][GCM tag + ciphertext]
type localKMS struct {
	aead cipher.AEAD
}

func newLocalKMS() (*localKMS, error) {
	var key []byte

	if raw := os.Getenv("LOCAL_KMS_KEY"); raw != "" {
		var err error
		key, err = hex.DecodeString(raw)
		if err != nil || len(key) != 32 {
			return nil, fmt.Errorf("LOCAL_KMS_KEY must be a 64-character hex string (32 bytes)")
		}
		slog.Info("kms: using local AES-256-GCM shim with LOCAL_KMS_KEY")
	} else {
		key = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, fmt.Errorf("generate local kms key: %w", err)
		}
		slog.Warn("kms: LOCAL_KMS_KEY not set — using ephemeral key; credentials lost on restart")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("create aes cipher: %w", err)
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create gcm: %w", err)
	}
	return &localKMS{aead: aead}, nil
}

func (l *localKMS) encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, l.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}
	return l.aead.Seal(nonce, nonce, plaintext, nil), nil
}

func (l *localKMS) decrypt(ciphertext []byte) ([]byte, error) {
	ns := l.aead.NonceSize()
	if len(ciphertext) < ns {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, data := ciphertext[:ns], ciphertext[ns:]
	plaintext, err := l.aead.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}
	return plaintext, nil
}
