package services

import (
	"context"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/notification"
	"github.com/minio/minio-go/v7/pkg/replication"
	"github.com/minio/minio-go/v7/pkg/tags"
)

// DefaultPageSize is the default number of objects to return per page
const DefaultPageSize = 100

// ListObjectsOptions extends minio.ListObjectsOptions with pagination
type ListObjectsOptions struct {
	Prefix            string
	Recursive         bool
	MaxKeys           int
	ContinuationToken string
}

// ListObjectsResult contains paginated results from ListObjectsPaginated
type ListObjectsResult struct {
	Objects               []minio.ObjectInfo
	IsTruncated           bool
	NextContinuationToken string
}

// MinioAdminClient is an interface for the madmin methods we use
type MinioAdminClient interface {
	ServerInfo(ctx context.Context, opts ...func(*madmin.ServerInfoOpts)) (madmin.InfoMessage, error)
	ListUsers(ctx context.Context) (map[string]madmin.UserInfo, error)
	AddUser(ctx context.Context, accessKey, secretKey string) error
	RemoveUser(ctx context.Context, accessKey string) error
	SetPolicy(ctx context.Context, policyName, entityName string, isGroup bool) error
	SetUserStatus(ctx context.Context, accessKey string, status madmin.AccountStatus) error
	ServiceRestart(ctx context.Context) error
	DataUsageInfo(ctx context.Context) (madmin.DataUsageInfo, error)
	GetConfig(ctx context.Context) ([]byte, error)

	// Service Account methods
	ListServiceAccounts(ctx context.Context, user string) (madmin.ListServiceAccountsResp, error)
	AddServiceAccount(ctx context.Context, opts madmin.AddServiceAccountReq) (madmin.Credentials, error)
	DeleteServiceAccount(ctx context.Context, serviceAccount string) error

	// Policy methods
	ListCannedPolicies(ctx context.Context) (map[string]json.RawMessage, error)
	InfoCannedPolicyV2(ctx context.Context, policyName string) (*madmin.PolicyInfo, error)

	// Logs
	GetLogs(ctx context.Context, node string, lineCnt int, logKind string) <-chan madmin.LogInfo

	// Quota
	GetBucketQuota(ctx context.Context, bucket string) (madmin.BucketQuota, error)
	SetBucketQuota(ctx context.Context, bucket string, quota *madmin.BucketQuota) error

	// Group methods
	ListGroups(ctx context.Context) ([]string, error)
	GetGroupDescription(ctx context.Context, group string) (*madmin.GroupDesc, error)
	UpdateGroupMembers(ctx context.Context, req madmin.GroupAddRemove) error
	SetGroupStatus(ctx context.Context, group string, status madmin.GroupStatus) error
}

// MinioClient is an interface for the standard S3 methods we use
type MinioClient interface {
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	RemoveBucket(ctx context.Context, bucketName string) error

	// Object Operations
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) ([]minio.ObjectInfo, error)
	ListObjectsPaginated(ctx context.Context, bucketName string, opts ListObjectsOptions) (ListObjectsResult, error)
	ListObjectsChannel(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	GetObjectReader(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, int64, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error

	// Presigned URLs
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)

	// Versioning
	GetBucketVersioning(ctx context.Context, bucketName string) (minio.BucketVersioningConfiguration, error)
	SetBucketVersioning(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error

	// Lifecycle
	GetBucketLifecycle(ctx context.Context, bucketName string) (*lifecycle.Configuration, error)
	SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error

	// Object Metadata & Tags
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error)
	PutObjectTagging(ctx context.Context, bucketName, objectName string, otags *tags.Tags, opts minio.PutObjectTaggingOptions) error

	// Notifications
	GetBucketNotification(ctx context.Context, bucketName string) (notification.Configuration, error)
	SetBucketNotification(ctx context.Context, bucketName string, config notification.Configuration) error

	// Replication
	GetBucketReplication(ctx context.Context, bucketName string) (replication.Config, error)
}

// MinioClientFactory creates authenticated clients
type MinioClientFactory interface {
	NewAdminClient(creds Credentials) (MinioAdminClient, error)
	NewClient(creds Credentials) (MinioClient, error)
}

// WrappedMinioClient wraps minio.Client to implement our interface
type WrappedMinioClient struct {
	client *minio.Client
}

func (c *WrappedMinioClient) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	return c.client.ListBuckets(ctx)
}

func (c *WrappedMinioClient) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return c.client.MakeBucket(ctx, bucketName, opts)
}

func (c *WrappedMinioClient) RemoveBucket(ctx context.Context, bucketName string) error {
	return c.client.RemoveBucket(ctx, bucketName)
}

func (c *WrappedMinioClient) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) ([]minio.ObjectInfo, error) {
	// Convert channel to slice
	var objects []minio.ObjectInfo
	for obj := range c.client.ListObjects(ctx, bucketName, opts) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		objects = append(objects, obj)
	}
	return objects, nil
}

