package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogWritesStructuredEntry(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	e := echo.New()
	e.Use(AuditLog(logger))
	e.POST("/users/create", func(c echo.Context) error {
		// Simulate auth middleware having set credentials
		c.Set(utils.ContextKeyCreds, &services.Credentials{AccessKey: "admin-key"})
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/users/create", nil)
	req.RemoteAddr = "10.0.0.1:4567"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "POST", entry["method"])
	assert.Equal(t, "/users/create", entry["path"])
	assert.Equal(t, "admin-key", entry["user"])
	assert.NotEmpty(t, entry["remote_ip"])
	assert.NotEmpty(t, entry["time"])
}
