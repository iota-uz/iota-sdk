# IOTA Testing Framework (ITF)

The IOTA Testing Framework provides comprehensive tools for testing HTTP controllers in IOTA SDK applications. ITF features a SuiteBuilder pattern, fluent assertions, HTMX support, and table-driven testing capabilities.

## Key Features

### SuiteBuilder Pattern
Eliminates repetitive setup code with a fluent API:

```go
suite := itf.NewSuiteBuilder(t).
    WithModules(myModule).
    AsAdmin().
    Build()
```

### Enhanced Assertions (40+ Methods)
Fluent API for HTTP response assertions:

```go
suite.GET("/api/users").
    Assert(t).
    ExpectOK().
    ExpectJSON().
    ExpectField("count", 5)
```

### Upload Helper Method
Simplified file upload testing:

```go
// Upload file content in a single line
suite.Upload("/upload", fileContent, "filename.xlsx").
    Assert(t).
    ExpectOK()
```

### Query Parameter Builder
Clean query parameter handling:

```go
suite.GET("/api/users").
    WithQuery(map[string]string{
        "status": "active",
        "limit":  "10",
    }).
    Assert(t).ExpectOK()
```

### Enhanced Form Builder
Type-safe form field building:

```go
suite.POST("/users").
    FormField("Name", "John Doe").
    FormField("Age", 30).              // Supports multiple types
    FormField("Email", "john@example.com").
    Assert(t).ExpectCreated()
```

### Test Case Builder Pattern
Fluent test case construction:

```go
testCases := itf.Cases(
    itf.GET("/api/users").
        Named("List all users").
        ExpectOK().
        ExpectHTML().
        ExpectElement("//table"),
        
    itf.POST("/api/users").
        Named("Create user").
        FormField("Name", "Jane").
        ExpectCreated(),
)
suite.RunCases(testCases)
```

### HTMX-Aware Assertions
First-class HTMX testing support:

```go
suite.POST("/submit").
    FormField("data", "value").
    HTMX().
    HTMXTarget("#content").
    Assert(t).
    ExpectHTMXSwap("innerHTML").
    ExpectHTMXTrigger("dataUpdated")
```

### Database Name Truncation Fix
Automatic database name handling for long test names.

## Quick Start

### Basic Setup
```go
func TestMyController(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        WithModules(myModule).
        AsAdmin().
        Build()
    
    suite.Register(&MyController{})
}
```

### Common Assertion Shortcuts
```go
// Before
response := suite.GET("/api/data").Assert(t)
response.ExpectStatus(200)
response.ExpectHTML()
response.ExpectElement("//table")

// After
suite.GET("/api/data").
    Assert(t).
    ExpectOK().
    ExpectHTML().
    ExpectElement("//table")
```

## API Reference

### SuiteBuilder Methods

**Configuration:**
- `NewSuiteBuilder(t testing.TB) *SuiteBuilder`
- `WithModules(...application.Module) *SuiteBuilder`
- `WithUser(user.User) *SuiteBuilder`
- `WithTenant(string) *SuiteBuilder`

**User Presets:**
- `AsUser(...*permission.Permission) *SuiteBuilder`
- `AsAdmin() *SuiteBuilder`
- `AsReadOnly() *SuiteBuilder`
- `AsGuest() *SuiteBuilder`
- `AsAnonymous() *SuiteBuilder`

**Building:**
- `Build() *Suite`
- `BuildWithOptions(...Option) *Suite`

### Request Methods

**HTTP Verbs:**
- `GET(path string) *Request`
- `POST(path string) *Request`
- `PUT(path string) *Request`
- `PATCH(path string) *Request`
- `DELETE(path string) *Request`

**Form Building:**
- `Form(url.Values) *Request`
- `FormField(name string, value interface{}) *Request`

**Query Parameters:**
- `WithQuery(map[string]string) *Request`

**File Uploads:**
- `Upload(path string, content []byte, filename string) *Response`

**HTMX Support:**
- `HTMX() *Request`
- `HTMXTarget(selector string) *Request`

### Response Assertions

**Status Assertions:**
- `ExpectStatus(int) *Assertion`
- `ExpectOK()` - Status 200
- `ExpectCreated()` - Status 201
- `ExpectBadRequest()` - Status 400
- `ExpectUnauthorized()` - Status 401
- `ExpectForbidden()` - Status 403
- `ExpectNotFound()` - Status 404
- `ExpectInternalServerError()` - Status 500

**Content Type Assertions:**
- `ExpectHTML() *HTMLAssertion`
- `ExpectJSON() *JSONAssertion`
- `ExpectText()`
- `ExpectContentType(string)`

