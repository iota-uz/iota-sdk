---
layout: default
title: Technical Architecture
parent: Projects
nav_order: 2
---

# Technical Architecture

## Module Structure

```
modules/projects/
├── domain/
│   ├── aggregates/
│   │   ├── project/
│   │   │   ├── project.go              # Aggregate interface (immutable)
│   │   │   ├── project_impl.go         # Private struct implementation
│   │   │   ├── project_repository.go   # Repository interface
│   │   │   └── project_events.go       # Domain events (Created, Updated, Deleted)
│   │   └── project_stage/
│   │       ├── project_stage.go
│   │       ├── project_stage_impl.go
│   │       ├── project_stage_repository.go
│   │       └── project_stage_events.go
│   ├── entities/
│   └── value_objects/
├── infrastructure/
│   ├── persistence/
│   │   ├── project_repository.go       # PostgreSQL implementation
│   │   ├── project_stage_repository.go
│   │   ├── projects_mappers.go         # Domain <-> Persistence DTO mapping
│   │   ├── models/
│   │   │   └── models.go               # Persistence models (DB structs)
│   │   └── queries/                    # Dynamic query builders (if needed)
│   └── providers/                      # External service integrations
├── services/
│   ├── project_service.go              # Business logic, transaction coordination
│   └── project_stage_service.go
├── presentation/
│   ├── controllers/
│   │   ├── project_controller.go       # HTTP handlers: GET /projects, POST, etc.
│   │   └── project_stage_controller.go
│   ├── viewmodels/
│   │   ├── project_viewmodel.go        # Transform domain → presentation data
│   │   └── project_stage_viewmodel.go
│   ├── templates/pages/
│   │   ├── projects/                   # Project listing, detail pages
│   │   └── project_stages/             # Stage management UI
│   ├── mappers/                        # DTO transforms
│   ├── locales/
│   │   ├── en.toml
│   │   ├── ru.toml
│   │   └── uz.toml
│   └── forms/                          # Form DTOs (if used)
├── permissions/
│   └── constants.go                    # Permission constants
├── links.go                            # Module route registration
├── module.go                           # Module initialization
└── permissions.sql                     # Permission seed data (if applicable)
```

## Domain Layer

### Project Aggregate

**Interface** (`project.go`):
```go
type Project interface {
    ID() uuid.UUID
    SetID(uuid.UUID)

    TenantID() uuid.UUID

    CounterpartyID() uuid.UUID
    UpdateCounterpartyID(uuid.UUID) Project

    Name() string
    UpdateName(string) Project

    Description() string
    UpdateDescription(string) Project

    CreatedAt() time.Time
    UpdatedAt() time.Time
}
```

**Key Principles**:
- **Interface-based**: Aggregate exposed as interface, not struct
- **Immutability**: Updates return new instance, don't mutate
- **Private struct**: `project` struct is unexported, accessible only via interface
- **Functional options**: Constructor uses functional options pattern for optional fields

### Project Stage Aggregate

Similar structure to Project, with stage-specific fields:
- `StageNumber`, `TotalAmount`, `StartDate`, `PlannedEndDate`, `FactualEndDate`

### Domain Events

Events published for all state changes:

```go
type ProjectCreatedEvent struct {
    Result Project
    // Metadata: timestamp, user, etc.
}

type ProjectUpdatedEvent struct {
    Result Project
}

type ProjectDeletedEvent struct {
    Result Project
}
```

Events enable:
- **Event sourcing**: Full audit trail
- **Downstream reactions**: Finance module updates, notifications
- **Eventual consistency**: Multiple services coordinating via events

## Service Layer

### ProjectService

**Responsibilities**:
- Query operations (GetByID, GetAll, GetPaginated, GetByCounterpartyID)
- Create operations (Save, Create)
- Update operations (Update)
- Delete operations (Delete)
- Event publishing

**Transaction Handling**:
- Transactional consistency via `composables.InTx()`
- Events published after successful persistence
- Rollback on any error within transaction

