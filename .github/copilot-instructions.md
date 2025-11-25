# GitHub Copilot Instructions

Read and follow all instructions in [AGENTS.md](../AGENTS.md) at the repository root.

## Quick Reference

### Project

- **Language**: Go 1.25+ with Echo v4 framework
- **Frontend**: Server-rendered HTML templates with HTMX
- **Testing**: Go unit tests + Playwright E2E tests

### Mandatory Rules

1. **TDD Required**: Write a failing test before any production code
2. **Conventional Commits**: Use `feat:`, `fix:`, `refactor:`, `test:`, `docs:`, `chore:`
3. **Small Changes**: One logical change per commit
4. **Strong Typing**: Avoid `interface{}` where possible

### Commands

```bash
task test:e2e      # Run E2E tests
go test ./...      # Run unit tests
task docker:up     # Start dev environment
task docker:down   # Stop dev environment
```

### Code Style

- Guard clauses for early returns
- Service objects for business logic
- Handlers focused on HTTP concerns only

For complete instructions, see [AGENTS.md](../AGENTS.md).
