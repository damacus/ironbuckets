package services

import (
	"testing"

	"github.com/minio/minio-go/v7"
)

func TestCollectPaginatedObjects_NotTruncatedWhenExactlyMaxKeys(t *testing.T) {
	stream := make(chan minio.ObjectInfo, 2)
	stream <- minio.ObjectInfo{Key: "a.txt"}
	stream <- minio.ObjectInfo{Key: "b.txt"}
	close(stream)

	objects, truncated, nextToken, err := collectPaginatedObjects(stream, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if truncated {
		t.Fatalf("expected not truncated")
	}
	if nextToken != "" {
		t.Fatalf("expected empty token, got %q", nextToken)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
}

func TestCollectPaginatedObjects_TruncatedWhenMoreThanMaxKeys(t *testing.T) {
	stream := make(chan minio.ObjectInfo, 3)
	stream <- minio.ObjectInfo{Key: "a.txt"}
	stream <- minio.ObjectInfo{Key: "b.txt"}
	stream <- minio.ObjectInfo{Key: "c.txt"}
	close(stream)

	objects, truncated, nextToken, err := collectPaginatedObjects(stream, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !truncated {
		t.Fatalf("expected truncated")
	}
	if nextToken != "b.txt" {
		t.Fatalf("expected next token b.txt, got %q", nextToken)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
}

func TestCollectPaginatedObjects_PropagatesStreamError(t *testing.T) {
	stream := make(chan minio.ObjectInfo, 2)
	stream <- minio.ObjectInfo{Key: "a.txt"}
	stream <- minio.ObjectInfo{Err: assertErr{}}
	close(stream)

	_, _, _, err := collectPaginatedObjects(stream, 2)
	if err == nil {
		t.Fatalf("expected error")
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }
