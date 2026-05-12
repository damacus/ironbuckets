package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/damacus/iron-buckets/internal/handlers"
	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/stretchr/testify/mock"
)

type mockRenderer struct{}

func (r *mockRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return nil
}

func BenchmarkGetUsersWidget(b *testing.B) {
	e := echo.New()
	e.Renderer = &mockRenderer{}

	// Setup user mock responses
	numUsers := 100
	usersMap := make(map[string]madmin.UserInfo)

	for i := 0; i < numUsers; i++ {
		username := fmt.Sprintf("user%d", i)
		usersMap[username] = madmin.UserInfo{
			Status: "enabled",
		}
	}

	svcAccountsResp := madmin.ListServiceAccountsResp{
		Accounts: []madmin.ServiceAccountInfo{
			{AccountStatus: "on"},
			{AccountStatus: "off"},
		},
	}

	bulkSvcResp := make(map[string]madmin.ListAccessKeysResp)
	for i := 0; i < numUsers; i++ {
		username := fmt.Sprintf("user%d", i)
		bulkSvcResp[username] = madmin.ListAccessKeysResp{
			ServiceAccounts: []madmin.ServiceAccountInfo{
				{AccountStatus: "on"},
				{AccountStatus: "off"},
			},
		}
	}

	mockClient := new(MockMinioClient)
	mockClient.On("ListUsers", mock.Anything).Return(usersMap, nil)

	// For existing approach (N+1 queries)
	mockClient.On("ListServiceAccounts", mock.Anything, mock.Anything).Return(svcAccountsResp, nil)

	// For optimized approach (Bulk) - Note: Need to mock ListAccessKeysBulk if using bulk API
	mockClient.On("ListAccessKeysBulk", mock.Anything, mock.Anything, mock.Anything).Return(bulkSvcResp, nil)

	mockFactory := new(MockMinioFactory)
	mockFactory.On("NewAdminClient", mock.Anything).Return(mockClient, nil)

	handler := handlers.NewDashboardHandler(mockFactory)

	// Create auth cookie
	creds := services.Credentials{
		Endpoint:  "localhost:9000",
		AccessKey: "admin",
		SecretKey: "admin",
	}

	cValue := "some-cookie"
	cookie := &http.Cookie{
		Name:     "session",
		Value:    cValue,
		Path:     "/",
		Expires:  time.Now().Add(24 * time.Hour),
		HttpOnly: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/dashboard/users", nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.Set(utils.ContextKeyCreds, &creds)

		err := handler.GetUsersWidget(c)
		if err != nil {
			b.Fatal(err)
		}
	}
}
