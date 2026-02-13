package handlers

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
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

type uploadTestMinioClient struct {
	putObjectCalls int
	lastObjectKey  string
}

func (m *uploadTestMinioClient) ListBuckets(_ context.Context) ([]minio.BucketInfo, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) MakeBucket(_ context.Context, _ string, _ minio.MakeBucketOptions) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) RemoveBucket(_ context.Context, _ string) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) ListObjects(_ context.Context, _ string, _ minio.ListObjectsOptions) ([]minio.ObjectInfo, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) ListObjectsPaginated(_ context.Context, _ string, _ services.ListObjectsOptions) (services.ListObjectsResult, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) ListObjectsChannel(_ context.Context, _ string, _ minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) PutObject(_ context.Context, _ string, objectName string, _ io.Reader, _ int64, _ minio.PutObjectOptions) (minio.UploadInfo, error) {
	m.putObjectCalls++
	m.lastObjectKey = objectName
	return minio.UploadInfo{}, nil
}

func (m *uploadTestMinioClient) GetObject(_ context.Context, _, _ string, _ minio.GetObjectOptions) (*minio.Object, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetObjectReader(_ context.Context, _, _ string, _ minio.GetObjectOptions) (io.ReadCloser, int64, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) RemoveObject(_ context.Context, _, _ string, _ minio.RemoveObjectOptions) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) PresignedGetObject(_ context.Context, _, _ string, _ time.Duration, _ url.Values) (*url.URL, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetBucketVersioning(_ context.Context, _ string) (minio.BucketVersioningConfiguration, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) SetBucketVersioning(_ context.Context, _ string, _ minio.BucketVersioningConfiguration) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetBucketLifecycle(_ context.Context, _ string) (*lifecycle.Configuration, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) SetBucketLifecycle(_ context.Context, _ string, _ *lifecycle.Configuration) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) StatObject(_ context.Context, _, _ string, _ minio.StatObjectOptions) (minio.ObjectInfo, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetObjectTagging(_ context.Context, _, _ string, _ minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) PutObjectTagging(_ context.Context, _, _ string, _ *tags.Tags, _ minio.PutObjectTaggingOptions) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetBucketNotification(_ context.Context, _ string) (notification.Configuration, error) {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) SetBucketNotification(_ context.Context, _ string, _ notification.Configuration) error {
	panic("unexpected test call")
}

func (m *uploadTestMinioClient) GetBucketReplication(_ context.Context, _ string) (replication.Config, error) {
	panic("unexpected test call")
}

type uploadTestFactory struct {
	client services.MinioClient
}

func (f *uploadTestFactory) NewAdminClient(_ services.Credentials) (services.MinioAdminClient, error) {
	panic("unexpected test call")
}

func (f *uploadTestFactory) NewClient(_ services.Credentials) (services.MinioClient, error) {
	return f.client, nil
}

func TestUploadObject_UsesOriginalFilenameWhenAlreadySafe(t *testing.T) {
	e := echo.New()
	client := &uploadTestMinioClient{}
	handler := NewBucketsHandler(&uploadTestFactory{client: client})

	req, rec := newUploadRequest(t, "report.txt")
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.UploadObject(c)
	require.NoError(t, err)

	assert.Equal(t, 1, client.putObjectCalls)
	assert.Equal(t, "docs/report.txt", client.lastObjectKey)
	assert.Equal(t, "/buckets/photos?prefix=docs/", rec.Header().Get("HX-Redirect"))
}

func TestUploadObject_SanitizesNestedFilenameToBaseName(t *testing.T) {
	e := echo.New()
	client := &uploadTestMinioClient{}
	handler := NewBucketsHandler(&uploadTestFactory{client: client})

	req, rec := newUploadRequest(t, "nested/report.txt")
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.UploadObject(c)
	require.NoError(t, err)

	assert.Equal(t, 1, client.putObjectCalls)
	assert.Equal(t, "docs/report.txt", client.lastObjectKey)
}

func TestUploadObject_RejectsInvalidFilename(t *testing.T) {
	e := echo.New()
	client := &uploadTestMinioClient{}
	handler := NewBucketsHandler(&uploadTestFactory{client: client})

	req, rec := newUploadRequest(t, ".")
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.UploadObject(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.Equal(t, 0, client.putObjectCalls)
	assert.Empty(t, rec.Header().Get("HX-Redirect"))
}

func newUploadRequest(t *testing.T, filename string) (*http.Request, *httptest.ResponseRecorder) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = part.Write([]byte("test data"))
	require.NoError(t, err)
	require.NoError(t, writer.Close())

	req := httptest.NewRequest(http.MethodPost, "/buckets/photos/upload?prefix=docs/", body)
	req.Header.Set(echo.HeaderContentType, writer.FormDataContentType())

	return req, httptest.NewRecorder()
}
