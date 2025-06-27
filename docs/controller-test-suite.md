# Controller Test Suite

A minimalistic, fluent API for testing HTTP controllers in the IOTA SDK.

## Quick Start

```go
package mymodule_test

import (
    "testing"
    "github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
)

func TestMyController(t *testing.T) {
    suite := controllertest.New(t, myModule)
    suite.Register(myController)
    
    suite.GET("/users").Expect(t).Status(200)
}
```

## API Reference

### Suite Creation

#### `New(t *testing.T, modules ...application.Module) *Suite`

Creates a new test suite with the specified modules.

```go
suite := controllertest.New(t, userModule, authModule)
```

### Suite Configuration

#### `Register(controller) *Suite`

Registers a controller with the test suite.

```go
suite.Register(userController).Register(authController)
```

#### `AsUser(user) *Suite`

Sets the authenticated user context for subsequent requests.

```go
adminUser := &user.User{ID: 1, Role: "admin"}
suite.AsUser(adminUser).GET("/admin").Expect(t).Status(200)
```

#### `Environment() *builder.TestEnvironment`

Returns the underlying test environment for advanced access.

```go
env := suite.Environment()
db := env.Pool // Access database connection
```

### HTTP Request Methods

All HTTP methods return a `*Request` for method chaining.

```go
suite.GET("/path")
suite.POST("/path") 
suite.PUT("/path")
suite.DELETE("/path")
```

### Request Building

#### `JSON(v interface{}) *Request`

Sets the request body as JSON.

```go
userData := map[string]string{"name": "John", "email": "john@example.com"}
suite.POST("/users").JSON(userData).Expect(t).Status(201)
```

#### `Form(values url.Values) *Request`

Sets the request body as form data.

```go
values := url.Values{}
values.Set("username", "john")
values.Set("password", "secret")

suite.POST("/login").Form(values).Expect(t).Status(302)
```

#### `File(fieldName, fileName string, content []byte) *Request`

Sends a multipart form with a file upload.

```go
content := []byte("file content")
suite.POST("/upload").
    File("document", "test.txt", content).
    Expect(t).Status(200)
```

#### `Header(key, value string) *Request`

Adds a custom header to the request.

```go
suite.GET("/api/data").
    Header("Authorization", "Bearer token123").
    Expect(t).Status(200)
```

#### `Cookie(name, value string) *Request`

Adds a cookie to the request.

```go
suite.GET("/dashboard").
    Cookie("session_id", "abc123").
    Expect(t).Status(200)
```

#### `HTMX() *Request`

Marks the request as an HTMX request (adds `Hx-Request: true` header).

```go
suite.POST("/partial").
    HTMX().
    Expect(t).Status(200)
```

### Response Assertions

#### `Expect(t *testing.T) *Response`

Executes the request and returns a response for assertions.

```go
response := suite.GET("/users").Expect(t)
```

### Response Methods

#### `Status(code int) *Response`

Asserts the HTTP status code.

```go
suite.GET("/users").
    Expect(t).
    Status(200)
```

#### `RedirectTo(location string) *Response`

Asserts the redirect location.

```go
suite.POST("/login").
    Form(loginData).
    Expect(t).
    Status(302).
    RedirectTo("/dashboard")
```

#### `Contains(text string) *Response`

Asserts the response body contains the specified text.

```go
suite.GET("/users").
    Expect(t).
    Status(200).
    Contains("John Doe")
```

#### `NotContains(text string) *Response`

Asserts the response body does not contain the specified text.

```go
suite.GET("/users").
    Expect(t).
    Status(200).
    NotContains("Admin Panel")
```

#### `Body() string`

Returns the response body as a string.

```go
body := suite.GET("/api/data").Expect(t).Body()
// Parse or inspect body content
```

#### `Header(key string) string`

Returns a response header value.

```go
contentType := suite.GET("/api/data").
    Expect(t).
    Header("Content-Type")
```

#### `Cookies() []*http.Cookie`

Returns response cookies.

```go
cookies := suite.POST("/login").
    Form(loginData).
    Expect(t).
    Cookies()
```

#### `Raw() *http.Response`

Returns the raw HTTP response for advanced inspection.

```go
response := suite.GET("/api/data").Expect(t).Raw()
```

### HTML Assertions

#### `HTML() *HTML`

Parses the response body as HTML and returns an HTML assertion object.

