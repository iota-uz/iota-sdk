# Routing & Module Registration Guide

**Module registration, routing patterns, and middleware for IOTA SDK.**

## Overview

IOTA SDK uses a modular architecture with:
- **Module pattern**: Self-contained feature modules
- **Router registration**: Controllers register their own routes
- **Middleware**: Auth, logging, context injection
- **DI (Dependency Injection)**: Via `di.H` for handler dependencies

## Module Structure

### Module Interface

```go
type Module interface {
    Name() string
    ConfigureServices(services di.ServiceCollection)
    ConfigureRoutes(router *mux.Router, app application.Application)
}
```

### Module Implementation

```go
package modulename

import (
    "github.com/gorilla/mux"
    "github.com/iota-uz/iota-sdk/pkg/application"
    "github.com/iota-uz/iota-sdk/pkg/composables/di"
)

type Module struct {
    basePath string
}

func NewModule(basePath string) *Module {
    return &Module{basePath: basePath}
}

func (m *Module) Name() string {
    return "modulename"
}

func (m *Module) ConfigureServices(services di.ServiceCollection) {
    // Register repositories
    services.AddScoped(
        reflect.TypeOf((*domain.EntityRepository)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            return persistence.NewEntityRepository(), nil
        },
    )

    // Register services
    services.AddScoped(
        reflect.TypeOf((*services.EntityService)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            repo := sp.GetService(reflect.TypeOf((*domain.EntityRepository)(nil)).Elem()).(domain.EntityRepository)
            return services.NewEntityService(repo), nil
        },
    )
}

func (m *Module) ConfigureRoutes(router *mux.Router, app application.Application) {
    // Register controllers
    controllers.NewEntityController(app, m.basePath+"/entities").Register(router)
    controllers.NewDashboardController(app, m.basePath).Register(router)
}
```

## Controller Registration

### Basic Controller

```go
type EntityController struct {
    app      application.Application
    basePath string
}

func NewEntityController(app application.Application, basePath string) *EntityController {
    return &EntityController{
        app:      app,
        basePath: basePath,
    }
}

func (c *EntityController) Register(r *mux.Router) {
    // Create subrouter for this controller
    s := r.PathPrefix(c.basePath).Subrouter()

    // Apply middleware
    s.Use(middleware.Authorize(), middleware.WithPageContext())

    // Register routes with DI
    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
    s.HandleFunc("/new", di.H(c.New)).Methods(http.MethodGet)
    s.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)
    s.HandleFunc("/{id}/edit", di.H(c.Edit)).Methods(http.MethodGet)
    s.HandleFunc("/{id}", di.H(c.Update)).Methods(http.MethodPut)
    s.HandleFunc("/{id}", di.H(c.Delete)).Methods(http.MethodDelete)
}
```

### RESTful Route Patterns

**Standard CRUD routes**:

| Method | Path           | Handler  | Purpose              |
|--------|----------------|----------|----------------------|
| GET    | `/entities`    | List     | List all entities    |
| GET    | `/entities/new`| New      | Show create form     |
| POST   | `/entities`    | Create   | Create new entity    |
| GET    | `/entities/{id}`| Show    | Show single entity   |
| GET    | `/entities/{id}/edit`| Edit| Show edit form       |
| PUT    | `/entities/{id}`| Update  | Update entity        |
| DELETE | `/entities/{id}`| Delete  | Delete entity        |

## Middleware

### Built-in Middleware

**Authentication**:

```go
import "github.com/iota-uz/iota-sdk/pkg/middleware"

// Require authentication
s.Use(middleware.Authorize())

// Allow anonymous access
s.Use(middleware.AllowAnonymous())
```

**Page Context**:

```go
// Inject page context (translations, user, org)
s.Use(middleware.WithPageContext())
```

**Logging**:

```go
// Request logging
s.Use(middleware.Logger())
```

**Organization Context**:

```go
// Inject organization ID
s.Use(middleware.WithOrganization())
```

### Applying Middleware

**Route-level**:

```go
func (c *Controller) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()

    // Apply to all routes in this subrouter
    s.Use(
        middleware.Authorize(),
        middleware.WithPageContext(),
        middleware.Logger(),
    )

    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
}
```

**Group-level** (for admin routes):

```go
func (c *Controller) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()
    s.Use(middleware.Authorize(), middleware.WithPageContext())

    // Public routes
    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)

    // Admin routes
    adminRouter := s.PathPrefix("/admin").Subrouter()
    adminRouter.Use(middleware.RequirePermission(permissions.AdminAccess))

    adminRouter.HandleFunc("", di.H(c.AdminPanel)).Methods(http.MethodGet)
    adminRouter.HandleFunc("/settings", di.H(c.Settings)).Methods(http.MethodGet)
}
```

