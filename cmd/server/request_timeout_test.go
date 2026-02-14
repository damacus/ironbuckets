package main

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	customMiddleware "github.com/damacus/iron-buckets/internal/middleware"
	"github.com/labstack/echo/v4"
)

func TestRequestTimeoutMiddleware_ReturnsGatewayTimeoutForSlowHandler(t *testing.T) {
	e := echo.New()
	e.Use(customMiddleware.RequestTimeout(20 * time.Millisecond))
	e.GET("/slow", func(c echo.Context) error {
		select {
		case <-c.Request().Context().Done():
			return echo.NewHTTPError(http.StatusGatewayTimeout, "Request timed out")
		case <-time.After(200 * time.Millisecond):
			return c.String(http.StatusOK, "ok")
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()

	start := time.Now()
	e.ServeHTTP(rec, req)
	elapsed := time.Since(start)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status 504, got %d", rec.Code)
	}
	if elapsed > 100*time.Millisecond {
		t.Fatalf("expected request to time out quickly, took %s", elapsed)
	}
}

func TestRequestTimeoutMiddleware_ExposesDeadlineExceededContext(t *testing.T) {
	e := echo.New()
	e.Use(customMiddleware.RequestTimeout(20 * time.Millisecond))
	e.GET("/slow", func(c echo.Context) error {
		<-c.Request().Context().Done()
		if !errors.Is(c.Request().Context().Err(), context.DeadlineExceeded) {
			t.Fatalf("expected context deadline exceeded, got %v", c.Request().Context().Err())
		}
		return echo.NewHTTPError(http.StatusGatewayTimeout, "Request timed out")
	})

	req := httptest.NewRequest(http.MethodGet, "/slow", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusGatewayTimeout {
		t.Fatalf("expected status 504, got %d", rec.Code)
	}
}
