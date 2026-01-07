package handlers

import (
	"archive/zip"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/damacus/iron-buckets/internal/models"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/replication"
	"github.com/minio/minio-go/v7/pkg/tags"
)

type BucketsHandler struct {
	minioFactory services.MinioClientFactory
}

func NewBucketsHandler(minioFactory services.MinioClientFactory) *BucketsHandler {
	return &BucketsHandler{minioFactory: minioFactory}
}

// ListBuckets renders the buckets page
func (h *BucketsHandler) ListBuckets(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	// Connect to MinIO (Standard Client)
	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Fetch Buckets
	buckets, err := client.ListBuckets(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list buckets")
	}

	// Fetch Data Usage for sizes
	mdm, err := h.minioFactory.NewAdminClient(*creds)
	var usage madmin.DataUsageInfo
	if err == nil {
		usage, _ = mdm.DataUsageInfo(c.Request().Context())
	}

	type BucketWithStats struct {
		minio.BucketInfo
		Size          uint64
		FormattedSize string
	}

	var bucketsWithStats []BucketWithStats
	for _, b := range buckets {
		size := uint64(0)
		if usage.BucketSizes != nil {
			size = usage.BucketSizes[b.Name]
		}
		bucketsWithStats = append(bucketsWithStats, BucketWithStats{
			BucketInfo:    b,
			Size:          size,
			FormattedSize: utils.FormatBytes(size),
		})
	}

	return c.Render(http.StatusOK, "buckets", map[string]interface{}{
		"ActiveNav": "buckets",
		"Buckets":   bucketsWithStats,
	})
}

// CreateBucketModal renders the bucket creation modal
func (h *BucketsHandler) CreateBucketModal(c echo.Context) error {
	return c.Render(http.StatusOK, "bucket_create_modal", nil)
}

// CreateBucket handles the creation of a new bucket
func (h *BucketsHandler) CreateBucket(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return c.Render(http.StatusBadRequest, "bucket_create_modal", map[string]interface{}{
			"Error": "Authentication required",
		})
	}

	bucketName := c.FormValue("bucketName")

	// Validate bucket name
	if bucketName == "" {
		return c.Render(http.StatusBadRequest, "bucket_create_modal", map[string]interface{}{
			"Error": "Bucket name is required",
		})
	}
	if len(bucketName) < 3 || len(bucketName) > 63 {
		return c.Render(http.StatusBadRequest, "bucket_create_modal", map[string]interface{}{
			"Error": "Bucket name must be between 3 and 63 characters",
		})
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return c.Render(http.StatusInternalServerError, "bucket_create_modal", map[string]interface{}{
			"Error": "Failed to connect to MinIO",
		})
	}

	// Create Bucket
	region := c.FormValue("region")
	opts := minio.MakeBucketOptions{
		Region: region,
	}
	if err := client.MakeBucket(c.Request().Context(), bucketName, opts); err != nil {
		return c.Render(http.StatusBadRequest, "bucket_create_modal", map[string]interface{}{
			"Error": "Failed to create bucket: " + err.Error(),
		})
	}

	// Success - close modal and refresh page
	return HTMXRedirect(c, "/buckets")
}

// DeleteBucket handles removing a bucket
func (h *BucketsHandler) DeleteBucket(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.FormValue("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := client.RemoveBucket(c.Request().Context(), bucketName); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete bucket")
	}

	return c.NoContent(http.StatusOK)
}

