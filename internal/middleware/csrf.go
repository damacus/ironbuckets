package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func CSRF() echo.MiddlewareFunc {
	return echoMiddleware.CSRFWithConfig(echoMiddleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token,form:_csrf",
		CookieName:     "csrf",
		CookiePath:     "/",
		CookieSameSite: http.SameSiteStrictMode,
		// CookieHTTPOnly is intentionally false: HTMX reads the CSRF cookie from
		// JavaScript (document.cookie) to attach it as the X-CSRF-Token header.
		// CookieSecure is intentionally false here; TLS deployments should set it
		// via an environment-aware wrapper if needed.
		Skipper: echoMiddleware.DefaultSkipper,
	})
}
