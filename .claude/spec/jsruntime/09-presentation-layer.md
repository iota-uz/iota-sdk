# JavaScript Runtime - Presentation Layer Specification

## Overview

This specification defines the HTTP controllers, templates, ViewModels, DTOs, and UI components for the JavaScript Runtime module's presentation layer.

## File Structure

```
modules/scripts/
├── presentation/
│   ├── controllers/
│   │   └── script_controller.go
│   ├── viewmodels/
│   │   ├── script_viewmodel.go
│   │   ├── execution_viewmodel.go
│   │   └── version_viewmodel.go
│   ├── templates/
│   │   └── pages/
│   │       └── scripts/
│   │           ├── index.templ
│   │           ├── new.templ
│   │           ├── edit.templ
│   │           ├── view.templ
│   │           ├── _table.templ
│   │           └── _execution_row.templ
│   └── locales/
│       ├── en.toml
│       ├── ru.toml
│       └── uz.toml
```

## ScriptController

### Interface

```go
package controllers

import (
    "net/http"

    "github.com/gorilla/mux"
    "github.com/iota-uz/iota-sdk/pkg/application"
    "github.com/iota-uz/iota-sdk/pkg/composables"
    "github.com/iota-uz/iota-sdk/pkg/htmx"
    "github.com/iota-uz/iota-sdk/pkg/middleware"
    "github.com/iota-uz/iota-sdk/pkg/permission"
    scriptservices "github.com/yourorg/yourapp/modules/scripts/services"
)

type ScriptController struct {
    app           application.Application
    scriptSvc     *scriptservices.ScriptService
    executionSvc  *scriptservices.ExecutionService
    basePath      string
}

func NewScriptController(
    app application.Application,
    scriptSvc *scriptservices.ScriptService,
    executionSvc *scriptservices.ExecutionService,
) *ScriptController {
    return &ScriptController{
        app:          app,
        scriptSvc:    scriptSvc,
        executionSvc: executionSvc,
        basePath:     "/scripts",
    }
}

func (c *ScriptController) Key() string {
    return "scripts"
}

func (c *ScriptController) Register(r *mux.Router) {
    // Apply authentication middleware
    subRouter := r.PathPrefix(c.basePath).Subrouter()
    subRouter.Use(middleware.Authorize(c.app.Auth()))

    // List and create
    subRouter.HandleFunc("", middleware.RequirePermission(
        permission.ScriptRead,
    )(c.List)).Methods(http.MethodGet)

    subRouter.HandleFunc("/new", middleware.RequirePermission(
        permission.ScriptCreate,
    )(c.New)).Methods(http.MethodGet)

    subRouter.HandleFunc("", middleware.RequirePermission(
        permission.ScriptCreate,
    )(c.Create)).Methods(http.MethodPost)

    // View, edit, update, delete
    subRouter.HandleFunc("/{id:[0-9]+}", middleware.RequirePermission(
        permission.ScriptRead,
    )(c.View)).Methods(http.MethodGet)

    subRouter.HandleFunc("/{id:[0-9]+}/edit", middleware.RequirePermission(
        permission.ScriptUpdate,
    )(c.Edit)).Methods(http.MethodGet)

    subRouter.HandleFunc("/{id:[0-9]+}", middleware.RequirePermission(
        permission.ScriptUpdate,
    )(c.Update)).Methods(http.MethodPut, http.MethodPost)

    subRouter.HandleFunc("/{id:[0-9]+}", middleware.RequirePermission(
        permission.ScriptDelete,
    )(c.Delete)).Methods(http.MethodDelete)

    // Execute
    subRouter.HandleFunc("/{id:[0-9]+}/execute", middleware.RequirePermission(
        permission.ScriptExecute,
    )(c.Execute)).Methods(http.MethodPost)

    // Execution history
    subRouter.HandleFunc("/{id:[0-9]+}/executions", middleware.RequirePermission(
        permission.ScriptRead,
    )(c.Executions)).Methods(http.MethodGet)
}
```

### Handler Methods

#### List Scripts