// BrowseBucket renders the object browser with folder support
func (h *BucketsHandler) BrowseBucket(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// List objects with prefix for folder support
	rawObjects, err := client.ListObjects(c.Request().Context(), bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false, // Non-recursive to get folders
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list objects")
	}

	var objects []models.ObjectInfo
	var folders []models.FolderInfo
	seenFolders := make(map[string]bool)

	for _, obj := range rawObjects {
		// Check if it's a folder (ends with /)
		if strings.HasSuffix(obj.Key, "/") {
			folderName := strings.TrimPrefix(obj.Key, prefix)
			folderName = strings.TrimSuffix(folderName, "/")
			if folderName != "" && !seenFolders[folderName] {
				seenFolders[folderName] = true
				folders = append(folders, models.FolderInfo{
					Name:   folderName,
					Prefix: obj.Key,
				})
			}
			continue
		}

		// It's a file
		displayName := strings.TrimPrefix(obj.Key, prefix)
		contentType := obj.ContentType
		if contentType == "" {
			contentType = getContentTypeFromExt(obj.Key)
		}

		objects = append(objects, models.ObjectInfo{
			Key:           obj.Key,
			DisplayName:   displayName,
			Size:          obj.Size,
			FormattedSize: utils.FormatFileSize(obj.Size),
			LastModified:  obj.LastModified,
			ContentType:   contentType,
			IsImage:       isImageType(contentType),
			IsText:        isTextType(contentType),
			IsVideo:       isVideoType(contentType),
			IsArchive:     isArchiveType(contentType, obj.Key),
			IsPreviewable: isPreviewable(contentType, obj.Size),
		})
	}

	// Build breadcrumbs
	var breadcrumbs []models.Breadcrumb
	if prefix != "" {
		parts := strings.Split(strings.TrimSuffix(prefix, "/"), "/")
		path := ""
		for _, part := range parts {
			if part == "" {
				continue
			}
			path += part + "/"
			breadcrumbs = append(breadcrumbs, models.Breadcrumb{
				Name: part,
				Path: path,
			})
		}
	}

	return c.Render(http.StatusOK, "browser", map[string]interface{}{
		"ActiveNav":   "buckets",
		"BucketName":  bucketName,
		"Prefix":      prefix,
		"Objects":     objects,
		"Folders":     folders,
		"Breadcrumbs": breadcrumbs,
	})
}

// UploadObject handles file upload
func (h *BucketsHandler) UploadObject(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")
	file, err := c.FormFile("file")
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "No file uploaded")
	}

	src, err := file.Open()
	if err != nil {
		return err
	}
	defer func() { _ = src.Close() }()

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Put Object with prefix support
	objectKey := prefix + file.Filename
	_, err = client.PutObject(c.Request().Context(), bucketName, objectKey, src, file.Size, minio.PutObjectOptions{
		ContentType: file.Header.Get("Content-Type"),
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to upload object: "+err.Error())
	}

	// Redirect back to the current folder
	redirectURL := "/buckets/" + bucketName
	if prefix != "" {
		redirectURL += "?prefix=" + prefix
	}
	return HTMXRedirect(c, redirectURL)
}

// DeleteObject handles deleting an object
func (h *BucketsHandler) DeleteObject(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	objectName := c.QueryParam("key")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := client.RemoveObject(c.Request().Context(), bucketName, objectName, minio.RemoveObjectOptions{}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete object")
	}

	return c.NoContent(http.StatusOK) // Row disappears
}

// DownloadObject handles file download
func (h *BucketsHandler) DownloadObject(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	objectName := c.QueryParam("key")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	obj, err := client.GetObject(c.Request().Context(), bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get object")
	}

	// Stat to get info
	info, err := obj.Stat()
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Object not found")
	}

	c.Response().Header().Set(echo.HeaderContentDisposition, "attachment; filename="+objectName)
	c.Response().Header().Set(echo.HeaderContentType, info.ContentType)
	c.Response().Header().Set(echo.HeaderContentLength, strconv.FormatInt(info.Size, 10))

	return c.Stream(http.StatusOK, info.ContentType, obj)
}

// CreateFolderModal shows the folder creation modal
func (h *BucketsHandler) CreateFolderModal(c echo.Context) error {
	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")
	return c.Render(http.StatusOK, "folder_create_modal", map[string]interface{}{
		"BucketName": bucketName,
		"Prefix":     prefix,
	})
}

// CreateFolder creates a new folder (empty object with trailing /)
func (h *BucketsHandler) CreateFolder(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")
	folderName := c.FormValue("folderName")

	if folderName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Folder name is required")
	}

	// Ensure folder name ends with /
	if !strings.HasSuffix(folderName, "/") {
		folderName += "/"
	}

	objectKey := prefix + folderName

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Create empty object with trailing slash to represent folder
	_, err = client.PutObject(c.Request().Context(), bucketName, objectKey, strings.NewReader(""), 0, minio.PutObjectOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create folder: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"?prefix="+prefix)
}

