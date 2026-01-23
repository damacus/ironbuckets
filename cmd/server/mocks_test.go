package main

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"time"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/labstack/echo/v4"
	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/replication"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/stretchr/testify/mock"
)

// MockMinioClient implements both MinioClient and MinioAdminClient interfaces for testing
type MockMinioClient struct {
	mock.Mock
}

// MinioAdminClient methods

func (m *MockMinioClient) ServerInfo(ctx context.Context, opts ...func(*madmin.ServerInfoOpts)) (madmin.InfoMessage, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(madmin.InfoMessage), args.Error(1)
}

func (m *MockMinioClient) ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]madmin.UserInfo), args.Error(1)
}

func (m *MockMinioClient) AddUser(ctx context.Context, accessKey, secretKey string) error {
	args := m.Called(ctx, accessKey, secretKey)
	return args.Error(0)
}

func (m *MockMinioClient) RemoveUser(ctx context.Context, accessKey string) error {
	args := m.Called(ctx, accessKey)
	return args.Error(0)
}

func (m *MockMinioClient) SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error {
	args := m.Called(ctx, policyName, entityName, isGroup)
	return args.Error(0)
}

func (m *MockMinioClient) SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error {
	args := m.Called(ctx, accessKey, status)
	return args.Error(0)
}

func (m *MockMinioClient) ServiceRestart(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockMinioClient) DataUsageInfo(ctx context.Context) (madmin.DataUsageInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).(madmin.DataUsageInfo), args.Error(1)
}

func (m *MockMinioClient) GetConfig(ctx context.Context) ([]byte, error) {
	args := m.Called(ctx)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockMinioClient) ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error) {
	args := m.Called(ctx, user)
	return args.Get(0).(madmin.ListServiceAccountsResp), args.Error(1)
}

func (m *MockMinioClient) AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error) {
	args := m.Called(ctx, opts)
	return args.Get(0).(madmin.Credentials), args.Error(1)
}

func (m *MockMinioClient) DeleteServiceAccount(ctx context.Context, serviceAccount string) error {
	args := m.Called(ctx, serviceAccount)
	return args.Error(0)
}

func (m *MockMinioClient) ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]json.RawMessage), args.Error(1)
}

func (m *MockMinioClient) InfoCannedPolicyV2(ctx context.Context, policyName string) (*madmin.PolicyInfo, error) {
	args := m.Called(ctx, policyName)
	return args.Get(0).(*madmin.PolicyInfo), args.Error(1)
}

func (m *MockMinioClient) ListGroups(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockMinioClient) GetGroupDescription(ctx context.Context, group string) (*madmin.GroupDesc, error) {
	args := m.Called(ctx, group)
	return args.Get(0).(*madmin.GroupDesc), args.Error(1)
}

func (m *MockMinioClient) UpdateGroupMembers(ctx context.Context, req madmin.GroupAddRemove) error {
	args := m.Called(ctx, req)
	return args.Error(0)
}

func (m *MockMinioClient) SetGroupStatus(ctx context.Context, group string, status madmin.GroupStatus) error {
	args := m.Called(ctx, group, status)
	return args.Error(0)
}

func (m *MockMinioClient) GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan madmin.LogInfo {
	args := m.Called(ctx, node, lineCnt, logKind)
	return args.Get(0).(<-chan madmin.LogInfo)
}

func (m *MockMinioClient) GetBucketQuota(ctx context.Context, bucket string) (madmin.BucketQuota, error) {
	args := m.Called(ctx, bucket)
	return args.Get(0).(madmin.BucketQuota), args.Error(1)
}

func (m *MockMinioClient) SetBucketQuota(ctx context.Context, bucket string, quota *madmin.BucketQuota) error {
	args := m.Called(ctx, bucket, quota)
	return args.Error(0)
}

// MinioClient methods

func (m *MockMinioClient) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]minio.BucketInfo), args.Error(1)
}

func (m *MockMinioClient) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	args := m.Called(ctx, bucketName, opts)
	return args.Error(0)
}

func (m *MockMinioClient) RemoveBucket(ctx context.Context, bucketName string) error {
	args := m.Called(ctx, bucketName)
	return args.Error(0)
}

func (m *MockMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) ([]minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).([]minio.ObjectInfo), args.Error(1)
}

func (m *MockMinioClient) ListObjectsPaginated(ctx context.Context, bucketName string, opts services.ListObjectsOptions) (services.ListObjectsResult, error) {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).(services.ListObjectsResult), args.Error(1)
}

func (m *MockMinioClient) ListObjectsChannel(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).(<-chan minio.ObjectInfo)
}

func (m *MockMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *MockMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(*minio.Object), args.Error(1)
}

func (m *MockMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

func (m *MockMinioClient) GetObjectReader(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, int64, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(io.ReadCloser), args.Get(1).(int64), args.Error(2)
}

func (m *MockMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires, reqParams)
	return args.Get(0).(*url.URL), args.Error(1)
}

func (m *MockMinioClient) GetBucketVersioning(ctx context.Context, bucketName string) (minio.BucketVersioningConfiguration, error) {
	args := m.Called(ctx, bucketName)
	return args.Get(0).(minio.BucketVersioningConfiguration), args.Error(1)
}

func (m *MockMinioClient) SetBucketVersioning(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error {
	args := m.Called(ctx, bucketName, config)
	return args.Error(0)
}

func (m *MockMinioClient) GetBucketLifecycle(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
	args := m.Called(ctx, bucketName)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*lifecycle.Configuration), args.Error(1)
}

func (m *MockMinioClient) SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
	args := m.Called(ctx, bucketName, config)
	return args.Error(0)
}

func (m *MockMinioClient) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}

func (m *MockMinioClient) GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*tags.Tags), args.Error(1)
}

func (m *MockMinioClient) PutObjectTagging(ctx context.Context, bucketName, objectName string, otags *tags.Tags, opts minio.PutObjectTaggingOptions) error {
	args := m.Called(ctx, bucketName, objectName, otags, opts)
	return args.Error(0)
}

func (m *MockMinioClient) GetBucketNotification(ctx context.Context, bucketName string) (notification.Configuration, error) {
	args := m.Called(ctx, bucketName)
	return args.Get(0).(notification.Configuration), args.Error(1)
}

func (m *MockMinioClient) SetBucketNotification(ctx context.Context, bucketName string, config notification.Configuration) error {
	args := m.Called(ctx, bucketName, config)
	return args.Error(0)
}

func (m *MockMinioClient) GetBucketReplication(ctx context.Context, bucketName string) (replication.Config, error) {
	args := m.Called(ctx, bucketName)
	return args.Get(0).(replication.Config), args.Error(1)
}

// MockMinioFactory implements MinioClientFactory for testing
type MockMinioFactory struct {
	mock.Mock
}

func (m *MockMinioFactory) NewAdminClient(creds services.Credentials) (services.MinioAdminClient, error) {
	args := m.Called(creds)
	return args.Get(0).(services.MinioAdminClient), args.Error(1)
}

func (m *MockMinioFactory) NewClient(creds services.Credentials) (services.MinioClient, error) {
	args := m.Called(creds)
	return args.Get(0).(services.MinioClient), args.Error(1)
}

// MockRenderer implements echo.Renderer for testing
type MockRenderer struct{}

func (r *MockRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return nil // Successfully "rendered" nothing
}