**Code Pattern**:
```go
type ProjectService struct {
    repo      project.Repository
    publisher eventbus.EventBus
}

func (s *ProjectService) Save(ctx context.Context, proj project.Project) (project.Project, error) {
    isNew := proj.ID() == uuid.Nil

    savedProj, err := s.repo.Save(ctx, proj)
    if err != nil {
        return nil, err
    }

    if isNew {
        event, err := project.NewCreatedEvent(ctx, savedProj)
        if err != nil {
            return nil, err
        }
        s.publisher.Publish(event)
    } else {
        event, err := project.NewUpdatedEvent(ctx, savedProj)
        if err != nil {
            return nil, err
        }
        s.publisher.Publish(event)
    }

    return savedProj, nil
}
```

## Repository Layer

### Repository Interface (Domain)

Located in `domain/aggregates/project/project_repository.go`:

```go
type Repository interface {
    GetByID(ctx context.Context, id uuid.UUID) (Project, error)
    GetAll(ctx context.Context) ([]Project, error)
    GetPaginated(ctx context.Context, limit, offset int, sortBy []string) ([]Project, error)
    GetByCounterpartyID(ctx context.Context, counterpartyID uuid.UUID) ([]Project, error)
    Save(ctx context.Context, p Project) (Project, error)
    Delete(ctx context.Context, id uuid.UUID) error
}
```

### Repository Implementation (Infrastructure)

Located in `infrastructure/persistence/project_repository.go`:

**Key Implementation Details**:

1. **Tenant Isolation**:
   ```go
   tenantID := composables.UseTenantID(ctx)
   const getByIDSQL = `
       SELECT id, tenant_id, counterparty_id, name, description, created_at, updated_at
       FROM projects
       WHERE id = $1 AND tenant_id = $2
   `
   ```

2. **Parameterized Queries**: All queries use `$1`, `$2` placeholders (no string concatenation)

3. **Error Wrapping**:
   ```go
   const op serrors.Op = "ProjectRepository.GetByID"
   if err != nil {
       return nil, serrors.E(op, err)
   }
   ```

4. **Mapper Usage**: Convert between persistence models and domain aggregates:
   ```go
   return projectsMappers.MapFromModel(tenantID, model), nil
   ```

### Database Models (Persistence)

Located in `infrastructure/persistence/models/models.go`:

```go
type Project struct {
    ID             string
    TenantID       string
    CounterpartyID string
    Name           string
    Description    sql.NullString
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type ProjectStage struct {
    ID             string
    ProjectID      string
    StageNumber    int
    Description    sql.NullString
    TotalAmount    int64       // Stored in cents
    StartDate      sql.NullTime
    PlannedEndDate sql.NullTime
    FactualEndDate sql.NullTime
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

type ProjectStagePayment struct {
    ID             string
    ProjectStageID string
    PaymentID      string
    CreatedAt      time.Time
}
```

## Presentation Layer

### Controllers

**ProjectController**:
- `GET /projects` - List projects (paginated)
- `GET /projects/:id` - View project details
- `POST /projects` - Create new project
- `PUT /projects/:id` - Update project
- `DELETE /projects/:id` - Archive project

**ProjectStageController**:
- `GET /projects/:projectId/stages` - List stages
- `GET /projects/:projectId/stages/:stageId` - View stage
- `POST /projects/:projectId/stages` - Create stage
- `PUT /projects/:projectId/stages/:stageId` - Update stage

**Pattern**:
```go
func (c *ProjectController) List(w http.ResponseWriter, r *http.Request) {
    org := composables.GetOrgID(r.Context())
    params := composables.UsePaginated(r)

    projects, _, err := c.service.GetPaginated(r.Context(), params.Limit, params.Offset, params.SortBy)
    if err != nil {
        c.handleError(w, err)
        return
    }

    // HTMX support
    if htmx.IsHxRequest(r) {
        // Return partial HTML
    } else {
        // Return full page
    }
}
```

### ViewModels

Transform domain aggregates to presentation-ready data structures.

**ProjectViewModel**:
```go
type ProjectViewModel struct {
    ID             string
    Name           string
    Description    string
    CounterpartyID string
    CounterpartyName string
    StageCount     int
    TotalBudget    int64
    Status         string // Calculated from stages
}
```

### Templates

Located in `presentation/templates/pages/projects/`:

- `index.templ` - Project listing with pagination
- `detail.templ` - Project detail view with stages
- `drawer.templ` - Create/edit form (HTMX drawer)
- `stages_list.templ` - Embedded stages list

