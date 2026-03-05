package handlers

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
)

type DashboardHandler struct {
	minioFactory services.MinioClientFactory
}

func NewDashboardHandler(minioFactory services.MinioClientFactory) *DashboardHandler {
	return &DashboardHandler{minioFactory: minioFactory}
}

// GetStorageWidget returns storage stats for the dashboard
func (h *DashboardHandler) GetStorageWidget(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return c.Render(http.StatusOK, "storage_widget", map[string]interface{}{
			"Error": true,
		})
	}

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return c.Render(http.StatusOK, "storage_widget", map[string]interface{}{
			"Error": true,
		})
	}

	// Fetch Storage Info
	storageInfo, err := mdm.DataUsageInfo(c.Request().Context())
	if err != nil {
		return c.Render(http.StatusOK, "storage_widget", map[string]interface{}{
			"Error": true,
		})
	}

	// Calculate percentage
	usedPercent := 0.0
	if storageInfo.TotalCapacity > 0 {
		usedPercent = float64(storageInfo.ObjectsTotalSize) / float64(storageInfo.TotalCapacity) * 100
	}

	return c.Render(http.StatusOK, "storage_widget", map[string]interface{}{
		"UsedSpace":    utils.FormatBytes(storageInfo.ObjectsTotalSize),
		"TotalSpace":   utils.FormatBytes(storageInfo.TotalCapacity),
		"UsedPercent":  fmt.Sprintf("%.0f", usedPercent),
		"BucketsCount": storageInfo.BucketsCount,
	})
}

// GetUsersWidget returns user stats for the dashboard
func (h *DashboardHandler) GetUsersWidget(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return c.Render(http.StatusOK, "users_widget", map[string]interface{}{
			"Error": true,
		})
	}

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return c.Render(http.StatusOK, "users_widget", map[string]interface{}{
			"Error": true,
		})
	}

	// Fetch Users
	users, err := mdm.ListUsers(c.Request().Context())
	if err != nil {
		return c.Render(http.StatusOK, "users_widget", map[string]interface{}{
			"Error": true,
		})
	}

	totalUsers := len(users)
	activeUsers := 0
	for _, user := range users {
		if user.Status == "enabled" {
			activeUsers++
		}
	}

	// Count service accounts across all users
	totalServiceAccounts := 0
	activeServiceAccounts := 0
	ctx := c.Request().Context()

	var userList []string
	for username := range users {
		userList = append(userList, username)
	}

	if len(userList) > 0 {
		bulkAccounts, err := mdm.ListAccessKeysBulk(ctx, userList, madmin.ListAccessKeysOpts{
			ListType: madmin.AccessKeyListSvcaccOnly,
		})
		if err == nil {
			for _, accounts := range bulkAccounts {
				for _, acc := range accounts.ServiceAccounts {
					totalServiceAccounts++
					if acc.AccountStatus == "on" {
						activeServiceAccounts++
					}
				}
			}
		}
	}

	return c.Render(http.StatusOK, "users_widget", map[string]interface{}{
		"TotalUsers":            totalUsers,
		"ActiveUsers":           activeUsers,
		"ServiceAccounts":       totalServiceAccounts,
		"ActiveServiceAccounts": activeServiceAccounts,
	})
}

// GetServerVersion returns just the version string for the header
func (h *DashboardHandler) GetServerVersion(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return c.String(http.StatusOK, "")
	}

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return c.String(http.StatusOK, "")
	}

	serverInfo, err := mdm.ServerInfo(c.Request().Context())
	if err != nil {
		return c.String(http.StatusOK, "")
	}

	if len(serverInfo.Servers) > 0 {
		return c.String(http.StatusOK, "v"+serverInfo.Servers[0].Version)
	}
	return c.String(http.StatusOK, "")
}

// GetServerWidget returns server info (version, uptime) for the dashboard
func (h *DashboardHandler) GetServerWidget(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return c.Render(http.StatusOK, "server_widget", map[string]interface{}{
			"Error": true,
		})
	}

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return c.Render(http.StatusOK, "server_widget", map[string]interface{}{
			"Error": true,
		})
	}

	serverInfo, err := mdm.ServerInfo(c.Request().Context())
	if err != nil {
		return c.Render(http.StatusOK, "server_widget", map[string]interface{}{
			"Error": true,
		})
	}

	// Get version and uptime from first server
	version := "Unknown"
	uptime := "Unknown"
	serverCount := len(serverInfo.Servers)

	if serverCount > 0 {
		version = formatVersion(serverInfo.Servers[0].Version)
		uptime = formatUptime(serverInfo.Servers[0].Uptime)
	}

	return c.Render(http.StatusOK, "server_widget", map[string]interface{}{
		"Version":     version,
		"Uptime":      uptime,
		"ServerCount": serverCount,
		"Mode":        serverInfo.Mode,
		"Region":      serverInfo.Region,
	})
}

// formatVersion extracts a clean version from MinIO version strings
// e.g., "RELEASE.2024-11-07T00-52-20Z" -> "2024-11-07" or "2025-09-07T16:13:09Z" -> "2025-09-07"
func formatVersion(version string) string {
	if version == "" {
		return "Unknown"
	}
	// Handle RELEASE.YYYY-MM-DDTHH-MM-SSZ format
	version = strings.TrimPrefix(version, "RELEASE.")
	// Extract just the date part (YYYY-MM-DD)
	if len(version) >= 10 {
		return version[:10]
	}
	return version
}

// formatUptime converts seconds to human-readable format
func formatUptime(seconds int64) string {
	days := seconds / 86400
	hours := (seconds % 86400) / 3600
	minutes := (seconds % 3600) / 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm", days, hours, minutes)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm", hours, minutes)
	}
	return fmt.Sprintf("%dm", minutes)
}
