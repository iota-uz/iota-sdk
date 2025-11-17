---
description: "Identify and remove dead/deprecated code using deadcode tool. Handles file deletion when empty, test cleanup, and marks test utilities with t.Helper()"
allowed-tools: |
  Bash(make check:*), Bash(deadcode:*), Bash(go vet:*), Bash(go test:*),
  Bash(git rm:*), Bash(git status:*), Bash(git diff:*),
  Read, Edit, Write, Glob, Grep,
  Task
---

Systematically remove unreachable/unused code detected by the deadcode tool, including dead functions, deprecated
functions (marked with `// Deprecated:`), empty files, obsolete tests, and unused test utilities.

Dead code analysis: !`make check deadcode 2>&1`

Deprecated code search: !`grep -r "// Deprecated:" --include="*.go" | wc -l`

Current git status: !`git status --short`

## Workflow

**Analyze and categorize**

- Parse deadcode output by file path, type (function/method), and module (logistics/finance/safety)
- Search for deprecated functions: `Grep: pattern="// Deprecated:" output_mode="content"`
- Classify: Safe to remove (private functions, deprecated internals), Review needed (exported functions, deprecated
  exports - may be public API), Keep (build tags, linkname)
- If exported functions or deprecated exports found, use AskUserQuestion to confirm removal strategy

**Remove dead code**

- Small scope (1-10 functions): Use direct Edit tool
- Medium/large scope (10+ functions): Use Task(subagent_type:editor), split into batches if 50+
- Preserve the surrounding code structure and unrelated comments
- Verify immediately: `go vet ./...` (fix type errors before proceeding)

**Clean up empty files**

- Check modified files for package-only or import-only content
- Search for imports: `Grep: pattern="import.*path/to/file"`
- Remove imports from dependent files, then `git rm path/to/file.go`
- Re-verify: `go vet ./...`

**Clean up tests**

- Find affected tests: `Grep: pattern="Test.*FunctionName|FunctionName" glob="*_test.go"`
- Remove test functions, table cases, mocks for deleted code
- Keep test utilities if used elsewhere; mark helpers with `t.Helper()` as first line
- Use Task(subagent_type:editor) for test cleanup
- Verify: `go test -v ./path/to/package -count=1`

**Final verification**

- `go vet ./...` (static checks)
- `make check lint` (code quality)
- `make test failures` (optional but recommended)
- `git diff --stat` and `git status --short` (review changes)

## Important Rules

- Always verify with `go vet` after each step before proceeding
- Be conservative with exports: Ask before removing exported functions (may be public API)
- Preserve test utilities unless truly unused
- Mark all test helpers with `t.Helper()` as first line
- Use `git rm` instead of `rm` for file deletion
- Use editor for bulk removals (10+ functions)

## Edge Cases

- Build tags: Dead code analysis is single GOOS/GOARCH; "dead" code may be alive in other configs
- //go:linkname: Tool doesn't understand linkname directives; review carefully for false positives
- Generated code: Skip files matching `*.pb.go`, `*_templ.go`, `*.gen.go`
- Public API: Exported functions may be used by external packages even if unused internally
- Test-only exports: Consider internal_test pattern; document if keeping for testing
