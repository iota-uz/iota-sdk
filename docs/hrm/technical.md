---
layout: default
title: Technical Architecture
parent: HRM
nav_order: 2
---

# Technical Architecture

## Module Structure

```
modules/hrm/
├── domain/
│   ├── aggregates/
│   │   └── employee/
│   │       ├── employee.go             # Aggregate interface
│   │       ├── employee_impl.go        # Private implementation
│   │       ├── employee_repository.go  # Repository interface
│   │       ├── employee_events.go      # Domain events
│   │       ├── employee_create_dto.go  # Creation DTO
│   │       ├── employee_update_dto.go  # Update DTO
│   │       └── language_impl.go        # Language value object
│   └── entities/
│       └── position/
│           ├── position.go             # Entity interface
│           ├── position_impl.go        # Implementation
│           └── position_repository.go  # Repository
├── infrastructure/
│   ├── persistence/
│   │   ├── employee_repository.go
│   │   ├── position_repository.go
│   │   ├── hrm_mappers.go
│   │   ├── models/
│   │   │   └── models.go
│   │   └── queries/
│   └── providers/
├── services/
│   ├── employee_service.go
│   └── position_service.go
├── presentation/
│   ├── controllers/
│   │   ├── employee_controller.go
│   │   └── position_controller.go
│   ├── viewmodels/
│   │   └── viewmodels.go
│   ├── templates/pages/employees/
│   │   ├── index.templ
│   │   ├── edit.templ
│   │   ├── new.templ
│   │   └── shared.templ
│   ├── mappers/
│   │   └── mappers.go
│   ├── locales/
│   │   ├── en.toml
│   │   ├── ru.toml
│   │   └── uz.toml
│   └── forms/
├── permissions/
│   └── constants.go
├── links.go
└── module.go
```

## Domain Layer

### Employee Aggregate

**Interface** (`employee.go`):
```go
type Employee interface {
    ID() uint
    TenantID() uuid.UUID
    FirstName() string
    LastName() string
    MiddleName() string
    Email() internet.Email
    Phone() string
    Salary() *money.Money
    AvatarID() uint
    HireDate() time.Time
    BirthDate() time.Time
    Language() Language
    Passport() passport.Passport
    Tin() tax.Tin
    Pin() tax.Pin
    Notes() string
    ResignationDate() *time.Time

    // Behavioral methods
    UpdateName(firstName, lastName, middleName string) Employee
    MarkAsResigned(date time.Time) Employee

    // Timestamps
    CreatedAt() time.Time
    UpdatedAt() time.Time
}
```

**Key Principles**:
- **Interface-based**: Aggregate as interface, not struct
- **Immutability**: Updates return new instance
- **Private struct**: `employee` struct unexported
- **Value Objects**: Uses Money, Email, Tax, Passport value objects
- **Functional Options**: Constructor supports optional fields

### Language Value Object

```go
type Language interface {
    Primary() string    // e.g., "uz"
    Secondary() string  // e.g., "ru"
}
```

### Position Entity

Similar structure to Employee, but simpler:
- `ID`, `TenantID`, `Name`, `Description`
- No state modifications beyond basic updates
- Immutable setters returning new instance

### Domain Events

```go
type CreatedEvent struct {
    Result Employee
    // Metadata: timestamp, user
}

type UpdatedEvent struct {
    Result Employee
}

type DeletedEvent struct {
    Result Employee
}
```

Events enable:
- Complete audit trail
- Downstream integrations (payroll, user management)
- Event sourcing capabilities

## Service Layer

### EmployeeService

**Responsibilities**:
- Query operations
- Create operations with validation
- Update operations
- Delete operations (soft delete / archival)
- Event publishing

**Transaction Handling**:
- Transactional consistency via `composables.InTx()`
- Events published after successful persistence
- Rollback on error

**Code Pattern**:
```go
type EmployeeService struct {
    repo      employee.Repository
    publisher eventbus.EventBus
}

func (s *EmployeeService) Create(ctx context.Context, data *employee.CreateDTO) error {
    entity, err := data.ToEntity()
    if err != nil {
        return err
    }

    createdEntity, err := s.repo.Create(ctx, entity)
    if err != nil {
        return err
    }

    ev, err := employee.NewCreatedEvent(ctx, *data, createdEntity)
    if err != nil {
        return err
    }

    s.publisher.Publish(ev)
    return nil
}
```

