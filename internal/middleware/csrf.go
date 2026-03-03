package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	echoMiddleware "github.com/labstack/echo/v4/middleware"
)

func CSRF() echo.MiddlewareFunc {
	return echoMiddleware.CSRFWithConfig(echoMiddleware.CSRFConfig{
		TokenLookup:    "header:X-CSRF-Token",
		CookieName:     "csrf",
		CookiePath:     "/",
		CookieSameSite: http.SameSiteStrictMode,
		Skipper: echoMiddleware.DefaultSkipper,
	})
}
