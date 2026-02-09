package handlers

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/replication"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type authTestFactory struct {
	client services.MinioClient
}

func (f *authTestFactory) NewAdminClient(_ services.Credentials) (services.MinioAdminClient, error) {
	return nil, nil
}

func (f *authTestFactory) NewClient(_ services.Credentials) (services.MinioClient, error) {
	return f.client, nil
}

type authTestMinioClient struct{}

func (m *authTestMinioClient) ListBuckets(_ context.Context) ([]minio.BucketInfo, error) {
	return []minio.BucketInfo{}, nil
}

func (m *authTestMinioClient) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) RemoveBucket(_ context.Context, _ string) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) ([]minio.ObjectInfo, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) ListObjectsPaginated(_ context.Context, _ string, _ services.ListObjectsOptions) (services.ListObjectsResult, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) ListObjectsChannel(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	panic("unexpected test call")
}

func (m *authTestMinioClient) PutObject(_ context.Context, _, _ string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetObject(_ context.Context, _, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetObjectReader(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, int64, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) RemoveObject(_ context.Context, _, _ string, _ minio.RemoveObjectOptions) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) PresignedGetObject(_ context.Context, _, _ string, _ time.Duration, _ url.Values) (*url.URL, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetBucketVersioning(_ context.Context, _ string) (minio.BucketVersioningConfiguration, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) SetBucketVersioning(_ context.Context, _ string, _ minio.BucketVersioningConfiguration) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetBucketLifecycle(_ context.Context, _ string) (*lifecycle.Configuration, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) SetBucketLifecycle(_ context.Context, _ string, _ *lifecycle.Configuration) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) StatObject(_ context.Context, _, _ string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetObjectTagging(_ context.Context, _, _ string, _ minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) PutObjectTagging(_ context.Context, _, _ string, _ *tags.Tags, _ minio.PutObjectTaggingOptions) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetBucketNotification(_ context.Context, _ string) (notification.Configuration, error) {
	panic("unexpected test call")
}

func (m *authTestMinioClient) SetBucketNotification(_ context.Context, _ string, _ notification.Configuration) error {
	panic("unexpected test call")
}

func (m *authTestMinioClient) GetBucketReplication(_ context.Context, _ string) (replication.Config, error) {
	panic("unexpected test call")
}

func TestLoginSetsSecureCookieOverTLS(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("accessKey", "test-access")
	form.Set("secretKey", "test-secret")
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	authService := services.NewAuthService()
	handler := NewAuthHandler(authService, &authTestFactory{client: &authTestMinioClient{}}, "play.min.io:9000")

	err := handler.Login(c)
	require.NoError(t, err)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			sessionCookie = cookie
			break
		}
	}

	require.NotNil(t, sessionCookie)
	assert.True(t, sessionCookie.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite)
	assert.True(t, sessionCookie.Secure)
	assert.Equal(t, "/", sessionCookie.Path)
}

func TestLogoutClearsCookieWithMatchingSecurityAttributes(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/logout", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := NewAuthHandler(services.NewAuthService(), &authTestFactory{client: &authTestMinioClient{}}, "play.min.io:9000")

	err := handler.Logout(c)
	require.NoError(t, err)

	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, cookie := range cookies {
		if cookie.Name == utils.CookieName {
			sessionCookie = cookie
			break
		}
	}

	require.NotNil(t, sessionCookie)
	assert.Equal(t, "", sessionCookie.Value)
	assert.Equal(t, -1, sessionCookie.MaxAge)
	assert.True(t, sessionCookie.HttpOnly)
	assert.Equal(t, http.SameSiteStrictMode, sessionCookie.SameSite)
	assert.True(t, sessionCookie.Secure)
	assert.Equal(t, "/", sessionCookie.Path)
}