### Custom Middleware

```go
func CustomMiddleware() mux.MiddlewareFunc {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Pre-processing
            logger := composables.UseLogger(r.Context())
            logger.Info("Custom middleware")

            // Call next handler
            next.ServeHTTP(w, r)

            // Post-processing (if needed)
        })
    }
}

// Usage
s.Use(CustomMiddleware())
```

## Dependency Injection with di.H

### How di.H Works

**Magic DI via parameter types**:

```go
// Controller handler
func (c *Controller) List(
    r *http.Request,           // Always injected
    w http.ResponseWriter,     // Always injected
    u useraggregate.User,      // Injected by type
    service *services.EntityService, // Injected from DI container
    logger *logrus.Entry,      // Injected by type
) {
    // Use injected dependencies
}

// Registration
s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
```

**di.H automatically resolves**:
- `*http.Request` and `http.ResponseWriter` (from HTTP)
- `useraggregate.User` (from auth middleware)
- `*logrus.Entry` (from logger middleware)
- Services registered in `ConfigureServices`

### Service Registration

**In module's `ConfigureServices`**:

```go
func (m *Module) ConfigureServices(services di.ServiceCollection) {
    // Scoped: New instance per request
    services.AddScoped(
        reflect.TypeOf((*services.EntityService)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            repo := sp.GetService(reflect.TypeOf((*domain.EntityRepository)(nil)).Elem()).(domain.EntityRepository)
            return services.NewEntityService(repo), nil
        },
    )

    // Singleton: Single instance for application lifetime
    services.AddSingleton(
        reflect.TypeOf((*services.CacheService)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            return services.NewCacheService(), nil
        },
    )

    // Transient: New instance every time
    services.AddTransient(
        reflect.TypeOf((*services.NotificationService)(nil)).Elem(),
        func(sp di.ServiceProvider) (interface{}, error) {
            return services.NewNotificationService(), nil
        },
    )
}
```

## Permission-Based Routing

### Middleware Permissions

```go
import "github.com/iota-uz/iota-sdk/modules/core/domain/permissions"

func (c *Controller) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()
    s.Use(middleware.Authorize())

    // View permission required
    viewRouter := s.PathPrefix("").Subrouter()
    viewRouter.Use(middleware.RequirePermission(permissions.ViewEntity))
    viewRouter.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    viewRouter.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)

    // Create permission required
    createRouter := s.PathPrefix("").Subrouter()
    createRouter.Use(middleware.RequirePermission(permissions.CreateEntity))
    createRouter.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)

    // Update permission required
    updateRouter := s.PathPrefix("").Subrouter()
    updateRouter.Use(middleware.RequirePermission(permissions.UpdateEntity))
    updateRouter.HandleFunc("/{id}", di.H(c.Update)).Methods(http.MethodPut)

    // Delete permission required
    deleteRouter := s.PathPrefix("").Subrouter()
    deleteRouter.Use(middleware.RequirePermission(permissions.DeleteEntity))
    deleteRouter.HandleFunc("/{id}", di.H(c.Delete)).Methods(http.MethodDelete)
}
```

### Handler-Level Permissions

```go
func (c *Controller) Update(
    r *http.Request,
    w http.ResponseWriter,
    u useraggregate.User,
    service *services.EntityService,
) {
    // Fine-grained permission check
    if !u.HasPermission(permissions.UpdateEntity) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Additional business logic permission
    entityID := mux.Vars(r)["id"]
    entity, _ := service.FindByID(r.Context(), uuid.MustParse(entityID))

    if entity.OwnerID() != u.ID() && !u.HasRole(roles.Admin) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Proceed with update
}
```

## Route Parameters

### Path Parameters

```go
// Route definition
s.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)

// Handler extraction
func (c *Controller) Show(r *http.Request, w http.ResponseWriter) {
    // Get path parameter
    vars := mux.Vars(r)
    idStr := vars["id"]

    // Parse as UUID
    id, err := uuid.Parse(idStr)
    if err != nil {
        http.Error(w, "Invalid ID", http.StatusBadRequest)
        return
    }

    // Or use helper
    id := shared.ParseUUID(r)  // Returns uuid.UUID
}
```

### Query Parameters

```go
// Route definition
s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)

// Handler extraction
func (c *Controller) List(r *http.Request, w http.ResponseWriter) {
    // Manual parsing
    query := r.URL.Query()
    page := query.Get("page")
    limit := query.Get("limit")

    // Or use composables
    params, err := composables.UseQuery(&ListParams{}, r)
}

type ListParams struct {
    Page   int    `query:"page"`
    Limit  int    `query:"limit"`
    Search string `query:"search"`
}
```

