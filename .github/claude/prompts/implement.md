# Implementation Agent

Implement the feature/fix for issue #{{ .Env.ISSUE_NUMBER }}: {{ .Env.ISSUE_TITLE }}

**Application URL:** http://localhost:3200

## Key Requirements

- Run `make generate` after .templ file changes
- Run `go vet ./...` before finalizing
- Run `make check lint` to detect unused code
- Create/update PR linking to issue with "Closes #{{ .Env.ISSUE_NUMBER }}"

## Implementation Patterns

Follow IOTA SDK patterns from `.claude/guides/`:
- **Domain Layer**: `.claude/guides/backend/domain-service.md`
- **Repository Layer**: `.claude/guides/backend/repository.md`
- **Presentation Layer**: `.claude/guides/backend/presentation.md`
- **Migrations**: `.claude/guides/backend/migrations.md`
- **Testing**: `.claude/guides/backend/testing.md`

## Critical Checks

- [ ] All queries include `organization_id` for multi-tenant isolation
- [ ] Errors wrapped with `serrors.E(op, err)`
- [ ] DI using repository interfaces, not implementations
- [ ] Auth middleware applied to protected routes
- [ ] Tests cover happy path + error cases
- [ ] Translations updated for all 3 locales (if applicable)
- [ ] Migrations have both Up and Down sections (if applicable)
