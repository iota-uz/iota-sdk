---
layout: default
title: Controller Testing
parent: Advanced
nav_order: 5
description: "Controller testing with fluent API in IOTA SDK"
---

# Controller Testing

The IOTA SDK provides a minimalistic, fluent API for testing HTTP controllers with support for authentication, forms, file uploads, and HTMX interactions.

## Overview

The Controller Test Suite enables:

- **Fluent Test Builder**: Chain assertions for readable tests
- **HTTP Methods**: GET, POST, PUT, DELETE with full request control
- **Form Testing**: Submit forms with validation error checking
- **File Uploads**: Test multipart file uploads
- **Authentication**: Test with authenticated users
- **HTMX Testing**: Verify HTMX request handling
- **HTML Assertions**: XPath-based element assertions
- **Response Validation**: Status codes, headers, redirects, content

## Quick Start

### Basic Test

```go
import (
    "testing"
    "github.com/iota-uz/iota-sdk/pkg/testutils/controllertest"
)

func TestUserController(t *testing.T) {
    t.Parallel()

    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // Test GET request
    suite.GET("/users").
        Expect(t).
        Status(200).
        Contains("Users List")
}
```

## Test Suite Setup

### Creating a Suite

```go
// Basic setup
suite := controllertest.New(t, userModule)

// Multiple modules
suite := controllertest.New(t, coreModule, userModule, financeModule)

// Register controllers
suite.Register(userController).
      Register(paymentController).
      Register(reportController)
```

### Accessing Test Environment

```go
// Get underlying environment for direct access
env := suite.Environment()

// Access database
db := env.DB()

// Access services
userService := env.GetService[*services.UserService]()

// Access context
ctx := env.Context()
```

## HTTP Methods

All HTTP methods return a `*Request` for method chaining:

```go
// GET request
suite.GET("/users").
    Expect(t).
    Status(200)

// POST request
suite.POST("/users").
    JSON(data).
    Expect(t).
    Status(201)

// PUT request
suite.PUT("/users/123").
    JSON(updateData).
    Expect(t).
    Status(200)

// DELETE request
suite.DELETE("/users/123").
    Expect(t).
    Status(204)

// PATCH request
suite.PATCH("/users/123").
    JSON(patchData).
    Expect(t).
    Status(200)
```

## Request Building

### JSON Payload

```go
data := map[string]interface{}{
    "firstName": "John",
    "lastName": "Doe",
    "email": "john@example.com",
}

suite.POST("/users").
    JSON(data).
    Expect(t).
    Status(201)
```

### Form Data

```go
import "net/url"

values := url.Values{}
values.Set("firstName", "John")
values.Set("lastName", "Doe")
values.Set("email", "john@example.com")

suite.POST("/users").
    Form(values).
    Expect(t).
    Status(201)
```

### Headers

```go
suite.GET("/api/data").
    Header("Authorization", "Bearer token123").
    Header("Accept", "application/json").
    Expect(t).
    Status(200)
```

### Cookies

```go
suite.GET("/dashboard").
    Cookie("session_id", "abc123").
    Cookie("preference", "dark_mode").
    Expect(t).
    Status(200)
```

### File Upload

```go
fileContent := []byte("CSV file content")

suite.POST("/import").
    File("csv", "data.csv", fileContent).
    Expect(t).
    Status(200).
    Contains("Import successful")
```

### HTMX Request

```go
suite.POST("/search").
    HTMX().  // Adds HX-Request: true header
    Form(url.Values{"q": {"search query"}}).
    Expect(t).
    Status(200).
    HTML().
    Element("//div[@class='results']").
    Exists()
```

## Response Assertions

### Status Code

```go
// Assert status
suite.GET("/users").
    Expect(t).
    Status(200)

// Multiple assertions in chain
suite.POST("/users").
    JSON(data).
    Expect(t).
    Status(201).
    Contains("Created")
```

### Body Content

```go
// Contains text
suite.GET("/users").
    Expect(t).
    Contains("John Doe")

// Not contains text
suite.GET("/users").
    Expect(t).
    NotContains("Admin Panel")

// Get raw body
body := suite.GET("/api/data").
    Expect(t).
    Body()

// Parse as JSON
var result map[string]interface{}
json.Unmarshal([]byte(body), &result)
```

### Redirect

```go
suite.POST("/login").
    Form(loginData).
    Expect(t).
    Status(302).
    RedirectTo("/dashboard")
```

### Headers

```go
contentType := suite.GET("/api/data").
    Expect(t).
    Header("Content-Type")

if contentType != "application/json" {
    t.Error("Expected JSON response")
}
```