```go
func (c *ScriptController) List(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.List")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)
    params := composables.UsePaginated(r)

    // Get filter parameters
    search := r.URL.Query().Get("search")
    triggerType := r.URL.Query().Get("trigger_type")

    // Fetch scripts
    scripts, total, err := c.scriptSvc.FindAll(ctx, script.FindParams{
        Limit:       params.Limit,
        Offset:      params.Offset,
        Search:      search,
        TriggerType: triggerType,
    })
    if err != nil {
        http.Error(w, "Failed to load scripts", http.StatusInternalServerError)
        return
    }

    // Convert to ViewModels
    scriptVMs := make([]viewmodels.ScriptViewModel, len(scripts))
    for i, s := range scripts {
        scriptVMs[i] = viewmodels.NewScriptViewModel(s)
    }

    // Check if HTMX partial request
    if htmx.IsHxRequest(r) {
        templates.ScriptsTable(scriptVMs, params, total).Render(ctx, w)
        return
    }

    // Full page render
    templates.ScriptsIndex(pageCtx, scriptVMs, params, total).Render(ctx, w)
}
```

#### New Script Form

```go
func (c *ScriptController) New(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.New")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)

    templates.ScriptsNew(pageCtx, nil).Render(ctx, w)
}
```

#### Create Script

```go
type CreateScriptDTO struct {
    Name        string `form:"Name" validate:"required,min=3,max=100"`
    Description string `form:"Description" validate:"max=500"`
    Source      string `form:"Source" validate:"required"`
    TriggerType string `form:"TriggerType" validate:"required,oneof=manual scheduled webhook event"`
    Schedule    string `form:"Schedule" validate:"omitempty,cron"`
    WebhookPath string `form:"WebhookPath" validate:"omitempty,uri"`
    EventType   string `form:"EventType" validate:"omitempty"`
    Enabled     bool   `form:"Enabled"`
}

func (c *ScriptController) Create(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Create")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)

    // Parse form
    dto, err := composables.UseForm[CreateScriptDTO](r)
    if err != nil {
        templates.ScriptsNew(pageCtx, err).Render(ctx, w)
        return
    }

    // Create script
    newScript, err := c.scriptSvc.Create(ctx, script.CreateParams{
        Name:        dto.Name,
        Description: dto.Description,
        Source:      dto.Source,
        TriggerType: dto.TriggerType,
        Schedule:    dto.Schedule,
        WebhookPath: dto.WebhookPath,
        EventType:   dto.EventType,
        Enabled:     dto.Enabled,
    })
    if err != nil {
        templates.ScriptsNew(pageCtx, err).Render(ctx, w)
        return
    }

    // Redirect to view
    htmx.Redirect(w, fmt.Sprintf("/scripts/%d", newScript.GetID()))
}
```

#### View Script

```go
func (c *ScriptController) View(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.View")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])

    // Fetch script
    script, err := c.scriptSvc.FindByID(ctx, uint(id))
    if err != nil {
        http.Error(w, "Script not found", http.StatusNotFound)
        return
    }

    // Fetch recent executions
    executions, _, err := c.executionSvc.FindByScriptID(ctx, uint(id), execution.FindParams{
        Limit:  10,
        Offset: 0,
    })
    if err != nil {
        executions = []execution.Execution{}
    }

    // Convert to ViewModels
    scriptVM := viewmodels.NewScriptViewModel(script)
    executionVMs := make([]viewmodels.ExecutionViewModel, len(executions))
    for i, e := range executions {
        executionVMs[i] = viewmodels.NewExecutionViewModel(e)
    }

    templates.ScriptsView(pageCtx, scriptVM, executionVMs).Render(ctx, w)
}
```

#### Edit Script Form

```go
func (c *ScriptController) Edit(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Edit")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])

    // Fetch script
    script, err := c.scriptSvc.FindByID(ctx, uint(id))
    if err != nil {
        http.Error(w, "Script not found", http.StatusNotFound)
        return
    }

    scriptVM := viewmodels.NewScriptViewModel(script)
    templates.ScriptsEdit(pageCtx, scriptVM, nil).Render(ctx, w)
}
```

#### Update Script