```go
suite.GET("/form").
    Expect(t).
    Status(200).
    HTML().
    Element("//input[@name='email']").
    Exists()
```

### HTML Methods

#### `Element(xpath string) *Element`

Finds a single HTML element using XPath.

```go
element := response.HTML().Element("//h1[@class='title']")
```

#### `Elements(xpath string) []*html.Node`

Finds multiple HTML elements using XPath.

```go
links := response.HTML().Elements("//a[@href]")
```

#### `HasErrorFor(fieldID string) bool`

Checks if there's a validation error for a specific field.

```go
hasError := response.HTML().HasErrorFor("email")
```

### Element Assertions

#### `Exists() *Element`

Asserts the element exists.

```go
suite.GET("/form").
    Expect(t).
    HTML().
    Element("//form[@id='login']").
    Exists()
```

#### `NotExists() *Element`

Asserts the element doesn't exist.

```go
suite.GET("/public").
    Expect(t).
    HTML().
    Element("//a[@href='/admin']").
    NotExists()
```

#### `Text() string`

Returns the element's text content.

```go
title := suite.GET("/page").
    Expect(t).
    HTML().
    Element("//h1").
    Text()
```

#### `Attr(name string) string`

Returns an element attribute value.

```go
href := suite.GET("/page").
    Expect(t).
    HTML().
    Element("//a[@class='button']").
    Attr("href")
```

## Examples

### Basic CRUD Testing

```go
func TestUserCRUD(t *testing.T) {
    suite := controllertest.New(t, userModule)
    suite.Register(userController)
    
    // List users
    suite.GET("/users").
        Expect(t).
        Status(200).
        Contains("Users List")
    
    // Create user
    userData := map[string]string{
        "name":  "John Doe", 
        "email": "john@example.com",
    }
    suite.POST("/users").
        JSON(userData).
        Expect(t).
        Status(201)
    
    // Update user
    updateData := map[string]string{"name": "Jane Doe"}
    suite.PUT("/users/1").
        JSON(updateData).
        Expect(t).
        Status(200)
    
    // Delete user
    suite.DELETE("/users/1").
        Expect(t).
        Status(204)
}
```

### Authentication Testing

```go
func TestAuthenticatedRoutes(t *testing.T) {
    suite := controllertest.New(t, authModule, userModule)
    suite.Register(authController).Register(userController)
    
    // Unauthenticated access should redirect
    suite.GET("/profile").
        Expect(t).
        Status(302).
        RedirectTo("/login")
    
    // Authenticated access should work
    user := &user.User{ID: 1, Email: "user@example.com"}
    suite.AsUser(user).
        GET("/profile").
        Expect(t).
        Status(200).
        Contains("user@example.com")
}
```

### Form Validation Testing

```go
func TestFormValidation(t *testing.T) {
    suite := controllertest.New(t, userModule)
    suite.Register(userController)
    
    // Invalid form should show errors
    invalidData := url.Values{}
    invalidData.Set("email", "invalid-email")
    
    suite.POST("/users").
        Form(invalidData).
        Expect(t).
        Status(422).
        HTML().
        HasErrorFor("email") // true
}
```

### File Upload Testing

```go
func TestFileUpload(t *testing.T) {
    suite := controllertest.New(t, uploadModule)
    suite.Register(uploadController)
    
    fileContent := []byte("test file content")
    suite.POST("/upload").
        File("document", "test.txt", fileContent).
        Expect(t).
        Status(200).
        Contains("Upload successful")
}
```

### HTMX Testing

```go
func TestHTMXEndpoints(t *testing.T) {
    suite := controllertest.New(t, htmxModule)
    suite.Register(htmxController)
    
    // HTMX request should return partial HTML
    suite.POST("/search").
        HTMX().
        Form(url.Values{"q": {"query"}}).
        Expect(t).
        Status(200).
        HTML().
        Element("//div[@class='search-results']").
        Exists()
}
```

## Best Practices

1. **One suite per test function**: Create a fresh suite for each test to avoid state pollution.

2. **Use descriptive test names**: Make test intentions clear.

3. **Chain assertions**: Use the fluent API to make tests readable.

4. **Test edge cases**: Include tests for validation errors, authentication failures, etc.

5. **Leverage user context**: Use `AsUser()` to test authorization scenarios.

6. **Validate HTML structure**: Use XPath assertions to verify UI elements.

7. **Test both success and failure paths**: Ensure your controllers handle errors gracefully.