### PositionService

Simpler than EmployeeService, handling position CRUD operations.

## Repository Layer

### Employee Repository Interface (Domain)

```go
type Repository interface {
    Count(ctx context.Context) (int64, error)
    GetAll(ctx context.Context) ([]Employee, error)
    GetByID(ctx context.Context, id uint) (Employee, error)
    GetPaginated(ctx context.Context, params *FindParams) ([]Employee, error)
    Create(ctx context.Context, entity Employee) (Employee, error)
    Update(ctx context.Context, entity Employee) error
    Delete(ctx context.Context, id uint) error
}

type FindParams struct {
    Limit  int
    Offset int
    Search string // Search by name/email
    // Additional filter params
}
```

### Employee Repository Implementation

**Key Implementation Details**:

1. **Tenant Isolation**:
   ```go
   tenantID := composables.UseTenantID(ctx)
   const getByIDSQL = `
       SELECT id, tenant_id, first_name, last_name, ...
       FROM employees
       WHERE id = $1 AND tenant_id = $2
   `
   ```

2. **Parameterized Queries**: All use `$1`, `$2` placeholders

3. **Mapper Usage**: Convert between persistence and domain models
   ```go
   return hrmMappers.MapEmployeeFromModel(model), nil
   ```

4. **Error Wrapping**:
   ```go
   const op serrors.Op = "EmployeeRepository.GetByID"
   if err != nil {
       return nil, serrors.E(op, err)
   }
   ```

### Position Repository

Similar structure but for Position entity.

## Presentation Layer

### Controllers

**EmployeeController**:
- `GET /employees` - List employees (paginated)
- `GET /employees/:id` - View employee details
- `POST /employees` - Create new employee
- `PUT /employees/:id` - Update employee
- `DELETE /employees/:id` - Archive employee

**PositionController**:
- `GET /positions` - List positions
- `POST /positions` - Create position
- `PUT /positions/:id` - Update position

**Pattern**:
```go
func (c *EmployeeController) List(w http.ResponseWriter, r *http.Request) {
    org := composables.GetOrgID(r.Context())
    params := composables.UsePaginated(r)

    employees, err := c.service.GetPaginated(r.Context(), params)
    if err != nil {
        c.handleError(w, err)
        return
    }

    // HTMX support
    if htmx.IsHxRequest(r) {
        component.EmployeeList(employees).Render(r.Context(), w)
    } else {
        templates.Layout(pageCtx, component.EmployeeList(employees)).Render(r.Context(), w)
    }
}
```

### ViewModels

Transform domain aggregates to presentation structures:

```go
type EmployeeViewModel struct {
    ID              uint
    FirstName       string
    LastName        string
    MiddleName      string
    Email           string
    Phone           string
    HireDate        time.Time
    ResignationDate *time.Time
    Status          string // "Active" or "Resigned"
    FullName        string // Computed
}
```

### Templates

Located in `presentation/templates/pages/employees/`:

- `index.templ` - Employee listing with pagination and search
- `new.templ` - Create employee form
- `edit.templ` - Edit employee form
- `shared.templ` - Reusable components

**HTMX Integration**:
```templ
templ EmployeeForm(ctx context.Context, form *EmployeeCreateForm) {
    <div id="employee-form" class="drawer">
        <form hx-post="/employees" hx-target="#employees-list" hx-swap="outerHTML">
            <input type="text" name="FirstName" required />
            <input type="text" name="LastName" required />
            <input type="email" name="Email" required />
            <input type="tel" name="Phone" />
            <input type="date" name="HireDate" required />
            <button type="submit">Create</button>
        </form>
    </div>
}
```

## Persistence Models

### Employees Table

```sql
CREATE TABLE employees (
    id serial8 PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    first_name varchar(255) NOT NULL,
    last_name varchar(255) NOT NULL,
    middle_name varchar(255),
    email varchar(255) NOT NULL,
    phone varchar(255),
    salary decimal(9,2) NOT NULL,
    salary_currency_id varchar(3) REFERENCES currencies(code),
    hourly_rate decimal(9,2) NOT NULL,
    coefficient float8 NOT NULL,
    avatar_id bigint REFERENCES uploads(id),
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, email),
    UNIQUE(tenant_id, phone)
);

CREATE INDEX employees_tenant_id_idx ON employees(tenant_id);
CREATE INDEX employees_email_idx ON employees(email);
CREATE INDEX employees_phone_idx ON employees(phone);
CREATE INDEX employees_first_name_idx ON employees(first_name);
CREATE INDEX employees_last_name_idx ON employees(last_name);
```

