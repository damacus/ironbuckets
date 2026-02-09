package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCSRFMiddlewareRejectsPostWithoutToken(t *testing.T) {
	e := echo.New()
	e.Use(CSRF())
	e.POST("/submit", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("x=1"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestCSRFMiddlewareAllowsPostWithTokenHeaderAndCookie(t *testing.T) {
	e := echo.New()
	e.Use(CSRF())
	e.GET("/csrf", func(c echo.Context) error {
		token, _ := c.Get("csrf").(string)
		return c.String(http.StatusOK, token)
	})
	e.POST("/submit", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	getReq := httptest.NewRequest(http.MethodGet, "/csrf", nil)
	getRec := httptest.NewRecorder()
	e.ServeHTTP(getRec, getReq)

	require.Equal(t, http.StatusOK, getRec.Code)
	token := strings.TrimSpace(getRec.Body.String())
	require.NotEmpty(t, token)

	var csrfCookie *http.Cookie
	for _, cookie := range getRec.Result().Cookies() {
		if cookie.Name == "csrf" {
			csrfCookie = cookie
			break
		}
	}
	require.NotNil(t, csrfCookie)

	postReq := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("x=1"))
	postReq.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	postReq.Header.Set("HX-Request", "true")
	postReq.Header.Set("X-CSRF-Token", token)
	postReq.AddCookie(csrfCookie)
	postRec := httptest.NewRecorder()
	e.ServeHTTP(postRec, postReq)

	assert.Equal(t, http.StatusOK, postRec.Code)
}