```go
type UpdateScriptDTO struct {
    Name        string `form:"Name" validate:"required,min=3,max=100"`
    Description string `form:"Description" validate:"max=500"`
    Source      string `form:"Source" validate:"required"`
    TriggerType string `form:"TriggerType" validate:"required,oneof=manual scheduled webhook event"`
    Schedule    string `form:"Schedule" validate:"omitempty,cron"`
    WebhookPath string `form:"WebhookPath" validate:"omitempty,uri"`
    EventType   string `form:"EventType" validate:"omitempty"`
    Enabled     bool   `form:"Enabled"`
}

func (c *ScriptController) Update(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Update")

    ctx := r.Context()
    pageCtx := composables.UsePageCtx(ctx)
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])

    // Parse form
    dto, err := composables.UseForm[UpdateScriptDTO](r)
    if err != nil {
        // Re-fetch script for error display
        script, _ := c.scriptSvc.FindByID(ctx, uint(id))
        scriptVM := viewmodels.NewScriptViewModel(script)
        templates.ScriptsEdit(pageCtx, scriptVM, err).Render(ctx, w)
        return
    }

    // Update script
    updatedScript, err := c.scriptSvc.Update(ctx, uint(id), script.UpdateParams{
        Name:        dto.Name,
        Description: dto.Description,
        Source:      dto.Source,
        TriggerType: dto.TriggerType,
        Schedule:    dto.Schedule,
        WebhookPath: dto.WebhookPath,
        EventType:   dto.EventType,
        Enabled:     dto.Enabled,
    })
    if err != nil {
        script, _ := c.scriptSvc.FindByID(ctx, uint(id))
        scriptVM := viewmodels.NewScriptViewModel(script)
        templates.ScriptsEdit(pageCtx, scriptVM, err).Render(ctx, w)
        return
    }

    // Redirect to view
    htmx.Redirect(w, fmt.Sprintf("/scripts/%d", updatedScript.GetID()))
}
```

#### Delete Script

```go
func (c *ScriptController) Delete(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Delete")

    ctx := r.Context()
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])

    // Delete script
    if err := c.scriptSvc.Delete(ctx, uint(id)); err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        w.Write([]byte("Failed to delete script"))
        return
    }

    // HTMX redirect
    htmx.Redirect(w, "/scripts")
}
```

#### Execute Script

```go
func (c *ScriptController) Execute(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Execute")

    ctx := r.Context()
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])

    // Parse input data (optional JSON payload)
    var inputData map[string]interface{}
    if r.Body != nil {
        json.NewDecoder(r.Body).Decode(&inputData)
    }

    // Execute script asynchronously
    execution, err := c.executionSvc.Execute(ctx, uint(id), inputData)
    if err != nil {
        w.WriteHeader(http.StatusInternalServerError)
        json.NewEncoder(w).Encode(map[string]string{
            "error": err.Error(),
        })
        return
    }

    // Return execution ID
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "execution_id": execution.GetID(),
        "status":       execution.GetStatus(),
    })
}
```

#### Execution History

```go
func (c *ScriptController) Executions(w http.ResponseWriter, r *http.Request) {
    const op = serrors.Op("controllers.ScriptController.Executions")

    ctx := r.Context()
    vars := mux.Vars(r)
    id, _ := strconv.Atoi(vars["id"])
    params := composables.UsePaginated(r)

    // Fetch executions
    executions, total, err := c.executionSvc.FindByScriptID(ctx, uint(id), execution.FindParams{
        Limit:  params.Limit,
        Offset: params.Offset,
    })
    if err != nil {
        http.Error(w, "Failed to load executions", http.StatusInternalServerError)
        return
    }

    // Convert to ViewModels
    executionVMs := make([]viewmodels.ExecutionViewModel, len(executions))
    for i, e := range executions {
        executionVMs[i] = viewmodels.NewExecutionViewModel(e)
    }

    // HTMX partial
    if htmx.IsHxRequest(r) {
        templates.ExecutionRows(executionVMs).Render(ctx, w)
        return
    }

    // Full response
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "executions": executionVMs,
        "total":      total,
    })
}
```

## ViewModels

### ScriptViewModel

```go
package viewmodels

import (
    "time"

    "github.com/yourorg/yourapp/modules/scripts/domain/script"
)

type ScriptViewModel struct {
    ID              uint
    Name            string
    Description     string
    Source          string
    TriggerType     string
    Schedule        string
    WebhookPath     string
    EventType       string
    Enabled         bool
    Version         int
    LastExecutedAt  *time.Time
    ExecutionCount  int
    SuccessCount    int
    FailureCount    int
    CreatedAt       time.Time
    UpdatedAt       time.Time
}

func NewScriptViewModel(s script.Script) ScriptViewModel {
    var lastExec *time.Time
    if s.GetLastExecutedAt() != nil {
        t := *s.GetLastExecutedAt()
        lastExec = &t
    }

    return ScriptViewModel{
        ID:             s.GetID(),
        Name:           s.GetName(),
        Description:    s.GetDescription(),
        Source:         s.GetSource(),
        TriggerType:    s.GetTriggerType(),
        Schedule:       s.GetSchedule(),
        WebhookPath:    s.GetWebhookPath(),
        EventType:      s.GetEventType(),
        Enabled:        s.IsEnabled(),
        Version:        s.GetVersion(),
        LastExecutedAt: lastExec,
        ExecutionCount: s.GetExecutionCount(),
        SuccessCount:   s.GetSuccessCount(),
        FailureCount:   s.GetFailureCount(),
        CreatedAt:      s.GetCreatedAt(),
        UpdatedAt:      s.GetUpdatedAt(),
    }
}

func (vm ScriptViewModel) SuccessRate() float64 {
    if vm.ExecutionCount == 0 {
        return 0
    }
    return float64(vm.SuccessCount) / float64(vm.ExecutionCount) * 100
}

func (vm ScriptViewModel) StatusBadgeClass() string {
    if !vm.Enabled {
        return "badge-gray"
    }
    if vm.SuccessRate() >= 80 {
        return "badge-success"
    }
    if vm.SuccessRate() >= 50 {
        return "badge-warning"
    }
    return "badge-danger"
}
```

