# Presentation Layer Guide

**Controllers, ViewModels, templates (Templ), HTMX integration, and UI components for IOTA SDK.**

## Overview

The presentation layer handles HTTP requests, user interactions, and UI rendering:
- **Controllers**: HTTP handlers with DI via `di.H`
- **ViewModels**: Data transformation for presentation
- **Templates**: Templ files for HTML rendering
- **HTMX**: Dynamic interactions without JavaScript
- **Components**: IOTA SDK component library

## Controllers

### Structure

**Location**: `modules/{module}/presentation/controllers/*_controller.go`

```go
type EntityNameController struct {
    app      application.Application
    basePath string
}

func NewEntityNameController(app application.Application, basePath string) *EntityNameController {
    return &EntityNameController{
        app:      app,
        basePath: basePath,
    }
}
```

### Registration with DI

```go
func (c *EntityNameController) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()

    // Apply middleware
    s.Use(middleware.Authorize(), middleware.WithPageContext())

    // Register routes with DI
    s.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
    s.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
    s.HandleFunc("/{id}", di.H(c.Update)).Methods(http.MethodPut)
    s.HandleFunc("/{id}", di.H(c.Delete)).Methods(http.MethodDelete)
}
```

### Handler Pattern

**Dependencies injected by type signature**:

```go
func (c *EntityNameController) List(
    r *http.Request,
    w http.ResponseWriter,
    u useraggregate.User,                // Current user
    service *services.EntityNameService, // Service dependency
    logger *logrus.Entry,                // Logger
) {
    const op serrors.Op = "EntityNameController.List"

    // 1. Check permissions
    if !u.HasPermission(permissions.ViewEntity) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // 2. Parse query parameters
    params, err := composables.UseQuery(&ListParams{}, r)
    if err != nil {
        logger.WithError(err).Error("Failed to parse query")
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 3. Call service
    entities, total, err := service.FindAll(r.Context(), params)
    if err != nil {
        logger.WithError(err).Error("Failed to fetch entities")
        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }

    // 4. Build view model
    vm := viewmodels.NewEntityListViewModel(entities, total, params)

    // 5. Render template
    pageCtx := composables.UsePageCtx(r.Context())
    component := templates.EntityList(pageCtx, vm)

    // 6. Handle HTMX vs full page
    if htmx.IsHxRequest(r) {
        component.Render(r.Context(), w)
    } else {
        templates.Layout(pageCtx, component).Render(r.Context(), w)
    }
}
```

### Form Handling

```go
func (c *EntityNameController) Create(
    r *http.Request,
    w http.ResponseWriter,
    u useraggregate.User,
    service *services.EntityNameService,
    logger *logrus.Entry,
) {
    const op serrors.Op = "EntityNameController.Create"

    // 1. Check permissions
    if !u.HasPermission(permissions.CreateEntity) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // 2. Parse form data (CamelCase field names)
    formData, err := composables.UseForm(&CreateDTO{}, r)
    if err != nil {
        logger.WithError(err).Error("Failed to parse form")
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // 3. Call service
    entity, err := service.Create(r.Context(), formData)
    if err != nil {
        logger.WithError(err).Error("Failed to create entity")

        // Handle validation errors
        if serrors.Kind(err) == serrors.KindValidation {
            pageCtx := composables.UsePageCtx(r.Context())
            vm := viewmodels.NewEntityFormViewModel(formData, err)
            templates.EntityForm(pageCtx, vm).Render(r.Context(), w)
            return
        }

        http.Error(w, "Internal Server Error", http.StatusInternalServerError)
        return
    }

    // 4. Set success flash
    shared.SetFlash(w, "success", "Entity created successfully")

    // 5. Redirect (handles HTMX vs regular)
    if htmx.IsHxRequest(r) {
        htmx.Redirect(w, fmt.Sprintf("/entities/%s", entity.ID()))
    } else {
        shared.Redirect(w, r, fmt.Sprintf("/entities/%s", entity.ID()))
    }
}
```

### Auth Guards

**Middleware-level (route protection)**:

```go
func (c *Controller) Register(r *mux.Router) {
    s := r.PathPrefix(c.basePath).Subrouter()

    // Require authentication for all routes
    s.Use(middleware.Authorize())

    // Additional auth for admin routes
    adminRouter := s.PathPrefix("/admin").Subrouter()
    adminRouter.Use(middleware.RequirePermission(permissions.AdminAccess))

    adminRouter.HandleFunc("", di.H(c.AdminPanel)).Methods(http.MethodGet)
}
```

**Handler-level (fine-grained control)**:

```go
func (c *Controller) Handler(
    r *http.Request,
    w http.ResponseWriter,
    u useraggregate.User,
) {
    // Check specific permission
    if !u.HasPermission(permissions.ViewSensitive) {
        http.Error(w, "Forbidden", http.StatusForbidden)
        return
    }

    // Proceed with handler logic
}
```

## ViewModels

### Purpose

ViewModels transform domain entities into presentation-friendly structures:
- Located in `modules/{module}/presentation/viewmodels/`
- Pure transformation logic (no business logic)
- Map from domain to presentation
- Separate from component Props

### Pattern

```go
// Location: modules/{module}/presentation/viewmodels/entity_list_viewmodel.go

type EntityListViewModel struct {
    Entities   []*EntityViewModel
    Total      int
    Page       int
    Limit      int
    HasNext    bool
    HasPrev    bool
    Filters    FilterViewModel
}

type EntityViewModel struct {
    ID          string
    Name        string
    Status      string
    StatusBadge badge.Variant
    CreatedAt   string
    UpdatedAt   string
}

func NewEntityListViewModel(
    entities []domain.Entity,
    total int,
    params ListParams,
) *EntityListViewModel {
    vms := make([]*EntityViewModel, len(entities))
    for i, e := range entities {
        vms[i] = &EntityViewModel{
            ID:          e.ID().String(),
            Name:        e.Name(),
            Status:      e.Status(),
            StatusBadge: mapStatusToBadge(e.Status()),
            CreatedAt:   e.CreatedAt().Format("2006-01-02"),
            UpdatedAt:   e.UpdatedAt().Format("2006-01-02"),
        }
    }

    return &EntityListViewModel{
        Entities: vms,
        Total:    total,
        Page:     params.Page,
        Limit:    params.Limit,
        HasNext:  (params.Page * params.Limit) < total,
        HasPrev:  params.Page > 1,
    }
}

func mapStatusToBadge(status string) badge.Variant {
    switch status {
    case "active":
        return badge.VariantGreen
    case "pending":
        return badge.VariantYellow
    case "inactive":
        return badge.VariantGray
    default:
        return badge.VariantGray
    }
}
```

### ViewModel vs Props

**ViewModels** (`viewmodels/` directory):
- Business logic and data transformation
- Map from domain to presentation
- Reusable across multiple components

**Props** (defined near components):
- Component-specific configuration
- Layout and styling options
- Not reusable business logic

## Templates (Templ)

### Basic Template

```go
// Location: modules/{module}/presentation/templates/pages/entities/index.templ

package entities

import (
    "github.com/iota-uz/iota-sdk/components/base/button"
    "github.com/iota-uz/iota-sdk/components/base/badge"
    "github.com/iota-uz/iota-sdk/pkg/types"
)

templ EntityList(pageCtx *types.PageContext, vm *viewmodels.EntityListViewModel) {
    <div class="container">
        <div class="header">
            <h1>{ pageCtx.T("Entities") }</h1>

            @button.Primary(button.Props{
                Attrs: templ.Attributes{
                    "hx-get":    "/entities/new",
                    "hx-target": "#content",
                },
            }) {
                { pageCtx.T("CreateNew") }
            }
        </div>

        <div class="list" id="entity-list">
            for _, entity := range vm.Entities {
                @EntityRow(pageCtx, entity)
            }
        </div>

        if vm.Total > vm.Limit {
            @pagination.Pagination(&pagination.State{
                Page:    vm.Page,
                PerPage: vm.Limit,
                Total:   vm.Total,
            })
        }
    </div>
}

templ EntityRow(pageCtx *types.PageContext, entity *viewmodels.EntityViewModel) {
    <div class="entity-row">
        <div class="entity-name">{ entity.Name }</div>
        <div class="entity-status">
            @badge.New(badge.Props{
                Variant: entity.StatusBadge,
            }) {
                { pageCtx.T("Status." + entity.Status) }
            }
        </div>
        <div class="entity-actions">
            @button.Secondary(button.Props{
                Size: button.SizeSM,
                Attrs: templ.Attributes{
                    "hx-get":    templ.URL("/entities/" + entity.ID),
                    "hx-target": "#content",
                },
            }) {
                { pageCtx.T("View") }
            }
        </div>
    </div>
}
```