**HTMX Integration Pattern**:
```templ
templ ProjectForm(ctx context.Context, form *ProjectCreateForm) {
    <div id="project-form" class="drawer">
        <form hx-post="/projects" hx-target="#projects-list" hx-swap="outerHTML">
            <input type="text" name="Name" required />
            <textarea name="Description"></textarea>
            <select name="CounterpartyID" required>
                <!-- Options -->
            </select>
            <button type="submit" hx-disabled-elt="this">Create</button>
        </form>
    </div>
}
```

## Persistence Models

### Projects Table

```sql
CREATE TABLE projects (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    counterparty_id uuid NOT NULL REFERENCES counterparty(id) ON DELETE RESTRICT,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX projects_tenant_id_idx ON projects(tenant_id);
CREATE INDEX projects_counterparty_id_idx ON projects(counterparty_id);
CREATE INDEX projects_name_idx ON projects(name);
```

### Project Stages Table

```sql
CREATE TABLE project_stages (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_id uuid NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    stage_number int NOT NULL,
    description text,
    total_amount bigint NOT NULL,
    start_date date,
    planned_end_date date,
    factual_end_date date,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(project_id, stage_number)
);

CREATE INDEX project_stages_project_id_idx ON project_stages(project_id);
CREATE INDEX project_stages_stage_number_idx ON project_stages(stage_number);
```

### Project Stage Payments Table

```sql
CREATE TABLE project_stage_payments (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    project_stage_id uuid NOT NULL REFERENCES project_stages(id) ON DELETE CASCADE,
    payment_id uuid NOT NULL REFERENCES payments(id) ON DELETE CASCADE,
    created_at timestamp with time zone DEFAULT now(),
    UNIQUE(project_stage_id, payment_id)
);

CREATE INDEX project_stage_payments_project_stage_id_idx ON project_stage_payments(project_stage_id);
CREATE INDEX project_stage_payments_payment_id_idx ON project_stage_payments(payment_id);
```

## API Contracts

### Create Project

**Request**:
```
POST /projects
Content-Type: application/x-www-form-urlencoded

Name=Mobile App Development&Description=Build mobile app&CounterpartyID=123e4567-e89b-12d3-a456-426614174000
```

**Response** (201 Created):
```json
{
    "id": "987e6543-e89b-12d3-a456-426614174000",
    "name": "Mobile App Development",
    "description": "Build mobile app",
    "counterpartyId": "123e4567-e89b-12d3-a456-426614174000",
    "createdAt": "2024-12-12T10:30:00Z"
}
```

### Get Project with Stages

**Request**:
```
GET /projects/987e6543-e89b-12d3-a456-426614174000
```

**Response** (200 OK):
```json
{
    "id": "987e6543-e89b-12d3-a456-426614174000",
    "name": "Mobile App Development",
    "counterpartyId": "123e4567-e89b-12d3-a456-426614174000",
    "stages": [
        {
            "id": "stage-id-1",
            "stageNumber": 1,
            "description": "Design phase",
            "totalAmount": 50000,
            "startDate": "2024-01-01",
            "plannedEndDate": "2024-02-01"
        }
    ]
}
```

## Error Handling

All errors use the `serrors` package with operation tracking:

```go
const op serrors.Op = "ProjectService.Save"

if err := s.repo.Save(ctx, proj); err != nil {
    return nil, serrors.E(op, err)
}
```

Error types include:
- `KindValidation`: Input validation failures
- `KindNotFound`: Entity not found
- `KindPermission`: Authorization failures
- `KindDatabase`: Database operation failures
- `KindConflict`: Unique constraint or state conflicts

## Testing Strategy

### Service Layer Tests
- Happy path: Create, update, delete projects
- Validation: Invalid counterparty, duplicate names
- Permission checks: Unauthorized access
- Event publishing: Verify events published correctly

### Repository Layer Tests
- CRUD operations
- Tenant isolation: Verify queries include tenant_id
- FK constraints: Verify counterparty references
- Unique constraints: Duplicate names rejected

### Controller Tests
- Route handlers respond correctly
- Form parsing works
- HTMX requests return partial HTML
- Authentication/authorization enforced