### ExecutionViewModel

```go
package viewmodels

import (
    "time"

    "github.com/yourorg/yourapp/modules/scripts/domain/execution"
)

type ExecutionViewModel struct {
    ID          uint
    ScriptID    uint
    Status      string
    Input       string
    Output      string
    ErrorMsg    string
    StartedAt   time.Time
    CompletedAt *time.Time
    Duration    time.Duration
}

func NewExecutionViewModel(e execution.Execution) ExecutionViewModel {
    var completed *time.Time
    if e.GetCompletedAt() != nil {
        t := *e.GetCompletedAt()
        completed = &t
    }

    var duration time.Duration
    if completed != nil {
        duration = completed.Sub(e.GetStartedAt())
    }

    return ExecutionViewModel{
        ID:          e.GetID(),
        ScriptID:    e.GetScriptID(),
        Status:      e.GetStatus(),
        Input:       e.GetInput(),
        Output:      e.GetOutput(),
        ErrorMsg:    e.GetErrorMessage(),
        StartedAt:   e.GetStartedAt(),
        CompletedAt: completed,
        Duration:    duration,
    }
}

func (vm ExecutionViewModel) StatusBadgeClass() string {
    switch vm.Status {
    case "completed":
        return "badge-success"
    case "failed":
        return "badge-danger"
    case "running":
        return "badge-info"
    case "pending":
        return "badge-warning"
    default:
        return "badge-gray"
    }
}

func (vm ExecutionViewModel) DurationMs() int64 {
    return vm.Duration.Milliseconds()
}
```

### ScriptVersionViewModel

```go
package viewmodels

import (
    "time"

    "github.com/yourorg/yourapp/modules/scripts/domain/version"
)

type ScriptVersionViewModel struct {
    ID        uint
    ScriptID  uint
    Version   int
    Source    string
    CreatedBy uint
    CreatedAt time.Time
}

func NewScriptVersionViewModel(v version.ScriptVersion) ScriptVersionViewModel {
    return ScriptVersionViewModel{
        ID:        v.GetID(),
        ScriptID:  v.GetScriptID(),
        Version:   v.GetVersion(),
        Source:    v.GetSource(),
        CreatedBy: v.GetCreatedBy(),
        CreatedAt: v.GetCreatedAt(),
    }
}
```

## Templates

### Index Template (index.templ)

```templ
package scripts

import (
    "github.com/iota-uz/iota-sdk/components"
    "github.com/iota-uz/iota-sdk/pkg/types"
    "github.com/yourorg/yourapp/modules/scripts/presentation/viewmodels"
    "github.com/yourorg/yourapp/modules/core/presentation/templates/layouts"
)

templ ScriptsIndex(
    pageCtx types.PageContextProvider,
    scripts []viewmodels.ScriptViewModel,
    params types.PaginatedQuery,
    total int,
) {
    @layouts.Default(pageCtx) {
        <div class="container mx-auto px-4 py-8">
            <!-- Header -->
            <div class="flex justify-between items-center mb-6">
                <h1 class="text-3xl font-bold">{ pageCtx.T("Scripts.List.Title") }</h1>
                <a href="/scripts/new" class="btn btn-primary">
                    { pageCtx.T("Scripts.List.New") }
                </a>
            </div>

            <!-- Filters -->
            <div class="bg-white rounded-lg shadow p-4 mb-6">
                <form
                    hx-get="/scripts"
                    hx-target="#scripts-table"
                    hx-trigger="change, submit"
                    class="flex gap-4"
                >
                    <input
                        type="text"
                        name="search"
                        placeholder={ pageCtx.T("Scripts.List.Search") }
                        class="input input-bordered flex-1"
                    />
                    <select name="trigger_type" class="select select-bordered">
                        <option value="">{ pageCtx.T("Scripts.List.AllTriggers") }</option>
                        <option value="manual">{ pageCtx.T("Scripts.TriggerType.Manual") }</option>
                        <option value="scheduled">{ pageCtx.T("Scripts.TriggerType.Scheduled") }</option>
                        <option value="webhook">{ pageCtx.T("Scripts.TriggerType.Webhook") }</option>
                        <option value="event">{ pageCtx.T("Scripts.TriggerType.Event") }</option>
                    </select>
                    <button type="submit" class="btn btn-secondary">
                        { pageCtx.T("Scripts.List.Filter") }
                    </button>
                </form>
            </div>

            <!-- Table -->
            <div id="scripts-table">
                @ScriptsTable(scripts, params, total)
            </div>
        </div>
    }
}
```

