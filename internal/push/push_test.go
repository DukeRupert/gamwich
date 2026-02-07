package push

import (
	"encoding/base64"
	"testing"
)

func TestGenerateVAPIDKeys(t *testing.T) {
	pub, priv, err := GenerateVAPIDKeys()
	if err != nil {
		t.Fatalf("generate VAPID keys: %v", err)
	}

	if pub == "" {
		t.Error("expected non-empty public key")
	}
	if priv == "" {
		t.Error("expected non-empty private key")
	}

	// Public key should be base64url-encoded, 65 bytes uncompressed P-256 point
	pubBytes, err := base64.RawURLEncoding.DecodeString(pub)
	if err != nil {
		t.Fatalf("decode public key: %v", err)
	}
	if len(pubBytes) != 65 {
		t.Errorf("public key length = %d, want 65", len(pubBytes))
	}

	// Private key should be base64url-encoded, 32 bytes P-256 scalar
	privBytes, err := base64.RawURLEncoding.DecodeString(priv)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("private key length = %d, want 32", len(privBytes))
	}

	// Generate again — should be different
	pub2, _, _ := GenerateVAPIDKeys()
	if pub == pub2 {
		t.Error("expected different keys on second generation")
	}
}

func TestPayloadJSON(t *testing.T) {
	p := Payload{
		Title: "Test",
		Body:  "Hello",
		URL:   "/test",
		Tag:   "test-tag",
	}

	if p.Title != "Test" {
		t.Errorf("title = %q, want %q", p.Title, "Test")
	}
	if p.Tag != "test-tag" {
		t.Errorf("tag = %q, want %q", p.Tag, "test-tag")
	}
}
