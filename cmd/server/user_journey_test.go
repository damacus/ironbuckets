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

func TestUserManagementJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{} // Add Renderer
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	// Pre-login the mock factory
	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewAdminClient", creds).Return(mockClient, nil)

	// Mock User Operations
	mockClient.On("AddUser", mock.Anything, "newuser", "secret123").Return(nil)
	mockClient.On("ListUsers", mock.Anything).Return(map[string]madmin.UserInfo{
		"existing": {Status: "enabled"},
		"newuser":  {Status: "enabled"},
	}, nil)
	mockClient.On("RemoveUser", mock.Anything, "newuser").Return(nil)
	// Mock group operations (called by ListUsers)
	mockClient.On("ListGroups", mock.Anything).Return([]string{}, nil)

	// Login to get cookie (Manual simulation)
	// reuse creds defined above
	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Protected Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))

	usersHandler := handlers.NewUsersHandler(mockFactory)
	app.GET("/users", usersHandler.ListUsers)
	app.POST("/users/create", usersHandler.CreateUser)
	app.POST("/users/delete", usersHandler.DeleteUser)

	// Step A: Create User
	form := make(url.Values)
	form.Set("accessKey", "newuser")
	form.Set("secretKey", "secret123")

	reqCreate := httptest.NewRequest(http.MethodPost, "/users/create", strings.NewReader(form.Encode()))
	reqCreate.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqCreate.AddCookie(cookie)
	recCreate := httptest.NewRecorder()

	e.ServeHTTP(recCreate, reqCreate)

	// EXPECT SUCCESS - Handler returns 200 OK with HX-Redirect header
	assert.Equal(t, http.StatusOK, recCreate.Code)
	assert.Equal(t, "/users", recCreate.Header().Get("HX-Redirect"))

	// Step B: List Users
	reqList := httptest.NewRequest(http.MethodGet, "/users", nil)
	reqList.AddCookie(cookie)
	recList := httptest.NewRecorder()

	e.ServeHTTP(recList, reqList)

	assert.Equal(t, http.StatusOK, recList.Code)

	// Step C: Delete User
	formDelete := make(url.Values)
	formDelete.Set("accessKey", "newuser")

	reqDelete := httptest.NewRequest(http.MethodPost, "/users/delete", strings.NewReader(formDelete.Encode()))
	reqDelete.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqDelete.AddCookie(cookie)
	recDelete := httptest.NewRecorder()

	e.ServeHTTP(recDelete, reqDelete)

	assert.Equal(t, http.StatusOK, recDelete.Code)
}

func TestUserJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	// Setup Mock Behavior
	minioEndpoint := "play.minio.io:9000"
	creds := services.Credentials{Endpoint: minioEndpoint, AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewClient", creds).Return(mockClient, nil)
	// Login uses ListBuckets to verify credentials
	mockClient.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{}, nil)

	authHandler := handlers.NewAuthHandler(authService, mockFactory, minioEndpoint, false)

	// Setup Routes
	e.POST("/login", authHandler.Login)
	e.GET("/logout", authHandler.Logout)

	// Protected Group
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	app.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "Dashboard")
	})

	// 2. Journey: Login -> Dashboard -> Logout

	// Step A: Login
	form := make(url.Values)
	form.Set("accessKey", "admin")
	form.Set("secretKey", "password")

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)

	// Extract Cookie
	cookies := rec.Result().Cookies()
	var ironSeal *http.Cookie
	for _, c := range cookies {
		if c.Name == "IronSeal" {
			ironSeal = c
			break
		}
	}
	assert.NotNil(t, ironSeal, "IronSeal cookie should be set")

	// Step B: Access Dashboard (Protected)
	reqDash := httptest.NewRequest(http.MethodGet, "/", nil)
	reqDash.AddCookie(ironSeal)
	recDash := httptest.NewRecorder()

	e.ServeHTTP(recDash, reqDash)

	assert.Equal(t, http.StatusOK, recDash.Code)
	assert.Equal(t, "Dashboard", recDash.Body.String())

	// Step C: Logout
	reqLogout := httptest.NewRequest(http.MethodGet, "/logout", nil)
	reqLogout.AddCookie(ironSeal)
	recLogout := httptest.NewRecorder()

	e.ServeHTTP(recLogout, reqLogout)

	assert.Equal(t, http.StatusSeeOther, recLogout.Code) // Redirects to login

	// Verify Cookie Cleared
	logoutCookies := recLogout.Result().Cookies()
	var clearedCookie *http.Cookie
	for _, c := range logoutCookies {
		if c.Name == "IronSeal" {
			clearedCookie = c
			break
		}
	}
	assert.NotNil(t, clearedCookie)
	assert.True(t, clearedCookie.Expires.Before(time.Now()))
}

