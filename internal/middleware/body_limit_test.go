package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestUploadBodyLimitRejectsOversizedRequest(t *testing.T) {
	e := echo.New()

	e.POST("/upload", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, echomiddleware.BodyLimit("10B"))

	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("12345678901234567890"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
