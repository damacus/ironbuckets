package utils

import (
	"crypto/tls"
	"net/http/httptest"
	"testing"
)

func TestIsSecureRequest_UsesTLSState(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	req.TLS = &tls.ConnectionState{}

	if !IsSecureRequest(req) {
		t.Fatal("expected request with TLS state to be secure")
	}
}

func TestIsSecureRequest_UsesForwardedProtoCaseInsensitive(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	req.Header.Set("X-Forwarded-Proto", "HTTPS")

	if !IsSecureRequest(req) {
		t.Fatal("expected X-Forwarded-Proto=HTTPS to be secure")
	}
}

func TestIsSecureRequest_ReturnsFalseForNonSecureRequest(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)

	if IsSecureRequest(req) {
		t.Fatal("expected plain HTTP request to be non-secure")
	}
}