func TestServiceAccountsJourney(t *testing.T) {
	// 1. Setup
	e := echo.New()
	e.Renderer = &MockRenderer{}
	authService := services.NewAuthService()
	mockFactory := new(MockMinioFactory)
	mockClient := new(MockMinioClient)

	creds := services.Credentials{Endpoint: "play.minio.io:9000", AccessKey: "admin", SecretKey: "password"}
	mockFactory.On("NewAdminClient", creds).Return(mockClient, nil)

	// Mock listing service accounts for a user
	mockClient.On("ListServiceAccounts", mock.Anything, "testuser").Return(madmin.ListServiceAccountsResp{
		Accounts: []madmin.ServiceAccountInfo{
			{AccessKey: "svc-key-1", ParentUser: "testuser", AccountStatus: "on", Name: "backup-key"},
			{AccessKey: "svc-key-2", ParentUser: "testuser", AccountStatus: "on", Name: "ci-key"},
		},
	}, nil)

	// Mock creating a service account
	mockClient.On("AddServiceAccount", mock.Anything, mock.MatchedBy(func(opts madmin.AddServiceAccountReq) bool {
		return opts.TargetUser == "testuser" && opts.Name == "new-key"
	})).Return(madmin.Credentials{
		AccessKey: "new-access-key",
		SecretKey: "new-secret-key",
	}, nil)

	// Mock deleting a service account
	mockClient.On("DeleteServiceAccount", mock.Anything, "svc-key-1").Return(nil)

	encrypted, _ := authService.EncryptCredentials(creds)
	cookie := &http.Cookie{Name: "IronSeal", Value: encrypted}

	// Routes
	app := e.Group("")
	app.Use(middleware.AuthMiddleware(authService))
	usersHandler := handlers.NewUsersHandler(mockFactory)
	app.GET("/users/:accessKey/keys", usersHandler.ListServiceAccounts)
	app.POST("/users/:accessKey/keys/create", usersHandler.CreateServiceAccount)
	app.POST("/users/:accessKey/keys/delete", usersHandler.DeleteServiceAccount)

	// Step A: List Service Accounts
	reqList := httptest.NewRequest(http.MethodGet, "/users/testuser/keys", nil)
	reqList.AddCookie(cookie)
	recList := httptest.NewRecorder()
	e.ServeHTTP(recList, reqList)
	assert.Equal(t, http.StatusOK, recList.Code)

	// Step B: Create Service Account
	form := make(url.Values)
	form.Set("name", "new-key")

	reqCreate := httptest.NewRequest(http.MethodPost, "/users/testuser/keys/create", strings.NewReader(form.Encode()))
	reqCreate.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqCreate.AddCookie(cookie)
	recCreate := httptest.NewRecorder()
	e.ServeHTTP(recCreate, reqCreate)
	assert.Equal(t, http.StatusOK, recCreate.Code)

	// Step C: Delete Service Account
	formDelete := make(url.Values)
	formDelete.Set("serviceAccountKey", "svc-key-1")

	reqDelete := httptest.NewRequest(http.MethodPost, "/users/testuser/keys/delete", strings.NewReader(formDelete.Encode()))
	reqDelete.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	reqDelete.AddCookie(cookie)
	recDelete := httptest.NewRecorder()
	e.ServeHTTP(recDelete, reqDelete)
	assert.Equal(t, http.StatusOK, recDelete.Code)
}
