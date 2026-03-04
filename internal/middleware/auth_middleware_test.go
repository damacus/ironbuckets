package middleware

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
)

func TestAuthMiddleware_SkipsPublicRoutes(t *testing.T) {
	publicPaths := []string{
		"/login",
		"/health",
		"/logout",
	}

	authService := services.NewAuthService()

	for _, path := range publicPaths {
		t.Run(path, func(t *testing.T) {
			e := echo.New()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			handlerCalled := false
			handler := func(c echo.Context) error {
				handlerCalled = true
				return c.String(http.StatusOK, "OK")
			}

			middleware := AuthMiddleware(authService)
			err := middleware(handler)(c)

			assert.NoError(t, err)
			assert.True(t, handlerCalled, "handler should be called for public path %s", path)
		})
	}
}

func TestAuthMiddleware_RedirectsWithoutCookie(t *testing.T) {
	e := echo.New()
	authService := services.NewAuthService()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	middleware := AuthMiddleware(authService)
	err := middleware(handler)(c)

	assert.NoError(t, err)
	assert.False(t, handlerCalled, "handler should not be called without cookie")
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))
}

func TestAuthMiddleware_RedirectsWithInvalidCookie(t *testing.T) {
	e := echo.New()
	authService := services.NewAuthService()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "invalid-encrypted-value",
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	middleware := AuthMiddleware(authService)
	err := middleware(handler)(c)

	assert.NoError(t, err)
	assert.False(t, handlerCalled, "handler should not be called with invalid cookie")
	assert.Equal(t, http.StatusSeeOther, rec.Code)
	assert.Equal(t, "/login", rec.Header().Get("Location"))
}

func TestAuthMiddleware_SetsCredentialsInContext(t *testing.T) {
	e := echo.New()
	authService := services.NewAuthService()

	// Create valid credentials and encrypt them
	creds := services.Credentials{
		Endpoint:  "localhost:9000",
		AccessKey: "admin",
		SecretKey: "password",
	}
	encrypted, err := authService.EncryptCredentials(creds)
	assert.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: encrypted,
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	var contextCreds *services.Credentials
	handler := func(c echo.Context) error {
		val := c.Get(utils.ContextKeyCreds)
		if val != nil {
			contextCreds = val.(*services.Credentials)
		}
		return c.String(http.StatusOK, "OK")
	}

	middleware := AuthMiddleware(authService)
	err = middleware(handler)(c)

	assert.NoError(t, err)
	assert.NotNil(t, contextCreds)
	assert.Equal(t, creds.Endpoint, contextCreds.Endpoint)
	assert.Equal(t, creds.AccessKey, contextCreds.AccessKey)
	assert.Equal(t, creds.SecretKey, contextCreds.SecretKey)
}

func TestAuthMiddleware_ClearsInvalidCookie(t *testing.T) {
	e := echo.New()
	authService := services.NewAuthService()

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	req.TLS = &tls.ConnectionState{}
	req.AddCookie(&http.Cookie{
		Name:  utils.CookieName,
		Value: "invalid-encrypted-value",
	})
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	}

	middleware := AuthMiddleware(authService)
	_ = middleware(handler)(c)

	// Check that a Set-Cookie header was set to clear the cookie
	cookies := rec.Result().Cookies()
	var foundClearCookie bool
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			assert.Equal(t, "", cookie.Value)
			assert.Equal(t, -1, cookie.MaxAge)
			assert.Equal(t, "/", cookie.Path)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
			assert.True(t, cookie.Secure)
			assert.False(t, cookie.Expires.IsZero())
			assert.True(t, cookie.Expires.Before(time.Now().Add(-30*time.Minute)))
			foundClearCookie = true
			break
		}
	}
	assert.True(t, foundClearCookie, "should set cookie with MaxAge=-1 to clear it")
}
