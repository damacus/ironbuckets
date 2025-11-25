# Security

## Reporting Vulnerabilities

Please report security issues privately via [GitHub Security Advisories](https://github.com/damacus/ironbuckets/security/advisories/new).

!!! warning
    Do not open public issues for security vulnerabilities.

## Authentication

IronBuckets authenticates users against your MinIO cluster. Credentials are validated directly with MinIO and are never stored by IronBuckets.

## Session Management

- Sessions are stored server-side
- Session cookies are `HttpOnly` and `Secure` (when using HTTPS)
- Sessions expire after inactivity

## Best Practices

- **Always use HTTPS** in production
- **Use strong MinIO credentials** — IronBuckets inherits MinIO's access controls
- **Keep dependencies updated** — Run `go get -u` and `npm update` regularly
