package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockTemplateRenderer struct{}

func (t *mockTemplateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return nil
}

type mockUsersAdminClient struct {
	listUsersResp           map[string]madmin.UserInfo
	listUsersErr            error
	listGroupsResp          []string
	listGroupsErr           error
	getGroupDescriptionResp *madmin.GroupDesc
	getGroupDescriptionErr  error
	addUserErr              error
	removeUserErr           error
	setPolicyErr            error
	setUserStatusErr        error
	listServiceAccountsResp madmin.ListServiceAccountsResp
	listServiceAccountsErr  error
	addServiceAccountResp   madmin.Credentials
	addServiceAccountErr    error
	deleteServiceAccountErr error
	listCannedPoliciesResp  map[string]json.RawMessage
	listCannedPoliciesErr   error
}

func (m *mockUsersAdminClient) ServerInfo(ctx context.Context, opts ...func(*madmin.ServerInfoOpts)) (madmin.InfoMessage, error) {
	return madmin.InfoMessage{}, nil
}

func (m *mockUsersAdminClient) ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	return m.listUsersResp, m.listUsersErr
}

func (m *mockUsersAdminClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	return m.addUserErr
}

func (m *mockUsersAdminClient) RemoveUser(ctx context.Context, accessKey string) error {
	return m.removeUserErr
}

func (m *mockUsersAdminClient) SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error {
	return m.setPolicyErr
}

func (m *mockUsersAdminClient) SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error {
	return m.setUserStatusErr
}

func (m *mockUsersAdminClient) ServiceRestart(ctx context.Context) error {
	return nil
}

func (m *mockUsersAdminClient) DataUsageInfo(ctx context.Context) (madmin.DataUsageInfo, error) {
	return madmin.DataUsageInfo{}, nil
}

func (m *mockUsersAdminClient) GetConfig(ctx context.Context) ([]byte, error) {
	return nil, nil
}

func (m *mockUsersAdminClient) ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error) {
	return m.listServiceAccountsResp, m.listServiceAccountsErr
}

func (m *mockUsersAdminClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	return m.addServiceAccountResp, m.addServiceAccountErr
}

func (m *mockUsersAdminClient) DeleteServiceAccount(ctx context.Context, serviceAccount string) error {
	return m.deleteServiceAccountErr
}

func (m *mockUsersAdminClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	return m.listCannedPoliciesResp, m.listCannedPoliciesErr
}

func (m *mockUsersAdminClient) InfoCannedPolicyV2(ctx context.Context, policyName string) (*madmin.PolicyInfo, error) {
	return nil, nil
}

func (m *mockUsersAdminClient) GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan madmin.LogInfo {
	return nil
}

func (m *mockUsersAdminClient) GetBucketQuota(ctx context.Context, bucket string) (madmin.BucketQuota, error) {
	return madmin.BucketQuota{}, nil
}

func (m *mockUsersAdminClient) SetBucketQuota(ctx context.Context, bucket string, quota *madmin.BucketQuota) error {
	return nil
}

func (m *mockUsersAdminClient) ListGroups(ctx context.Context) ([]string, error) {
	return m.listGroupsResp, m.listGroupsErr
}

func (m *mockUsersAdminClient) GetGroupDescription(ctx context.Context, group string) (*madmin.GroupDesc, error) {
	if m.getGroupDescriptionErr != nil {
		return nil, m.getGroupDescriptionErr
	}
	return m.getGroupDescriptionResp, nil
}

func (m *mockUsersAdminClient) UpdateGroupMembers(ctx context.Context, req madmin.GroupAddRemove) error {
	return nil
}

func (m *mockUsersAdminClient) SetGroupStatus(ctx context.Context, group string, status madmin.GroupStatus) error {
	return nil
}

func (m *mockUsersAdminClient) ListAccessKeysBulk(ctx context.Context, users []string, opts madmin.ListAccessKeysOpts) (map[string]madmin.ListAccessKeysResp, error) {
	return nil, nil
}

type mockUsersFactory struct {
	adminClient *mockUsersAdminClient
	err         error
}

func (f *mockUsersFactory) NewAdminClient(creds services.Credentials) (services.MinioAdminClient, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.adminClient, nil
}

func (f *mockUsersFactory) NewClient(creds services.Credentials) (services.MinioClient, error) {
	return nil, nil
}

func setupUsersTestContext(method, path string, form url.Values) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	e.Renderer = &mockTemplateRenderer{}

	var req *http.Request
	if form != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	} else {
		req = httptest.NewRequest(method, path, nil)
	}

	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	// Set credentials in context to simulate AuthMiddleware
	creds := &services.Credentials{
		Endpoint:  "test-endpoint",
		AccessKey: "test-access",
		SecretKey: "test-secret",
	}
	c.Set("creds", creds)

	return c, rec
}