### Form Template

```go
templ EntityForm(pageCtx *types.PageContext, vm *viewmodels.EntityFormViewModel) {
    <form
        hx-post="/entities"
        hx-target="#content"
        hx-swap="innerHTML"
    >
        <input type="hidden" name="gorilla.csrf.Token" value={ ctx.Value("gorilla.csrf.Token").(string) }/>

        @input.Text(&input.Props{
            Label:       pageCtx.T("Name"),
            Placeholder: pageCtx.T("EnterName"),
            Error:       vm.Errors["Name"],
            Attrs: templ.Attributes{
                "name":     "Name",  // CamelCase!
                "value":    vm.Name,
                "required": true,
            },
        })

        @input.TextArea(&input.Props{
            Label:       pageCtx.T("Description"),
            Placeholder: pageCtx.T("EnterDescription"),
            Error:       vm.Errors["Description"],
            Attrs: templ.Attributes{
                "name":  "Description",  // CamelCase!
                "value": vm.Description,
                "rows":  "5",
            },
        })

        <div class="form-actions">
            @button.Primary(button.Props{
                Attrs: templ.Attributes{
                    "type": "submit",
                },
            }) {
                { pageCtx.T("Save") }
            }

            @button.Secondary(button.Props{
                Attrs: templ.Attributes{
                    "hx-get":    "/entities",
                    "hx-target": "#content",
                },
            }) {
                { pageCtx.T("Cancel") }
            }
        </div>
    </form>
}
```

### Security Patterns

**URL Sanitization**:

```go
// Always use templ.URL for dynamic URLs
<a href={ templ.URL("/entities/" + id) }>View</a>

// For HTMX attributes
<div hx-get={ templ.URL(fmt.Sprintf("/api/entities/%s", id)) }>
```

**CSRF Protection**:

```go
<form method="post">
    <input type="hidden" name="gorilla.csrf.Token" value={ ctx.Value("gorilla.csrf.Token").(string) }/>
    // Form fields
</form>
```

**NO Raw HTML** (unless absolutely trusted):

```go
// NEVER do this with user input
@templ.Raw(userInput)  // DANGEROUS!

// Safe rendering (auto-escaped)
{ userInput }
```

## HTMX Integration

### Using pkg/htmx Package

**ALWAYS use pkg/htmx functions, NEVER raw headers**:

**Response Headers**:

```go
import "github.com/iota-uz/iota-sdk/pkg/htmx"

// Redirect
htmx.Redirect(w, "/entities")

// Trigger event
htmx.SetTrigger(w, "entityCreated", `{"id": "123"}`)

// Refresh page
htmx.Refresh(w)

// Retarget response
htmx.Retarget(w, "#other-div")
```

**Request Headers**:

```go
// Check if HTMX request
if htmx.IsHxRequest(r) {
    // Return partial HTML
} else {
    // Return full page
}

// Get trigger
trigger := htmx.TriggerName(r)

// Get current URL
currentURL := htmx.CurrentUrl(r)
```

### Common HTMX Patterns

**Infinite Scroll**:

```templ
<div hx-get="/entities?page=2" hx-trigger="revealed" hx-swap="afterend">
    Load More...
</div>
```

**Search with Debounce**:

```templ
@input.Text(&input.Props{
    Attrs: templ.Attributes{
        "hx-get":      "/entities/search",
        "hx-trigger":  "keyup changed delay:500ms",
        "hx-target":   "#results",
        "name":        "Query",
    },
})
```

**Confirm Before Delete**:

```templ
@button.Danger(button.Props{
    Attrs: templ.Attributes{
        "hx-delete":  templ.URL("/entities/" + id),
        "hx-confirm": pageCtx.T("ConfirmDelete"),
    },
}) {
    { pageCtx.T("Delete") }
}
```

## IOTA SDK Components

### Button Components

