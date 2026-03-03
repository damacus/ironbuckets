# Security Hardening Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Fix 10 security findings across high/medium/low severity tiers, leaving the codebase demonstrably more secure with a full test suite.

**Architecture:** All changes are confined to `internal/middleware/`, `internal/handlers/`, `internal/services/`, `views/`, and `cmd/server/main.go`. No new dependencies — Echo's built-in middleware and Go stdlib cover everything. TDD throughout: write failing test, implement minimally, commit.

**Tech Stack:** Go 1.25+, Echo v4, `log/slog` (stdlib), `net/http/httptest`, `github.com/stretchr/testify`

---

## Task 1: Fix CSRF Bypass (H1)

**Files:**
- Modify: `internal/middleware/csrf.go`
- Modify: `internal/middleware/csrf_test.go`

**Step 1: Write the failing test**

Add to `internal/middleware/csrf_test.go`:

```go
func TestCSRFMiddlewareRejectsPlainPostWithoutToken(t *testing.T) {
	e := echo.New()
	e.Use(CSRF())
	e.POST("/submit", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	})

	// Regular POST with no HX-Request header — should still require CSRF token
	req := httptest.NewRequest(http.MethodPost, "/submit", strings.NewReader("x=1"))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationForm)
	// Note: no HX-Request header set
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/middleware/ -run TestCSRFMiddlewareRejectsPlainPostWithoutToken -v
```

Expected: PASS (the test currently passes because the middleware skips non-HTMX requests returning 200 — wait, let me re-check: the skipper returns `true` for non-HTMX, which means CSRF is skipped, so the handler returns 200, so the test asserting 400 FAILS). Expected: FAIL.

**Step 3: Fix the CSRF skipper**

Replace the `Skipper` in `internal/middleware/csrf.go` with:

```go
Skipper: func(c echo.Context) bool {
	// Only skip CSRF validation for safe (read-only) methods
	switch c.Request().Method {
	case http.MethodGet, http.MethodHead, http.MethodOptions, http.MethodTrace:
		return true
	}
	return false
},
```

Remove the `HX-Request` condition entirely. HTMX automatically sends `X-CSRF-Token` via the `htmx:configRequest` event listener already in `base.html` — no template changes needed.

**Step 4: Update the existing test that relies on HX-Request**

`TestCSRFMiddlewareRejectsPostWithoutToken` sets `HX-Request: true` on its POST. That test is still valid — it tests that a POST without a token is rejected. The test does not need to change.

`TestCSRFMiddlewareAllowsPostWithTokenHeaderAndCookie` also sets `HX-Request: true` — that test remains valid.

**Step 5: Run all CSRF tests**

```bash
go test ./internal/middleware/ -run TestCSRF -v
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add internal/middleware/csrf.go internal/middleware/csrf_test.go
git commit -m "fix: enforce CSRF on all mutating requests, not just HTMX"
```

---

## Task 2: Rate Limit Login (H2)

**Files:**
- Modify: `cmd/server/main.go`
- Create: `internal/middleware/rate_limit_test.go` (integration test via server setup)

**Step 1: Write the failing test**

Create `internal/middleware/rate_limit_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestLoginRateLimiterRejectsExcessiveRequests(t *testing.T) {
	e := echo.New()

	store := echomiddleware.NewRateLimiterMemoryStoreWithConfig(
		echomiddleware.RateLimiterMemoryStoreConfig{
			Rate:      10,
			Burst:     10,
			ExpiresIn: time.Minute,
		},
	)
	rl := echomiddleware.RateLimiterWithConfig(echomiddleware.RateLimiterConfig{
		Store: store,
		IdentifierExtractor: func(c echo.Context) (string, error) {
			return c.RealIP(), nil
		},
		ErrorHandler: func(c echo.Context, err error) error {
			return c.String(http.StatusTooManyRequests, "rate limit exceeded")
		},
		DenyHandler: func(c echo.Context, identifier string, err error) error {
			return c.String(http.StatusTooManyRequests, "rate limit exceeded")
		},
	})

	e.POST("/login", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, rl)

	// Exhaust the burst allowance
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "192.168.1.1:1234"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "request %d should succeed", i+1)
	}

	// 11th request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	req.RemoteAddr = "192.168.1.1:1234"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusTooManyRequests, rec.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/middleware/ -run TestLoginRateLimiterRejectsExcessiveRequests -v
```

