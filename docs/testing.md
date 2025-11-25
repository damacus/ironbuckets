# Testing

## Run Tests

```bash
task test:e2e          # Run all E2E tests
task test:e2e:watch    # Interactive UI mode
go test ./...          # Unit tests
```

## When to Write E2E Tests

Write an E2E test when:

- **User flows** — Login, create bucket, upload file, etc.
- **Critical paths** — Anything that would break the app if it failed
- **Regressions** — After fixing a bug, add a test to prevent it returning

Don't write E2E tests for:

- **Unit logic** — Use Go tests for handlers and business logic
- **Styling** — Visual changes don't need E2E coverage
- **Edge cases** — Unit tests are faster and more precise

## Test Environment

```bash
task docker:up     # Start MinIO + app containers
task docker:down   # Stop and clean up
```

Tests run against `http://localhost:8080` with MinIO credentials `minioadmin` / `minioadmin`.

## Debugging

```bash
task test:e2e:debug    # Step through tests
task docker:logs       # View container logs
```

Failed tests save screenshots to `test-results/`.
