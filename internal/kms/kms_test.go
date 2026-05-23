package kms

import (
	"context"
	"bytes"
	"testing"
)

func TestLocalKMSRoundtrip(t *testing.T) {
	c, err := New(context.Background(), "local")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	plaintext := []byte("super secret api key")

	ciphertext, err := c.Encrypt(context.Background(), plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext should not equal plaintext")
	}

	got, err := c.Decrypt(context.Background(), ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(got, plaintext) {
		t.Errorf("roundtrip mismatch: got %q, want %q", got, plaintext)
	}
}

func TestLocalKMSDistinctCiphertexts(t *testing.T) {
	c, err := New(context.Background(), "local")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	plaintext := []byte("same input")
	ct1, _ := c.Encrypt(context.Background(), plaintext)
	ct2, _ := c.Encrypt(context.Background(), plaintext)
	if bytes.Equal(ct1, ct2) {
		t.Error("two encryptions of the same plaintext should produce distinct ciphertexts (random nonce)")
	}
}

func TestLocalKMSBadCiphertext(t *testing.T) {
	c, err := New(context.Background(), "local")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Close()

	_, err = c.Decrypt(context.Background(), []byte("too short"))
	if err == nil {
		t.Error("expected error decrypting invalid ciphertext, got nil")
	}
}
