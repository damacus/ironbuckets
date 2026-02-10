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
		Skipper: func(c echo.Context) bool {
			switch c.Request().Method {
			case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
				return false
			}

			return c.Request().Header.Get("HX-Request") != "true"
		},
	})
}