### Table Template (_table.templ)

```templ
package scripts

import (
    "fmt"
    "github.com/iota-uz/iota-sdk/components"
    "github.com/yourorg/yourapp/modules/scripts/presentation/viewmodels"
)

templ ScriptsTable(
    scripts []viewmodels.ScriptViewModel,
    params types.PaginatedQuery,
    total int,
) {
    <div class="bg-white rounded-lg shadow overflow-hidden">
        <table class="table w-full">
            <thead>
                <tr>
                    <th>{ "Name" }</th>
                    <th>{ "Trigger" }</th>
                    <th>{ "Status" }</th>
                    <th>{ "Success Rate" }</th>
                    <th>{ "Last Run" }</th>
                    <th>{ "Actions" }</th>
                </tr>
            </thead>
            <tbody>
                for _, s := range scripts {
                    <tr>
                        <td>
                            <div class="font-medium">{ s.Name }</div>
                            <div class="text-sm text-gray-500">{ s.Description }</div>
                        </td>
                        <td>
                            <span class="badge">{ s.TriggerType }</span>
                        </td>
                        <td>
                            <span class={ "badge", s.StatusBadgeClass() }>
                                if s.Enabled {
                                    { "Enabled" }
                                } else {
                                    { "Disabled" }
                                }
                            </span>
                        </td>
                        <td>
                            <div class="flex items-center gap-2">
                                <div class="progress-bar w-20">
                                    <div
                                        class="progress-fill bg-success"
                                        style={ fmt.Sprintf("width: %.0f%%", s.SuccessRate()) }
                                    ></div>
                                </div>
                                <span class="text-sm">{ fmt.Sprintf("%.0f%%", s.SuccessRate()) }</span>
                            </div>
                        </td>
                        <td>
                            if s.LastExecutedAt != nil {
                                <time datetime={ s.LastExecutedAt.Format("2006-01-02T15:04:05Z") }>
                                    { s.LastExecutedAt.Format("Jan 02, 15:04") }
                                </time>
                            } else {
                                <span class="text-gray-400">{ "Never" }</span>
                            }
                        </td>
                        <td>
                            <div class="flex gap-2">
                                <a href={ templ.URL(fmt.Sprintf("/scripts/%d", s.ID)) } class="btn btn-sm btn-ghost">
                                    { "View" }
                                </a>
                                <a href={ templ.URL(fmt.Sprintf("/scripts/%d/edit", s.ID)) } class="btn btn-sm btn-ghost">
                                    { "Edit" }
                                </a>
                                <button
                                    hx-post={ fmt.Sprintf("/scripts/%d/execute", s.ID) }
                                    hx-trigger="click"
                                    class="btn btn-sm btn-primary"
                                >
                                    { "Run" }
                                </button>
                            </div>
                        </td>
                    </tr>
                }
            </tbody>
        </table>

        <!-- Pagination -->
        @components.Pagination(components.PaginationProps{
            Total: total,
            Limit: params.Limit,
            Page:  params.Page,
            URL:   "/scripts",
        })
    </div>
}
```

### New Template (new.templ)

