package main

import (
	"log"
	"net/http"
	"os"

	"github.com/damacus/iron-buckets/internal/handlers"
	customMiddleware "github.com/damacus/iron-buckets/internal/middleware"
	"github.com/damacus/iron-buckets/internal/renderer"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

func main() {
	// Load MinIO endpoint from environment
	minioEndpoint := os.Getenv("MINIO_ENDPOINT")
	if minioEndpoint == "" {
		minioEndpoint = "play.min.io:9000" // Default for development
		log.Printf("MINIO_ENDPOINT not set, using default: %s", minioEndpoint)
	}

	e := newServer(minioEndpoint)

	// Start Server
	e.Logger.Fatal(e.Start(":8080"))
}

func newServer(minioEndpoint string) *echo.Echo {
	e := echo.New()

	// Services
	authService := services.NewAuthService()
	minioFactory := &services.RealMinioFactory{}
	authHandler := handlers.NewAuthHandler(authService, minioFactory, minioEndpoint)
	usersHandler := handlers.NewUsersHandler(minioFactory)
	groupsHandler := handlers.NewGroupsHandler(minioFactory)
	bucketsHandler := handlers.NewBucketsHandler(minioFactory)
	settingsHandler := handlers.NewSettingsHandler(minioFactory, minioEndpoint)
	drivesHandler := handlers.NewDrivesHandler(minioFactory)
	dashboardHandler := handlers.NewDashboardHandler(minioFactory)

	// Middleware
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus: true,
		LogURI:    true,
		LogValuesFunc: func(c echo.Context, v middleware.RequestLoggerValues) error {
			log.Printf("REQUEST: uri: %v, status: %v\n", v.URI, v.Status)
			return nil
		},
	}))
	e.Use(middleware.Recover())
	e.Use(customMiddleware.SecurityHeaders())
	e.Use(customMiddleware.CSRF())
	// Apply auth middleware globally - it will skip public routes internally
	e.Use(customMiddleware.AuthMiddleware(authService))

	// Template Renderer
	e.Renderer = renderer.New()

	// Public Routes (auth middleware will skip these)
	e.GET("/health", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})
	e.GET("/login", authHandler.LoginPage)
	e.POST("/login", authHandler.Login)
	e.GET("/login/oauth", authHandler.LoginOIDC)
	e.GET("/oauth/callback", authHandler.CallbackOIDC)
	e.GET("/logout", authHandler.Logout)

	// Protected Routes
	e.GET("/", func(c echo.Context) error {
		return c.Render(http.StatusOK, "dashboard", map[string]interface{}{
			"ActiveNav": "dashboard",
		})
	})
	e.GET("/drives", drivesHandler.ListDrives)
	e.GET("/api/server/widget", dashboardHandler.GetServerWidget)
	e.GET("/api/server/version", dashboardHandler.GetServerVersion)
	e.GET("/api/drives/widget", drivesHandler.GetDrivesWidget)
	e.GET("/api/storage/widget", dashboardHandler.GetStorageWidget)
	e.GET("/api/users/widget", dashboardHandler.GetUsersWidget)
	e.GET("/users", usersHandler.ListUsers)
	e.GET("/users/create", usersHandler.CreateUserModal)
	e.POST("/users/create", usersHandler.CreateUser)
	e.POST("/users/delete", usersHandler.DeleteUser)
	e.POST("/users/enable", usersHandler.EnableUser)
	e.POST("/users/disable", usersHandler.DisableUser)

	// Service Accounts
	e.GET("/users/:accessKey/keys", usersHandler.ListServiceAccounts)
	e.GET("/users/:accessKey/keys/create", usersHandler.CreateServiceAccountModal)
	e.POST("/users/:accessKey/keys/create", usersHandler.CreateServiceAccount)
	e.POST("/users/:accessKey/keys/delete", usersHandler.DeleteServiceAccount)

	// Policies
	e.GET("/policies", usersHandler.ListPolicies)
	e.GET("/users/:accessKey/policy/modal", usersHandler.PolicyModal)
	e.POST("/users/:accessKey/policy", usersHandler.AttachPolicy)

	// Groups
	e.GET("/groups", groupsHandler.ListGroups)
	e.GET("/groups/create", groupsHandler.CreateGroupModal)
	e.POST("/groups/create", groupsHandler.CreateGroup)
	e.GET("/groups/:groupName", groupsHandler.ViewGroup)
	e.POST("/groups/:groupName/members/add", groupsHandler.AddMembers)
	e.POST("/groups/:groupName/members/remove", groupsHandler.RemoveMembers)
	e.POST("/groups/:groupName/disable", groupsHandler.DisableGroup)
	e.POST("/groups/:groupName/enable", groupsHandler.EnableGroup)
	e.POST("/groups/:groupName/policy", groupsHandler.AttachPolicy)

	e.GET("/buckets", bucketsHandler.ListBuckets)
	e.GET("/buckets/create", bucketsHandler.CreateBucketModal)
	e.POST("/buckets/create", bucketsHandler.CreateBucket)
	e.POST("/buckets/delete", bucketsHandler.DeleteBucket)

	// Object Browser
	e.GET("/buckets/:bucketName", bucketsHandler.BrowseBucket)
	e.POST("/buckets/:bucketName/upload", bucketsHandler.UploadObject)
	e.POST("/buckets/:bucketName/delete", bucketsHandler.DeleteObject)
	e.GET("/buckets/:bucketName/download", bucketsHandler.DownloadObject)
	e.GET("/buckets/:bucketName/zip", bucketsHandler.DownloadZip)
	e.POST("/buckets/:bucketName/share", bucketsHandler.GenerateShareLink)
	e.GET("/buckets/:bucketName/folder/create", bucketsHandler.CreateFolderModal)
	e.POST("/buckets/:bucketName/folder/create", bucketsHandler.CreateFolder)
	e.POST("/buckets/:bucketName/folder/delete", bucketsHandler.DeleteFolder)

	// Bucket Settings
	e.GET("/buckets/:bucketName/settings", bucketsHandler.BucketSettings)
	e.GET("/buckets/:bucketName/versioning", bucketsHandler.GetVersioningStatus)
	e.POST("/buckets/:bucketName/versioning/enable", bucketsHandler.EnableVersioning)
	e.POST("/buckets/:bucketName/versioning/suspend", bucketsHandler.SuspendVersioning)
	e.GET("/buckets/:bucketName/lifecycle", bucketsHandler.GetLifecycleRules)
	e.POST("/buckets/:bucketName/lifecycle", bucketsHandler.AddLifecycleRule)
	e.POST("/buckets/:bucketName/lifecycle/delete", bucketsHandler.DeleteLifecycleRule)
	e.GET("/buckets/:bucketName/object/info", bucketsHandler.GetObjectInfo)
	e.POST("/buckets/:bucketName/object/tags", bucketsHandler.SetObjectTags)
	e.GET("/buckets/:bucketName/notifications", bucketsHandler.GetNotifications)
	e.GET("/buckets/:bucketName/replication", bucketsHandler.GetReplication)
	e.GET("/buckets/:bucketName/quota", bucketsHandler.GetBucketQuota)
	e.POST("/buckets/:bucketName/quota", bucketsHandler.SetBucketQuota)

	e.GET("/settings", settingsHandler.ShowSettings)
	e.POST("/settings/restart", settingsHandler.RestartService)
	e.GET("/settings/logs", settingsHandler.GetLogs)

	return e
}
