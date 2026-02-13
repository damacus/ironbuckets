package main

import "testing"

func TestResolveMinioEndpoint_UsesLocalhostFallbackWhenUnset(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "")

	endpoint, usedDefault := resolveMinioEndpoint()
	if endpoint != "localhost:9000" {
		t.Fatalf("expected localhost fallback, got %q", endpoint)
	}
	if !usedDefault {
		t.Fatal("expected usedDefault=true when env var is unset")
	}
}

func TestResolveMinioEndpoint_UsesConfiguredValue(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "minio.example.internal:9000")

	endpoint, usedDefault := resolveMinioEndpoint()
	if endpoint != "minio.example.internal:9000" {
		t.Fatalf("expected configured endpoint, got %q", endpoint)
	}
	if usedDefault {
		t.Fatal("expected usedDefault=false when env var is set")
	}
}
