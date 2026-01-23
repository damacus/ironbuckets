package services

import (
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestListObjectsResult_Structure(t *testing.T) {
	result := ListObjectsResult{
		Objects:               []minio.ObjectInfo{},
		IsTruncated:           false,
		NextContinuationToken: "",
	}

	if result.IsTruncated {
		t.Error("expected IsTruncated to be false")
	}
	if result.NextContinuationToken != "" {
		t.Error("expected NextContinuationToken to be empty")
	}
	if len(result.Objects) != 0 {
		t.Error("expected Objects to be empty")
	}
}

func TestListObjectsOptions_Structure(t *testing.T) {
	opts := ListObjectsOptions{
		Prefix:            "folder/",
		Recursive:         true,
		MaxKeys:           50,
		ContinuationToken: "last-key",
	}

	if opts.Prefix != "folder/" {
		t.Errorf("expected Prefix to be 'folder/', got %s", opts.Prefix)
	}
	if !opts.Recursive {
		t.Error("expected Recursive to be true")
	}
	if opts.MaxKeys != 50 {
		t.Errorf("expected MaxKeys to be 50, got %d", opts.MaxKeys)
	}
	if opts.ContinuationToken != "last-key" {
		t.Errorf("expected ContinuationToken to be 'last-key', got %s", opts.ContinuationToken)
	}
}

func TestDefaultPageSize(t *testing.T) {
	if DefaultPageSize != 100 {
		t.Errorf("expected DefaultPageSize to be 100, got %d", DefaultPageSize)
	}
}

func TestListObjectsResult_WithTruncation(t *testing.T) {
	objects := []minio.ObjectInfo{
		{Key: "file1.txt"},
		{Key: "file2.txt"},
	}

	result := ListObjectsResult{
		Objects:               objects,
		IsTruncated:           true,
		NextContinuationToken: "file2.txt",
	}

	if !result.IsTruncated {
		t.Error("expected IsTruncated to be true")
	}
	if result.NextContinuationToken != "file2.txt" {
		t.Errorf("expected NextContinuationToken to be 'file2.txt', got %s", result.NextContinuationToken)
	}
	if len(result.Objects) != 2 {
		t.Errorf("expected 2 objects, got %d", len(result.Objects))
	}
}
