package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServerAddsSecurityHeadersOnHealth(t *testing.T) {
	originalWD, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir("../.."))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	e := newServer("localhost:9000")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "DENY", rec.Header().Get("X-Frame-Options"))
	assert.NotEmpty(t, rec.Header().Get("Content-Security-Policy"))
}

func TestServerRejectsProtectedPostWithoutCSRFToken(t *testing.T) {
	originalWD, err := os.Getwd()
	assert.NoError(t, err)
	assert.NoError(t, os.Chdir("../.."))
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	e := newServer("localhost:9000")

	req := httptest.NewRequest(http.MethodPost, "/users/create", nil)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
