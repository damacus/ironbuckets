package services

import (
	"os"
	"testing"
)

func TestNewAuthService_GeneratesKey(t *testing.T) {
	// Ensure no env key is set
	_ = os.Unsetenv("IRON_SESSION_KEY")

	svc := NewAuthService()

	if len(svc.encryptionKey) != 32 {
		t.Errorf("expected 32-byte key, got %d bytes", len(svc.encryptionKey))
	}
}

func TestNewAuthService_UsesEnvKey(t *testing.T) {
	testKey := "12345678901234567890123456789012" // 32 bytes
	_ = os.Setenv("IRON_SESSION_KEY", testKey)
	defer func() { _ = os.Unsetenv("IRON_SESSION_KEY") }()

	svc := NewAuthService()

	if string(svc.encryptionKey) != testKey {
		t.Errorf("expected key %q, got %q", testKey, string(svc.encryptionKey))
	}
}

func TestNewAuthService_IgnoresShortEnvKey(t *testing.T) {
	_ = os.Setenv("IRON_SESSION_KEY", "tooshort")
	defer func() { _ = os.Unsetenv("IRON_SESSION_KEY") }()

	svc := NewAuthService()

	// Should generate a new key since env key is too short
	if len(svc.encryptionKey) != 32 {
		t.Errorf("expected 32-byte generated key, got %d bytes", len(svc.encryptionKey))
	}
	if string(svc.encryptionKey) == "tooshort" {
		t.Error("should not use short key")
	}
}

func TestEncryptDecryptCredentials_RoundTrip(t *testing.T) {
	svc := NewAuthService()

	original := Credentials{
		Endpoint:     "play.minio.io:9000",
		AccessKey:    "testuser",
		SecretKey:    "testpassword",
		SessionToken: "optional-token",
	}

	encrypted, err := svc.EncryptCredentials(original)
	if err != nil {
		t.Fatalf("EncryptCredentials failed: %v", err)
	}

	if encrypted == "" {
		t.Fatal("expected non-empty encrypted string")
	}

	decrypted, err := svc.DecryptCredentials(encrypted)
	if err != nil {
		t.Fatalf("DecryptCredentials failed: %v", err)
	}

	if decrypted.Endpoint != original.Endpoint {
		t.Errorf("Endpoint mismatch: got %q, want %q", decrypted.Endpoint, original.Endpoint)
	}
	if decrypted.AccessKey != original.AccessKey {
		t.Errorf("AccessKey mismatch: got %q, want %q", decrypted.AccessKey, original.AccessKey)
	}
	if decrypted.SecretKey != original.SecretKey {
		t.Errorf("SecretKey mismatch: got %q, want %q", decrypted.SecretKey, original.SecretKey)
	}
	if decrypted.SessionToken != original.SessionToken {
		t.Errorf("SessionToken mismatch: got %q, want %q", decrypted.SessionToken, original.SessionToken)
	}
}

func TestDecryptCredentials_InvalidBase64(t *testing.T) {
	svc := NewAuthService()

	_, err := svc.DecryptCredentials("not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecryptCredentials_InvalidCiphertext(t *testing.T) {
	svc := NewAuthService()

	// Valid base64 but not valid ciphertext
	_, err := svc.DecryptCredentials("dGVzdA==") // "test" in base64
	if err == nil {
		t.Error("expected error for invalid ciphertext")
	}
}

func TestDecryptCredentials_WrongKey(t *testing.T) {
	svc1 := NewAuthService()
	svc2 := NewAuthService() // Different key

	creds := Credentials{
		Endpoint:  "localhost:9000",
		AccessKey: "admin",
		SecretKey: "password",
	}

	encrypted, err := svc1.EncryptCredentials(creds)
	if err != nil {
		t.Fatalf("EncryptCredentials failed: %v", err)
	}

	// Try to decrypt with different key
	_, err = svc2.DecryptCredentials(encrypted)
	if err == nil {
		t.Error("expected error when decrypting with wrong key")
	}
}

func TestEncryptCredentials_ProducesDifferentOutput(t *testing.T) {
	svc := NewAuthService()

	creds := Credentials{
		Endpoint:  "localhost:9000",
		AccessKey: "admin",
		SecretKey: "password",
	}

	encrypted1, err := svc.EncryptCredentials(creds)
	if err != nil {
		t.Fatalf("EncryptCredentials failed: %v", err)
	}

	encrypted2, err := svc.EncryptCredentials(creds)
	if err != nil {
		t.Fatalf("EncryptCredentials failed: %v", err)
	}

	// Due to random nonce, same input should produce different output
	if encrypted1 == encrypted2 {
		t.Error("expected different encrypted outputs due to random nonce")
	}
}
