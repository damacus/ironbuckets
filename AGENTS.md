# AI Agent Instructions

This document provides instructions for AI coding assistants (GitHub Copilot, Windsurf
Cascade, Cursor, etc.) working on this codebase.

## Project Overview

**IronBuckets** is a Go web application providing a modern UI for managing MinIO clusters.

- **Language**: Go 1.25+
- **Framework**: Echo v4 (web framework)
- **Frontend**: Server-rendered HTML templates with HTMX
- **Testing**: Go unit tests + Playwright E2E tests
- **Infrastructure**: Docker Compose for local development

## Mandatory Practices

### Test-Driven Development (TDD)

**Every line of production code must be written in response to a failing test.**

1. **Red**: Write a failing test that defines the expected behavior
2. **Green**: Write the minimum code to make the test pass
3. **Refactor**: Clean up the code while keeping tests green

Rules:

- Write behavior-driven tests from the user's perspective
- Test public APIs only, not implementation details
- Refactor only when all tests pass
- Make incremental changes that maintain a working state

### Commit Standards

Use [Conventional Commits](https://www.conventionalcommits.org/) format:

- `feat:` — New feature
- `fix:` — Bug fix
- `refactor:` — Code change that neither fixes a bug nor adds a feature
- `test:` — Adding or updating tests
- `docs:` — Documentation only changes
- `chore:` — Maintenance tasks

Example: `feat: add bucket quota display to settings page`

### Code Changes

- Make small, atomic commits focused on a single change
- Keep PRs small and focused with clear descriptions
- Work in small, verifiable incremental steps
- Never leave the codebase in a broken state

## Architecture

### Project Structure

```text
cmd/server/          # Application entry point and journey tests
internal/
  handlers/          # HTTP request handlers
  middleware/        # Echo middleware (auth, etc.)
  models/            # Data structures
  renderer/          # Template rendering
  services/          # Business logic and MinIO client wrappers
  utils/             # Shared utilities
views/
  layouts/           # Base HTML templates
  pages/             # Full page templates
  partials/          # Reusable template fragments
e2e/                 # Playwright E2E test specs
```

### Code Style

- Use guard clauses for early returns to reduce nesting
- Implement service objects for complex business logic
- Keep handlers clean and focused on HTTP concerns
- Use strong typing—avoid `interface{}` where possible

## Testing

### Running Tests

```bash
# Unit tests
go test ./...

# Unit tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# E2E tests (starts Docker, runs tests, cleans up)
task test:e2e

# E2E tests in UI mode (requires Docker running)
task test:e2e:watch

# E2E tests in debug mode
task test:e2e:debug
```

### Test Environment

```bash
task docker:up     # Start MinIO + app containers
task docker:down   # Stop and clean up
```

Tests run against `http://localhost:8080` with MinIO credentials `minioadmin` /
`minioadmin`.

### When to Write Which Tests

**E2E tests (Playwright):**

- User flows (login, create bucket, upload file)
- Critical paths that would break the app if they failed
- Regression tests after fixing bugs

**Unit tests (Go):**

- Handler logic and business rules
- Service layer functions
- Utility functions
- Edge cases and error handling

## Development Workflow

### Setup

```bash
# Install dependencies
task setup

# Or manually:
go mod download
npm install
npx playwright install --with-deps
```

### Local Development

```bash
# Start dev server (local, not Docker)
task dev

# Or with Docker:
task docker:up
```

### Available Tasks

Run `task --list` to see all available tasks. Key ones:

| Task              | Description                                |
| ----------------- | ------------------------------------------ |
| `task setup`      | Install all dependencies                   |
| `task dev`        | Start development server                   |
| `task build`      | Build the application binary               |
| `task docker:up`  | Start Docker Compose test environment      |
| `task docker:down`| Stop and clean up Docker environment       |
| `task test:e2e`   | Run E2E tests (full lifecycle)             |
| `task clean`      | Clean up all artifacts                     |

## Tooling

- **Task**: Use `task` (Taskfile) to run tasks—prefer this over bare scripts
- **Docker Compose**: CLI is `docker compose` (not `docker-compose`)
- **Playwright**: Use for E2E testing
- **golangci-lint**: Used in CI for linting

## Important Files

| File                | Purpose                              |
| ------------------- | ------------------------------------ |
| `Taskfile.yml`      | Task runner configuration            |
| `docker-compose.yml`| Local development environment        |
| `playwright.config.js` | E2E test configuration            |
| `.env.example`      | Environment variable template        |
| `TESTING.md`        | Detailed testing documentation       |
| `SECURITY.md`       | Security policy                      |

## AI Assistant Guidelines

1. **Follow TDD strictly**—write tests before implementation
2. **Make small changes**—one logical change per edit
3. **Verify each change**—run relevant tests before proceeding
4. **Avoid large updates**—break big changes into smaller PRs
5. **Use existing patterns**—follow the code style already in the codebase
6. **Run linting**—ensure code passes `golangci-lint`
7. **Document changes**—update relevant docs when adding features
