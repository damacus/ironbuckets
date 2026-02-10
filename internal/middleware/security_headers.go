package middleware

import (
	"strings"

	"github.com/labstack/echo/v4"
)

const contentSecurityPolicy = "default-src 'self'; " +
	"script-src 'self' 'unsafe-inline' https://cdn.tailwindcss.com https://unpkg.com; " +
	"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			headers := c.Response().Header()
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			headers.Set("Content-Security-Policy", contentSecurityPolicy)

			if isSecureRequest(c) {
				headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			return next(c)
		}
	}
}

func isSecureRequest(c echo.Context) bool {
	req := c.Request()
	if req.TLS != nil {
		return true
	}

	return strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
}
