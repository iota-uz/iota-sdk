# Backend Testing Guide - ITF Framework

**Testing patterns and standards using the IOTA Testing Framework (ITF).**

## Overview & Philosophy

### Purpose

ITF provides:

- **Isolated databases** - Clean, isolated test database per test
- **Fluent builders** - Modern, readable test APIs
- **Multi-tenant support** - Built-in tenant isolation
- **HTTP testing** - Complete HTTP request/response testing
- **Modern assertions** - Chainable, expressive assertions

### Core Principles

1. **Test Isolation**: Every test runs in its own database instance
2. **No Raw SQL**: Use repositories and services in tests
3. **Success-First**: Write happy path tests first, then edge cases
4. **Parallel Execution**: Use `t.Parallel()` for all tests

## ITF Framework Core APIs

### Test Environment Setup

```go
// Basic environment setup
itf.Setup(tb testing.TB, opts ...Option) *TestEnvironment

// HTTP suite with modules
itf.HTTP(tb testing.TB, modules ...application.Module) *Suite

// Modern fluent builder (recommended)
itf.NewSuiteBuilder(t) *SuiteBuilder

// Test context
itf.NewTestContext() *TestContext

// Database manager
itf.NewDatabaseManager(t *testing.T) *DatabaseManager
```

### Setup Options

```go
// Module configuration
itf.WithModules(modules ...application.Module)

// Database configuration
itf.WithDatabase(name string)

// User configuration
itf.WithUser(u user.User)
```

### TestEnvironment

```go
te.Service(service interface{}) interface{}        // Legacy service access
itf.GetService[T any](te *TestEnvironment) *T     // Modern generic service access
te.AssertNoError(tb testing.TB, err error)
te.TenantID() uuid.UUID
te.WithTx(ctx context.Context) context.Context
```

### User/Permissions/Tx/Tenant

```go
itf.User(permissions ...*permission.Permission) user.User
itf.Transaction(tb testing.TB, env *TestEnvironment) pgx.Tx
itf.CreateTestTenant(ctx context.Context, pool *pgxpool.Pool) (*composables.Tenant, error)
```

### HTTP Suite

```go
suite.AsUser(u user.User) *Suite
suite.Register(controller interface{ Register(*mux.Router) })
suite.WithMiddleware(mw MiddlewareFunc)
suite.BeforeEach(hook HookFunc)
suite.Environment() *TestEnvironment
suite.Env() *TestEnvironment // Alias

// HTTP methods (PATCH not implemented)
suite.GET(path string) *Request
suite.POST(path string) *Request
suite.PUT(path string) *Request
suite.DELETE(path string) *Request
```

### Request Builders

```go
suite.GET|POST|PUT|DELETE(path string) *Request
```

### Request Configuration

```go
req.JSON(v any)
req.Form(url.Values)
req.FormField(key string, v any)
req.FormFields(map[string]any)
req.FormString/Int/Bool/Float
req.Header(k,v)
req.Cookie(name,value)
req.HTMX()

// HTMX enhanced
req.HTMXTarget/HTMXTrigger/HTMXSwap/HTMXCurrentURL/HTMXPrompt/HTMXHistoryRestore/HTMXBoosted

// Query
req.WithQuery(map[string]string)
req.WithQueryValue(key,value)
```

### Execute / Assert (classic)

```go
req.Expect(tb) *Response
req.Assert(tb) *ResponseAssertion

// Response methods
.Status(code)
.RedirectTo(loc)
.Contains(text)
.NotContains(text)
.Body()
.Header(k)
.Cookies()
.Raw()
.HTML()
```

### Fluent Response Assertions

**Status**:
```go
ExpectOK/Created/Accepted/NoContent/BadRequest/Unauthorized/Forbidden/NotFound/MethodNotAllowed/Conflict/UnprocessableEntity/InternalServerError/ExpectStatus(code)
```

**Content-Type**:
```go
ExpectHTML()/ExpectJSON()/ExpectText()/ExpectContentType(type)
```

**Body**:
```go
ExpectBodyContains/NotContains/Equals/Empty
```

**Headers**:
```go
ExpectHeader/ExpectHeaderContains/ExpectHeaderExists
```

**Redirects**:
```go
ExpectRedirectTo/ExpectRedirect
```

**HTMX**:
```go
ExpectHTMXTrigger/Redirect/Reswap/Retarget/Swap/Location/PushURL/ReplaceURL/Refresh/TriggerAfterSwap/TriggerAfterSettle/Reselect/TriggerWithData/NoHTMXHeaders
```

**Shortcuts**:
```go
ExpectFlash(message)
ExpectDownload(contentType, fileName)
```

### HTML/DOM

```go
html.Element(xpath)
html.Elements(xpath)
html.HasErrorFor/ExpectErrorFor/ExpectNoErrorFor

element.Exists/NotExists/Text/Attr

// HTML Assertions
ExpectTitle(title)
ExpectElement(xpath)
ExpectNoElement(xpath)
ExpectForm(xpath) → FormAssertion.ExpectAction/ExpectMethod/ExpectFieldValue
ElementAssertion.ExpectText/ExpectTextContains/ExpectAttribute/ExpectClass
```

