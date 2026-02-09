package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/minio/minio-go/v7"
)

const (
	policyTypePrivate         = "private"
	policyTypePublicRead      = "public-read"
	policyTypePublicReadWrite = "public-read-write"
	policyTypeCustom          = "custom"
	policyTypeUnknown         = "unknown"
)

type bucketPolicyState struct {
	Policy     string
	PolicyType string
	Error      string
}

type bucketPolicyStatement struct {
	Effect    string          `json:"Effect"`
	Principal json.RawMessage `json:"Principal"`
	Action    json.RawMessage `json:"Action"`
	Resource  []string        `json:"Resource"`
}

type bucketPolicyDocument struct {
	Version   string                  `json:"Version"`
	Statement []bucketPolicyStatement `json:"Statement"`
}

func isNoSuchBucketPolicy(err error) bool {
	if err == nil {
		return false
	}

	errResp := minio.ToErrorResponse(err)
	return errResp.Code == "NoSuchBucketPolicy" || errResp.StatusCode == http.StatusNotFound
}

func canonicalJSON(in string) (string, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(in), &raw); err != nil {
		return "", err
	}

	out, err := json.Marshal(raw)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

func detectPolicyType(policy string, bucketName string) string {
	policy = strings.TrimSpace(policy)
	if policy == "" {
		return policyTypePrivate
	}

	var doc bucketPolicyDocument
	if err := json.Unmarshal([]byte(policy), &doc); err != nil {
		return policyTypeCustom
	}

	if len(doc.Statement) != 1 {
		return policyTypeCustom
	}

	stmt := doc.Statement[0]
	if stmt.Effect != "Allow" || !isPublicPrincipal(stmt.Principal) {
		return policyTypeCustom
	}

	expectedResource := "arn:aws:s3:::" + bucketName + "/*"
	if len(stmt.Resource) != 1 || stmt.Resource[0] != expectedResource {
		return policyTypeCustom
	}

	actions := normalizeActions(stmt.Action)
	if len(actions) == 0 {
		return policyTypeCustom
	}
	slices.Sort(actions)

	switch {
	case slices.Equal(actions, []string{"s3:GetObject"}):
		return policyTypePublicRead
	case slices.Equal(actions, []string{"s3:DeleteObject", "s3:GetObject", "s3:PutObject"}):
		return policyTypePublicReadWrite
	default:
		return policyTypeCustom
	}
}

func normalizeActions(raw json.RawMessage) []string {
	var single string
	if err := json.Unmarshal(raw, &single); err == nil {
		return []string{single}
	}

	var list []string
	if err := json.Unmarshal(raw, &list); err == nil {
		return list
	}
	return nil
}

func isPublicPrincipal(raw json.RawMessage) bool {
	var direct string
	if err := json.Unmarshal(raw, &direct); err == nil {
		return direct == "*"
	}

	var principalMap map[string]interface{}
	if err := json.Unmarshal(raw, &principalMap); err != nil {
		return false
	}

	awsPrincipal, ok := principalMap["AWS"]
	if !ok {
		return false
	}

	switch v := awsPrincipal.(type) {
	case string:
		return v == "*"
	case []interface{}:
		return len(v) == 1 && v[0] == "*"
	default:
		return false
	}
}

func buildPolicyForType(policyType, bucketName, customPolicy string) (string, error) {
	switch policyType {
	case policyTypePrivate:
		return "", nil
	case policyTypePublicRead:
		return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucketName), nil
	case policyTypePublicReadWrite:
		return fmt.Sprintf(`{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject","s3:PutObject","s3:DeleteObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`, bucketName), nil
	case policyTypeCustom:
		policy, err := canonicalJSON(customPolicy)
		if err != nil {
			return "", fmt.Errorf("invalid custom policy JSON")
		}
		return policy, nil
	default:
		return "", fmt.Errorf("invalid policy type")
	}
}

func loadBucketPolicy(ctx context.Context, client services.MinioClient, bucketName string) bucketPolicyState {
	policy, err := client.GetBucketPolicy(ctx, bucketName)
	if err != nil {
		if isNoSuchBucketPolicy(err) {
			return bucketPolicyState{
				Policy:     "",
				PolicyType: policyTypePrivate,
			}
		}
		return bucketPolicyState{
			Policy:     "",
			PolicyType: policyTypeUnknown,
			Error:      fmt.Sprintf("Failed to get bucket policy: %s", err.Error()),
		}
	}

	return bucketPolicyState{
		Policy:     policy,
		PolicyType: detectPolicyType(policy, bucketName),
	}
}

func policyViewData(bucketName string, state bucketPolicyState) map[string]interface{} {
	formattedPolicy := ""
	if strings.TrimSpace(state.Policy) != "" {
		formatted, err := canonicalJSON(state.Policy)
		if err == nil {
			formattedPolicy = formatted
		} else {
			formattedPolicy = state.Policy
		}
	}

	return map[string]interface{}{
		"BucketName":      bucketName,
		"Policy":          state.Policy,
		"PolicyType":      state.PolicyType,
		"FormattedPolicy": formattedPolicy,
		"HasPolicy":       strings.TrimSpace(state.Policy) != "",
		"Error":           state.Error,
	}
}
