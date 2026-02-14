package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
)

func RequestTimeout(timeout time.Duration) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx, cancel := context.WithTimeout(c.Request().Context(), timeout)
			defer cancel()

			c.SetRequest(c.Request().WithContext(ctx))

			err := next(c)
			if err != nil {
				return err
			}

			if ctx.Err() == context.DeadlineExceeded {
				return echo.NewHTTPError(http.StatusGatewayTimeout, "Request timed out")
			}

			return nil
		}
	}
}