### Cookies

```go
response := suite.POST("/login").
    Form(loginData).
    Expect(t).
    Status(302)

cookies := response.Cookies()
for _, cookie := range cookies {
    if cookie.Name == "session_id" {
        t.Log("Session created:", cookie.Value)
    }
}
```

## HTML Testing

### Find Elements

```go
// Find single element
suite.GET("/form").
    Expect(t).
    Status(200).
    HTML().
    Element("//input[@name='email']").
    Exists()

// Find multiple elements
response := suite.GET("/users").
    Expect(t).
    HTML()

userRows := response.Elements("//tr[@class='user-row']")
if len(userRows) != 10 {
    t.Errorf("Expected 10 user rows, got %d", len(userRows))
}
```

### Element Assertions

```go
suite.GET("/form").
    Expect(t).
    HTML().
    Element("//h1[@class='title']").
    Text() // Returns element text
```

### Element Attributes

```go
href := suite.GET("/page").
    Expect(t).
    HTML().
    Element("//a[@class='download']").
    Attr("href")

if href != "/files/download.pdf" {
    t.Errorf("Expected /files/download.pdf, got %s", href)
}
```

### Form Validation

```go
// Test validation error display
suite.POST("/users").
    Form(url.Values{
        "email": {"invalid-email"},
    }).
    Expect(t).
    Status(422).
    HTML().
    HasErrorFor("email") // Returns true if error exists
```

## Authentication Testing

### Authenticated Requests

```go
testUser := &user.User{
    ID: uuid.New(),
    Email: "test@example.com",
    FirstName: "Test",
    LastName: "User",
}

suite.AsUser(testUser).
    GET("/profile").
    Expect(t).
    Status(200).
    Contains(testUser.Email)
```

### Testing Unauthorized Access

```go
// Without authentication
suite.GET("/admin").
    Expect(t).
    Status(302).
    RedirectTo("/login")

// With regular user trying admin endpoint
regularUser := &user.User{ID: uuid.New(), Email: "user@example.com"}
suite.AsUser(regularUser).
    GET("/admin").
    Expect(t).
    Status(403) // Forbidden
```

### Testing Multiple Users

```go
func TestMultiUserAccess(t *testing.T) {
    suite := controllertest.New(t, userModule)

    adminUser := &user.User{ID: uuid.New(), Role: "admin"}
    regularUser := &user.User{ID: uuid.New(), Role: "user"}

    // Admin can access
    suite.AsUser(adminUser).
        GET("/admin").
        Expect(t).
        Status(200)

    // Regular user cannot
    suite.AsUser(regularUser).
        GET("/admin").
        Expect(t).
        Status(403)
}
```

## Test Examples

### CRUD Testing

```go
func TestUserCRUD(t *testing.T) {
    t.Parallel()

    suite := controllertest.New(t, userModule)
    suite.Register(userController)

    // CREATE
    createData := map[string]string{
        "firstName": "Jane",
        "lastName": "Smith",
        "email": "jane@example.com",
    }

    createResponse := suite.POST("/users").
        JSON(createData).
        Expect(t).
        Status(201)

    // Extract ID from response (example)
    body := createResponse.Body()
    var created map[string]interface{}
    json.Unmarshal([]byte(body), &created)
    userID := created["id"].(string)

    // READ
    suite.GET("/users/" + userID).
        Expect(t).
        Status(200).
        Contains("jane@example.com")

    // UPDATE
    updateData := map[string]string{
        "firstName": "Janet",
    }

    suite.PUT("/users/" + userID).
        JSON(updateData).
        Expect(t).
        Status(200)

    // DELETE
    suite.DELETE("/users/" + userID).
        Expect(t).
        Status(204)

    // Verify deleted
    suite.GET("/users/" + userID).
        Expect(t).
        Status(404)
}
```

### Form Validation Testing

```go
func TestUserFormValidation(t *testing.T) {
    suite := controllertest.New(t, userModule)

    // Test required field
    suite.POST("/users").
        Form(url.Values{}).
        Expect(t).
        Status(422).
        HTML().
        HasErrorFor("email")

    // Test email format
    suite.POST("/users").
        Form(url.Values{
            "firstName": {"John"},
            "lastName": {"Doe"},
            "email": {"not-an-email"},
        }).
        Expect(t).
        Status(422).
        HTML().
        HasErrorFor("email")

    // Valid form
    suite.POST("/users").
        Form(url.Values{
            "firstName": {"John"},
            "lastName": {"Doe"},
            "email": {"john@example.com"},
        }).
        Expect(t).
        Status(201)
}
```