// DeleteFolder deletes a folder and all its contents
func (h *BucketsHandler) DeleteFolder(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// List all objects with this prefix
	objectsList, err := client.ListObjects(c.Request().Context(), bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list objects")
	}

	// Delete all objects
	for _, obj := range objectsList {
		err := client.RemoveObject(c.Request().Context(), bucketName, obj.Key, minio.RemoveObjectOptions{})
		if err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete object: "+obj.Key)
		}
	}

	return c.NoContent(http.StatusOK)
}

// DownloadZip streams a folder as a ZIP archive
func (h *BucketsHandler) DownloadZip(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	prefix := c.QueryParam("prefix")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// List all objects with this prefix recursively
	objectsList, err := client.ListObjects(c.Request().Context(), bucketName, minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: true,
	})
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list objects")
	}

	// Filter out folder markers (objects ending with /)
	var files []minio.ObjectInfo
	for _, obj := range objectsList {
		if !strings.HasSuffix(obj.Key, "/") {
			files = append(files, obj)
		}
	}

	if len(files) == 0 {
		return echo.NewHTTPError(http.StatusNotFound, "No files to download")
	}

	// Determine ZIP filename from prefix or bucket name
	zipName := bucketName + ".zip"
	if prefix != "" {
		// Remove trailing slash and get the folder name
		folderName := strings.TrimSuffix(prefix, "/")
		if idx := strings.LastIndex(folderName, "/"); idx >= 0 {
			folderName = folderName[idx+1:]
		}
		zipName = folderName + ".zip"
	}

	// Set response headers for streaming ZIP
	c.Response().Header().Set(echo.HeaderContentType, "application/zip")
	c.Response().Header().Set(echo.HeaderContentDisposition, fmt.Sprintf("attachment; filename=%q", zipName))
	c.Response().WriteHeader(http.StatusOK)

	// Create ZIP writer
	zipWriter := zip.NewWriter(c.Response().Writer)
	defer func() { _ = zipWriter.Close() }()

	// Add each file to the ZIP
	for _, obj := range files {
		// Get the file content
		reader, _, err := client.GetObjectReader(c.Request().Context(), bucketName, obj.Key, minio.GetObjectOptions{})
		if err != nil {
			// Log error but continue with other files
			continue
		}

		// Create file in ZIP with path relative to prefix
		relativePath := strings.TrimPrefix(obj.Key, prefix)
		writer, err := zipWriter.Create(relativePath)
		if err != nil {
			_ = reader.Close()
			continue
		}

		// Copy content
		_, err = io.Copy(writer, reader)
		_ = reader.Close()
		if err != nil {
			continue
		}
	}

	return nil
}