```go
import "github.com/iota-uz/iota-sdk/components/base/button"

@button.Primary(button.Props{
    Size: button.SizeNormal,
    Icon: icons.Plus(icons.Props{Size: "16"}),
    Attrs: templ.Attributes{
        "hx-post": "/create",
    },
}) {
    { pageCtx.T("Create") }
}

@button.Secondary(button.Props{})
@button.Danger(button.Props{})
@button.PrimaryOutline(button.Props{})
```

### Input Components

```go
import "github.com/iota-uz/iota-sdk/components/base/input"

@input.Text(&input.Props{
    Label:       "Name",
    Placeholder: "Enter name",
    Error:       errors["Name"],
    Attrs: templ.Attributes{
        "name": "Name",
    },
})

@input.Email(&input.Props{})
@input.Password(&input.Props{})
@input.Date(&input.Props{})
@input.TextArea(&input.Props{})
```

### Badge Component

```go
import "github.com/iota-uz/iota-sdk/components/base/badge"

@badge.New(badge.Props{
    Variant: badge.VariantGreen,  // Pink, Yellow, Green, Blue, Purple, Gray
    Size:    badge.SizeNormal,
}) {
    Active
}
```

### Dialog Components

```go
import "github.com/iota-uz/iota-sdk/components/base/dialog"

@dialog.Confirmation(dialog.Props{
    ID:     "delete-confirm",
    Title:  pageCtx.T("ConfirmDelete"),
    Text:   pageCtx.T("DeleteWarning"),
    Action: "/entities/" + id,
})
```

## Composables & Helpers

### Page Context

```go
// Get page context (includes translations)
pageCtx := composables.UsePageCtx(r.Context())

// Use in templates
pageCtx.T("KeyName")                      // Translation
pageCtx.User                              // Current user
pageCtx.OrganizationID                    // Organization ID
```

### Form Parsing

```go
// Parse form data with CamelCase fields
type CreateDTO struct {
    Name        string `form:"Name"`
    Description string `form:"Description"`
}

formData, err := composables.UseForm(&CreateDTO{}, r)
```

### Flash Messages

```go
// Set flash
shared.SetFlash(w, "success", "Entity created")

// Get flash
flash, err := composables.UseFlash(w, r, "success")
```

## Best Practices

### Controllers

- [ ] Apply auth middleware via `s.Use()`
- [ ] Check permissions in handlers
- [ ] Use DI for service dependencies
- [ ] Parse form/query with `composables.UseForm/UseQuery`
- [ ] Use `pkg/htmx` package (never raw headers)
- [ ] Wrap errors with `serrors.E(op, err)`
- [ ] Handle HTMX vs full page rendering

### ViewModels

- [ ] Located in `viewmodels/` directory
- [ ] Pure transformation logic
- [ ] Separate from Props
- [ ] Map domain entities to presentation

### Templates

- [ ] Use `templ.URL()` for dynamic URLs
- [ ] Include CSRF tokens in forms
- [ ] Use CamelCase for form field names
- [ ] Never use `@templ.Raw()` with user input
- [ ] Use IOTA SDK components
- [ ] Handle HTMX partial vs full rendering

### HTMX

- [ ] Always use `pkg/htmx` package
- [ ] Check `htmx.IsHxRequest(r)` for partial rendering
- [ ] Use debouncing for search inputs
- [ ] Add loading indicators
- [ ] Use proper HTMX swap strategies

## Testing

Controllers should be tested with ITF framework (see `testing.md`):

```go
func TestControllerName_Handler(t *testing.T) {
    suite := itf.NewSuiteBuilder(t).
        WithModules(module.NewModule(opts)).
        AsUser(permissions.ViewPermission).
        Build()

    c := controllers.NewControllerName(suite.Env().App)
    suite.Register(c)

    suite.GET("/path").Assert(t).ExpectOK()
}
```

## Common Pitfalls

### Don't

- Use raw HTMX headers (use `pkg/htmx`)
- Mix ViewModels with Props
- Put business logic in controllers
- Forget CSRF tokens in forms
- Skip auth checks
- Use snake_case for form fields (use CamelCase)
- Use `@templ.Raw()` with user input

### Do

- Use `pkg/htmx` exclusively
- Separate ViewModels and Props
- Keep controllers thin (delegate to services)
- Include CSRF protection
- Check permissions early
- Use CamelCase for form fields
- Sanitize URLs with `templ.URL()`