### File Upload Testing

```go
func TestFileUpload(t *testing.T) {
    suite := controllertest.New(t, uploadModule)
    suite.Register(uploadController)

    // Test with valid file
    validContent := []byte("valid CSV content")
    suite.POST("/import").
        File("csv", "data.csv", validContent).
        Expect(t).
        Status(200).
        Contains("File imported successfully")

    // Test with invalid file type
    invalidContent := []byte("not a csv file")
    suite.POST("/import").
        File("csv", "data.txt", invalidContent).
        Expect(t).
        Status(400).
        Contains("Invalid file type")
}
```

### HTMX Testing

```go
func TestHTMXSearch(t *testing.T) {
    suite := controllertest.New(t, searchModule)
    suite.Register(searchController)

    // Test HTMX partial response
    suite.POST("/search").
        HTMX().
        Form(url.Values{"q": {"test"}}).
        Expect(t).
        Status(200).
        HTML().
        Element("//div[@class='search-results']").
        Exists()

    // Regular request returns full page
    suite.GET("/search").
        Expect(t).
        Status(200).
        Contains("<html")
}
```

## Best Practices

### 1. Use Table-Driven Tests

```go
func TestUserController(t *testing.T) {
    tests := []struct {
        name       string
        method     string
        path       string
        status     int
        authUser   *user.User
        expectErr  bool
    }{
        {
            name:   "GET users as admin",
            method: "GET",
            path:   "/users",
            status: 200,
            authUser: &user.User{Role: "admin"},
        },
        {
            name:     "GET admin panel as regular user",
            method:   "GET",
            path:     "/admin",
            status:   403,
            authUser: &user.User{Role: "user"},
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            suite := controllertest.New(t, userModule)
            if tt.authUser != nil {
                suite = suite.AsUser(tt.authUser)
            }

            suite.GET(tt.path).
                Expect(t).
                Status(tt.status)
        })
    }
}
```

### 2. Keep Tests Focused

```go
// Good: Test one thing
func TestCreateUserValidation(t *testing.T) {
    suite := controllertest.New(t, userModule)

    suite.POST("/users").
        Form(url.Values{"email": {"invalid"}}).
        Expect(t).
        Status(422).
        HTML().
        HasErrorFor("email")
}

// Bad: Test multiple things
func TestUserController(t *testing.T) {
    // Tests create, read, update, delete, validation, auth...
}
```

### 3. Use Descriptive Test Names

```go
// Good
func TestCreateUserWithValidEmail(t *testing.T) {}
func TestDeleteUnauthorizedReturns403(t *testing.T) {}
func TestSearchWithEmptyQueryReturnsEmpty(t *testing.T) {}

// Bad
func TestUser(t *testing.T) {}
func TestCreate(t *testing.T) {}
func TestAPI(t *testing.T) {}
```

### 4. Test Both Success and Failure

```go
func TestUserForm(t *testing.T) {
    suite := controllertest.New(t, userModule)

    // Success path
    suite.POST("/users").
        JSON(validData).
        Expect(t).
        Status(201)

    // Validation error
    suite.POST("/users").
        JSON(invalidData).
        Expect(t).
        Status(422)

    // Conflict error
    suite.POST("/users").
        JSON(duplicateEmail).
        Expect(t).
        Status(409)
}
```

### 5. Verify Edge Cases

```go
func TestPaginationEdgeCases(t *testing.T) {
    suite := controllertest.New(t, userModule)

    // Test limit=0
    suite.GET("/users?limit=0").
        Expect(t).
        Status(400)

    // Test offset larger than results
    suite.GET("/users?offset=1000&limit=10").
        Expect(t).
        Status(200).
        Body() // Should return empty list

    // Test negative values
    suite.GET("/users?limit=-1").
        Expect(t).
        Status(400)
}
```

## Debugging Failed Tests

### Print Response

```go
response := suite.GET("/users").Expect(t)
fmt.Println("Status:", response.Raw().StatusCode)
fmt.Println("Body:", response.Body())
fmt.Println("Headers:", response.Raw().Header)
```

### Check HTML Structure

```go
html := suite.GET("/form").
    Expect(t).
    HTML()

// Debug: print all form inputs
inputs := html.Elements("//input")
for _, input := range inputs {
    // Inspect input...
}
```

### Verify Request Was Made

```go
// If test fails, check if controller was called at all
// by examining logs or adding debug prints
logger.Info("Controller called") // Added for debugging
```

---

For more information, see the [Advanced Features Overview](./index.md) or the [Controller Test Suite documentation](../controller-test-suite.md).
