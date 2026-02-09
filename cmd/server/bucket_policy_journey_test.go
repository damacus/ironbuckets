package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/damacus/iron-buckets/internal/handlers"
	"github.com/damacus/iron-buckets/internal/middleware"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBucketPolicyJourney_GetBucketPolicyNoSuchPolicy(t *testing.T) {
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)
	mockClient.On("GetBucketPolicy", mock.Anything, "my-bucket").Return("", minio.ErrorResponse{
		Code:       "NoSuchBucketPolicy",
		StatusCode: http.StatusNotFound,
	})

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/policy", bucketsHandler.GetBucketPolicy)

	req := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/policy", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockClient.AssertExpectations(t)
}

func TestBucketPolicyJourney_SetBucketPolicyValidationError(t *testing.T) {
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.POST("/buckets/:bucketName/policy", bucketsHandler.SetBucketPolicy)

	form := make(url.Values)
	form.Set("policyType", "custom")
	form.Set("customPolicy", "{bad")

	req := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/policy", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	mockClient.AssertNotCalled(t, "SetBucketPolicy", mock.Anything, mock.Anything, mock.Anything)
}

func TestBucketPolicyJourney_SetBucketPolicySuccess(t *testing.T) {
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)
	mockClient.On("SetBucketPolicy", mock.Anything, "my-bucket", mock.MatchedBy(func(policy string) bool {
		return strings.Contains(policy, "\"s3:GetObject\"")
	})).Return(nil)
	mockClient.On("GetBucketPolicy", mock.Anything, "my-bucket").Return(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::my-bucket/*"]}]}`, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.POST("/buckets/:bucketName/policy", bucketsHandler.SetBucketPolicy)

	form := make(url.Values)
	form.Set("policyType", "public-read")

	req := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/policy", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockClient.AssertExpectations(t)
}

func TestBucketPolicyJourney_ListBucketsDoesNotFetchPolicyPerBucket(t *testing.T) {
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)
	mockFactory.On("NewAdminClient", creds).Return(mockClient, nil)
	mockClient.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{
		{Name: "bucket-1", CreationDate: time.Now()},
		{Name: "bucket-2", CreationDate: time.Now()},
	}, nil)
	mockClient.On("DataUsageInfo", mock.Anything).Return(madmin.DataUsageInfo{
		BucketSizes: map[string]uint64{"bucket-1": 100, "bucket-2": 200},
	}, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets", bucketsHandler.ListBuckets)

	req := httptest.NewRequest(http.MethodGet, "/buckets", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	mockClient.AssertNotCalled(t, "GetBucketPolicy", mock.Anything, mock.Anything)
}
