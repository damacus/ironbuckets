package main

import (
	"archive/zip"
	"bytes"
	"io"
	"mime/multipart"
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
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestBucketJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{} // Add Renderer
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	// Pre-login the mock factory
	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)
	mockFactory.On("NewAdminClient", creds).Return(mockClient, nil)

	// Mock Bucket Operations
	mockClient.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{
		{Name: "bucket-1", CreationDate: time.Now()},
		{Name: "bucket-2", CreationDate: time.Now()},
	}, nil)
	mockClient.On("DataUsageInfo", mock.Anything).Return(madmin.DataUsageInfo{
		BucketSizes: map[string]uint64{"bucket-1": 1024, "bucket-2": 2048},
	}, nil)
	mockClient.On("MakeBucket", mock.Anything, "newbucket", mock.Anything).Return(nil)
	mockClient.On("RemoveBucket", mock.Anything, "newbucket").Return(nil)

	// Login Cookie
	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets", bucketsHandler.ListBuckets)
	app.POST("/buckets/create", bucketsHandler.CreateBucket)
	app.POST("/buckets/delete", bucketsHandler.DeleteBucket)

	// Step A: List Buckets
	reqList := httptest.NewRequest(http.MethodGet, "/buckets", nil)
	reqList.AddCookie(cookie)
	recList := httptest.NewRecorder()

	e.ServeHTTP(recList, reqList)

	assert.Equal(t, http.StatusOK, recList.Code)

	// Step B: Create Bucket
	formCreate := make(url.Values)
	formCreate.Set("bucketName", "newbucket")

	reqCreate := httptest.NewRequest(http.MethodPost, "/buckets/create", strings.NewReader(formCreate.Encode()))
	reqCreate.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqCreate.AddCookie(cookie)
	recCreate := httptest.NewRecorder()

	e.ServeHTTP(recCreate, reqCreate)

	assert.Equal(t, http.StatusOK, recCreate.Code)
	assert.Equal(t, "/buckets", recCreate.Header().Get("HX-Redirect"))

	// Step C: Delete Bucket
	formDelete := make(url.Values)
	formDelete.Set("bucketName", "newbucket")

	reqDelete := httptest.NewRequest(http.MethodPost, "/buckets/delete", strings.NewReader(formDelete.Encode()))
	reqDelete.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqDelete.AddCookie(cookie)
	recDelete := httptest.NewRecorder()

	e.ServeHTTP(recDelete, reqDelete)

	assert.Equal(t, http.StatusOK, recDelete.Code)
}

func TestObjectBrowserJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock Object Operations
	mockClient.On("ListObjects", mock.Anything, "my-bucket", mock.Anything).Return([]minio.ObjectInfo{
		{Key: "file1.txt", Size: 123, LastModified: time.Now()},
	}, nil)
	mockClient.On("PutObject", mock.Anything, "my-bucket", "testfile.txt", mock.Anything, mock.Anything, mock.Anything).Return(minio.UploadInfo{}, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName", bucketsHandler.BrowseBucket)
	app.POST("/buckets/:bucketName/upload", bucketsHandler.UploadObject)

	// Step A: Browse Bucket
	reqBrowse := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket", nil)
	reqBrowse.AddCookie(cookie)
	recBrowse := httptest.NewRecorder()
	e.ServeHTTP(recBrowse, reqBrowse)
	assert.Equal(t, http.StatusOK, recBrowse.Code)

	// Step B: Upload Object
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "testfile.txt")
	_, _ = part.Write([]byte("content"))
	_ = writer.Close()

	reqUpload := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/upload", body)
	reqUpload.Header.Set(echo.HeaderContentType, writer.FormDataContentType())
	reqUpload.AddCookie(cookie)
	recUpload := httptest.NewRecorder()
	e.ServeHTTP(recUpload, reqUpload)

	// We now use HX-Redirect, which returns 200 OK
	assert.Equal(t, http.StatusOK, recUpload.Code)
	assert.Equal(t, "/buckets/my-bucket", recUpload.Header().Get("HX-Redirect"))
}

func TestZipDownloadJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock listing objects in a folder
	mockClient.On("ListObjects", mock.Anything, "my-bucket", mock.MatchedBy(func(opts minio.ListObjectsOptions) bool {
		return opts.Prefix == "folder/" && opts.Recursive == true
	})).Return([]minio.ObjectInfo{
		{Key: "folder/file1.txt", Size: 13, LastModified: time.Now()},
		{Key: "folder/file2.txt", Size: 13, LastModified: time.Now()},
		{Key: "folder/subfolder/file3.txt", Size: 17, LastModified: time.Now()},
	}, nil)

	// Mock getting each object's content
	mockClient.On("GetObjectReader", mock.Anything, "my-bucket", "folder/file1.txt", mock.Anything).
		Return(io.NopCloser(strings.NewReader("Hello, World!")), int64(13), nil)
	mockClient.On("GetObjectReader", mock.Anything, "my-bucket", "folder/file2.txt", mock.Anything).
		Return(io.NopCloser(strings.NewReader("Hello, World!")), int64(13), nil)
	mockClient.On("GetObjectReader", mock.Anything, "my-bucket", "folder/subfolder/file3.txt", mock.Anything).
		Return(io.NopCloser(strings.NewReader("Nested file here!")), int64(17), nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/zip", bucketsHandler.DownloadZip)

	// Step A: Download folder as ZIP
	reqZip := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/zip?prefix=folder/", nil)
	reqZip.AddCookie(cookie)
	recZip := httptest.NewRecorder()
	e.ServeHTTP(recZip, reqZip)

	assert.Equal(t, http.StatusOK, recZip.Code)
	assert.Equal(t, "application/zip", recZip.Header().Get("Content-Type"))
	assert.Contains(t, recZip.Header().Get("Content-Disposition"), "attachment")
	assert.Contains(t, recZip.Header().Get("Content-Disposition"), "folder.zip")

	// Verify ZIP contents
	zipReader, err := zip.NewReader(bytes.NewReader(recZip.Body.Bytes()), int64(recZip.Body.Len()))
	assert.NoError(t, err)
	assert.Len(t, zipReader.File, 3)

	// Verify file names in ZIP (should be relative to the prefix)
	fileNames := make(map[string]bool)
	for _, f := range zipReader.File {
		fileNames[f.Name] = true
	}
	assert.True(t, fileNames["file1.txt"])
	assert.True(t, fileNames["file2.txt"])
	assert.True(t, fileNames["subfolder/file3.txt"])
}

func TestPresignedURLJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock presigned URL generation
	expectedURL, _ := url.Parse("https://play.minio.io:9000/my-bucket/test-file.txt?X-Amz-Signature=abc123")
	mockClient.On("PresignedGetObject", mock.Anything, "my-bucket", "test-file.txt", mock.Anything, mock.Anything).
		Return(expectedURL, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.POST("/buckets/:bucketName/share", bucketsHandler.GenerateShareLink)

	// Generate Share Link
	form := make(url.Values)
	form.Set("key", "test-file.txt")
	form.Set("expires", "3600") // 1 hour

	reqShare := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/share", strings.NewReader(form.Encode()))
	reqShare.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqShare.AddCookie(cookie)
	recShare := httptest.NewRecorder()
	e.ServeHTTP(recShare, reqShare)

	assert.Equal(t, http.StatusOK, recShare.Code)
}

func TestBucketLifecycleJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock get lifecycle (initially empty)
	mockClient.On("GetBucketLifecycle", mock.Anything, "my-bucket").Return(&lifecycle.Configuration{
		Rules: []lifecycle.Rule{},
	}, nil).Once()

	// Mock set lifecycle (add expiration rule)
	mockClient.On("SetBucketLifecycle", mock.Anything, "my-bucket", mock.MatchedBy(func(cfg *lifecycle.Configuration) bool {
		return len(cfg.Rules) > 0 && cfg.Rules[0].ID == "expire-old-files"
	})).Return(nil)

	// Mock get lifecycle (after adding rule)
	mockClient.On("GetBucketLifecycle", mock.Anything, "my-bucket").Return(&lifecycle.Configuration{
		Rules: []lifecycle.Rule{
			{
				ID:     "expire-old-files",
				Status: "Enabled",
				Expiration: lifecycle.Expiration{
					Days: 30,
				},
			},
		},
	}, nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/lifecycle", bucketsHandler.GetLifecycleRules)
	app.POST("/buckets/:bucketName/lifecycle", bucketsHandler.AddLifecycleRule)
	app.POST("/buckets/:bucketName/lifecycle/delete", bucketsHandler.DeleteLifecycleRule)

	// Step A: Get Lifecycle Rules (empty)
	reqGet := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/lifecycle", nil)
	reqGet.AddCookie(cookie)
	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, reqGet)
	assert.Equal(t, http.StatusOK, recGet.Code)

	// Step B: Add Lifecycle Rule
	form := make(url.Values)
	form.Set("id", "expire-old-files")
	form.Set("prefix", "logs/")
	form.Set("expirationDays", "30")

	reqAdd := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/lifecycle", strings.NewReader(form.Encode()))
	reqAdd.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqAdd.AddCookie(cookie)
	recAdd := httptest.NewRecorder()
	e.ServeHTTP(recAdd, reqAdd)
	assert.Equal(t, http.StatusOK, recAdd.Code)
}

func TestObjectMetadataJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock StatObject
	mockClient.On("StatObject", mock.Anything, "my-bucket", "test-file.txt", mock.Anything).Return(minio.ObjectInfo{
		Key:         "test-file.txt",
		Size:        1024,
		ContentType: "text/plain",
		ETag:        "abc123",
		UserMetadata: map[string]string{
			"Author": "test-user",
		},
	}, nil)

	// Mock GetObjectTagging
	testTags, _ := tags.NewTags(map[string]string{"env": "production", "team": "backend"}, false)
	mockClient.On("GetObjectTagging", mock.Anything, "my-bucket", "test-file.txt", mock.Anything).Return(testTags, nil)

	// Mock PutObjectTagging
	mockClient.On("PutObjectTagging", mock.Anything, "my-bucket", "test-file.txt", mock.Anything, mock.Anything).Return(nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/object/info", bucketsHandler.GetObjectInfo)
	app.POST("/buckets/:bucketName/object/tags", bucketsHandler.SetObjectTags)

	// Step A: Get Object Info
	reqInfo := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/object/info?key=test-file.txt", nil)
	reqInfo.AddCookie(cookie)
	recInfo := httptest.NewRecorder()
	e.ServeHTTP(recInfo, reqInfo)
	assert.Equal(t, http.StatusOK, recInfo.Code)

	// Step B: Set Object Tags
	form := make(url.Values)
	form.Set("key", "test-file.txt")
	form.Set("tags", "env=staging,team=frontend")

	reqTags := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/object/tags", strings.NewReader(form.Encode()))
	reqTags.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqTags.AddCookie(cookie)
	recTags := httptest.NewRecorder()
	e.ServeHTTP(recTags, reqTags)
	assert.Equal(t, http.StatusOK, recTags.Code)
}

func TestBucketNotificationsJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock GetBucketNotification (empty)
	mockClient.On("GetBucketNotification", mock.Anything, "my-bucket").Return(notification.Configuration{}, nil)

	// Mock SetBucketNotification
	mockClient.On("SetBucketNotification", mock.Anything, "my-bucket", mock.Anything).Return(nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/notifications", bucketsHandler.GetNotifications)

	// Step A: Get Notifications
	reqGet := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/notifications", nil)
	reqGet.AddCookie(cookie)
	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, reqGet)
	assert.Equal(t, http.StatusOK, recGet.Code)
}

func TestBucketVersioningJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)

	// Mock get versioning status (initially disabled)
	mockClient.On("GetBucketVersioning", mock.Anything, "my-bucket").Return(minio.BucketVersioningConfiguration{
		Status: "",
	}, nil).Once()

	// Mock enable versioning
	mockClient.On("SetBucketVersioning", mock.Anything, "my-bucket", mock.MatchedBy(func(cfg minio.BucketVersioningConfiguration) bool {
		return cfg.Status == "Enabled"
	})).Return(nil)

	// Mock get versioning status (after enabling)
	mockClient.On("GetBucketVersioning", mock.Anything, "my-bucket").Return(minio.BucketVersioningConfiguration{
		Status: "Enabled",
	}, nil).Once()

	// Mock suspend versioning
	mockClient.On("SetBucketVersioning", mock.Anything, "my-bucket", mock.MatchedBy(func(cfg minio.BucketVersioningConfiguration) bool {
		return cfg.Status == "Suspended"
	})).Return(nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	bucketsHandler := handlers.NewBucketsHandler(mockFactory)
	app.GET("/buckets/:bucketName/versioning", bucketsHandler.GetVersioningStatus)
	app.POST("/buckets/:bucketName/versioning/enable", bucketsHandler.EnableVersioning)
	app.POST("/buckets/:bucketName/versioning/suspend", bucketsHandler.SuspendVersioning)

	// Step A: Get Versioning Status
	reqGet := httptest.NewRequest(http.MethodGet, "/buckets/my-bucket/versioning", nil)
	reqGet.AddCookie(cookie)
	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, reqGet)
	assert.Equal(t, http.StatusOK, recGet.Code)

	// Step B: Enable Versioning
	reqEnable := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/versioning/enable", nil)
	reqEnable.AddCookie(cookie)
	recEnable := httptest.NewRecorder()
	e.ServeHTTP(recEnable, reqEnable)
	assert.Equal(t, http.StatusOK, recEnable.Code)

	// Step C: Suspend Versioning
	reqSuspend := httptest.NewRequest(http.MethodPost, "/buckets/my-bucket/versioning/suspend", nil)
	reqSuspend.AddCookie(cookie)
	recSuspend := httptest.NewRecorder()
	e.ServeHTTP(recSuspend, reqSuspend)
	assert.Equal(t, http.StatusOK, recSuspend.Code)
}
