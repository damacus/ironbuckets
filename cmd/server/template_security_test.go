package main

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplatesUsePinnedCDNVersions(t *testing.T) {
	files := []string{
		"../../views/layouts/base.html",
		"../../views/pages/login.html",
		"../../views/pages/browser.html",
	}

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		require.NoError(t, err)
		content := string(contentBytes)

		assert.NotContains(t, content, "@latest", file)
		assert.NotContains(t, content, "3.x.x", file)
	}
}

func TestInteractiveTemplatesAttachCSRFHeaderForHTMX(t *testing.T) {
	files := []string{
		"../../views/layouts/base.html",
		"../../views/pages/login.html",
		"../../views/pages/browser.html",
	}

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		require.NoError(t, err)
		content := string(contentBytes)

		assert.True(t, strings.Contains(content, "htmx:configRequest") && strings.Contains(content, "X-CSRF-Token"), file)
	}
}