### JSON

```go
ExpectJSON().ExpectField(path, expected)
ExpectJSON().ExpectStructure(target)
```

### SuiteBuilder (fluent)

```go
itf.NewSuiteBuilder(t).
    WithModules(modules...).
    WithUser(u).AsUser(perms...).AsAdmin().AsReadOnly().AsGuest().AsAnonymous().
    Tenant().WithID(tenantID).Isolated().
    Build()/BuildWithOptions(opts...)
```

**Presets**:
```go
AdminWithAllModules(modules...)
ReadOnlyWithCore()
Anonymous()
QuickTest()
```

### TestCaseBuilder (fluent)

**Entry**:
```go
itf.GET/POST/PUT/DELETE(path)
```

**Config (immutable)**:
```go
Named
WithQuery/WithQueryParam
WithForm/WithFormField
WithJSON
WithHeader
HTMX/HTMXTarget/HTMXTrigger/HTMXSwap
```

**Expectations**:
```go
ExpectOK/.../ExpectStatus
ExpectElement
ExpectRedirect
```

**Lifecycle**:
```go
Setup
Cleanup
Skip
Only
Assert(func(t,*Response))
```

**Build**:
```go
TestCase()
```

**Batch**:
```go
itf.Cases(...)
suite.RunCases(cases)
suite.RunBatch(cases, *BatchTestConfig{Parallel, MaxWorkers, BeforeEach, AfterEach, OnError})
```

### File Uploads

```go
// Two-step helper
suite.Upload(targetPath, fileContent []byte, fileName string) *Response

// Validates non-empty input, extracts FileID via HTML, posts to target; detailed errors
```

### Excel Utilities

**Builder**:
```go
itf.Excel().
    WithSheet(name).
    WithHeaders(...).
    AddRow(...).
    AddRows(...).
    Build(t) string / BuildBytes(t) []byte
```

**Helpers**:
```go
BuildEmptyExcel
BuildEmptyExcelBytes
BuildInvalidExcelBytes
BuildWithCustomHeaders
BuildWithCustomHeadersBytes
```

## Layer-Specific Testing Patterns

### Repository Tests

**Location**: `modules/{module}/infrastructure/persistence/*_repository_test.go`

**Purpose**: Verify database operations

**Coverage**:
- CRUD operations
- Unique/FK violations
- Tenant isolation
- Pagination/filtering
- Complex queries
- Rollback behavior

**Pattern (Minimal→Expanded)**:

```go
func TestRepositoryName_Create(t *testing.T) {
    f := setupTest(t)
    repo := persistence.NewRepositoryName()
    entity := domain.New("Test Name")
    created, err := repo.Create(f.Ctx, entity)
    require.NoError(t, err)
    require.NotNil(t, created)
}
```

**Expanded with assertions**:

```go
func TestRepositoryName_Create(t *testing.T) {
    f := setupTest(t)
    repo := persistence.NewRepositoryName()
    created, err := repo.Create(f.Ctx, domain.New("Test Name"))
    require.NoError(t, err)
    assert.NotZero(t, created.ID())
    assert.Equal(t, "Test Name", created.Name())
}
```

**Table-driven CRUD**:

```go
func TestRepositoryName_CRUD(t *testing.T) {
    t.Parallel()
    f := setupTest(t)
    repo := persistence.NewRepositoryName()

    t.Run("Create", func(t *testing.T) {
        _, err := repo.Create(f.Ctx, domain.New("X"))
        require.NoError(t, err)
    })

    t.Run("GetByID", func(t *testing.T) {
        // ... implementation
    })

    t.Run("Update", func(t *testing.T) {
        // ... implementation
    })

    t.Run("Delete", func(t *testing.T) {
        // ... implementation
    })
}
```

### Service Tests

**Location**: `modules/{module}/services/*_service_test.go`

**Purpose**: Verify business logic

**Coverage**:
- Happy path
- Validation errors
- Business rules
- Permission checks
- External errors
- Transaction boundaries
- Event publishing

**Pattern (Minimal→Table)**:

```go
func TestServiceName_Method(t *testing.T) {
    f := setupTest(t, permissions.RequiredPermission)
    svc := getServiceFromEnv[services.ServiceName](f)
    res, err := svc.Method(f.Ctx, "valid")
    require.NoError(t, err)
    require.NotNil(t, res)
}
```

**Table-driven**:

```go
func TestServiceName_Method(t *testing.T) {
    t.Parallel()
    f := setupTest(t, permissions.RequiredPermission)
    svc := getServiceFromEnv[services.ServiceName](f)

    cases := []struct{
        name string
        in   string
        want error
    }{
        {"happy", "valid", nil},
        {"empty", "", ErrValidation},
    }

    for _, tc := range cases {
        t.Run(tc.name, func (t *testing.T) {
            _, err := svc.Method(f.Ctx, tc.in)
            if tc.want != nil {
                require.ErrorIs(t, err, tc.want)
            } else {
                require.NoError(t, err)
            }
        })
    }
}
```

