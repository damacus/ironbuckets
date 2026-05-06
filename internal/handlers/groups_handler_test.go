package handlers

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type groupsTestAdminClient struct {
	listGroupsResponse     []string
	getGroupDescResponse   *madmin.GroupDesc
	listUsersResponse      map[string]madmin.UserInfo
	listCannedPoliciesResp map[string]json.RawMessage

	updateGroupMembersCalls []madmin.GroupAddRemove
	setGroupStatusCalls     []struct {
		group  string
		status madmin.GroupStatus
	}
	setPolicyCalls []struct {
		policyName, entityName string
		isGroup                bool
	}
}

func (m *groupsTestAdminClient) ServerInfo(ctx context.Context, opts ...func(*madmin.ServerInfoOpts)) (madmin.InfoMessage, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	return m.listUsersResponse, nil
}
func (m *groupsTestAdminClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	panic("unexpected")
}
func (m *groupsTestAdminClient) RemoveUser(ctx context.Context, accessKey string) error {
	panic("unexpected")
}
func (m *groupsTestAdminClient) SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error {
	m.setPolicyCalls = append(m.setPolicyCalls, struct {
		policyName, entityName string
		isGroup                bool
	}{policyName, entityName, isGroup})
	return nil
}
func (m *groupsTestAdminClient) SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error {
	panic("unexpected")
}
func (m *groupsTestAdminClient) ServiceRestart(ctx context.Context) error { panic("unexpected") }
func (m *groupsTestAdminClient) DataUsageInfo(ctx context.Context) (madmin.DataUsageInfo, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) GetConfig(ctx context.Context) ([]byte, error) { panic("unexpected") }
func (m *groupsTestAdminClient) ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) ListAccessKeysBulk(ctx context.Context, users []string, opts madmin.ListAccessKeysOpts) (map[string]madmin.ListAccessKeysResp, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) DeleteServiceAccount(ctx context.Context, serviceAccount string) error {
	panic("unexpected")
}
func (m *groupsTestAdminClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	return m.listCannedPoliciesResp, nil
}
func (m *groupsTestAdminClient) InfoCannedPolicyV2(ctx context.Context, policyName string) (*madmin.PolicyInfo, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan madmin.LogInfo {
	panic("unexpected")
}
func (m *groupsTestAdminClient) GetBucketQuota(ctx context.Context, bucket string) (madmin.BucketQuota, error) {
	panic("unexpected")
}
func (m *groupsTestAdminClient) SetBucketQuota(ctx context.Context, bucket string, quota *madmin.BucketQuota) error {
	panic("unexpected")
}
func (m *groupsTestAdminClient) ListGroups(ctx context.Context) ([]string, error) {
	return m.listGroupsResponse, nil
}
func (m *groupsTestAdminClient) GetGroupDescription(ctx context.Context, group string) (*madmin.GroupDesc, error) {
	return m.getGroupDescResponse, nil
}
func (m *groupsTestAdminClient) UpdateGroupMembers(ctx context.Context, req madmin.GroupAddRemove) error {
	m.updateGroupMembersCalls = append(m.updateGroupMembersCalls, req)
	return nil
}
func (m *groupsTestAdminClient) SetGroupStatus(ctx context.Context, group string, status madmin.GroupStatus) error {
	m.setGroupStatusCalls = append(m.setGroupStatusCalls, struct {
		group  string
		status madmin.GroupStatus
	}{group, status})
	return nil
}

type groupsTestFactory struct {
	adminClient services.MinioAdminClient
}

func (f *groupsTestFactory) NewAdminClient(creds services.Credentials) (services.MinioAdminClient, error) {
	return f.adminClient, nil
}

func (f *groupsTestFactory) NewClient(creds services.Credentials) (services.MinioClient, error) {
	panic("unexpected test call")
}

type mockRenderer struct {
	RenderedTemplate string
	RenderedData     interface{}
}

func (m *mockRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	m.RenderedTemplate = name
	m.RenderedData = data
	return nil
}

func TestListGroups_Success(t *testing.T) {
	e := echo.New()
	renderer := &mockRenderer{}
	e.Renderer = renderer
	req := httptest.NewRequest(http.MethodGet, "/groups", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{
		listGroupsResponse: []string{"group1", "group2"},
		getGroupDescResponse: &madmin.GroupDesc{
			Name:    "group1",
			Members: []string{"user1", "user2"},
			Policy:  "readwrite",
			Status:  string(madmin.GroupEnabled),
		},
	}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.ListGroups(c)
	require.NoError(t, err)

	assert.Equal(t, "groups", renderer.RenderedTemplate)
	data, ok := renderer.RenderedData.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "groups", data["ActiveNav"])
	groups, ok := data["Groups"].([]GroupInfo)
	require.True(t, ok)
	assert.Len(t, groups, 2)
	assert.Equal(t, "group1", groups[0].Name)
	assert.Equal(t, 2, groups[0].MemberCount)
}

func TestCreateGroupModal_Success(t *testing.T) {
	e := echo.New()
	renderer := &mockRenderer{}
	e.Renderer = renderer
	req := httptest.NewRequest(http.MethodGet, "/groups/new", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{
		listUsersResponse: map[string]madmin.UserInfo{
			"user1": {},
			"user2": {},
		},
		listCannedPoliciesResp: map[string]json.RawMessage{
			"readwrite": json.RawMessage(`{}`),
		},
	}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.CreateGroupModal(c)
	require.NoError(t, err)

	assert.Equal(t, "group_create_modal", renderer.RenderedTemplate)
	data, ok := renderer.RenderedData.(map[string]interface{})
	require.True(t, ok)

	users, ok := data["Users"].([]string)
	require.True(t, ok)
	assert.Len(t, users, 2)
	assert.Contains(t, users, "user1")
	assert.Contains(t, users, "user2")

	policies, ok := data["Policies"].([]string)
	require.True(t, ok)
	assert.Len(t, policies, 1)
	assert.Contains(t, policies, "readwrite")
}

func TestViewGroup_Success(t *testing.T) {
	e := echo.New()
	renderer := &mockRenderer{}
	e.Renderer = renderer
	req := httptest.NewRequest(http.MethodGet, "/groups/group1", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{
		getGroupDescResponse: &madmin.GroupDesc{
			Name:    "group1",
			Members: []string{"user1"},
		},
		listUsersResponse: map[string]madmin.UserInfo{
			"user1": {},
			"user2": {},
		},
		listCannedPoliciesResp: map[string]json.RawMessage{
			"readwrite": json.RawMessage(`{}`),
		},
	}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.ViewGroup(c)
	require.NoError(t, err)

	assert.Equal(t, "group_detail", renderer.RenderedTemplate)
	data, ok := renderer.RenderedData.(map[string]interface{})
	require.True(t, ok)

	assert.Equal(t, "groups", data["ActiveNav"])

	group, ok := data["Group"].(*madmin.GroupDesc)
	require.True(t, ok)
	assert.Equal(t, "group1", group.Name)

	availableUsers, ok := data["AvailableUsers"].([]string)
	require.True(t, ok)
	assert.Len(t, availableUsers, 1)
	assert.Contains(t, availableUsers, "user2")

	policies, ok := data["Policies"].([]string)
	require.True(t, ok)
	assert.Len(t, policies, 1)
	assert.Contains(t, policies, "readwrite")
}

func TestCreateGroup_Success(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("groupName", "newgroup")
	form.Set("members", "user1, user2")
	req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.CreateGroup(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.updateGroupMembersCalls, 1)
	reqObj := adminClient.updateGroupMembersCalls[0]
	assert.Equal(t, "newgroup", reqObj.Group)
	assert.Equal(t, []string{"user1", "user2"}, reqObj.Members)
	assert.Equal(t, madmin.GroupEnabled, reqObj.Status)
	assert.False(t, reqObj.IsRemove)

	assert.Equal(t, "/groups", rec.Header().Get("HX-Redirect"))
}

func TestCreateGroup_WithPolicy(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("groupName", "newgroup")
	form.Set("members", "")
	form.Set("policy", "readwrite")
	req := httptest.NewRequest(http.MethodPost, "/groups", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.CreateGroup(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.updateGroupMembersCalls, 1)
	assert.Len(t, adminClient.setPolicyCalls, 1)
	assert.Equal(t, "readwrite", adminClient.setPolicyCalls[0].policyName)
	assert.Equal(t, "newgroup", adminClient.setPolicyCalls[0].entityName)
	assert.True(t, adminClient.setPolicyCalls[0].isGroup)

	assert.Equal(t, "/groups", rec.Header().Get("HX-Redirect"))
}

func TestAddMembers_Success(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("members", "user3, user4")
	req := httptest.NewRequest(http.MethodPost, "/groups/group1/members", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.AddMembers(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.updateGroupMembersCalls, 1)
	reqObj := adminClient.updateGroupMembersCalls[0]
	assert.Equal(t, "group1", reqObj.Group)
	assert.Equal(t, []string{"user3", "user4"}, reqObj.Members)
	assert.False(t, reqObj.IsRemove)

	assert.Equal(t, "/groups/group1", rec.Header().Get("HX-Redirect"))
}

func TestRemoveMembers_Success(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("members", "user1")
	req := httptest.NewRequest(http.MethodPost, "/groups/group1/members/remove", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.RemoveMembers(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.updateGroupMembersCalls, 1)
	reqObj := adminClient.updateGroupMembersCalls[0]
	assert.Equal(t, "group1", reqObj.Group)
	assert.Equal(t, []string{"user1"}, reqObj.Members)
	assert.True(t, reqObj.IsRemove)

	assert.Equal(t, "/groups/group1", rec.Header().Get("HX-Redirect"))
}

func TestDisableGroup_Success(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/groups/group1/disable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.DisableGroup(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.setGroupStatusCalls, 1)
	assert.Equal(t, "group1", adminClient.setGroupStatusCalls[0].group)
	assert.Equal(t, madmin.GroupDisabled, adminClient.setGroupStatusCalls[0].status)

	assert.Equal(t, "/groups", rec.Header().Get("HX-Redirect"))
}

func TestEnableGroup_Success(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/groups/group1/enable", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.EnableGroup(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.setGroupStatusCalls, 1)
	assert.Equal(t, "group1", adminClient.setGroupStatusCalls[0].group)
	assert.Equal(t, madmin.GroupEnabled, adminClient.setGroupStatusCalls[0].status)

	assert.Equal(t, "/groups", rec.Header().Get("HX-Redirect"))
}

func TestAttachGroupPolicy_Success(t *testing.T) {
	e := echo.New()
	form := url.Values{}
	form.Set("policy", "readwrite")
	req := httptest.NewRequest(http.MethodPost, "/groups/group1/policy", strings.NewReader(form.Encode()))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("groupName")
	c.SetParamValues("group1")
	c.Set(utils.ContextKeyCreds, &services.Credentials{})

	adminClient := &groupsTestAdminClient{}
	handler := NewGroupsHandler(&groupsTestFactory{adminClient: adminClient})

	err := handler.AttachPolicy(c)
	require.NoError(t, err)

	assert.Len(t, adminClient.setPolicyCalls, 1)
	assert.Equal(t, "readwrite", adminClient.setPolicyCalls[0].policyName)
	assert.Equal(t, "group1", adminClient.setPolicyCalls[0].entityName)
	assert.True(t, adminClient.setPolicyCalls[0].isGroup)

	assert.Equal(t, "/groups/group1", rec.Header().Get("HX-Redirect"))
}
