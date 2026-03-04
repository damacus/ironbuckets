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

func TestTemplatesUseCSPCompatibleAlpineBuild(t *testing.T) {
	files := []string{
		"../../views/layouts/base.html",
		"../../views/pages/browser.html",
	}

	for _, file := range files {
		contentBytes, err := os.ReadFile(file)
		require.NoError(t, err)
		content := string(contentBytes)

		// Alpine.js is vendored locally; the CSP build is served from /static
		assert.Contains(t, content, "alpine-csp.min.js", file)
		assert.NotContains(t, content, "alpinejs/dist/cdn.min.js", file) // non-CSP build must not be used
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

func TestBaseLayoutDoesNotGloballyOverrideHTMXTargeting(t *testing.T) {
	contentBytes, err := os.ReadFile("../../views/layouts/base.html")
	require.NoError(t, err)
	content := string(contentBytes)

	assert.NotContains(t, content, `hx-target="#main-content"`)
	assert.NotContains(t, content, `hx-select="#main-content"`)
	assert.NotContains(t, content, `hx-swap="outerHTML"`)
}

func TestBucketsPageHasSingleCreateBucketTrigger(t *testing.T) {
	contentBytes, err := os.ReadFile("../../views/pages/buckets.html")
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Equal(t, 1, strings.Count(content, `hx-get="/buckets/create"`))
}

func TestBucketsDropdownIsHiddenByDefaultAndToggledByButton(t *testing.T) {
	contentBytes, err := os.ReadFile("../../views/pages/buckets.html")
	require.NoError(t, err)
	content := string(contentBytes)

	// Dropdown uses Alpine.js x-show with open:false (hidden by default), toggled via @click
	assert.Contains(t, content, `x-data="{ open: false }"`)
	assert.Contains(t, content, `x-show="open"`)
	assert.Contains(t, content, `@click.stop="open = !open"`)
	// Ensure no inline onclick handlers remain for this toggle
	assert.NotContains(t, content, `.classList.toggle('hidden')`)
}

func TestBrowserUploadProgressModalIsHiddenByDefault(t *testing.T) {
	contentBytes, err := os.ReadFile("../../views/pages/browser.html")
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Contains(t, content, `id="upload-progress-modal"`)
	assert.Contains(t, content, `style="display: none;"`)
	assert.Contains(t, content, `:style="uploadProgress.show ? 'display: flex' : 'display: none'"`)
}