**Common Shortcuts:**
- `ExpectOKWithHTML()` - 200 + HTML
- `ExpectOKWithForm(action string)` - 200 + HTML + form element
- `ExpectSuccess()` - Any 2xx status
- `ExpectError(substring string)` - Error status + message contains

**Body Assertions:**
- `ExpectBodyContains(string)`
- `ExpectBodyNotContains(string)`
- `ExpectBodyEquals(string)`
- `ExpectBodyEmpty()`

**Header Assertions:**
- `ExpectHeader(name, value string)`
- `ExpectHeaderContains(name, substring string)`
- `ExpectHeaderExists(string)`
- `ExpectRedirectTo(string)`

**HTMX Assertions:**
- `ExpectHTMXTrigger(string)`
- `ExpectHTMXRedirect(string)`
- `ExpectHTMXSwap(string)`
- `ExpectHTMXRetarget(string)`

### HTML Assertions

**Element Assertions:**
- `ExpectTitle(string)`
- `ExpectElement(xpath string) *ElementAssertion`
- `ExpectNoElement(xpath string)`
- `ExpectForm(xpath string) *FormAssertion`

**Element Methods:**
- `ExpectText(string)`
- `ExpectTextContains(string)`
- `ExpectAttribute(name, value string)`
- `ExpectClass(string)`

**Form Methods:**
- `ExpectAction(string)`
- `ExpectMethod(string)`
- `ExpectFieldValue(name, value string)`

### JSON Assertions
- `ExpectField(path string, value interface{})`
- `ExpectStructure(interface{})`

### Table-Driven Testing

**Test Case Structure:**
```go
testCases := itf.Cases(
    itf.GET("/endpoint").Named("test name").ExpectOK(),
    itf.POST("/endpoint").FormField("key", "value").ExpectCreated(),
)
```

**Execution:**
- `RunCases([]TestCase)`
- `RunCase(TestCase)`

### Test Case Builders
- `Cases(...*TestCaseBuilder) []TestCase`
- `GET(path string) *TestCaseBuilder`
- `POST(path string) *TestCaseBuilder`
- `PUT(path string) *TestCaseBuilder`
- `DELETE(path string) *TestCaseBuilder`
- `Upload(path, content, filename) *TestCaseBuilder`

## Migration Guide

### From Legacy ITF
```go
// Old way
suite := itf.HTTP(t, modules...)
response := suite.GET("/test").Expect(t)
assert.Equal(t, 200, response.Raw().StatusCode)

// New way
suite := itf.NewSuiteBuilder(t).
    WithModules(modules...).
    AsAdmin().
    Build()

suite.GET("/test").
    Assert(t).
    ExpectOK()
```

### Backwards Compatibility
All existing ITF code continues to work without modification.

## Best Practices

### 1. Use SuiteBuilder for New Tests
```go
suite := itf.NewSuiteBuilder(t).AsAdmin().Build()
```

### 2. Leverage Element Assertions
```go
suite.GET("/api").Assert(t).ExpectOK().ExpectHTML().ExpectElement("//table")
```

### 3. Use Table-Driven Testing for Multiple Scenarios
```go
testCases := itf.Cases(
    itf.GET("/users").Named("list users").ExpectOK().ExpectHTML().ExpectElement("//table"),
    itf.POST("/users").FormField("name", "John").ExpectCreated(),
)
suite.RunCases(testCases)
```

### 4. Test HTMX Interactions
```go
suite.POST("/form").
    FormField("data", "value").
    HTMX().
    HTMXTarget("#content").
    Assert(t).
    ExpectHTMXSwap("innerHTML")
```

### 5. Use Upload Helper for File Tests
```go
suite.Upload("/import", csvContent, "data.csv").
    Assert(t).
    ExpectOK()
```

## Examples

### Complete Controller Test
```go
func TestUserController(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        WithModules(userModule).
        AsAdmin().
        Build()
    
    suite.Register(&UserController{})
    
    // Table-driven test cases
    testCases := itf.Cases(
        itf.GET("/users").
            Named("list users").
            ExpectOK().
            ExpectHTML().
            ExpectElement("//table"),
            
        itf.POST("/users").
            Named("create user").
            FormField("Name", "John Doe").
            FormField("Email", "john@example.com").
            ExpectCreated(),
            
        itf.GET("/users/search").
            Named("search users").
            WithQuery(map[string]string{"q": "john"}).
            ExpectOK().
            ExpectHTML().
            ExpectElement("//table"),
    )
    
    suite.RunCases(testCases)
}
```

The ITF provides a comprehensive, fluent testing experience that reduces boilerplate while maintaining full compatibility with existing test suites.