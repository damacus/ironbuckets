package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
)

const cspBase = "default-src 'self'; " +
	"script-src 'self' 'nonce-%s'; " +
	"style-src 'self' 'nonce-%s'; " +
	"img-src 'self' data: https:; " +
	"font-src 'self' https://fonts.gstatic.com; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// SecurityHeaders sets security-related HTTP response headers.
// It generates a per-request CSP nonce and stores it in the Echo context
// under the key "csp-nonce" for use in templates.
func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			nonce, err := generateNonce()
			if err != nil {
				return err
			}

			c.Set("csp-nonce", nonce)

			headers := c.Response().Header()
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			headers.Set("Content-Security-Policy", fmt.Sprintf(cspBase, nonce, nonce))

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