Expected: FAIL (rate limiter not yet wired up).

**Step 3: Add rate limiter to login route in main.go**

In `cmd/server/main.go`, add the import and build the rate limiter before route registration:

```go
import (
    // existing imports ...
    "time"
    echomiddleware "github.com/labstack/echo/v4/middleware"
)
```

In `newServer()`, before route registration:

```go
// Rate limiter for login endpoint (10 requests/minute per IP)
loginRateLimiter := echomiddleware.RateLimiterWithConfig(echomiddleware.RateLimiterConfig{
    Store: echomiddleware.NewRateLimiterMemoryStoreWithConfig(
        echomiddleware.RateLimiterMemoryStoreConfig{
            Rate:      10,
            Burst:     10,
            ExpiresIn: time.Minute,
        },
    ),
    IdentifierExtractor: func(c echo.Context) (string, error) {
        return c.RealIP(), nil
    },
    ErrorHandler: func(c echo.Context, err error) error {
        return echo.NewHTTPError(http.StatusTooManyRequests, "Too many login attempts. Try again later.")
    },
    DenyHandler: func(c echo.Context, identifier string, err error) error {
        return echo.NewHTTPError(http.StatusTooManyRequests, "Too many login attempts. Try again later.")
    },
})
```

Apply to the login POST route:

```go
e.POST("/login", authHandler.Login, loginRateLimiter)
```

**Step 4: Run test**

```bash
go test ./internal/middleware/ -run TestLoginRateLimiterRejectsExcessiveRequests -v
go test ./...
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/server/main.go internal/middleware/rate_limit_test.go
git commit -m "feat: add rate limiting to login endpoint (10 req/min per IP)"
```

---

## Task 3: Logout via POST (M1)

**Files:**
- Modify: `internal/handlers/auth_handler.go`
- Modify: `internal/handlers/auth_handler_test.go`
- Modify: `internal/middleware/auth_middleware.go`
- Modify: `cmd/server/main.go`
- Modify: `views/layouts/base.html`

**Step 1: Write the failing test**

Add to `internal/handlers/auth_handler_test.go`:

```go
func TestLogoutHandlerAcceptsPost(t *testing.T) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	req.TLS = &tls.ConnectionState{}
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handler := NewAuthHandler(services.NewAuthService(), &authTestFactory{client: &authTestMinioClient{}}, "play.min.io:9000")

	err := handler.Logout(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/handlers/ -run TestLogoutHandlerAcceptsPost -v
```

Expected: FAIL (route is GET, test passes a POST context but handler doesn't care about method — actually this may PASS already since handler logic doesn't check method). If it passes, add a test that the GET route no longer exists after the next step.

**Step 3: Change route in main.go**

```go
// Before:
e.GET("/logout", authHandler.Logout)

// After:
e.POST("/logout", authHandler.Logout)
```

**Step 4: Update auth_middleware.go to remove /logout from public skip list**

