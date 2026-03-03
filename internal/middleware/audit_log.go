package middleware

import (
	"log/slog"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
)

// AuditLog records a structured log entry for every request.
// Mutating methods (POST, PUT, DELETE, PATCH) are logged at INFO level;
// safe methods are logged at DEBUG level.
// The authenticated user's access key is read from the Echo context (set by AuthMiddleware).
func AuditLog(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			req := c.Request()
			user := ""
			if creds, ok := c.Get(utils.ContextKeyCreds).(*services.Credentials); ok && creds != nil {
				user = creds.AccessKey
			}

			attrs := []slog.Attr{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", c.Response().Status),
				slog.String("remote_ip", c.RealIP()),
				slog.String("user", user),
			}

			level := slog.LevelDebug
			switch req.Method {
			case "POST", "PUT", "DELETE", "PATCH":
				level = slog.LevelInfo
			}

			logger.LogAttrs(req.Context(), level, "request", attrs...)

			return err
		}
	}
}
