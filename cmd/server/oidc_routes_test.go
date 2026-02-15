package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/handlers"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
)

func TestOIDCRoutesDisabledByDefault(t *testing.T) {
	e := echo.New()
	authHandler := handlers.NewAuthHandler(services.NewAuthService(), &services.RealMinioFactory{}, "localhost:9000", false)
	registerPublicRoutes(e, authHandler, false)

	req := httptest.NewRequest(http.MethodGet, "/login/oauth", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 when OIDC is disabled, got %d", rec.Code)
	}
}

func TestOIDCRoutesEnabledWithEnvFlag(t *testing.T) {
	e := echo.New()
	authHandler := handlers.NewAuthHandler(services.NewAuthService(), &services.RealMinioFactory{}, "localhost:9000", true)
	registerPublicRoutes(e, authHandler, true)

	req := httptest.NewRequest(http.MethodGet, "/login/oauth", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 when OIDC is enabled, got %d", rec.Code)
	}
}

func TestOIDCEnabledFromEnv(t *testing.T) {
	t.Setenv("OIDC_ENABLED", "TRUE")
	if !oidcEnabledFromEnv() {
		t.Fatal("expected OIDC_ENABLED=TRUE to be treated as enabled")
	}

	t.Setenv("OIDC_ENABLED", "false")
	if oidcEnabledFromEnv() {
		t.Fatal("expected OIDC_ENABLED=false to be treated as disabled")
	}
}
