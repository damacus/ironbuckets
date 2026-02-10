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

		assert.Contains(t, content, "@alpinejs/csp@3.14.8", file)
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

	assert.Contains(t, content, `nextElementSibling`)
	assert.Contains(t, content, `.classList.toggle('hidden')`)
	assert.Contains(t, content, `class="hidden absolute`)
}

func TestBrowserUploadProgressModalIsHiddenByDefault(t *testing.T) {
	contentBytes, err := os.ReadFile("../../views/pages/browser.html")
	require.NoError(t, err)
	content := string(contentBytes)

	assert.Contains(t, content, `id="upload-progress-modal"`)
	assert.Contains(t, content, `style="display: none;"`)
	assert.Contains(t, content, `:style="uploadProgress.show ? 'display: flex' : 'display: none'"`)
}