// GenerateShareLink creates a presigned URL for sharing an object
func (h *BucketsHandler) GenerateShareLink(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	objectKey := c.FormValue("key")
	expiresStr := c.FormValue("expires") // seconds

	// Parse expiration (default 1 hour, max 7 days)
	expires := time.Hour
	if expiresStr != "" {
		expiresSeconds, err := strconv.ParseInt(expiresStr, 10, 64)
		if err == nil && expiresSeconds > 0 {
			expires = time.Duration(expiresSeconds) * time.Second
			// Cap at 7 days
			if expires > 7*24*time.Hour {
				expires = 7 * 24 * time.Hour
			}
		}
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	presignedURL, err := client.PresignedGetObject(c.Request().Context(), bucketName, objectKey, expires, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to generate share link: "+err.Error())
	}

	// Format expiration for display
	expiresAt := time.Now().Add(expires)
	expiresDisplay := formatExpiration(expires)

	return c.Render(http.StatusOK, "share_link", map[string]interface{}{
		"URL":            presignedURL.String(),
		"BucketName":     bucketName,
		"ObjectKey":      objectKey,
		"ExpiresAt":      expiresAt.Format("Jan 02, 2006 15:04 MST"),
		"ExpiresDisplay": expiresDisplay,
	})
}

// BucketSettings renders the bucket settings page
func (h *BucketsHandler) BucketSettings(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Get versioning status
	versioningConfig, err := client.GetBucketVersioning(c.Request().Context(), bucketName)
	versioningStatus := "Disabled"
	if err == nil {
		if versioningConfig.Enabled() {
			versioningStatus = "Enabled"
		} else if versioningConfig.Suspended() {
			versioningStatus = "Suspended"
		}
	}

	return c.Render(http.StatusOK, "bucket_settings", map[string]interface{}{
		"ActiveNav":         "buckets",
		"BucketName":        bucketName,
		"VersioningStatus":  versioningStatus,
		"VersioningEnabled": versioningConfig.Enabled(),
	})
}

// GetVersioningStatus returns the versioning status for a bucket
func (h *BucketsHandler) GetVersioningStatus(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config, err := client.GetBucketVersioning(c.Request().Context(), bucketName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get versioning status: "+err.Error())
	}

	status := "Disabled"
	if config.Enabled() {
		status = "Enabled"
	} else if config.Suspended() {
		status = "Suspended"
	}

	return c.Render(http.StatusOK, "versioning_status", map[string]interface{}{
		"BucketName": bucketName,
		"Status":     status,
		"Enabled":    config.Enabled(),
		"Suspended":  config.Suspended(),
	})
}

// EnableVersioning enables versioning for a bucket
func (h *BucketsHandler) EnableVersioning(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config := minio.BucketVersioningConfiguration{
		Status: "Enabled",
	}

	if err := client.SetBucketVersioning(c.Request().Context(), bucketName, config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enable versioning: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"/settings")
}

// SuspendVersioning suspends versioning for a bucket
func (h *BucketsHandler) SuspendVersioning(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config := minio.BucketVersioningConfiguration{
		Status: "Suspended",
	}

	if err := client.SetBucketVersioning(c.Request().Context(), bucketName, config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to suspend versioning: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"/settings")
}

// GetLifecycleRules returns the lifecycle rules for a bucket
func (h *BucketsHandler) GetLifecycleRules(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config, err := client.GetBucketLifecycle(c.Request().Context(), bucketName)
	if err != nil {
		// No lifecycle config is not an error, just return empty
		config = &lifecycle.Configuration{Rules: []lifecycle.Rule{}}
	}

	// Transform rules for display
	var rules []map[string]interface{}
	for _, rule := range config.Rules {
		ruleData := map[string]interface{}{
			"ID":     rule.ID,
			"Status": rule.Status,
			"Prefix": rule.Prefix,
		}

		if rule.Expiration.Days > 0 {
			ruleData["ExpirationDays"] = rule.Expiration.Days
			ruleData["Action"] = fmt.Sprintf("Delete after %d days", rule.Expiration.Days)
		} else if !rule.Expiration.Date.IsZero() {
			ruleData["ExpirationDate"] = rule.Expiration.Date.Format("2006-01-02")
			ruleData["Action"] = fmt.Sprintf("Delete on %s", rule.Expiration.Date.Format("Jan 02, 2006"))
		}

		if rule.NoncurrentVersionExpiration.NoncurrentDays > 0 {
			ruleData["NoncurrentDays"] = rule.NoncurrentVersionExpiration.NoncurrentDays
		}

		rules = append(rules, ruleData)
	}

	return c.Render(http.StatusOK, "lifecycle_rules", map[string]interface{}{
		"BucketName": bucketName,
		"Rules":      rules,
		"HasRules":   len(rules) > 0,
	})
}

// AddLifecycleRule adds a new lifecycle rule to a bucket
func (h *BucketsHandler) AddLifecycleRule(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	ruleID := c.FormValue("id")
	prefix := c.FormValue("prefix")
	expirationDaysStr := c.FormValue("expirationDays")

	if ruleID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Rule ID is required")
	}

	expirationDays, err := strconv.Atoi(expirationDaysStr)
	if err != nil || expirationDays <= 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid expiration days")
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Get existing config
	config, err := client.GetBucketLifecycle(c.Request().Context(), bucketName)
	if err != nil {
		config = &lifecycle.Configuration{Rules: []lifecycle.Rule{}}
	}

	// Add new rule
	newRule := lifecycle.Rule{
		ID:     ruleID,
		Status: "Enabled",
		Expiration: lifecycle.Expiration{
			Days: lifecycle.ExpirationDays(expirationDays),
		},
	}

	if prefix != "" {
		newRule.RuleFilter = lifecycle.Filter{
			Prefix: prefix,
		}
	}

	config.Rules = append(config.Rules, newRule)

	if err := client.SetBucketLifecycle(c.Request().Context(), bucketName, config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to add lifecycle rule: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"/settings")
}

