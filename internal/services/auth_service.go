package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log"
	"os"
	"strings"
)

// Credentials represents the MinIO login details
type Credentials struct {
	Endpoint     string `json:"endpoint"`
	AccessKey    string `json:"accessKey"`
	SecretKey    string `json:"secretKey"`
	SessionToken string `json:"sessionToken,omitempty"` // For STS/OIDC
}

type AuthService struct {
	encryptionKey []byte
}

// NewAuthService creates a new auth service with a key from env or generates one (ephemeral)
func NewAuthService() *AuthService {
	key := os.Getenv("IRON_SESSION_KEY")
	isProduction := strings.EqualFold(os.Getenv("APP_ENV"), "production")

	if len(key) != 32 {
		if isProduction {
			panic("IRON_SESSION_KEY must be exactly 32 bytes in production. " +
				"Set APP_ENV=production and IRON_SESSION_KEY to a 32-byte secret.")
		}
		log.Println("WARNING: IRON_SESSION_KEY not set or not 32 bytes — " +
			"using ephemeral key. Sessions will be invalidated on restart.")
		newKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
			panic("failed to generate random key")
		}
		return &AuthService{encryptionKey: newKey}
	}
	return &AuthService{encryptionKey: []byte(key)}
}

// EncryptCredentials serializes and encrypts credentials into a string (for the cookie)
func (s *AuthService) EncryptCredentials(creds Credentials) (string, error) {
	data, err := json.Marshal(creds)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)
	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

// DecryptCredentials decodes the cookie value back into Credentials
func (s *AuthService) DecryptCredentials(encrypted string) (*Credentials, error) {
	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(s.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("malformed ciphertext")
	}

	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	var creds Credentials
	if err := json.Unmarshal(plaintext, &creds); err != nil {
		return nil, err
	}

	return &creds, nil
}
