package handlers

import "testing"

func TestIsValidBucketName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{name: "valid simple", input: "my-bucket", expected: true},
		{name: "valid dotted", input: "my.bucket.name", expected: true},
		{name: "too short", input: "ab", expected: false},
		{name: "too long", input: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", expected: false},
		{name: "uppercase", input: "MyBucket", expected: false},
		{name: "underscore", input: "my_bucket", expected: false},
		{name: "leading hyphen", input: "-bucket", expected: false},
		{name: "trailing hyphen", input: "bucket-", expected: false},
		{name: "leading dot", input: ".bucket", expected: false},
		{name: "trailing dot", input: "bucket.", expected: false},
		{name: "consecutive dots", input: "bucket..name", expected: false},
		{name: "dot hyphen pair", input: "bucket.-name", expected: false},
		{name: "hyphen dot pair", input: "bucket-.name", expected: false},
		{name: "ip format", input: "192.168.1.1", expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := isValidBucketName(tt.input)
			if actual != tt.expected {
				t.Fatalf("isValidBucketName(%q) = %v, want %v", tt.input, actual, tt.expected)
			}
		})
	}
}
