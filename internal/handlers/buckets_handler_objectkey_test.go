package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net/url"
)

type objectKeyFactory struct {
	newClientCalls int
}

func (f *objectKeyFactory) NewAdminClient(_ services.Credentials) (services.MinioAdminClient, error) {
	panic("unexpected test call")
}

func (f *objectKeyFactory) NewClient(_ services.Credentials) (services.MinioClient, error) {
	f.newClientCalls++
	return &authTestMinioClient{}, nil
}

func TestDeleteObject_RequiresKey(t *testing.T) {
	e := echo.New()
	factory := &objectKeyFactory{}
	handler := NewBucketsHandler(factory)

	req := httptest.NewRequest(http.MethodPost, "/buckets/photos/delete", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.DeleteObject(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.Equal(t, "Object key is required", httpErr.Message)
	assert.Equal(t, 0, factory.newClientCalls)
}

func TestDownloadObject_RequiresKey(t *testing.T) {
	e := echo.New()
	factory := &objectKeyFactory{}
	handler := NewBucketsHandler(factory)

	req := httptest.NewRequest(http.MethodGet, "/buckets/photos/download", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.DownloadObject(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.Equal(t, "Object key is required", httpErr.Message)
	assert.Equal(t, 0, factory.newClientCalls)
}

func TestCreateFolder_RejectsInvalidFolderName(t *testing.T) {
	e := echo.New()
	factory := &objectKeyFactory{}
	handler := NewBucketsHandler(factory)

	req := httptest.NewRequest(http.MethodPost, "/buckets/photos/folder", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("bucketName")
	c.SetParamValues("photos")

	// Create a mock form to supply folderName
	c.Request().Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	form := url.Values{}
	form.Add("folderName", "../../secret")
	c.Request().Form = form

	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	err := handler.CreateFolder(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusBadRequest, httpErr.Code)
	assert.Equal(t, "Invalid folder name or prefix", httpErr.Message)
	assert.Equal(t, 0, factory.newClientCalls)
}
