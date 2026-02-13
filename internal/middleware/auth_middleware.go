package middleware

import (
	"net/http"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
)

// AuthMiddleware checks for the IronSeal cookie and validates it
func AuthMiddleware(authService *services.AuthService) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Skip for public routes
			path := c.Request().URL.Path
			if path == "/login" || path == "/health" || path == "/logout" ||
				path == "/login/oauth" || path == "/oauth/callback" {
				return next(c)
			}

			// Get Cookie
			cookie, err := c.Cookie(utils.CookieName)
			if err != nil {
				return c.Redirect(http.StatusSeeOther, "/login")
			}

			// Decrypt
			creds, err := authService.DecryptCredentials(cookie.Value)
			if err != nil {
				// Invalid cookie - clear it with the same attributes as auth handlers.
				c.SetCookie(&http.Cookie{
					Name:     utils.CookieName,
					Value:    "",
					Expires:  time.Now().Add(-1 * time.Hour),
					MaxAge:   -1,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
					Secure:   isSecureRequest(c),
				})
				return c.Redirect(http.StatusSeeOther, "/login")
			}

			// Store creds in context for handlers to use
			c.Set(utils.ContextKeyCreds, creds)

			return next(c)
		}
	}
}