## Superadmin Routes

**Critical**: Superadmin routes MUST use `RequireSuperAdmin()` middleware

```go
func (c *SuperadminController) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()

    // CRITICAL: Require superadmin for ALL routes
    s.Use(middleware.RequireSuperAdmin())
    s.Use(middleware.WithPageContext())

    s.HandleFunc("", di.H(c.Dashboard)).Methods(http.MethodGet)
    s.HandleFunc("/tenants", di.H(c.ListTenants)).Methods(http.MethodGet)
    s.HandleFunc("/analytics", di.H(c.Analytics)).Methods(http.MethodGet)
}
```

## API vs Page Routes

### API Routes (JSON)

```go
func (c *Controller) Register(r *mux.Router) {
    // API routes
    api := r.PathPrefix("/api/entities").Subrouter()
    api.Use(middleware.Authorize())

    api.HandleFunc("", di.H(c.ListJSON)).Methods(http.MethodGet)
    api.HandleFunc("", di.H(c.CreateJSON)).Methods(http.MethodPost)
    api.HandleFunc("/{id}", di.H(c.ShowJSON)).Methods(http.MethodGet)
}

func (c *Controller) ListJSON(
    r *http.Request,
    w http.ResponseWriter,
    service *services.EntityService,
) {
    entities, err := service.FindAll(r.Context(), ListParams{})
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(entities)
}
```

### Page Routes (HTML)

```go
func (c *Controller) Register(r *mux.Router) {
    // Page routes
    pages := r.PathPrefix("/entities").Subrouter()
    pages.Use(middleware.Authorize(), middleware.WithPageContext())

    pages.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    pages.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)
}

func (c *Controller) List(
    r *http.Request,
    w http.ResponseWriter,
    service *services.EntityService,
) {
    entities, _ := service.FindAll(r.Context(), ListParams{})

    pageCtx := composables.UsePageCtx(r.Context())
    vm := viewmodels.NewEntityListViewModel(entities)

    templates.EntityList(pageCtx, vm).Render(r.Context(), w)
}
```

## Best Practices

### Route Organization

- [ ] Group routes by resource (entities, orders, etc.)
- [ ] Use RESTful conventions
- [ ] Apply middleware at appropriate level
- [ ] Use `di.H` for all handlers
- [ ] Separate API and page routes

### Middleware Application

- [ ] `Authorize()` for protected routes
- [ ] `WithPageContext()` for page routes
- [ ] `RequirePermission()` for permission-based access
- [ ] `RequireSuperAdmin()` for superadmin routes
- [ ] Custom middleware for cross-cutting concerns

### DI Registration

- [ ] Register repositories as Scoped
- [ ] Register services as Scoped
- [ ] Use Singleton for stateless services
- [ ] Resolve dependencies through DI container
- [ ] Test DI resolution in module tests

## Common Patterns

### CRUD Controller

```go
func (c *EntityController) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()
    s.Use(middleware.Authorize(), middleware.WithPageContext())

    // List & Show (read)
    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("/{id}", di.H(c.Show)).Methods(http.MethodGet)

    // Create
    s.HandleFunc("/new", di.H(c.New)).Methods(http.MethodGet)
    s.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)

    // Update
    s.HandleFunc("/{id}/edit", di.H(c.Edit)).Methods(http.MethodGet)
    s.HandleFunc("/{id}", di.H(c.Update)).Methods(http.MethodPut)

    // Delete
    s.HandleFunc("/{id}", di.H(c.Delete)).Methods(http.MethodDelete)
}
```

### Nested Resources

```go
// Parent: /entities/{entityID}
// Nested: /entities/{entityID}/comments

func (c *CommentController) Register(r *mux.Router) {
    s := r.PathPrefix("/entities/{entityID}/comments").Subrouter()
    s.Use(middleware.Authorize())

    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
}

func (c *CommentController) List(r *http.Request, w http.ResponseWriter) {
    entityID := mux.Vars(r)["entityID"]
    // Use entityID to filter comments
}
```

## Testing

Controllers should be tested with ITF framework (see `testing.md`):

```go
func TestController_Routes(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        WithModules(module.NewModule("/test")).
        AsUser(permissions.ViewEntity).
        Build()

    c := controllers.NewEntityController(suite.Env().App, "/test/entities")
    suite.Register(c)

    // Test route registration
    suite.GET("/test/entities").Assert(t).ExpectOK()
    suite.GET("/test/entities/123").Assert(t).ExpectOK()
}
```