### Controller Tests

**Location**: `modules/{module}/presentation/controllers/*_controller_test.go`

**Purpose**: Verify HTTP layer

**Coverage**:
- Routes/methods
- Parsing (JSON/form/multipart)
- AuthN/Z
- HTMX interactions
- Error formats
- File uploads
- Redirects/status codes

**Pattern (Minimal→Builder)**:

```go
func TestControllerName_Get(t *testing.T) {
    suite := itf.HTTP(t, module.NewModule(opts))
    c := controllers.NewControllerName(suite.Env().App)
    suite.Register(c)
    suite.GET("/path").Assert(t).ExpectOK()
}
```

**With SuiteBuilder**:

```go
func TestControllerName_Handler(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        WithModules(module.NewModule(opts)).
        AsUser(permissions.ViewPermission).
        Build()

    c := controllers.NewControllerName(suite.Env().App)
    suite.Register(c)

    cases := itf.Cases(
        itf.GET("/path").
            Named("Happy").
            ExpectOK().
            Assert(func(t *testing.T, r *itf.Response) {
                r.Contains("expected")
            }),
        itf.GET("/nonexistent").
            Named("NotFound").
            ExpectNotFound(),
    )

    suite.RunCases(cases)
}
```

**File Upload Example**:

```go
func TestControllerName_FileUpload(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        Presets().
        AdminWithAllModules(module.NewModule(opts))

    c := controllers.NewControllerName(suite.Env().App)
    suite.Register(c)

    bytes := itf.Excel().
        WithHeaders("Name", "Email").
        AddRow(map[string]any{"Name":"John", "Email":"j@x"}).
        BuildBytes(t)

    suite.Upload("/import-contacts", bytes, "contacts.xlsx").
        Status(200).
        Contains("imported successfully")
}
```

## Setup Helpers

```go
func setupTest(t *testing.T, perms ...*permission.Permission) *itf.TestEnvironment {
    t.Helper()
    return itf.Setup(t,
        itf.WithModules(modules.BuiltInModules...),
        itf.WithUser(itf.User(perms...)),
    )
}

func setupHTTPSuite(t *testing.T) *itf.Suite {
    t.Helper()
    return itf.NewSuiteBuilder(t).
        WithModules(modules.BuiltInModules...).
        AsUser(permissions.DefaultPermissions...).
        Build()
}

func getServiceFromEnv[T any](env *itf.TestEnvironment) *T {
    return itf.GetService[T](env)
}

func setupDatabaseTest(t *testing.T) *itf.DatabaseManager {
    t.Helper()
    return itf.NewDatabaseManager(t)
}
```

## Test Execution Commands

```bash
# Quick run
make test

# Failures only
make test failures

# Coverage
make test coverage

# Detailed coverage
make test detailed-coverage

# Verbose
make test verbose

# Single test
go test -run ^TestName$ ./path

# Watch mode
make test watch
```

## Regression Testing

**Bug fixes**: Create failing test → fix → add edge cases

**New features**: TDD tests first → cover requirements → errors/edges

## Critical Lessons & Constraints

### PostgreSQL Constraints

- **DB name ≤ 63 chars** → keep test names short (`Valid`, `InvalidID`, `NoPerm`)

### Org vs Tenant

- Many ops require **organization ID**
- Create organizations first
- Use `composables.GetOrgID(ctx)`

### DI Pattern

- Inject by parameter type
- Mock services via signatures

### Permissions

- Analyze `sdkcomposables.CanUser()` and service-level checks

### FK Order

- Create parents first (organization → child)

### Mocks

- Expectations must match real params/returns

### Middleware

- Ensure context (tenant/org/user) setup

### Success-First

- Get happy path green, then errors

### Don't

- Raw SQL in tests
- External network calls
- Test implementation details
- Delete tests without instruction
- Long test names
- Tenant≠org confusion
- Direct HX headers (use `pkg/htmx`)

## Test Patterns (Iterative)

### Start Small

1. **Minimal test** for happy path first
2. Prefer table-driven
3. Add one assertion/case at a time

### Iterate

1. **Static check**: `go vet ./...` (fail fast) → fix
2. **Targeted run**: `go test ./pkg -run ^TestName$ -count=1` or `make test`
3. **Expand**: Add error cases, edge cases, permissions
4. **Coverage**: `make test coverage` when behavior stabilizes

### Finalize

1. `go vet`
2. `make fix imports`
3. Full `make test`

## Common Pitfalls

### Missing `t.Parallel()`

Breaks test isolation - always include

### Raw SQL in Tests

Use repositories and services

### Missing Parent Entity Creation

FK constraints require parent entities first

### Tenant vs Organization Confusion

Many operations require organization ID, not just tenant ID