// DeleteLifecycleRule removes a lifecycle rule from a bucket
func (h *BucketsHandler) DeleteLifecycleRule(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	ruleID := c.FormValue("ruleId")

	if ruleID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Rule ID is required")
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Get existing config
	config, err := client.GetBucketLifecycle(c.Request().Context(), bucketName)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to get lifecycle config")
	}

	// Remove the rule
	var newRules []lifecycle.Rule
	for _, rule := range config.Rules {
		if rule.ID != ruleID {
			newRules = append(newRules, rule)
		}
	}
	config.Rules = newRules

	if err := client.SetBucketLifecycle(c.Request().Context(), bucketName, config); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete lifecycle rule: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"/settings")
}

// GetObjectInfo returns metadata and tags for an object
func (h *BucketsHandler) GetObjectInfo(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	objectKey := c.QueryParam("key")

	if objectKey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Object key is required")
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Get object info
	objInfo, err := client.StatObject(c.Request().Context(), bucketName, objectKey, minio.StatObjectOptions{})
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "Object not found: "+err.Error())
	}

	// Get object tags
	objTags, err := client.GetObjectTagging(c.Request().Context(), bucketName, objectKey, minio.GetObjectTaggingOptions{})
	var tagsMap map[string]string
	if err == nil && objTags != nil {
		tagsMap = objTags.ToMap()
	}

	// Build metadata list
	var metadata []map[string]string
	for key, value := range objInfo.UserMetadata {
		metadata = append(metadata, map[string]string{"Key": key, "Value": value})
	}

	// Build tags list
	var tagsList []map[string]string
	for key, value := range tagsMap {
		tagsList = append(tagsList, map[string]string{"Key": key, "Value": value})
	}

	return c.Render(http.StatusOK, "object_info", map[string]interface{}{
		"BucketName":   bucketName,
		"ObjectKey":    objectKey,
		"Size":         utils.FormatFileSize(objInfo.Size),
		"ContentType":  objInfo.ContentType,
		"ETag":         objInfo.ETag,
		"LastModified": objInfo.LastModified.Format("Jan 02, 2006 15:04 MST"),
		"Metadata":     metadata,
		"Tags":         tagsList,
		"HasMetadata":  len(metadata) > 0,
		"HasTags":      len(tagsList) > 0,
	})
}

// SetObjectTags sets tags on an object
func (h *BucketsHandler) SetObjectTags(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	objectKey := c.FormValue("key")
	tagsStr := c.FormValue("tags")

	if objectKey == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "Object key is required")
	}

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Parse tags from "key1=value1,key2=value2" format
	tagsMap := make(map[string]string)
	if tagsStr != "" {
		pairs := strings.Split(tagsStr, ",")
		for _, pair := range pairs {
			kv := strings.SplitN(strings.TrimSpace(pair), "=", 2)
			if len(kv) == 2 {
				tagsMap[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
			}
		}
	}

	objTags, err := tags.NewTags(tagsMap, false)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "Invalid tags format: "+err.Error())
	}

	if err := client.PutObjectTagging(c.Request().Context(), bucketName, objectKey, objTags, minio.PutObjectTaggingOptions{}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to set tags: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"?key="+objectKey)
}

// GetNotifications returns the notification configuration for a bucket
func (h *BucketsHandler) GetNotifications(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config, err := client.GetBucketNotification(c.Request().Context(), bucketName)
	if err != nil {
		// No notification config is not an error
		config = notification.Configuration{}
	}

	// Transform configs for display
	var notifications []map[string]interface{}

	for _, qc := range config.QueueConfigs {
		notifications = append(notifications, map[string]interface{}{
			"Type":   "Queue",
			"ARN":    qc.Queue,
			"Events": qc.Events,
			"ID":     qc.ID,
		})
	}

	for _, tc := range config.TopicConfigs {
		notifications = append(notifications, map[string]interface{}{
			"Type":   "Topic",
			"ARN":    tc.Topic,
			"Events": tc.Events,
			"ID":     tc.ID,
		})
	}

	for _, lc := range config.LambdaConfigs {
		notifications = append(notifications, map[string]interface{}{
			"Type":   "Lambda",
			"ARN":    lc.Lambda,
			"Events": lc.Events,
			"ID":     lc.ID,
		})
	}

	return c.Render(http.StatusOK, "notifications", map[string]interface{}{
		"BucketName":       bucketName,
		"Notifications":    notifications,
		"HasNotifications": len(notifications) > 0,
	})
}