```templ
package scripts

import (
    "github.com/iota-uz/iota-sdk/pkg/types"
    "github.com/yourorg/yourapp/modules/core/presentation/templates/layouts"
)

templ ScriptsNew(
    pageCtx types.PageContextProvider,
    err error,
) {
    @layouts.Default(pageCtx) {
        <div class="container mx-auto px-4 py-8 max-w-4xl">
            <h1 class="text-3xl font-bold mb-6">{ pageCtx.T("Scripts.New.Title") }</h1>

            <form
                method="POST"
                action="/scripts"
                class="bg-white rounded-lg shadow p-6 space-y-6"
            >
                <input type="hidden" name="gorilla.csrf.Token" value={ ctx.Value("gorilla.csrf.Token").(string) }/>

                if err != nil {
                    <div class="alert alert-error">{ err.Error() }</div>
                }

                <!-- Name -->
                <div class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.Name.Label") }</span>
                    </label>
                    <input
                        type="text"
                        name="Name"
                        required
                        class="input input-bordered"
                        placeholder={ pageCtx.T("Scripts.Single.Name.Placeholder") }
                    />
                </div>

                <!-- Description -->
                <div class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.Description.Label") }</span>
                    </label>
                    <textarea
                        name="Description"
                        rows="3"
                        class="textarea textarea-bordered"
                        placeholder={ pageCtx.T("Scripts.Single.Description.Placeholder") }
                    ></textarea>
                </div>

                <!-- Trigger Type -->
                <div class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.TriggerType.Label") }</span>
                    </label>
                    <select
                        name="TriggerType"
                        required
                        class="select select-bordered"
                        x-data="{ type: 'manual' }"
                        x-model="type"
                    >
                        <option value="manual">{ pageCtx.T("Scripts.TriggerType.Manual") }</option>
                        <option value="scheduled">{ pageCtx.T("Scripts.TriggerType.Scheduled") }</option>
                        <option value="webhook">{ pageCtx.T("Scripts.TriggerType.Webhook") }</option>
                        <option value="event">{ pageCtx.T("Scripts.TriggerType.Event") }</option>
                    </select>
                </div>

                <!-- Conditional Fields (Alpine.js) -->
                <div x-show="type === 'scheduled'" class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.Schedule.Label") }</span>
                    </label>
                    <input
                        type="text"
                        name="Schedule"
                        class="input input-bordered"
                        placeholder="0 0 * * *"
                    />
                    <label class="label">
                        <span class="label-text-alt">{ pageCtx.T("Scripts.Single.Schedule.Help") }</span>
                    </label>
                </div>

                <div x-show="type === 'webhook'" class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.WebhookPath.Label") }</span>
                    </label>
                    <input
                        type="text"
                        name="WebhookPath"
                        class="input input-bordered"
                        placeholder="/api/webhooks/my-script"
                    />
                </div>

                <div x-show="type === 'event'" class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.EventType.Label") }</span>
                    </label>
                    <input
                        type="text"
                        name="EventType"
                        class="input input-bordered"
                        placeholder="user.created"
                    />
                </div>

                <!-- Monaco Editor -->
                <div class="form-control">
                    <label class="label">
                        <span class="label-text">{ pageCtx.T("Scripts.Single.Source.Label") }</span>
                    </label>
                    <div id="monaco-editor" class="border rounded-lg" style="height: 400px;"></div>
                    <textarea
                        name="Source"
                        id="source-input"
                        required
                        class="hidden"
                    ></textarea>
                </div>

                <!-- Enabled Toggle -->
                <div class="form-control">
                    <label class="label cursor-pointer justify-start gap-4">
                        <input
                            type="checkbox"
                            name="Enabled"
                            class="toggle toggle-primary"
                        />
                        <span class="label-text">{ pageCtx.T("Scripts.Single.Enabled.Label") }</span>
                    </label>
                </div>

                <!-- Actions -->
                <div class="flex justify-end gap-4">
                    <a href="/scripts" class="btn btn-ghost">
                        { pageCtx.T("Common.Cancel") }
                    </a>
                    <button type="submit" class="btn btn-primary">
                        { pageCtx.T("Scripts.New.Submit") }
                    </button>
                </div>
            </form>
        </div>

        <!-- Monaco Editor Integration -->
        @MonacoEditorScript()
    }
}
```

### Monaco Editor Script Component

```templ
package scripts

script MonacoEditorScript() {
    // Load Monaco from CDN
    require.config({ paths: { vs: 'https://cdn.jsdelivr.net/npm/monaco-editor@0.45.0/min/vs' }});

    require(['vs/editor/editor.main'], function() {
        // TypeScript definitions for SDK
        const sdkTypings = `