Logout is now a POST protected by CSRF. It no longer needs to be in the skip list (auth middleware will still redirect unauthenticated requests — which is fine, unauthenticated users can't POST to logout). However, to keep things simple, keep it in the skip list so the redirect loop doesn't occur:

Actually, keep `/logout` in the skip list — an unauthenticated POST to `/logout` should still work (just clears an already-absent cookie). No change needed to `auth_middleware.go`.

**Step 5: Update the logout link in base.html**

```html
<!-- Before: -->
<a href="/logout" hx-boost="false"
    class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap text-zinc-400 hover:text-red-400 hover:bg-red-500/10">
    <i data-lucide="log-out" size="18" class="flex-shrink-0"></i>
    <span class="opacity-100" :class="collapsed ? '!opacity-0 !w-0' : ''">Logout</span>
</a>

<!-- After: -->
<form method="POST" action="/logout" class="w-full">
    <input type="hidden" name="_csrf" value="{{ .CSRFToken }}">
    <button type="submit"
        class="w-full flex items-center gap-3 px-3 py-2 rounded-md text-sm font-medium transition-colors whitespace-nowrap text-zinc-400 hover:text-red-400 hover:bg-red-500/10">
        <i data-lucide="log-out" size="18" class="flex-shrink-0"></i>
        <span class="opacity-100" :class="collapsed ? '!opacity-0 !w-0' : ''">Logout</span>
    </button>
</form>
```

**Step 6: Pass CSRFToken to all protected page renders**

The CSRF token is available via Echo's context key `"csrf"`. Pass it in the renderer or pass it per-handler. The simplest approach: add it in a middleware that runs after CSRF middleware, injecting it into the template data map via a custom Echo context method.

Alternative (simpler, no middleware): In each handler that renders a full page, read the token and pass it. But that's repetitive.

Best approach: update `renderer.go` to inject the CSRF token from context into every template render automatically.

In `internal/renderer/renderer.go`, find the `Render` method and add:

```go
// Inject CSRF token if available
if m, ok := data.(map[string]interface{}); ok {
    if token, ok := c.Get("csrf").(string); ok {
        m["CSRFToken"] = token
    }
}
```

**Step 7: Run all tests**

```bash
go test ./...
```

Expected: all PASS.

**Step 8: Commit**

```bash
git add cmd/server/main.go internal/handlers/auth_handler_test.go views/layouts/base.html internal/renderer/renderer.go
git commit -m "fix: change logout to POST to prevent CSRF-based forced logout"
```

---

## Task 4: CSP Nonce — Remove unsafe-inline (M2)

**Files:**
- Modify: `internal/middleware/security_headers.go`
- Modify: `internal/middleware/security_headers_test.go`
- Modify: `views/layouts/base.html`
- Modify: `views/pages/browser.html`
- Modify: `views/pages/login.html`

**Step 1: Write the failing test**

Add to `internal/middleware/security_headers_test.go`:

```go
func TestSecurityHeadersCSPHasNoUnsafeInline(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeaders())
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	assert.NotContains(t, csp, "'unsafe-inline'")
}

func TestSecurityHeadersCSPContainsNonce(t *testing.T) {
	e := echo.New()
	e.Use(SecurityHeaders())
	e.GET("/", func(c echo.Context) error {
		nonce := c.Get("csp-nonce").(string)
		return c.String(http.StatusOK, nonce)
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	csp := rec.Header().Get("Content-Security-Policy")
	nonce := strings.TrimSpace(rec.Body.String())
	assert.NotEmpty(t, nonce)
	assert.Contains(t, csp, "'nonce-"+nonce+"'")
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/middleware/ -run "TestSecurityHeadersCSP" -v
```

Expected: FAIL.

**Step 3: Update security_headers.go**

```go
package middleware

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
)

const cspBase = "default-src 'self'; " +
	"script-src 'self' 'nonce-%s'; " +
	"style-src 'self' 'nonce-%s'; " +
	"img-src 'self' data:; " +
	"font-src 'self'; " +
	"connect-src 'self'; " +
	"frame-ancestors 'none'; " +
	"base-uri 'self'; " +
	"form-action 'self'"

func generateNonce() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

func SecurityHeaders() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			nonce, err := generateNonce()
			if err != nil {
				return err
			}

			c.Set("csp-nonce", nonce)

			headers := c.Response().Header()
			headers.Set("X-Frame-Options", "DENY")
			headers.Set("X-Content-Type-Options", "nosniff")
			headers.Set("Referrer-Policy", "strict-origin-when-cross-origin")
			headers.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
			headers.Set("Content-Security-Policy", fmt.Sprintf(cspBase, nonce, nonce))

			if isSecureRequest(c) {
				headers.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			return next(c)
		}
	}
}

func isSecureRequest(c echo.Context) bool {
	req := c.Request()
	if req.TLS != nil {
		return true
	}
	return strings.EqualFold(req.Header.Get("X-Forwarded-Proto"), "https")
}
```

**Step 4: Inject nonce into template data via renderer**

In `internal/renderer/renderer.go`, also inject `CSPNonce`:

```go
if m, ok := data.(map[string]interface{}); ok {
    if token, ok := c.Get("csrf").(string); ok {
        m["CSRFToken"] = token
    }
    if nonce, ok := c.Get("csp-nonce").(string); ok {
        m["CSPNonce"] = nonce
    }
}
```

**Step 5: Add nonce to inline scripts/styles in templates**

In `views/layouts/base.html`, `views/pages/browser.html`, and `views/pages/login.html`, add `nonce="{{ .CSPNonce }}"` to every `<script>` and `<style>` tag. Example:

```html
<!-- Before -->
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
<style> ... </style>
<script> ... </script>

<!-- After (once assets are vendored in Task 8, the src changes; for now just add nonce) -->
<script nonce="{{ .CSPNonce }}" src="/static/htmx.min.js"></script>
<style nonce="{{ .CSPNonce }}"> ... </style>
<script nonce="{{ .CSPNonce }}"> ... </script>
```

Note: External CDN `src=` attributes with a nonce are not trusted by browsers for external origins unless they are in the CSP allowlist. Task 8 (vendor assets) removes the CDN dependency entirely. For now, temporarily keep CDN srcs AND add nonce so inline scripts are secured. The `img-src` is also tightened by removing `https:` wildcard — vendor fonts/images in Task 8.

**Step 6: Run tests**

```bash
go test ./internal/middleware/ -run "TestSecurityHeaders" -v
go test ./...
```

Expected: all PASS.

**Step 7: Commit**

```bash
git add internal/middleware/security_headers.go internal/middleware/security_headers_test.go internal/renderer/renderer.go views/layouts/base.html views/pages/browser.html views/pages/login.html
git commit -m "feat: replace unsafe-inline CSP with per-request nonce"
```

---

## Task 5: Audit Logging (M3)

**Files:**
- Create: `internal/middleware/audit_log.go`
- Create: `internal/middleware/audit_log_test.go`
- Modify: `cmd/server/main.go`

**Step 1: Write the failing test**

Create `internal/middleware/audit_log_test.go`:

```go
package middleware

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuditLogWritesStructuredEntry(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))

	e := echo.New()
	e.Use(AuditLog(logger))
	e.POST("/users/create", func(c echo.Context) error {
		// Simulate auth middleware having set credentials
		c.Set(utils.ContextKeyCreds, &services.Credentials{AccessKey: "admin-key"})
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodPost, "/users/create", nil)
	req.RemoteAddr = "10.0.0.1:4567"
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))
	assert.Equal(t, "POST", entry["method"])
	assert.Equal(t, "/users/create", entry["path"])
	assert.Equal(t, "admin-key", entry["user"])
	assert.NotEmpty(t, entry["remote_ip"])
	assert.NotEmpty(t, entry["time"])
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/middleware/ -run TestAuditLogWritesStructuredEntry -v
```

Expected: FAIL (AuditLog not defined).

**Step 3: Implement audit_log.go**

Create `internal/middleware/audit_log.go`:

```go
package middleware

import (
	"log/slog"

	"github.com/damacus/iron-buckets/internal/services"
	"github.com/damacus/iron-buckets/internal/utils"
	"github.com/labstack/echo/v4"
)

// AuditLog records structured log entries for every request after auth.
// Safe methods (GET, HEAD) are logged at DEBUG level; mutating methods at INFO.
func AuditLog(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			err := next(c)

			req := c.Request()
			user := ""
			if creds, ok := c.Get(utils.ContextKeyCreds).(*services.Credentials); ok && creds != nil {
				user = creds.AccessKey
			}

			attrs := []slog.Attr{
				slog.String("method", req.Method),
				slog.String("path", req.URL.Path),
				slog.Int("status", c.Response().Status),
				slog.String("remote_ip", c.RealIP()),
				slog.String("user", user),
			}

			level := slog.LevelDebug
			switch req.Method {
			case "POST", "PUT", "DELETE", "PATCH":
				level = slog.LevelInfo
			}

			logger.LogAttrs(req.Context(), level, "request", attrs...)

			return err
		}
	}
}
```

**Step 4: Wire into main.go**

```go
import "log/slog"

// In newServer(), after other middleware:
auditLogger := slog.Default()
e.Use(customMiddleware.AuditLog(auditLogger))
```

Place `AuditLog` after `AuthMiddleware` in the middleware chain so `ContextKeyCreds` is populated.

**Step 5: Run tests**

```bash
go test ./internal/middleware/ -run TestAuditLog -v
go test ./...
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add internal/middleware/audit_log.go internal/middleware/audit_log_test.go cmd/server/main.go
git commit -m "feat: add structured audit logging for all requests"
```

---

## Task 6: Session Key Validation (M4)

**Files:**
- Modify: `internal/services/auth_service.go`
- Modify: `internal/services/auth_service_test.go`

**Step 1: Write the failing test**

Add to `internal/services/auth_service_test.go`:

```go
func TestNewAuthServicePanicsInProductionWithoutKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("IRON_SESSION_KEY", "") // ensure not set

	assert.Panics(t, func() {
		NewAuthService()
	}, "should panic in production without IRON_SESSION_KEY")
}

func TestNewAuthServicePanicsInProductionWithShortKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("IRON_SESSION_KEY", "tooshort")

	assert.Panics(t, func() {
		NewAuthService()
	}, "should panic in production with a key that is not 32 bytes")
}

func TestNewAuthServiceSucceedsInProductionWithValidKey(t *testing.T) {
	t.Setenv("APP_ENV", "production")
	t.Setenv("IRON_SESSION_KEY", "12345678901234567890123456789012") // exactly 32 bytes

	assert.NotPanics(t, func() {
		NewAuthService()
	})
}
```

**Step 2: Run tests to verify they fail**

```bash
go test ./internal/services/ -run "TestNewAuthService" -v
```

Expected: FAIL.

**Step 3: Update NewAuthService**

```go
func NewAuthService() *AuthService {
	key := os.Getenv("IRON_SESSION_KEY")
	isProduction := strings.EqualFold(os.Getenv("APP_ENV"), "production")

	if len(key) != 32 {
		if isProduction {
			panic("IRON_SESSION_KEY must be exactly 32 bytes in production. Set APP_ENV=production and IRON_SESSION_KEY=<32-byte-hex-string>")
		}
		log.Println("WARNING: IRON_SESSION_KEY not set or not 32 bytes — using ephemeral key. Sessions will be invalidated on restart.")
		newKey := make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
			panic("failed to generate random key")
		}
		return &AuthService{encryptionKey: newKey}
	}
	return &AuthService{encryptionKey: []byte(key)}
}
```

Add `"log"` and `"strings"` to imports.

**Step 4: Run tests**

```bash
go test ./internal/services/ -v
go test ./...
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add internal/services/auth_service.go internal/services/auth_service_test.go
git commit -m "fix: panic on startup in production if IRON_SESSION_KEY is missing or wrong length"
```

---

## Task 7: Upload Size Limit (L1)

**Files:**
- Modify: `cmd/server/main.go`
- Create: `internal/middleware/body_limit_test.go`

**Step 1: Write the failing test**

Create `internal/middleware/body_limit_test.go`:

```go
package middleware_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/stretchr/testify/assert"
)

func TestUploadBodyLimitRejectsOversizedRequest(t *testing.T) {
	e := echo.New()

	// 10 byte limit for test
	e.POST("/upload", func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, echomiddleware.BodyLimit("10B"))

	// Send 20 bytes
	req := httptest.NewRequest(http.MethodPost, "/upload", strings.NewReader("12345678901234567890"))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusRequestEntityTooLarge, rec.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/middleware/ -run TestUploadBodyLimitRejectsOversizedRequest -v
```

Expected: FAIL.

**Step 3: Wire BodyLimit to upload route in main.go**

Add to imports: (none new needed — `echomiddleware` already imported if rate limiter was added, otherwise add `echomiddleware "github.com/labstack/echo/v4/middleware"`)

```go
// Upload size limit — default 100MB, configurable via MAX_UPLOAD_SIZE env var
maxUploadSize := os.Getenv("MAX_UPLOAD_SIZE")
if maxUploadSize == "" {
	maxUploadSize = "100MB"
}
uploadLimit := echomiddleware.BodyLimit(maxUploadSize)
```

Apply to upload route:

```go
e.POST("/buckets/:bucketName/upload", bucketsHandler.UploadObject, uploadLimit)
```

**Step 4: Run tests**

```bash
go test ./internal/middleware/ -run TestUploadBodyLimit -v
go test ./...
```

Expected: all PASS.

**Step 5: Commit**

```bash
git add cmd/server/main.go internal/middleware/body_limit_test.go
git commit -m "feat: enforce upload size limit (default 100MB, configurable via MAX_UPLOAD_SIZE)"
```

---

## Task 8: Vendor Static Assets (L2)

**Files:**
- Create: `views/static/` directory with vendored JS/CSS
- Modify: `views/layouts/base.html`
- Modify: `views/pages/browser.html`
- Modify: `views/pages/login.html`
- Modify: `internal/middleware/security_headers.go` (remove CDN entries from CSP)
- Modify: `cmd/server/main.go` (serve static files)

**Step 1: Download and vendor assets**

```bash
mkdir -p views/static
# HTMX
curl -o views/static/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js
# Alpine.js CSP build
curl -o views/static/alpine-csp.min.js https://unpkg.com/@alpinejs/csp@3.14.8/dist/cdn.min.js
# Lucide icons
curl -o views/static/lucide.min.js https://unpkg.com/lucide@0.445.0/dist/umd/lucide.min.js
# Tailwind standalone CLI output — build from config
# Install tailwindcss CLI and generate output:
npx tailwindcss -i /dev/null -o views/static/tailwind.css --content "views/**/*.html"
```

**Step 2: Serve static files in main.go**

```go
e.Static("/static", "views/static")
```

**Step 3: Update templates to use local paths**

In `base.html`, `browser.html`, `login.html` — replace CDN URLs:

```html
<!-- Before -->
<script src="https://cdn.tailwindcss.com"></script>
<script src="https://unpkg.com/lucide@0.445.0"></script>
<script src="https://unpkg.com/htmx.org@1.9.10"></script>
<script defer src="https://unpkg.com/@alpinejs/csp@3.14.8/dist/cdn.min.js"></script>

<!-- After -->
<link rel="stylesheet" href="/static/tailwind.css" nonce="{{ .CSPNonce }}">
<script nonce="{{ .CSPNonce }}" src="/static/lucide.min.js"></script>
<script nonce="{{ .CSPNonce }}" src="/static/htmx.min.js"></script>
<script defer nonce="{{ .CSPNonce }}" src="/static/alpine-csp.min.js"></script>
```

Remove the inline `tailwind.config = { ... }` script block (Tailwind standalone CLI build captures the config at build time).

**Step 4: Remove CDN entries from CSP**

Update `security_headers.go` — the `cspBase` no longer needs CDN allowances. The `img-src` can also drop `https:` wildcard:

```go
const cspBase = "default-src 'self'; " +
    "script-src 'self' 'nonce-%s'; " +
    "style-src 'self' 'nonce-%s'; " +
    "img-src 'self' data:; " +
    "font-src 'self'; " +
    "connect-src 'self'; " +
    "frame-ancestors 'none'; " +
    "base-uri 'self'; " +
    "form-action 'self'"
```

**Step 5: Add static assets to .gitignore or commit them**

Vendored JS/CSS should be committed (they're build artifacts, but vendoring them is intentional for security):

```bash
git add views/static/
```

**Step 6: Run all tests and smoke test the UI**

```bash
go test ./...
task dev  # visually verify the app loads without CDN
```

**Step 7: Commit**

```bash
git add views/static/ views/layouts/base.html views/pages/browser.html views/pages/login.html internal/middleware/security_headers.go cmd/server/main.go
git commit -m "feat: vendor static assets and remove CDN dependencies from CSP"
```

---

## Task 9: Default Endpoint Warning (L3)

**Files:**
- Modify: `cmd/server/main.go`

**Step 1: Write the failing test**

This is a log output test — simplest to verify by reading the code. Add a unit test in `cmd/server/` if a `main_test.go` exists, otherwise test via code review.

Add to `cmd/server/main.go` in the `main()` function:

```go
if minioEndpoint == "play.min.io:9000" {
    log.Println("WARNING: MINIO_ENDPOINT not set — connected to MinIO public demo (play.min.io). " +
        "Do NOT use in production. Set MINIO_ENDPOINT in your environment.")
}
```

Replace the existing quiet log line at line 21.

**Step 2: Commit**

```bash
git add cmd/server/main.go
git commit -m "fix: warn prominently when default MinIO endpoint (play.min.io) is used"
```

---

## Task 10: Gate OIDC Routes (L4)

**Files:**
- Modify: `cmd/server/main.go`
- Modify: `internal/handlers/auth_handler.go`
- Modify: `internal/middleware/auth_middleware.go`

**Step 1: Write the failing test**

Add to `internal/middleware/auth_middleware_test.go`:

```go
func TestAuthMiddleware_DoesNotSkipOIDCByDefault(t *testing.T) {
	// When OIDC routes are gated, /login/oauth should no longer be in the skip list
	// unless ENABLE_OIDC=true. This test verifies the skip list doesn't include them.
	authService := services.NewAuthService()

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/login/oauth", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	handlerCalled := false
	handler := func(c echo.Context) error {
		handlerCalled = true
		return c.String(http.StatusOK, "OK")
	}

	mw := AuthMiddleware(authService)
	_ = mw(handler)(c)

	// Without a valid cookie, should redirect to /login — not call the handler
	assert.False(t, handlerCalled, "/login/oauth should not be skipped by default")
	assert.Equal(t, http.StatusSeeOther, rec.Code)
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/middleware/ -run TestAuthMiddleware_DoesNotSkipOIDCByDefault -v
```

Expected: FAIL (routes currently skipped unconditionally).

**Step 3: Update auth_middleware.go — remove OIDC paths from skip list**

```go
// Before:
if path == "/login" || path == "/health" || path == "/logout" ||
    path == "/login/oauth" || path == "/oauth/callback" {
    return next(c)
}

// After:
if path == "/login" || path == "/health" || path == "/logout" {
    return next(c)
}
```

**Step 4: Gate route registration in main.go**

```go
if os.Getenv("ENABLE_OIDC") == "true" {
    e.GET("/login/oauth", authHandler.LoginOIDC)
    e.GET("/oauth/callback", authHandler.CallbackOIDC)
}
```

**Step 5: Run tests**

```bash
go test ./internal/middleware/ -run TestAuthMiddleware -v
go test ./...
```

Expected: all PASS.

**Step 6: Commit**

```bash
git add cmd/server/main.go internal/middleware/auth_middleware.go internal/middleware/auth_middleware_test.go
git commit -m "fix: gate OIDC routes behind ENABLE_OIDC=true env flag"
```

---

## Final: Push

```bash
git pull --rebase
git push
```

Verify all CI checks pass.
