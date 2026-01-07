package handlers

import (
	"net/http"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
)

type UsersHandler struct {
	minioFactory services.MinioClientFactory
}

func NewUsersHandler(minioFactory services.MinioClientFactory) *UsersHandler {
	return &UsersHandler{minioFactory: minioFactory}
}

// UserWithGroups extends user info with group membership
type UserWithGroups struct {
	madmin.UserInfo
	Groups []string
}

// ListUsers renders the user management page
func (h *UsersHandler) ListUsers(c echo.Context) error {
	// Get credentials from context (set by AuthMiddleware)
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	// Connect to MinIO
	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Fetch Users
	users, err := mdm.ListUsers(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list users")
	}

	// Fetch Groups and build user->groups mapping
	userGroups := make(map[string][]string)
	groupNames, err := mdm.ListGroups(c.Request().Context())
	if err == nil {
		for _, groupName := range groupNames {
			desc, err := mdm.GetGroupDescription(c.Request().Context(), groupName)
			if err != nil {
				continue
			}
			for _, member := range desc.Members {
				userGroups[member] = append(userGroups[member], groupName)
			}
		}
	}

	// Build users with groups
	usersWithGroups := make(map[string]UserWithGroups)
	for username, userInfo := range users {
		usersWithGroups[username] = UserWithGroups{
			UserInfo: userInfo,
			Groups:   userGroups[username],
		}
	}

	return c.Render(http.StatusOK, "users", map[string]interface{}{
		"ActiveNav": "users",
		"Users":     usersWithGroups,
	})
}

// CreateUserModal renders the modal form
func (h *UsersHandler) CreateUserModal(c echo.Context) error {
	return c.Render(http.StatusOK, "user_create_modal", nil)
}

// CreateUser handles the creation of a new user
func (h *UsersHandler) CreateUser(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.FormValue("accessKey")
	secretKey := c.FormValue("secretKey")
	policy := c.FormValue("policy") // Get policy from form

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	// Create the user
	if err := mdm.AddUser(c.Request().Context(), accessKey, secretKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create user: "+err.Error())
	}

	// Assign policy if provided
	if policy != "" {
		if err := mdm.SetPolicy(c.Request().Context(), policy, accessKey, false); err != nil {
			// User created but policy assignment failed - log but don't fail
			// In production, you might want to handle this differently
			return echo.NewHTTPError(http.StatusInternalServerError, "User created but failed to assign policy: "+err.Error())
		}
	}

	// Use HX-Redirect to close modal and refresh the users page
	return HTMXRedirect(c, "/users")
}

// DeleteUser handles removing a user
func (h *UsersHandler) DeleteUser(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.FormValue("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := mdm.RemoveUser(c.Request().Context(), accessKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete user")
	}

	return HTMXRedirect(c, "/users")
}

// EnableUser handles enabling a user account
func (h *UsersHandler) EnableUser(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.FormValue("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := mdm.SetUserStatus(c.Request().Context(), accessKey, madmin.AccountEnabled); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to enable user")
	}

	return HTMXRedirect(c, "/users")
}

// DisableUser handles disabling a user account
func (h *UsersHandler) DisableUser(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.FormValue("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := mdm.SetUserStatus(c.Request().Context(), accessKey, madmin.AccountDisabled); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to disable user")
	}

	return HTMXRedirect(c, "/users")
}

// ListServiceAccounts returns service accounts for a user
func (h *UsersHandler) ListServiceAccounts(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	accessKey := c.Param("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	accounts, err := mdm.ListServiceAccounts(c.Request().Context(), accessKey)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list service accounts: "+err.Error())
	}

	return c.Render(http.StatusOK, "service_accounts", map[string]interface{}{
		"ActiveNav":       "users",
		"ParentUser":      accessKey,
		"ServiceAccounts": accounts.Accounts,
	})
}

// CreateServiceAccountModal renders the service account creation modal
func (h *UsersHandler) CreateServiceAccountModal(c echo.Context) error {
	accessKey := c.Param("accessKey")
	return c.Render(http.StatusOK, "service_account_create_modal", map[string]interface{}{
		"ParentUser": accessKey,
	})
}

// CreateServiceAccount creates a new service account for a user
func (h *UsersHandler) CreateServiceAccount(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.Param("accessKey")
	name := c.FormValue("name")
	description := c.FormValue("description")
	expiry := c.FormValue("expiry")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	req := madmin.AddServiceAccountReq{
		TargetUser:  accessKey,
		Name:        name,
		Description: description,
	}

	if expiry != "" {
		dur, err := time.ParseDuration(expiry)
		if err == nil {
			exp := time.Now().Add(dur)
			req.Expiration = &exp
		}
	}

	newCreds, err := mdm.AddServiceAccount(c.Request().Context(), req)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to create service account: "+err.Error())
	}

	// Return the new credentials - user needs to copy these!
	return c.Render(http.StatusOK, "service_account_created", map[string]interface{}{
		"AccessKey":  newCreds.AccessKey,
		"SecretKey":  newCreds.SecretKey,
		"ParentUser": accessKey,
	})
}

// DeleteServiceAccount removes a service account
func (h *UsersHandler) DeleteServiceAccount(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	serviceAccountKey := c.FormValue("serviceAccountKey")
	parentUser := c.Param("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := mdm.DeleteServiceAccount(c.Request().Context(), serviceAccountKey); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to delete service account: "+err.Error())
	}

	return HTMXRedirect(c, "/users/"+parentUser+"/keys")
}

// ListPolicies returns all available policies
func (h *UsersHandler) ListPolicies(c echo.Context) error {
	creds, err := GetCredentialsOrRedirect(c)
	if err != nil {
		return err
	}

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	policies, err := mdm.ListCannedPolicies(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list policies: "+err.Error())
	}

	// Convert to a simpler structure for the template
	policyNames := make([]string, 0, len(policies))
	for name := range policies {
		policyNames = append(policyNames, name)
	}

	return c.Render(http.StatusOK, "policies", map[string]interface{}{
		"ActiveNav": "users",
		"Policies":  policyNames,
	})
}

// PolicyModal renders the policy selection modal for a user
func (h *UsersHandler) PolicyModal(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.Param("accessKey")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	policies, err := mdm.ListCannedPolicies(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to list policies: "+err.Error())
	}

	policyNames := make([]string, 0, len(policies))
	for name := range policies {
		policyNames = append(policyNames, name)
	}

	return c.Render(http.StatusOK, "policy_modal", map[string]interface{}{
		"AccessKey": accessKey,
		"Policies":  policyNames,
	})
}

// AttachPolicy attaches a policy to a user
func (h *UsersHandler) AttachPolicy(c echo.Context) error {
	creds, err := GetCredentials(c)
	if err != nil {
		return err
	}

	accessKey := c.Param("accessKey")
	policy := c.FormValue("policy")

	mdm, err := h.minioFactory.NewAdminClient(*creds)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to connect to MinIO")
	}

	if err := mdm.SetPolicy(c.Request().Context(), policy, accessKey, false); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to attach policy: "+err.Error())
	}

	return HTMXRedirect(c, "/users")
}