declare namespace sdk {
    namespace http {
        function get(url: string, options?: RequestOptions): Promise<Response>;
        function post(url: string, body: any, options?: RequestOptions): Promise<Response>;
        function put(url: string, body: any, options?: RequestOptions): Promise<Response>;
        function delete(url: string, options?: RequestOptions): Promise<Response>;
    }

    namespace db {
        function query(sql: string, params?: any[]): Promise<QueryResult>;
        function execute(sql: string, params?: any[]): Promise<number>;
    }

    namespace cache {
        function get(key: string): Promise<any>;
        function set(key: string, value: any, ttlSeconds?: number): Promise<void>;
        function delete(key: string): Promise<void>;
    }

    namespace log {
        function info(message: string, ...args: any[]): void;
        function warn(message: string, ...args: any[]): void;
        function error(message: string, ...args: any[]): void;
    }
}

declare namespace events {
    function publish(eventType: string, payload: any): Promise<void>;
}

interface Context {
    tenantId: number;
    userId: number;
    orgId: number;
    input: any;
}

declare const ctx: Context;
`;

        // Register TypeScript definitions
        monaco.languages.typescript.javascriptDefaults.addExtraLib(
            sdkTypings,
            'sdk.d.ts'
        );

        // Create editor
        const editor = monaco.editor.create(document.getElementById('monaco-editor'), {
            value: '// Your script code here\n',
            language: 'javascript',
            theme: 'vs-dark',
            automaticLayout: true,
            minimap: { enabled: true },
            lineNumbers: 'on',
            scrollBeyondLastLine: false,
        });

        // Sync with form textarea
        const sourceInput = document.getElementById('source-input');
        editor.onDidChangeModelContent(() => {
            sourceInput.value = editor.getValue();
        });

        // Set initial value from textarea (for edit mode)
        if (sourceInput.value) {
            editor.setValue(sourceInput.value);
        }
    });
}
```

### Execution Row Template (_execution_row.templ)

```templ
package scripts

import (
    "fmt"
    "github.com/yourorg/yourapp/modules/scripts/presentation/viewmodels"
)

templ ExecutionRow(exec viewmodels.ExecutionViewModel) {
    <tr>
        <td>
            <span class={ "badge", exec.StatusBadgeClass() }>
                { exec.Status }
            </span>
        </td>
        <td>
            <time datetime={ exec.StartedAt.Format("2006-01-02T15:04:05Z") }>
                { exec.StartedAt.Format("Jan 02, 15:04:05") }
            </time>
        </td>
        <td>
            if exec.CompletedAt != nil {
                { fmt.Sprintf("%dms", exec.DurationMs()) }
            } else {
                <span class="text-gray-400">-</span>
            }
        </td>
        <td>
            <button
                class="btn btn-sm btn-ghost"
                onclick={ fmt.Sprintf("showExecutionDetails(%d)", exec.ID) }
            >
                { "Details" }
            </button>
        </td>
    </tr>
}

templ ExecutionRows(executions []viewmodels.ExecutionViewModel) {
    for _, exec := range executions {
        @ExecutionRow(exec)
    }
}
```

## Localization

### English (en.toml)

```toml
[Scripts]
[Scripts.List]
Title = "Scripts"
New = "New Script"
Search = "Search scripts..."
AllTriggers = "All Triggers"
Filter = "Filter"

[Scripts.New]
Title = "Create Script"
Submit = "Create Script"

[Scripts.Edit]
Title = "Edit Script"
Submit = "Update Script"

[Scripts.Single]
[Scripts.Single.Name]
Label = "Name"
Placeholder = "My automation script"

[Scripts.Single.Description]
Label = "Description"
Placeholder = "What does this script do?"

[Scripts.Single.Source]
Label = "Script Code"

[Scripts.Single.TriggerType]
Label = "Trigger Type"

[Scripts.Single.Schedule]
Label = "Cron Schedule"
Help = "Format: minute hour day month weekday (e.g., 0 0 * * * = daily at midnight)"

[Scripts.Single.WebhookPath]
Label = "Webhook Path"

[Scripts.Single.EventType]
Label = "Event Type"

[Scripts.Single.Enabled]
Label = "Enable script"

[Scripts.TriggerType]
Manual = "Manual"
Scheduled = "Scheduled"
Webhook = "Webhook"
Event = "Event"

[Scripts.Execution]
[Scripts.Execution.Status]
Pending = "Pending"
Running = "Running"
Completed = "Completed"
Failed = "Failed"
```

### Russian (ru.toml)

