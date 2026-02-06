package backup

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("generate salt: %v", err)
	}
	if len(salt1) != saltSize {
		t.Errorf("salt length = %d, want %d", len(salt1), saltSize)
	}

	salt2, err := GenerateSalt()
	if err != nil {
		t.Fatalf("generate salt 2: %v", err)
	}
	if bytes.Equal(salt1, salt2) {
		t.Error("two salts should not be equal")
	}
}

func TestDeriveKeyDeterminism(t *testing.T) {
	salt := []byte("1234567890abcdef")

	key1 := DeriveKey("mypassphrase", salt)
	key2 := DeriveKey("mypassphrase", salt)

	if !bytes.Equal(key1, key2) {
		t.Error("same passphrase+salt should produce same key")
	}
	if len(key1) != keySize {
		t.Errorf("key length = %d, want %d", len(key1), keySize)
	}
}

func TestDeriveKeyDifferentPassphrases(t *testing.T) {
	salt := []byte("1234567890abcdef")

	key1 := DeriveKey("password1", salt)
	key2 := DeriveKey("password2", salt)

	if bytes.Equal(key1, key2) {
		t.Error("different passphrases should produce different keys")
	}
}

func TestEncryptDecryptRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.db")
	encPath := filepath.Join(dir, "encrypted.db.enc")
	decPath := filepath.Join(dir, "decrypted.db")

	original := []byte("This is test database content with some data in it.")
	if err := os.WriteFile(srcPath, original, 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	salt, err := GenerateSalt()
	if err != nil {
		t.Fatalf("generate salt: %v", err)
	}

	passphrase := "test-passphrase-123"

	if err := EncryptFile(srcPath, encPath, passphrase, salt); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Encrypted file should be different from original
	encrypted, _ := os.ReadFile(encPath)
	if bytes.Equal(encrypted, original) {
		t.Error("encrypted content should differ from original")
	}

	// Encrypted file should start with salt
	if !bytes.Equal(encrypted[:saltSize], salt) {
		t.Error("encrypted file should start with salt")
	}

	if err := DecryptFile(encPath, decPath, passphrase); err != nil {
		t.Fatalf("decrypt: %v", err)
	}

	decrypted, _ := os.ReadFile(decPath)
	if !bytes.Equal(original, decrypted) {
		t.Error("decrypted content should match original")
	}
}

func TestDecryptWrongPassphrase(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.db")
	encPath := filepath.Join(dir, "encrypted.db.enc")
	decPath := filepath.Join(dir, "decrypted.db")

	if err := os.WriteFile(srcPath, []byte("secret data"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	salt, _ := GenerateSalt()
	if err := EncryptFile(srcPath, encPath, "correct-password", salt); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	err := DecryptFile(encPath, decPath, "wrong-password")
	if err == nil {
		t.Fatal("expected error with wrong passphrase")
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "source.db")
	encPath := filepath.Join(dir, "encrypted.db.enc")
	decPath := filepath.Join(dir, "decrypted.db")

	if err := os.WriteFile(srcPath, []byte("secret data"), 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	salt, _ := GenerateSalt()
	if err := EncryptFile(srcPath, encPath, "password", salt); err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Tamper with the ciphertext (after salt + nonce)
	data, _ := os.ReadFile(encPath)
	if len(data) > saltSize+nonceSize+1 {
		data[saltSize+nonceSize+1] ^= 0xFF
		os.WriteFile(encPath, data, 0600)
	}

	err := DecryptFile(encPath, decPath, "password")
	if err == nil {
		t.Fatal("expected error with tampered ciphertext")
	}
}

func TestEncryptDecryptEmptyFile(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "empty.db")
	encPath := filepath.Join(dir, "empty.db.enc")
	decPath := filepath.Join(dir, "empty-dec.db")

	if err := os.WriteFile(srcPath, []byte{}, 0600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	salt, _ := GenerateSalt()
	if err := EncryptFile(srcPath, encPath, "password", salt); err != nil {
		t.Fatalf("encrypt empty file: %v", err)
	}

	if err := DecryptFile(encPath, decPath, "password"); err != nil {
		t.Fatalf("decrypt empty file: %v", err)
	}

	decrypted, _ := os.ReadFile(decPath)
	if len(decrypted) != 0 {
		t.Errorf("expected empty decrypted file, got %d bytes", len(decrypted))
	}
}

func TestDecryptFileTooSmall(t *testing.T) {
	dir := t.TempDir()
	encPath := filepath.Join(dir, "small.db.enc")
	decPath := filepath.Join(dir, "dec.db")

	// Write a file that's too small to contain salt + nonce
	os.WriteFile(encPath, []byte("too short"), 0600)

	err := DecryptFile(encPath, decPath, "password")
	if err == nil {
		t.Fatal("expected error with file too small")
	}
}
