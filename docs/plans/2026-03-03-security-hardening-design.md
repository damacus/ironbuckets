# Security Hardening Design

**Date:** 2026-03-03
**Status:** Approved
**Approach:** B — Full systematic hardening

## Summary

Comprehensive security hardening of IronBuckets across 10 findings, ordered by severity. No new runtime dependencies required beyond Echo's built-in middleware and Go stdlib.

## Findings

### High Severity

#### H1: CSRF Bypass for Non-HTMX POSTs

**Location:** `internal/middleware/csrf.go:17`
**Issue:** The CSRF skipper returns `true` (skip) when `HX-Request != "true"`. Any state-changing POST from a regular HTML form — including cross-origin — bypasses CSRF protection entirely.
**Fix:** Remove the `HX-Request` condition. Enforce CSRF on all mutating methods (POST, PUT, DELETE, PATCH). Echo's CSRF via `X-CSRF-Token` header is compatible with HTMX out of the box.

#### H2: No Rate Limiting on Login

**Location:** `cmd/server/main.go` (route registration)
**Issue:** `/login` accepts unlimited credential attempts with no throttle, enabling brute force of MinIO credentials.
**Fix:** Add Echo's `middleware.RateLimiter` scoped to the `/login` POST route. Default: 10 requests per IP per minute, returning `429 Too Many Requests`.

### Medium Severity

#### M1: Logout via GET (CSRF-able)

**Location:** `cmd/server/main.go:72`, `internal/handlers/auth_handler.go:90`
**Issue:** `GET /logout` can be triggered by any embedded resource (e.g. `<img src="/logout">`), silently logging users out.
**Fix:** Change to `POST /logout`. Update the nav logout button to a `<form method="POST">` with a hidden CSRF token field.

#### M2: `unsafe-inline` in CSP

**Location:** `internal/middleware/security_headers.go:9-17`
**Issue:** `script-src` and `style-src` both allow `'unsafe-inline'`, substantially weakening XSS protection by allowing any injected inline script to execute.
**Fix:** Generate a cryptographically random nonce per request. Inject it into the CSP header (`script-src 'nonce-{value}'`, `style-src 'nonce-{value}'`) and pass it to templates via the Echo context. Add `nonce` attribute to all inline `<script>` and `<style>` tags. Remove `'unsafe-inline'` entirely.

#### M3: No Audit Logging

**Location:** N/A (missing)
**Issue:** No record of admin operations — user creation/deletion, bucket operations, policy changes. Critical gap for a MinIO admin UI.
**Fix:** Add a structured audit log middleware using `log/slog` (Go stdlib). Log: timestamp, user access key (from decrypted cookie), HTTP method, path, status code, remote IP. Write as JSON to stdout. Apply after auth middleware so the access key is available.

#### M4: Silent Session Key Fallback

**Location:** `internal/services/auth_service.go:29`
**Issue:** If `IRON_SESSION_KEY` is missing or not exactly 32 bytes, an ephemeral key is silently generated. Production restarts silently invalidate all sessions with no operator warning.
**Fix:** Check `APP_ENV` env var. When `APP_ENV=production`, log a fatal error and refuse to start if `IRON_SESSION_KEY` is absent or wrong length. In development, retain current behaviour with a prominent warning log.

### Low Severity / Hardening

#### L1: No Upload Size Limit

**Location:** `internal/handlers/buckets_handler.go:253`
**Issue:** File uploads have no size cap; a large upload can exhaust server memory.
**Fix:** Add Echo's `middleware.BodyLimit` scoped to the upload route. Default: `100MB`, configurable via `MAX_UPLOAD_SIZE` env var.

#### L2: External CDNs Without Subresource Integrity

**Location:** `internal/middleware/security_headers.go`, `views/`
**Issue:** CSP allows `cdn.tailwindcss.com` and `unpkg.com`. A compromised CDN delivers arbitrary scripts to users.
**Fix:** Vendor Tailwind CSS (build output) and HTMX minified JS into `views/static/`. Serve from `'self'`. Remove CDN entries from CSP.

#### L3: Default Endpoint Falls Back to `play.min.io`

**Location:** `cmd/server/main.go:21`
**Issue:** Unset `MINIO_ENDPOINT` silently connects to MinIO's public demo server. A misconfigured production deployment leaks data.
**Fix:** Log a prominent `WARN` when the default is used. Keep fallback for dev convenience but make it impossible to miss.

#### L4: OIDC Stub Routes Leak Unimplemented Surface

**Location:** `cmd/server/main.go`, `internal/handlers/auth_handler.go:113-124`
**Issue:** `/login/oauth` and `/oauth/callback` appear in route enumeration and suggest an incomplete auth surface.
**Fix:** Remove routes unless OIDC is actively configured. Gate behind `ENABLE_OIDC=true` env var — only register when set.

## Architecture

All changes are self-contained within existing packages. No new packages or dependencies required:

- `internal/middleware/` — CSRF fix, rate limiter, audit logger, nonce generator
- `internal/handlers/` — logout POST, upload body limit
- `internal/services/` — session key validation
- `views/` — nonce attributes, logout form, vendor static assets
- `cmd/server/main.go` — route changes, env var checks

## Testing

Each fix requires:
- **Unit tests** for middleware (CSRF, rate limiter, audit logger, nonce)
- **Handler tests** for logout POST, upload size enforcement
- **E2E tests** for login rate limiting (429 response), logout flow

## Implementation Order

1. H1: Fix CSRF bypass
2. H2: Add login rate limiting
3. M1: Logout → POST
4. M2: CSP nonce
5. M3: Audit logging
6. M4: Session key validation
7. L1: Upload size limit
8. L2: Vendor static assets
9. L3: Endpoint warning
10. L4: Gate OIDC routes
