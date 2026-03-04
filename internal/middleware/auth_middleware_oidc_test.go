package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/middleware"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_DoesNotSkipOIDCByDefault(t *testing.T) {
	authService := services.NewAuthService()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/login/oauth", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	mw := middleware.AuthMiddleware(authService)
	_ = mw(handler)(c)

	assert.False(t, handlerCalled, "/login/oauth should not be skipped by default")
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}
