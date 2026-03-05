package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/stretchr/testify/assert"
)

// dashboardMockRenderer implements echo.Renderer for testing
type dashboardMockRenderer struct{}

func (m *dashboardMockRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	// Simple JSON-like serialization for tests to assert contents without HTML formatting
	d, err := json.Marshal(data)
	if err == nil {
		w.Write([]byte(name + " "))
		w.Write(d)
	}
	return nil
}

// dashboardTestFactory implements services.MinioClientFactory
type dashboardTestFactory struct {
	adminClient services.MinioAdminClient
	adminErr    error
}

func (f *dashboardTestFactory) NewAdminClient(_ services.Credentials) (services.MinioAdminClient, error) {
	if f.adminErr != nil {
		return nil, f.adminErr
	}
	return f.adminClient, nil
}

func (f *dashboardTestFactory) NewClient(_ services.Credentials) (services.MinioClient, error) {
	return nil, nil // Not used in dashboard handler
}

// mockMinioAdminClient implements services.MinioAdminClient
type mockMinioAdminClient struct {
	// Stub responses
	dataUsageInfo   madmin.DataUsageInfo
	dataUsageErr    error
	listUsers       map[string]madmin.UserInfo
	listUsersErr    error
	serviceAccounts madmin.ListServiceAccountsResp
	serviceAccsErr  error
	serverInfo      madmin.InfoMessage
	serverInfoErr   error
}

// Implement required MinioAdminClient methods
func (m *mockMinioAdminClient) DataUsageInfo(ctx context.Context) (madmin.DataUsageInfo, error) {
	return m.dataUsageInfo, m.dataUsageErr
}

func (m *mockMinioAdminClient) ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	return m.listUsers, m.listUsersErr
}

func (m *mockMinioAdminClient) ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error) {
	return m.serviceAccounts, m.serviceAccsErr
}

func (m *mockMinioAdminClient) ServerInfo(ctx context.Context, opts ...func(*madmin.ServerInfoOpts)) (madmin.InfoMessage, error) {
	return m.serverInfo, m.serverInfoErr
}

// Stubs for remaining interface methods
func (m *mockMinioAdminClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	return nil
}
func (m *mockMinioAdminClient) RemoveUser(ctx context.Context, accessKey string) error { return nil }
func (m *mockMinioAdminClient) SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error {
	return nil
}
func (m *mockMinioAdminClient) SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error {
	return nil
}
func (m *mockMinioAdminClient) ServiceRestart(ctx context.Context) error      { return nil }
func (m *mockMinioAdminClient) GetConfig(ctx context.Context) ([]byte, error) { return nil, nil }
func (m *mockMinioAdminClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	return madmin.Credentials{}, nil
}
func (m *mockMinioAdminClient) DeleteServiceAccount(ctx context.Context, serviceAccount string) error {
	return nil
}
func (m *mockMinioAdminClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	return nil, nil
}
func (m *mockMinioAdminClient) InfoCannedPolicyV2(ctx context.Context, policyName string) (*madmin.PolicyInfo, error) {
	return nil, nil
}
func (m *mockMinioAdminClient) GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan madmin.LogInfo {
	return nil
}
func (m *mockMinioAdminClient) GetBucketQuota(ctx context.Context, bucket string) (madmin.BucketQuota, error) {
	return madmin.BucketQuota{}, nil
}
func (m *mockMinioAdminClient) SetBucketQuota(ctx context.Context, bucket string, quota *madmin.BucketQuota) error {
	return nil
}
func (m *mockMinioAdminClient) ListGroups(ctx context.Context) ([]string, error) { return nil, nil }
func (m *mockMinioAdminClient) GetGroupDescription(ctx context.Context, group string) (*madmin.GroupDesc, error) {
	return nil, nil
}
func (m *mockMinioAdminClient) UpdateGroupMembers(ctx context.Context, req madmin.GroupAddRemove) error {
	return nil
}
func (m *mockMinioAdminClient) SetGroupStatus(ctx context.Context, group string, status madmin.GroupStatus) error {
	return nil
}
func (m *mockMinioAdminClient) ListAccessKeysBulk(ctx context.Context, users []string, opts madmin.ListAccessKeysOpts) (map[string]madmin.ListAccessKeysResp, error) {
	if m.serviceAccsErr != nil {
		return nil, m.serviceAccsErr
	}
	resp := make(map[string]madmin.ListAccessKeysResp)
	var svcAccs []madmin.ServiceAccountInfo
	for _, sa := range m.serviceAccounts.Accounts {
		svcAccs = append(svcAccs, madmin.ServiceAccountInfo{
			AccessKey:     sa.AccessKey,
			AccountStatus: sa.AccountStatus,
		})
	}
	for _, u := range users {
		resp[u] = madmin.ListAccessKeysResp{
			ServiceAccounts: svcAccs,
		}
	}
	return resp, nil
}