func TestListUsers_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listUsersResp: map[string]madmin.UserInfo{
			"user1": {Status: madmin.AccountEnabled, PolicyName: "readwrite"},
			"user2": {Status: madmin.AccountDisabled, PolicyName: "readonly"},
		},
		listGroupsResp: []string{"group1", "group2"},
		getGroupDescriptionResp: &madmin.GroupDesc{
			Name:    "group1",
			Members: []string{"user1"},
			Status:  string(madmin.GroupEnabled),
		},
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, rec := setupUsersTestContext(http.MethodGet, "/users", nil)

	err := handler.ListUsers(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListUsers_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listUsersErr: errors.New("minio error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, _ := setupUsersTestContext(http.MethodGet, "/users", nil)

	err := handler.ListUsers(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
	assert.Equal(t, "Failed to list users", httpErr.Message)
}

func TestCreateUser_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "newuser")
	form.Set("secretKey", "newsecret")
	c, _ := setupUsersTestContext(http.MethodPost, "/users", form)

	err := handler.CreateUser(c)
	require.NoError(t, err)

	// Since HTMXRedirect does not write a response body right away in our stub but writes HX-Redirect header
	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestCreateUser_WithPolicySuccess(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "newuser")
	form.Set("secretKey", "newsecret")
	form.Set("policy", "readwrite")
	c, _ := setupUsersTestContext(http.MethodPost, "/users", form)

	err := handler.CreateUser(c)
	require.NoError(t, err)

	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestCreateUser_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		addUserErr: errors.New("minio add user error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "newuser")
	form.Set("secretKey", "newsecret")
	c, _ := setupUsersTestContext(http.MethodPost, "/users", form)

	err := handler.CreateUser(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
	assert.Contains(t, httpErr.Message.(string), "Failed to create user")
}

func TestCreateUser_PolicyError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		setPolicyErr: errors.New("minio set policy error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "newuser")
	form.Set("secretKey", "newsecret")
	form.Set("policy", "readwrite")
	c, _ := setupUsersTestContext(http.MethodPost, "/users", form)

	err := handler.CreateUser(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
	assert.Contains(t, httpErr.Message.(string), "User created but failed to assign policy")
}

func TestDeleteUser_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "deluser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/delete", form)

	err := handler.DeleteUser(c)
	require.NoError(t, err)

	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestDeleteUser_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		removeUserErr: errors.New("minio delete error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "deluser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/delete", form)

	err := handler.DeleteUser(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestEnableUser_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "enuser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/enable", form)

	err := handler.EnableUser(c)
	require.NoError(t, err)

	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestEnableUser_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		setUserStatusErr: errors.New("minio enable user error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "enuser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/enable", form)

	err := handler.EnableUser(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestDisableUser_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "disuser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/disable", form)

	err := handler.DisableUser(c)
	require.NoError(t, err)

	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestDisableUser_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		setUserStatusErr: errors.New("minio disable user error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("accessKey", "disuser")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/disable", form)

	err := handler.DisableUser(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestListServiceAccounts_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listServiceAccountsResp: madmin.ListServiceAccountsResp{
			Accounts: []madmin.ServiceAccountInfo{
				{AccessKey: "sa-1"},
				{AccessKey: "sa-2"},
			},
		},
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, rec := setupUsersTestContext(http.MethodGet, "/users/testuser/keys", nil)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.ListServiceAccounts(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListServiceAccounts_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listServiceAccountsErr: errors.New("minio service accounts error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, _ := setupUsersTestContext(http.MethodGet, "/users/testuser/keys", nil)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.ListServiceAccounts(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestCreateServiceAccount_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		addServiceAccountResp: madmin.Credentials{
			AccessKey: "sa-new",
			SecretKey: "sa-new-secret",
		},
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("name", "New SA")
	form.Set("description", "Testing")
	// Expiry can be an empty string without issue
	c, rec := setupUsersTestContext(http.MethodPost, "/users/testuser/keys", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.CreateServiceAccount(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateServiceAccount_WithExpiry(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		addServiceAccountResp: madmin.Credentials{
			AccessKey: "sa-new",
			SecretKey: "sa-new-secret",
		},
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("name", "New SA")
	form.Set("description", "Testing")
	form.Set("expiry", "24h")
	c, rec := setupUsersTestContext(http.MethodPost, "/users/testuser/keys", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.CreateServiceAccount(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestCreateServiceAccount_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		addServiceAccountErr: errors.New("minio add service account error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	c, _ := setupUsersTestContext(http.MethodPost, "/users/testuser/keys", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.CreateServiceAccount(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestDeleteServiceAccount_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("serviceAccountKey", "sa-del")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/testuser/keys/delete", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.DeleteServiceAccount(c)
	require.NoError(t, err)

	assert.Equal(t, "/users/testuser/keys", c.Response().Header().Get("HX-Redirect"))
}

func TestDeleteServiceAccount_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		deleteServiceAccountErr: errors.New("minio delete service account error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("serviceAccountKey", "sa-del")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/testuser/keys/delete", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.DeleteServiceAccount(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestListPolicies_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listCannedPoliciesResp: map[string]json.RawMessage{
			"readwrite": []byte(`{}`),
			"readonly":  []byte(`{}`),
		},
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, rec := setupUsersTestContext(http.MethodGet, "/policies", nil)

	err := handler.ListPolicies(c)
	require.NoError(t, err)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestListPolicies_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		listCannedPoliciesErr: errors.New("minio list policies error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	c, _ := setupUsersTestContext(http.MethodGet, "/policies", nil)

	err := handler.ListPolicies(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}

func TestUserAttachPolicy_Success(t *testing.T) {
	adminClient := &mockUsersAdminClient{}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("policy", "readwrite")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/testuser/policy", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.AttachPolicy(c)
	require.NoError(t, err)

	assert.Equal(t, "/users", c.Response().Header().Get("HX-Redirect"))
}

func TestAttachPolicy_MinioError(t *testing.T) {
	adminClient := &mockUsersAdminClient{
		setPolicyErr: errors.New("minio attach policy error"),
	}
	factory := &mockUsersFactory{adminClient: adminClient}
	handler := NewUsersHandler(factory)

	form := url.Values{}
	form.Set("policy", "readwrite")
	c, _ := setupUsersTestContext(http.MethodPost, "/users/testuser/policy", form)
	c.SetParamNames("accessKey")
	c.SetParamValues("testuser")

	err := handler.AttachPolicy(c)
	require.Error(t, err)

	httpErr, ok := err.(*echo.HTTPError)
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, httpErr.Code)
}
