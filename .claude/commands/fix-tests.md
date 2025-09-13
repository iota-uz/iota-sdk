---
allowed-tools: |
  Bash(go test:*), Bash(make test:*), Bash(go vet:*),
  Bash(make lint:*), Bash(golangci-lint:*),
  Read, Edit, MultiEdit, Grep, Glob,
  Bash(go build:*), Bash(make check-tr:*),
  Task(subagent_type:test-editor), Task(subagent_type:speed-editor)
description: Systematically identify and fix broken tests and linting errors using iterative approach
---

# Fix Tests & Linting

This command systematically identifies and fixes broken tests and linting errors using a structured approach.

## Strategy

### 1. Pre-Test Phase
- **ALWAYS start with `go vet ./...`** to catch static analysis issues
- **Run `make lint`** to identify unused variables/functions
- Fix all type errors, undefined variables, and compilation issues first
- Remove unused code that could cause confusion or maintenance issues
- Ensure code compiles cleanly before attempting to run tests
- This prevents cascade failures in test discovery

### 2. Discovery Phase
- Run `go vet ./...` to catch compilation and static analysis errors
- Run `make lint` to identify unused code
- Use `make test failures` to identify failing tests
- Categorize issues by type:
  - Compilation errors (highest priority)
  - Unused variables/functions (clean up for maintainability)
  - Test failures (assertion, panic, timeout)
- Prioritize compilation errors first, then unused code, then test failures

### 3. Analysis Phase
For each failing test:
- Read error messages and stack traces carefully
- Examine the test code to understand what it's testing
- Check the implementation being tested
- Look for recent changes that might have caused the break
- Identify root cause category:
  - **Implementation bug**: Code logic is wrong
  - **Outdated test**: Test expectations need updating
  - **Test setup issue**: Mocks, fixtures, or environment problems
  - **Dependency issue**: External services or database problems

### 4. Systematic Fix Phase (Iterative Approach)

#### Linting Issues:
- **Use speed-editor agent** for bulk removal of unused code
- Remove unused functions, variables, constants, and imports
- Run `make lint` after each batch of fixes to verify
- Clean code improves maintainability and reduces confusion

#### Test Failures:
- **Fix one test at a time using iterative approach**:
  1. Start with minimal fix to make test compile
  2. Run `go vet` to verify compilation and static analysis
  3. Run the specific test: `go test -v ./path/to/package -run TestName`
  4. If it passes, gradually improve the test
  5. If it fails, fix incrementally - don't rewrite everything at once
- For implementation bugs: Fix the actual code
- For outdated tests: Update test expectations incrementally
- For setup issues: Fix test initialization step by step
- **Use the test-editor agent** for complex test fixes or when adding new test cases

### 5. Validation Phase
- Run `go vet ./...` to catch static analysis issues
- Run `make lint` to ensure no unused code remains
- Run full test suite with `make test` to ensure no regressions (use 10-minute timeout for full suite)
- Use `make test failures` to quickly identify any remaining failed tests (use 10-minute timeout)
- Run `make check-tr` if translation files were modified

## Best Practices

- **Start small**: Fix minimal compilation errors first, then expand
- **Clean as you go**: Remove unused code to improve maintainability
- **Use iterative approach**: Don't try to fix everything at once
- **Leverage agents**: Use speed-editor for bulk cleanup, test-editor for complex tests
- **Leverage ITF**: Use framework features for database isolation and cleanup
- **NEVER delete tests unless specifically asked** - Tests are valuable
- **Fix root cause, not symptoms** - Don't just change assertions to pass
- **Use test-editor agent** for complex fixes or when adding coverage
- **Verify incrementally**: Run `go vet` and `make lint` after each change
- **Test patterns**: Follow repository/service/controller patterns from test-editor agent