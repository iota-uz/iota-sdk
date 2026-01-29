# Upload Source Configuration

The IOTA SDK provides a configurable middleware system for upload source access control. This allows child projects to implement custom logic for determining upload sources and controlling access to uploads by source.

## Overview

The upload source system consists of two main interfaces that child projects can implement:

1. **`UploadSourceProvider`**: Determines the source for uploads based on request context
2. **`UploadSourceAccessChecker`**: Controls access to uploads by source

## Default Behavior

By default, if no custom configuration is provided:
- All uploads use the "general" source
- All users can access and upload to any source

## Implementation Guide

### Step 1: Implement Custom Source Provider

Create a custom provider that determines the source based on your business logic:

```go
package myproject

import (
    "net/http"
    "strings"

    "github.com/iota-uz/iota-sdk/pkg/middleware"
)

type MySourceProvider struct{}

func (p *MySourceProvider) GetUploadSource(r *http.Request) string {
    // Custom logic based on URL path, user role, etc.
    if strings.HasPrefix(r.URL.Path, "/reports") {
        return "reports"
    }
    if strings.HasPrefix(r.URL.Path, "/marketing") {
        return "website"
    }
    return "general"
}
```

### Step 2: Implement Access Checker

Create a custom access checker to control who can access and upload to different sources:

```go
package myproject

import (
    "errors"
    "net/http"

    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type MyAccessChecker struct{}

func (c *MyAccessChecker) CanAccessSource(r *http.Request, source string) error {
    user, err := composables.UseUser(r.Context())
    if err != nil {
        return err
    }

    // Custom access logic
    if source == "reports" && !user.HasRole("accountant") {
        return errors.New("access denied to reports uploads")
    }

    if source == "confidential" && !user.HasRole("manager") {
        return errors.New("access denied to confidential uploads")
    }

    return nil
}

func (c *MyAccessChecker) CanUploadToSource(r *http.Request, source string) error {
    user, err := composables.UseUser(r.Context())
    if err != nil {
        return err
    }

    // Same or different logic for uploading
    if source == "reports" && !user.HasPermission("reports.upload") {
        return errors.New("no permission to upload reports")
    }

    // Can reuse the access check logic
    return c.CanAccessSource(r, source)
}
```

### Step 3: Configure Middleware in Router

Apply the middleware to your router with your custom implementations:

```go
package main

import (
    "github.com/gorilla/mux"
    "github.com/iota-uz/iota-sdk/pkg/middleware"
)

func setupRouter() *mux.Router {
    router := mux.NewRouter()

    // Apply upload source middleware with custom configuration
    router.Use(middleware.WithUploadSource(&middleware.UploadSourceConfig{
        Provider:      &MySourceProvider{},
        AccessChecker: &MyAccessChecker{},
    }))

    // ... rest of your routes

    return router
}
```

## Usage in Application Code

### Getting Current Upload Source

```go
import "github.com/iota-uz/iota-sdk/pkg/composables"

func myHandler(w http.ResponseWriter, r *http.Request) {
    source := composables.UseUploadSource(r.Context())
    // source will be determined by your custom provider
}
```

### Checking Access Programmatically

```go
import "github.com/iota-uz/iota-sdk/pkg/composables"

func myHandler(w http.ResponseWriter, r *http.Request) {
    // Check if user can access uploads with source "reports"
    if err := composables.CheckUploadSourceAccess(r.Context(), "reports", r); err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }

    // Check if user can upload to source "confidential"
    if err := composables.CheckUploadToSource(r.Context(), "confidential", r); err != nil {
        http.Error(w, err.Error(), http.StatusForbidden)
        return
    }
}
```

### Setting Source Manually in Context

In rare cases where you need to override the source in specific handlers:

```go
import "github.com/iota-uz/iota-sdk/pkg/composables"

func specialHandler(w http.ResponseWriter, r *http.Request) {
    // Override source for this specific request
    ctx := composables.WithUploadSource(r.Context(), "special-uploads")
    r = r.WithContext(ctx)

    // Continue with handler logic...
}
```

## GraphQL Integration

The upload source middleware works seamlessly with GraphQL:

```graphql
mutation UploadFile($file: File!) {
  uploadFile(file: $file, opts: { geoPoint: { lat: 40.7, lng: -74.0 } }) {
    id
    url
    source  # Will contain the source determined by your provider
  }
}

query GetUploads($source: String) {
  uploads(filter: { source: $source }) {
    id
    url
    source
  }
}
```

The source is automatically:
1. Determined by your `UploadSourceProvider` when uploading
2. Checked by your `UploadSourceAccessChecker` when querying
3. Stored in the database with the upload record