// GetReplication returns the replication configuration for a bucket
func (h *BucketsHandler) GetReplication(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	client, err := h.minioFactory.NewClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	config, err := client.GetBucketReplication(c.Request().Context(), bucketName)
	if err != nil {
		// No replication config is not an error
		config = replication.Config{}
	}

	// Transform rules for display
	var rules []map[string]interface{}
	for _, rule := range config.Rules {
		rules = append(rules, map[string]interface{}{
			"ID":          rule.ID,
			"Status":      rule.Status,
			"Priority":    rule.Priority,
			"Destination": rule.Destination.Bucket,
		})
	}

	return c.Render(http.StatusOK, "replication", map[string]interface{}{
		"BucketName":     bucketName,
		"Rules":          rules,
		"HasReplication": len(rules) > 0,
	})
}

// GetBucketQuota returns the quota for a bucket
func (h *BucketsHandler) GetBucketQuota(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	quota, err := mdm.GetBucketQuota(c.Request().Context(), bucketName)
	if err != nil {
		// No quota is not an error
		quota = madmin.BucketQuota{}
	}

	hasQuota := quota.Size > 0 || quota.Rate > 0 || quota.Requests > 0

	return c.Render(http.StatusOK, "bucket_quota", map[string]interface{}{
		"BucketName": bucketName,
		"Size":       utils.FormatBytes(quota.Size),
		"SizeBytes":  quota.Size,
		"Rate":       utils.FormatBytes(quota.Rate),
		"RateBytes":  quota.Rate,
		"Requests":   quota.Requests,
		"HasQuota":   hasQuota,
		"QuotaType":  string(quota.Type),
	})
}

// SetBucketQuota sets the quota for a bucket
func (h *BucketsHandler) SetBucketQuota(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	bucketName := c.Param("bucketName")
	sizeStr := c.FormValue("size")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	var size uint64
	if sizeStr != "" {
		// Parse size in GB
		sizeGB, parseErr := strconv.ParseUint(sizeStr, 10, 64)
		if parseErr != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid size")
		}
		size = sizeGB * 1024 * 1024 * 1024 // Convert GB to bytes
	}

	quota := &madmin.BucketQuota{
		Size: size,
	}

	if err := mdm.SetBucketQuota(c.Request().Context(), bucketName, quota); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to set quota: "+err.Error())
	}

	return HTMXRedirect(c, "/buckets/"+bucketName+"/settings")
}

// formatExpiration formats duration for display
func formatExpiration(d time.Duration) string {
	if d >= 24*time.Hour {
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day"
		}
		return fmt.Sprintf("%d days", days)
	}
	if d >= time.Hour {
		hours := int(d.Hours())
		if hours == 1 {
			return "1 hour"
		}
		return fmt.Sprintf("%d hours", hours)
	}
	minutes := int(d.Minutes())
	if minutes == 1 {
		return "1 minute"
	}
	return fmt.Sprintf("%d minutes", minutes)
}

// Helper functions for file type detection

func getContentTypeFromExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	types := map[string]string{
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".webp": "image/webp",
		".svg":  "image/svg+xml",
		".txt":  "text/plain",
		".md":   "text/markdown",
		".json": "application/json",
		".xml":  "application/xml",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".pdf":  "application/pdf",
		".mp4":  "video/mp4",
		".webm": "video/webm",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}
	if t, ok := types[ext]; ok {
		return t
	}
	return "application/octet-stream"
}

func isImageType(contentType string) bool {
	return strings.HasPrefix(contentType, "image/")
}

func isTextType(contentType string) bool {
	return strings.HasPrefix(contentType, "text/") ||
		contentType == "application/json" ||
		contentType == "application/xml" ||
		contentType == "application/javascript"
}

func isVideoType(contentType string) bool {
	return strings.HasPrefix(contentType, "video/")
}

func isArchiveType(contentType string, filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	return contentType == "application/zip" ||
		contentType == "application/x-tar" ||
		contentType == "application/gzip" ||
		ext == ".zip" || ext == ".tar" || ext == ".gz" || ext == ".rar" || ext == ".7z"
}

func isPreviewable(contentType string, size int64) bool {
	// Limit preview to files under 10MB
	if size > 10*1024*1024 {
		return false
	}
	return isImageType(contentType) || isTextType(contentType) || isVideoType(contentType)
}