func TestGetStorageWidget(t *testing.T) {
	e := echo.New()
	e.Renderer = &dashboardMockRenderer{}

	tests := []struct {
		name          string
		withCreds     bool
		adminErr      error
		dataUsageInfo madmin.DataUsageInfo
		dataUsageErr  error
		expectedUsed  string
		expectedTotal string
		expectedPct   string
		expectedErr   bool
	}{
		{
			name:      "Success",
			withCreds: true,
			dataUsageInfo: madmin.DataUsageInfo{
				ObjectsTotalSize: 50 * 1024 * 1024 * 1024,  // 50 GB
				TotalCapacity:    100 * 1024 * 1024 * 1024, // 100 GB
				BucketsCount:     5,
			},
			expectedUsed:  "50.0 GB",
			expectedTotal: "100.0 GB",
			expectedPct:   "50",
			expectedErr:   false,
		},
		{
			name:        "Missing Credentials",
			withCreds:   false,
			expectedErr: true,
		},
		{
			name:        "Admin Client Error",
			withCreds:   true,
			adminErr:    assert.AnError,
			expectedErr: true,
		},
		{
			name:         "Data Usage Error",
			withCreds:    true,
			dataUsageErr: assert.AnError,
			expectedErr:  true,
		},
		{
			name:      "Zero Capacity",
			withCreds: true,
			dataUsageInfo: madmin.DataUsageInfo{
				ObjectsTotalSize: 0,
				TotalCapacity:    0,
				BucketsCount:     0,
			},
			expectedUsed:  "0 B",
			expectedTotal: "0 B",
			expectedPct:   "0",
			expectedErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMinioAdminClient{
				dataUsageInfo: tt.dataUsageInfo,
				dataUsageErr:  tt.dataUsageErr,
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tt.withCreds {
				c.Set(utils.ContextKeyCreds, &services.Credentials{
					Endpoint:  "localhost:9000",
					AccessKey: "minioadmin",
					SecretKey: "minioadmin",
				})
			}

			factory := &dashboardTestFactory{
				adminClient: mockClient,
				adminErr:    tt.adminErr,
			}

			handler := NewDashboardHandler(factory)
			err := handler.GetStorageWidget(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()
			assert.Contains(t, body, "storage_widget")
			if tt.expectedErr {
				assert.Contains(t, body, "\"Error\":true")
			} else {
				assert.Contains(t, body, tt.expectedUsed)
				assert.Contains(t, body, tt.expectedTotal)
				assert.Contains(t, body, tt.expectedPct)
			}
		})
	}
}

func TestGetUsersWidget(t *testing.T) {
	e := echo.New()
	e.Renderer = &dashboardMockRenderer{}

	tests := []struct {
		name                string
		withCreds           bool
		adminErr            error
		listUsers           map[string]madmin.UserInfo
		listUsersErr        error
		serviceAccounts     madmin.ListServiceAccountsResp
		serviceAccsErr      error
		expectedTotalUsers  int
		expectedActiveUsers int
		expectedTotalSAs    int
		expectedActiveSAs   int
		expectedErr         bool
	}{
		{
			name:      "Success",
			withCreds: true,
			listUsers: map[string]madmin.UserInfo{
				"user1": {Status: "enabled"},
				"user2": {Status: "disabled"},
				"user3": {Status: "enabled"},
			},
			serviceAccounts: madmin.ListServiceAccountsResp{
				Accounts: []madmin.ServiceAccountInfo{
					{AccountStatus: "on"},
					{AccountStatus: "off"},
				},
			},
			expectedTotalUsers:  3,
			expectedActiveUsers: 2,
			expectedTotalSAs:    6, // 3 users * 2 service accounts each
			expectedActiveSAs:   3, // 3 users * 1 active service account each
			expectedErr:         false,
		},
		{
			name:        "Missing Credentials",
			withCreds:   false,
			expectedErr: true,
		},
		{
			name:        "Admin Client Error",
			withCreds:   true,
			adminErr:    assert.AnError,
			expectedErr: true,
		},
		{
			name:         "List Users Error",
			withCreds:    true,
			listUsersErr: assert.AnError,
			expectedErr:  true,
		},
		{
			name:      "Service Accounts Error Skipped",
			withCreds: true,
			listUsers: map[string]madmin.UserInfo{
				"user1": {Status: "enabled"},
			},
			serviceAccsErr:      assert.AnError, // Will cause skip of SA counting
			expectedTotalUsers:  1,
			expectedActiveUsers: 1,
			expectedTotalSAs:    0,
			expectedActiveSAs:   0,
			expectedErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMinioAdminClient{
				listUsers:       tt.listUsers,
				listUsersErr:    tt.listUsersErr,
				serviceAccounts: tt.serviceAccounts,
				serviceAccsErr:  tt.serviceAccsErr,
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tt.withCreds {
				c.Set(utils.ContextKeyCreds, &services.Credentials{
					Endpoint:  "localhost:9000",
					AccessKey: "minioadmin",
					SecretKey: "minioadmin",
				})
			}

			factory := &dashboardTestFactory{
				adminClient: mockClient,
				adminErr:    tt.adminErr,
			}

			handler := NewDashboardHandler(factory)
			err := handler.GetUsersWidget(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()
			assert.Contains(t, body, "users_widget")

			if tt.expectedErr {
				assert.Contains(t, body, "\"Error\":true")
			} else {
				// verify JSON string payload
				assert.Contains(t, body, "\"TotalUsers\":"+string(rune(tt.expectedTotalUsers+'0')))
				assert.Contains(t, body, "\"ActiveUsers\":"+string(rune(tt.expectedActiveUsers+'0')))
				assert.Contains(t, body, "\"ServiceAccounts\":"+string(rune(tt.expectedTotalSAs+'0')))
				assert.Contains(t, body, "\"ActiveServiceAccounts\":"+string(rune(tt.expectedActiveSAs+'0')))
			}
		})
	}
}

func TestGetServerVersion(t *testing.T) {
	e := echo.New()

	tests := []struct {
		name          string
		withCreds     bool
		adminErr      error
		serverInfo    madmin.InfoMessage
		serverInfoErr error
		expectedBody  string
	}{
		{
			name:      "Success",
			withCreds: true,
			serverInfo: madmin.InfoMessage{
				Servers: []madmin.ServerProperties{
					{Version: "RELEASE.2024-11-07T00-52-20Z"},
				},
			},
			expectedBody: "vRELEASE.2024-11-07T00-52-20Z", // The handler prepends 'v'
		},
		{
			name:         "No Servers",
			withCreds:    true,
			serverInfo:   madmin.InfoMessage{Servers: []madmin.ServerProperties{}},
			expectedBody: "",
		},
		{
			name:         "Missing Credentials",
			withCreds:    false,
			expectedBody: "",
		},
		{
			name:         "Admin Client Error",
			withCreds:    true,
			adminErr:     assert.AnError,
			expectedBody: "",
		},
		{
			name:          "Server Info Error",
			withCreds:     true,
			serverInfoErr: assert.AnError,
			expectedBody:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMinioAdminClient{
				serverInfo:    tt.serverInfo,
				serverInfoErr: tt.serverInfoErr,
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tt.withCreds {
				c.Set(utils.ContextKeyCreds, &services.Credentials{
					Endpoint:  "localhost:9000",
					AccessKey: "minioadmin",
					SecretKey: "minioadmin",
				})
			}

			factory := &dashboardTestFactory{
				adminClient: mockClient,
				adminErr:    tt.adminErr,
			}

			handler := NewDashboardHandler(factory)
			err := handler.GetServerVersion(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
		})
	}
}

func TestGetServerWidget(t *testing.T) {
	e := echo.New()
	e.Renderer = &dashboardMockRenderer{}

	tests := []struct {
		name          string
		withCreds     bool
		adminErr      error
		serverInfo    madmin.InfoMessage
		serverInfoErr error
		expectedVer   string
		expectedUp    string
		expectedCount int
		expectedMode  string
		expectedErr   bool
	}{
		{
			name:      "Success",
			withCreds: true,
			serverInfo: madmin.InfoMessage{
				Servers: []madmin.ServerProperties{
					{Version: "RELEASE.2024-11-07T00-52-20Z", Uptime: 3661}, // 1h 1m
					{Version: "RELEASE.2024-11-07T00-52-20Z", Uptime: 3661},
				},
				Mode: "standalone",
			},
			expectedVer:   "2024-11-07",
			expectedUp:    "1h 1m",
			expectedCount: 2,
			expectedMode:  "standalone",
			expectedErr:   false,
		},
		{
			name:      "No Servers",
			withCreds: true,
			serverInfo: madmin.InfoMessage{
				Servers: []madmin.ServerProperties{},
				Mode:    "standalone",
			},
			expectedVer:   "Unknown",
			expectedUp:    "Unknown",
			expectedCount: 0,
			expectedMode:  "standalone",
			expectedErr:   false,
		},
		{
			name:        "Missing Credentials",
			withCreds:   false,
			expectedErr: true,
		},
		{
			name:        "Admin Client Error",
			withCreds:   true,
			adminErr:    assert.AnError,
			expectedErr: true,
		},
		{
			name:          "Server Info Error",
			withCreds:     true,
			serverInfoErr: assert.AnError,
			expectedErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &mockMinioAdminClient{
				serverInfo:    tt.serverInfo,
				serverInfoErr: tt.serverInfoErr,
			}

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			if tt.withCreds {
				c.Set(utils.ContextKeyCreds, &services.Credentials{
					Endpoint:  "localhost:9000",
					AccessKey: "minioadmin",
					SecretKey: "minioadmin",
				})
			}

			factory := &dashboardTestFactory{
				adminClient: mockClient,
				adminErr:    tt.adminErr,
			}

			handler := NewDashboardHandler(factory)
			err := handler.GetServerWidget(c)

			assert.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			body := rec.Body.String()
			assert.Contains(t, body, "server_widget")

			if tt.expectedErr {
				assert.Contains(t, body, "\"Error\":true")
			} else {
				assert.Contains(t, body, tt.expectedVer)
				assert.Contains(t, body, tt.expectedUp)
				assert.Contains(t, body, tt.expectedMode)
				assert.Contains(t, body, "\"ServerCount\":"+string(rune(tt.expectedCount+'0')))
			}
		})
	}
}

func TestFormatHelpers(t *testing.T) {
	t.Run("formatVersion", func(t *testing.T) {
		tests := []struct {
			input    string
			expected string
		}{
			{"RELEASE.2024-11-07T00-52-20Z", "2024-11-07"},
			{"2025-09-07T16:13:09Z", "2025-09-07"},
			{"RELEASE.short", "short"},
			{"123456789", "123456789"},
			{"", "Unknown"},
		}

		for _, tt := range tests {
			t.Run(tt.input, func(t *testing.T) {
				assert.Equal(t, tt.expected, formatVersion(tt.input))
			})
		}
	})

	t.Run("formatUptime", func(t *testing.T) {
		tests := []struct {
			seconds  int64
			expected string
		}{
			{0, "0m"},
			{59, "0m"},
			{60, "1m"},
			{119, "1m"},
			{3600, "1h 0m"},
			{3660, "1h 1m"},
			{86400, "1d 0h 0m"},
			{86460, "1d 0h 1m"},
			{90060, "1d 1h 1m"},
			{172800, "2d 0h 0m"},
		}

		for _, tt := range tests {
			t.Run(string(rune(tt.seconds)), func(t *testing.T) {
				assert.Equal(t, tt.expected, formatUptime(tt.seconds))
			})
		}
	})
}