## Advanced Examples

### Path-Based Source Assignment

```go
type PathBasedSourceProvider struct{}

func (p *PathBasedSourceProvider) GetUploadSource(r *http.Request) string {
    path := r.URL.Path

    switch {
    case strings.HasPrefix(path, "/api/products"):
        return "products"
    case strings.HasPrefix(path, "/api/marketing"):
        return "marketing"
    case strings.HasPrefix(path, "/api/hr/documents"):
        return "hr-confidential"
    default:
        return "general"
    }
}
```

### Role-Based Access Control

```go
type RoleBasedAccessChecker struct{}

func (c *RoleBasedAccessChecker) CanAccessSource(r *http.Request, source string) error {
    user, err := composables.UseUser(r.Context())
    if err != nil {
        return err
    }

    // Define role-to-source mapping
    sourceRoles := map[string][]string{
        "hr-confidential": {"hr_manager", "admin"},
        "financial":       {"accountant", "cfo", "admin"},
        "marketing":       {"marketing", "admin"},
    }

    requiredRoles, exists := sourceRoles[source]
    if !exists {
        return nil // No restrictions for unknown sources
    }

    for _, role := range requiredRoles {
        if user.HasRole(role) {
            return nil
        }
    }

    return errors.New("insufficient permissions for this upload source")
}

func (c *RoleBasedAccessChecker) CanUploadToSource(r *http.Request, source string) error {
    // Use same logic for upload permission
    return c.CanAccessSource(r, source)
}
```

### Multi-Tenant Source Isolation

```go
type TenantIsolatedSourceProvider struct{}

func (p *TenantIsolatedSourceProvider) GetUploadSource(r *http.Request) string {
    tenantID, err := composables.UseTenantID(r.Context())
    if err != nil {
        return "general"
    }

    // Prefix source with tenant ID for isolation
    return fmt.Sprintf("tenant_%s_uploads", tenantID.String())
}

type TenantIsolatedAccessChecker struct{}

func (c *TenantIsolatedAccessChecker) CanAccessSource(r *http.Request, source string) error {
    tenantID, err := composables.UseTenantID(r.Context())
    if err != nil {
        return errors.New("no tenant context")
    }

    expectedPrefix := fmt.Sprintf("tenant_%s_", tenantID.String())
    if !strings.HasPrefix(source, expectedPrefix) {
        return errors.New("cannot access uploads from other tenants")
    }

    return nil
}

func (c *TenantIsolatedAccessChecker) CanUploadToSource(r *http.Request, source string) error {
    return c.CanAccessSource(r, source)
}
```

## Best Practices

1. **Keep source names consistent**: Use a predefined set of source names rather than generating them dynamically
2. **Document your sources**: Maintain documentation of what each source represents in your business domain
3. **Test access controls**: Write tests to verify your access checker logic works correctly
4. **Use descriptive errors**: Return clear error messages when access is denied
5. **Consider caching**: If your provider logic is expensive, consider caching the result
6. **Audit uploads**: Log upload attempts and access checks for security auditing

## Testing

Test your custom implementations:

```go
func TestMySourceProvider(t *testing.T) {
    provider := &MySourceProvider{}

    tests := []struct {
        path     string
        expected string
    }{
        {"/reports/upload", "reports"},
        {"/marketing/images", "website"},
        {"/other", "general"},
    }

    for _, tt := range tests {
        req := httptest.NewRequest(http.MethodPost, tt.path, nil)
        result := provider.GetUploadSource(req)
        if result != tt.expected {
            t.Errorf("path %s: expected %s, got %s", tt.path, tt.expected, result)
        }
    }
}
```

## Migration Guide

If you're migrating from a codebase that passed source directly in forms or GraphQL:

1. **Forms**: Remove `_source` hidden fields from your upload forms
2. **GraphQL**: Remove `source` parameter from `uploadFile` and `uploadFileWithSlug` mutations
3. **Implement provider**: Create your `UploadSourceProvider` to determine source based on context
4. **Configure middleware**: Apply the middleware with your custom provider
5. **Test thoroughly**: Verify uploads get the correct source assigned

## Troubleshooting

**Problem**: Uploads always get "general" source
- **Solution**: Ensure middleware is applied before your upload routes

**Problem**: Access checks are bypassed
- **Solution**: Verify you're using `composables.CheckUploadSourceAccess()` or `composables.CheckUploadToSource()`

**Problem**: GraphQL uploads fail with 403
- **Solution**: Check that your access checker's logic allows the determined source
