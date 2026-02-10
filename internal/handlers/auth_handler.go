package handlers

import (
	"net/http"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
)

type AuthHandler struct {
	authService   *services.AuthService
	minioFactory  services.MinioClientFactory
	minioEndpoint string
}

func NewAuthHandler(authService *services.AuthService, minioFactory services.MinioClientFactory, minioEndpoint string) *AuthHandler {
	return &AuthHandler{
		authService:   authService,
		minioFactory:  minioFactory,
		minioEndpoint: minioEndpoint,
	}
}

// LoginPage renders the login view
func (h *AuthHandler) LoginPage(c echo.Context) error {
	// If already logged in (cookie exists AND is valid), redirect to dashboard
	cookie, err := c.Cookie(utils.CookieName)
	if err == nil {
		_, err := h.authService.DecryptCredentials(cookie.Value)
		if err == nil {
			return c.Redirect(http.StatusSeeOther, "/")
		}
		// If invalid, we just ignore it and show login page.
		// Optionally we could clear it here too, but the login post will overwrite it.
	}
	// We use a specific "login" template set that doesn't use the main sidebar layout
	return c.Render(http.StatusOK, "login", nil)
}

// Login handles the form submission
func (h *AuthHandler) Login(c echo.Context) error {
	accessKey := c.FormValue("accessKey")
	secretKey := c.FormValue("secretKey")

	creds := services.Credentials{
		Endpoint:  h.minioEndpoint,
		AccessKey: accessKey,
		SecretKey: secretKey,
	}

	// 1. Validate Credentials with MinIO
	// Use S3 client instead of admin client so regular users can login
	s3Client, err := h.minioFactory.NewClient(creds)
	if err != nil {
		return c.Render(http.StatusOK, "login_error", "Invalid Configuration")
	}

	// Attempt a lightweight call to verify auth - ListBuckets works for all users
	_, err = s3Client.ListBuckets(c.Request().Context())
	if err != nil {
		// Return HTML fragment for error div if using HTMX, or re-render page
		return c.HTML(http.StatusOK, `<div id="error-message" class="text-red-500 text-sm text-center block mb-4">Authentication Failed: Invalid Credentials or Endpoint Unreachable</div>`)
	}

	// 2. Encrypt Session
	// creds is already populated above
	encrypted, err := h.authService.EncryptCredentials(creds)
	if err != nil {
		return c.HTML(http.StatusInternalServerError, "Failed to create session")
	}

	// 3. Set Cookie
	cookie := new(http.Cookie)
	cookie.Name = utils.CookieName
	cookie.Value = encrypted
	cookie.Expires = time.Now().Add(24 * time.Hour)
	cookie.Path = "/"
	cookie.HttpOnly = true
	cookie.SameSite = http.SameSiteStrictMode
	cookie.Secure = requestIsSecure(c)
	c.SetCookie(cookie)

	// 4. Redirect (HTMX handles 200 OK with HX-Redirect)
	return HTMXRedirect(c, "/")
}

// Logout clears the session
func (h *AuthHandler) Logout(c echo.Context) error {
	cookie := new(http.Cookie)
	cookie.Name = utils.CookieName
	cookie.Value = ""
	cookie.Expires = time.Now().Add(-1 * time.Hour)
	cookie.MaxAge = -1
	cookie.Path = "/"
	cookie.HttpOnly = true
	cookie.SameSite = http.SameSiteStrictMode
	cookie.Secure = requestIsSecure(c)
	c.SetCookie(cookie)
	return c.Redirect(http.StatusSeeOther, "/login")
}

func requestIsSecure(c echo.Context) bool {
	req := c.Request()
	if req.TLS != nil {
		return true
	}

	return req.Header.Get("X-Forwarded-Proto") == "https"
}

// LoginOIDC initiates the OIDC flow
func (h *AuthHandler) LoginOIDC(c echo.Context) error {
	// TODO: Generate state, store in cookie, redirect to OIDC provider
	// For now, just return a message
	return c.HTML(http.StatusOK, "<h1>OIDC Redirect...</h1><p>(Not fully configured yet)</p>")
}

// CallbackOIDC handles the OIDC callback
func (h *AuthHandler) CallbackOIDC(c echo.Context) error {
	// TODO: Exchange code for token, assume role with MinIO, set cookie
	return echo.NewHTTPError(http.StatusNotImplemented, "OIDC Callback not implemented")
}