func (c *WrappedMinioClient) ListObjectsPaginated(ctx context.Context, bucketName string, opts ListObjectsOptions) (ListObjectsResult, error) {
	maxKeys := opts.MaxKeys
	if maxKeys <= 0 {
		maxKeys = DefaultPageSize
	}

	minioOpts := minio.ListObjectsOptions{
		Prefix:    opts.Prefix,
		Recursive: opts.Recursive,
	}

	// Use StartAfter for continuation (MinIO uses marker-based pagination)
	if opts.ContinuationToken != "" {
		minioOpts.StartAfter = opts.ContinuationToken
	}

	var objects []minio.ObjectInfo
	var lastKey string

	for obj := range c.client.ListObjects(ctx, bucketName, minioOpts) {
		if obj.Err != nil {
			return ListObjectsResult{}, obj.Err
		}

		objects = append(objects, obj)
		lastKey = obj.Key

		// Stop after maxKeys objects
		if len(objects) >= maxKeys {
			break
		}
	}

	result := ListObjectsResult{
		Objects:     objects,
		IsTruncated: len(objects) >= maxKeys,
	}

	if result.IsTruncated {
		result.NextContinuationToken = lastKey
	}

	return result, nil
}

func (c *WrappedMinioClient) ListObjectsChannel(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	return c.client.ListObjects(ctx, bucketName, opts)
}

func (c *WrappedMinioClient) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	return c.client.PutObject(ctx, bucketName, objectName, reader, objectSize, opts)
}

func (c *WrappedMinioClient) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	return c.client.GetObject(ctx, bucketName, objectName, opts)
}

func (c *WrappedMinioClient) GetObjectReader(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (io.ReadCloser, int64, error) {
	obj, err := c.client.GetObject(ctx, bucketName, objectName, opts)
	if err != nil {
		return nil, 0, err
	}
	info, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, 0, err
	}
	return obj, info.Size, nil
}

func (c *WrappedMinioClient) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return c.client.RemoveObject(ctx, bucketName, objectName, opts)
}

func (c *WrappedMinioClient) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	return c.client.PresignedGetObject(ctx, bucketName, objectName, expires, reqParams)
}

func (c *WrappedMinioClient) GetBucketVersioning(ctx context.Context, bucketName string) (minio.BucketVersioningConfiguration, error) {
	return c.client.GetBucketVersioning(ctx, bucketName)
}

func (c *WrappedMinioClient) SetBucketVersioning(ctx context.Context, bucketName string, config minio.BucketVersioningConfiguration) error {
	return c.client.SetBucketVersioning(ctx, bucketName, config)
}

func (c *WrappedMinioClient) GetBucketLifecycle(ctx context.Context, bucketName string) (*lifecycle.Configuration, error) {
	return c.client.GetBucketLifecycle(ctx, bucketName)
}

func (c *WrappedMinioClient) SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
	return c.client.SetBucketLifecycle(ctx, bucketName, config)
}

func (c *WrappedMinioClient) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	return c.client.StatObject(ctx, bucketName, objectName, opts)
}

func (c *WrappedMinioClient) GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	return c.client.GetObjectTagging(ctx, bucketName, objectName, opts)
}

func (c *WrappedMinioClient) PutObjectTagging(ctx context.Context, bucketName, objectName string, otags *tags.Tags, opts minio.PutObjectTaggingOptions) error {
	return c.client.PutObjectTagging(ctx, bucketName, objectName, otags, opts)
}

func (c *WrappedMinioClient) GetBucketNotification(ctx context.Context, bucketName string) (notification.Configuration, error) {
	return c.client.GetBucketNotification(ctx, bucketName)
}

func (c *WrappedMinioClient) SetBucketNotification(ctx context.Context, bucketName string, config notification.Configuration) error {
	return c.client.SetBucketNotification(ctx, bucketName, config)
}

func (c *WrappedMinioClient) GetBucketReplication(ctx context.Context, bucketName string) (replication.Config, error) {
	return c.client.GetBucketReplication(ctx, bucketName)
}

// RealMinioFactory is the production implementation
type RealMinioFactory struct{}

// shouldUseSSL determines if SSL should be used based on the endpoint.
// Returns false for localhost, 127.0.0.1, and docker service names.
func shouldUseSSL(endpoint string) bool {
	// Local development endpoints
	if endpoint == "localhost:9000" || endpoint == "127.0.0.1:9000" {
		return false
	}
	// Docker service names (minio:9000, minio1:9000, minio2:9000, etc.)
	// Only match simple hostnames without dots (not domain names like minio.example.com)
	if strings.HasPrefix(endpoint, "minio") && !strings.Contains(strings.Split(endpoint, ":")[0], ".") && strings.Contains(endpoint, ":9000") {
		return false
	}
	return true
}

func (f *RealMinioFactory) NewAdminClient(creds Credentials) (MinioAdminClient, error) {
	return madmin.NewWithOptions(creds.Endpoint, &madmin.Options{
		Creds:  credentials.NewStaticV4(creds.AccessKey, creds.SecretKey, ""),
		Secure: shouldUseSSL(creds.Endpoint),
	})
}

func (f *RealMinioFactory) NewClient(creds Credentials) (MinioClient, error) {
	client, err := minio.New(creds.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(creds.AccessKey, creds.SecretKey, creds.SessionToken),
		Secure: shouldUseSSL(creds.Endpoint),
	})
	if err != nil {
		return nil, err
	}
	return &WrappedMinioClient{client: client}, nil
}