```toml
[Scripts]
[Scripts.List]
Title = "Скрипты"
New = "Новый скрипт"
Search = "Поиск скриптов..."
AllTriggers = "Все триггеры"
Filter = "Фильтр"

[Scripts.New]
Title = "Создать скрипт"
Submit = "Создать скрипт"

[Scripts.Edit]
Title = "Редактировать скрипт"
Submit = "Обновить скрипт"

[Scripts.Single]
[Scripts.Single.Name]
Label = "Название"
Placeholder = "Мой скрипт автоматизации"

[Scripts.Single.Description]
Label = "Описание"
Placeholder = "Что делает этот скрипт?"

[Scripts.Single.Source]
Label = "Код скрипта"

[Scripts.Single.TriggerType]
Label = "Тип триггера"

[Scripts.Single.Schedule]
Label = "Расписание Cron"
Help = "Формат: минута час день месяц день_недели (например, 0 0 * * * = ежедневно в полночь)"

[Scripts.Single.WebhookPath]
Label = "Путь вебхука"

[Scripts.Single.EventType]
Label = "Тип события"

[Scripts.Single.Enabled]
Label = "Включить скрипт"

[Scripts.TriggerType]
Manual = "Ручной"
Scheduled = "По расписанию"
Webhook = "Вебхук"
Event = "Событие"

[Scripts.Execution]
[Scripts.Execution.Status]
Pending = "Ожидание"
Running = "Выполняется"
Completed = "Завершено"
Failed = "Ошибка"
```

### Uzbek (uz.toml)

```toml
[Scripts]
[Scripts.List]
Title = "Skriptlar"
New = "Yangi skript"
Search = "Skriptlarni qidirish..."
AllTriggers = "Barcha triggerlar"
Filter = "Filter"

[Scripts.New]
Title = "Skript yaratish"
Submit = "Skript yaratish"

[Scripts.Edit]
Title = "Skriptni tahrirlash"
Submit = "Skriptni yangilash"

[Scripts.Single]
[Scripts.Single.Name]
Label = "Nomi"
Placeholder = "Mening avtomatlashtirish skriptim"

[Scripts.Single.Description]
Label = "Tavsif"
Placeholder = "Bu skript nima qiladi?"

[Scripts.Single.Source]
Label = "Skript kodi"

[Scripts.Single.TriggerType]
Label = "Trigger turi"

[Scripts.Single.Schedule]
Label = "Cron jadvali"
Help = "Format: daqiqa soat kun oy hafta_kuni (masalan, 0 0 * * * = har kuni yarim tunda)"

[Scripts.Single.WebhookPath]
Label = "Webhook yo'li"

[Scripts.Single.EventType]
Label = "Hodisa turi"

[Scripts.Single.Enabled]
Label = "Skriptni yoqish"

[Scripts.TriggerType]
Manual = "Qo'lda"
Scheduled = "Jadval bo'yicha"
Webhook = "Webhook"
Event = "Hodisa"

[Scripts.Execution]
[Scripts.Execution.Status]
Pending = "Kutilmoqda"
Running = "Bajarilmoqda"
Completed = "Tugallandi"
Failed = "Xatolik"
```

## RBAC Permissions

```go
package permissions

import "github.com/iota-uz/iota-sdk/pkg/permission"

const (
    ResourceScript permission.Resource = "Script"
    ActionExecute  permission.Action   = "Execute"
)

var (
    ScriptCreate  = permission.MustCreate("Script.Create", ResourceScript, permission.ActionCreate, permission.ModifierAll)
    ScriptRead    = permission.MustCreate("Script.Read", ResourceScript, permission.ActionRead, permission.ModifierAll)
    ScriptUpdate  = permission.MustCreate("Script.Update", ResourceScript, permission.ActionUpdate, permission.ModifierAll)
    ScriptDelete  = permission.MustCreate("Script.Delete", ResourceScript, permission.ActionDelete, permission.ModifierAll)
    ScriptExecute = permission.MustCreate("Script.Execute", ResourceScript, ActionExecute, permission.ModifierAll)
)
```

## Summary

This specification provides:

1. **Full CRUD controllers** with HTMX integration
2. **ViewModels** for clean data transformation
3. **Templ templates** with Monaco editor integration
4. **Multi-language support** (en, ru, uz)
5. **RBAC integration** for permission checks
6. **Form validation** with DTOs
7. **Execution history** with real-time updates
8. **Monaco TypeScript definitions** for SDK autocomplete

The presentation layer follows IOTA SDK patterns and integrates seamlessly with the domain/service layers defined in previous specifications.