### Employee Meta Table

```sql
CREATE TABLE employee_meta (
    employee_id bigint PRIMARY KEY REFERENCES employees(id) ON DELETE CASCADE,
    primary_language varchar(10),
    secondary_language varchar(10),
    tin varchar(50),
    pin varchar(50),
    notes text,
    birth_date date,
    hire_date date,
    resignation_date date
);
```

### Positions Table

```sql
CREATE TABLE positions (
    id serial8 PRIMARY KEY,
    tenant_id uuid NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
    name varchar(255) NOT NULL,
    description text,
    created_at timestamp with time zone DEFAULT now(),
    updated_at timestamp with time zone DEFAULT now(),
    UNIQUE(tenant_id, name)
);

CREATE INDEX positions_tenant_id_idx ON positions(tenant_id);
```

### Employee-Position Assignment Table

```sql
CREATE TABLE employee_positions (
    employee_id bigint REFERENCES employees(id) ON DELETE CASCADE,
    position_id bigint REFERENCES positions(id) ON DELETE CASCADE,
    PRIMARY KEY(employee_id, position_id)
);
```

## Database Models

```go
type Employee struct {
    ID               uint
    TenantID         string
    FirstName        string
    LastName         string
    MiddleName       sql.NullString
    Email            string
    Phone            sql.NullString
    Salary           float64
    SalaryCurrencyID sql.NullString
    HourlyRate       float64
    Coefficient      float64
    AvatarID         *uint
    CreatedAt        time.Time
    UpdatedAt        time.Time
}

type EmployeeMeta struct {
    PrimaryLanguage   sql.NullString
    SecondaryLanguage sql.NullString
    Tin               sql.NullString
    Pin               sql.NullString
    Notes             sql.NullString
    BirthDate         sql.NullTime
    HireDate          sql.NullTime
    ResignationDate   sql.NullTime
}

type Position struct {
    ID          uint
    TenantID    string
    Name        string
    Description sql.NullString
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type EmployeePosition struct {
    EmployeeID uint
    PositionID uint
}
```

## API Contracts

### Create Employee

**Request**:
```
POST /employees
Content-Type: application/x-www-form-urlencoded

FirstName=John&LastName=Doe&Email=john@example.com&Phone=+998901234567&HireDate=2024-01-15&Tin=1234567890
```

**Response** (201 Created):
```json
{
    "id": 123,
    "firstName": "John",
    "lastName": "Doe",
    "email": "john@example.com",
    "phone": "+998901234567",
    "hireDate": "2024-01-15",
    "createdAt": "2024-12-12T10:30:00Z"
}
```

### List Employees

**Request**:
```
GET /employees?page=1&limit=20&search=john
```

**Response** (200 OK):
```json
{
    "data": [
        {
            "id": 123,
            "firstName": "John",
            "lastName": "Doe",
            "email": "john@example.com",
            "status": "Active"
        }
    ],
    "total": 100,
    "page": 1,
    "limit": 20
}
```

## Error Handling

All errors use `serrors` package:

```go
const op serrors.Op = "EmployeeService.Create"

if err := s.repo.Create(ctx, entity); err != nil {
    return serrors.E(op, err)
}
```

Error types:
- `KindValidation`: Invalid input
- `KindNotFound`: Entity not found
- `KindPermission`: Authorization failures
- `KindConflict`: Duplicate email/phone
- `KindDatabase`: Database failures

## Testing Strategy

### Service Tests
- Happy path: Create, update, delete employees
- Validation: Invalid email, duplicate phone
- Permission checks: Unauthorized access
- Event publishing: Verify events published

### Repository Tests
- CRUD operations
- Tenant isolation
- Unique constraint enforcement (email, phone)
- Pagination and search

### Controller Tests
- Route handlers
- Form parsing
- HTMX requests
- Authentication/authorization
