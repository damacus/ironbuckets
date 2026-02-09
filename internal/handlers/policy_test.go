package handlers

import (
	"errors"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
)

func TestIsNoSuchBucketPolicy(t *testing.T) {
	t.Run("matches by error code", func(t *testing.T) {
		err := minio.ErrorResponse{
			Code:       "NoSuchBucketPolicy",
			StatusCode: 403,
		}
		assert.True(t, isNoSuchBucketPolicy(err))
	})

	t.Run("matches by status code", func(t *testing.T) {
		err := minio.ErrorResponse{
			Code:       "AccessDenied",
			StatusCode: 404,
		}
		assert.True(t, isNoSuchBucketPolicy(err))
	})

	t.Run("non matching error", func(t *testing.T) {
		assert.False(t, isNoSuchBucketPolicy(errors.New("network timeout")))
	})

	t.Run("nil error", func(t *testing.T) {
		assert.False(t, isNoSuchBucketPolicy(nil))
	})
}

func TestCanonicalJSON(t *testing.T) {
	t.Run("valid json", func(t *testing.T) {
		out, err := canonicalJSON(`{"Statement":[{"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::photos/*"],"Effect":"Allow","Principal":{"AWS":["*"]}}],"Version":"2012-10-17"}`)
		assert.NoError(t, err)
		assert.Contains(t, out, "\"Version\":\"2012-10-17\"")
		assert.Contains(t, out, "\"Action\":[\"s3:GetObject\"]")
	})

	t.Run("invalid json", func(t *testing.T) {
		_, err := canonicalJSON("{bad")
		assert.Error(t, err)
	})
}

func TestDetectPolicyType(t *testing.T) {
	bucketName := "photos"

	t.Run("empty policy is private", func(t *testing.T) {
		assert.Equal(t, "private", detectPolicyType("", bucketName))
	})

	t.Run("public read", func(t *testing.T) {
		policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::photos/*"]}]}`
		assert.Equal(t, "public-read", detectPolicyType(policy, bucketName))
	})

	t.Run("public read write", func(t *testing.T) {
		policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject","s3:PutObject","s3:DeleteObject"],"Resource":["arn:aws:s3:::photos/*"]}]}`
		assert.Equal(t, "public-read-write", detectPolicyType(policy, bucketName))
	})

	t.Run("custom policy", func(t *testing.T) {
		policy := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["arn:aws:iam::123456789012:user/app"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::photos/*"]}]}`
		assert.Equal(t, "custom", detectPolicyType(policy, bucketName))
	})
}
