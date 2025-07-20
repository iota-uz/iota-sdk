# Phase 2: Domain Entity & Repository (2 days)

## Overview
Create the domain model for scripts following IOTA SDK's Domain-Driven Design patterns, including immutable entities, repository interfaces, and domain events.

## Background
- IOTA SDK uses immutable domain entities
- Repository pattern with FindParams for flexible queries
- Domain events for audit trail and integration
- Multi-tenant isolation at the repository level
- All entities must follow the existing patterns from finance module

## Task 2.1: Domain Layer (Day 1)

### Objectives
- Create Script aggregate with immutable pattern
- Define value objects for script types and metadata
- Create repository interface following IOTA patterns
- Implement domain events for script lifecycle

### Detailed Steps

#### 1. Create Script Aggregate
Create `modules/scripts/domain/aggregates/script/script.go`:
```go
package script

import (
    "time"
    "github.com/google/uuid"
)

// Script represents an immutable script entity
type Script interface {
    // Getters
    ID() uuid.UUID
    TenantID() uuid.UUID
    Name() string
    Description() string
    Type() ScriptType
    Content() string
    Version() int
    Tags() []string
    Metadata() map[string]interface{}
    Enabled() bool
    CreatedAt() time.Time
    UpdatedAt() time.Time
    CreatedBy() uuid.UUID
    UpdatedBy() uuid.UUID
    
    // Business operations (return new instance)
    UpdateContent(content string, updatedBy uuid.UUID) (Script, error)
    UpdateMetadata(metadata map[string]interface{}, updatedBy uuid.UUID) (Script, error)
    Enable(updatedBy uuid.UUID) Script
    Disable(updatedBy uuid.UUID) Script
    AddTag(tag string, updatedBy uuid.UUID) Script
    RemoveTag(tag string, updatedBy uuid.UUID) Script
    
    // Validation
    Validate() error
}
```

#### 2. Implement Script Entity
Create `modules/scripts/domain/aggregates/script/script_impl.go`:
```go
package script

import (
    "fmt"
    "time"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/serrors"
)

type scriptImpl struct {
    id          uuid.UUID
    tenantID    uuid.UUID
    name        string
    description string
    scriptType  ScriptType
    content     string
    version     int
    tags        []string
    metadata    map[string]interface{}
    enabled     bool
    createdAt   time.Time
    updatedAt   time.Time
    createdBy   uuid.UUID
    updatedBy   uuid.UUID
}

// Constructor
func NewScript(
    tenantID uuid.UUID,
    name string,
    scriptType ScriptType,
    content string,
    createdBy uuid.UUID,
) (Script, error) {
    script := &scriptImpl{
        id:         uuid.New(),
        tenantID:   tenantID,
        name:       name,
        scriptType: scriptType,
        content:    content,
        version:    1,
        tags:       []string{},
        metadata:   make(map[string]interface{}),
        enabled:    true,
        createdAt:  time.Now(),
        updatedAt:  time.Now(),
        createdBy:  createdBy,
        updatedBy:  createdBy,
    }
    
    if err := script.Validate(); err != nil {
        return nil, err
    }
    
    return script, nil
}

// Getters
func (s *scriptImpl) ID() uuid.UUID                   { return s.id }
func (s *scriptImpl) TenantID() uuid.UUID            { return s.tenantID }
func (s *scriptImpl) Name() string                   { return s.name }
func (s *scriptImpl) Description() string            { return s.description }
func (s *scriptImpl) Type() ScriptType               { return s.scriptType }
func (s *scriptImpl) Content() string                { return s.content }
func (s *scriptImpl) Version() int                   { return s.version }
func (s *scriptImpl) Tags() []string                 { return append([]string{}, s.tags...) }
func (s *scriptImpl) Metadata() map[string]interface{} { 
    // Return copy to maintain immutability
    m := make(map[string]interface{})
    for k, v := range s.metadata {
        m[k] = v
    }
    return m
}
func (s *scriptImpl) Enabled() bool                  { return s.enabled }
func (s *scriptImpl) CreatedAt() time.Time           { return s.createdAt }
func (s *scriptImpl) UpdatedAt() time.Time           { return s.updatedAt }
func (s *scriptImpl) CreatedBy() uuid.UUID           { return s.createdBy }
func (s *scriptImpl) UpdatedBy() uuid.UUID           { return s.updatedBy }

// Business operations
func (s *scriptImpl) UpdateContent(content string, updatedBy uuid.UUID) (Script, error) {
    if content == "" {
        return nil, serrors.Validation("content cannot be empty")
    }
    
    // Create new instance (immutability)
    updated := *s
    updated.content = content
    updated.version++
    updated.updatedAt = time.Now()
    updated.updatedBy = updatedBy
    
    return &updated, nil
}

func (s *scriptImpl) Enable(updatedBy uuid.UUID) Script {
    updated := *s
    updated.enabled = true
    updated.updatedAt = time.Now()
    updated.updatedBy = updatedBy
    return &updated
}

func (s *scriptImpl) Disable(updatedBy uuid.UUID) Script {
    updated := *s
    updated.enabled = false
    updated.updatedAt = time.Now()
    updated.updatedBy = updatedBy
    return &updated
}

// Validation
func (s *scriptImpl) Validate() error {
    if s.name == "" {
        return serrors.Validation("script name is required")
    }
    if len(s.name) > 255 {
        return serrors.Validation("script name must be less than 255 characters")
    }
    if s.content == "" {
        return serrors.Validation("script content is required")
    }
    if !s.scriptType.IsValid() {
        return serrors.Validation("invalid script type")
    }
    return nil
}
```

#### 3. Create Value Objects
Create `modules/scripts/domain/value_objects/script_type.go`:
```go
package value_objects

type ScriptType string

const (
    ScriptTypeCron         ScriptType = "cron"
    ScriptTypeHTTPEndpoint ScriptType = "http_endpoint"
    ScriptTypeEventHandler ScriptType = "event_handler"
    ScriptTypeFunction     ScriptType = "function"
)

func (st ScriptType) IsValid() bool {
    switch st {
    case ScriptTypeCron, ScriptTypeHTTPEndpoint, ScriptTypeEventHandler, ScriptTypeFunction:
        return true
    default:
        return false
    }
}

func (st ScriptType) String() string {
    return string(st)
}

// Execution context requirements
func (st ScriptType) RequiresSchedule() bool {
    return st == ScriptTypeCron
}

func (st ScriptType) RequiresHTTPRoute() bool {
    return st == ScriptTypeHTTPEndpoint
}

func (st ScriptType) RequiresEventSubscription() bool {
    return st == ScriptTypeEventHandler
}
```

Create `modules/scripts/domain/value_objects/cron_schedule.go`:
```go
package value_objects

import (
    "fmt"
    "github.com/robfig/cron/v3"
)

type CronSchedule struct {
    expression string
    timezone   string
}

func NewCronSchedule(expression, timezone string) (*CronSchedule, error) {
    // Validate cron expression
    _, err := cron.ParseStandard(expression)
    if err != nil {
        return nil, fmt.Errorf("invalid cron expression: %w", err)
    }
    
    if timezone == "" {
        timezone = "UTC"
    }
    
    return &CronSchedule{
        expression: expression,
        timezone:   timezone,
    }, nil
}

func (c *CronSchedule) Expression() string { return c.expression }
func (c *CronSchedule) Timezone() string   { return c.timezone }

func (c *CronSchedule) NextRun(from time.Time) (time.Time, error) {
    schedule, err := cron.ParseStandard(c.expression)
    if err != nil {
        return time.Time{}, err
    }
    
    loc, err := time.LoadLocation(c.timezone)
    if err != nil {
        return time.Time{}, err
    }
    
    return schedule.Next(from.In(loc)), nil
}
```

#### 4. Define Repository Interface
Create `modules/scripts/domain/aggregates/script/script_repository.go`:
```go
package script

import (
    "context"
    "github.com/google/uuid"
    "github.com/iota-uz/iota-sdk/pkg/repo"
)

type Repository interface {
    // Standard CRUD operations
    Create(ctx context.Context, script Script) (Script, error)
    Update(ctx context.Context, script Script) (Script, error)
    Delete(ctx context.Context, id uuid.UUID) error
    GetByID(ctx context.Context, id uuid.UUID) (Script, error)
    
    // Query operations
    Find(ctx context.Context, params FindParams) ([]Script, error)
    Count(ctx context.Context, params FindParams) (int, error)
    
    // Specialized queries
    GetByName(ctx context.Context, name string) (Script, error)
    GetEnabledByType(ctx context.Context, scriptType ScriptType) ([]Script, error)
    GetByTag(ctx context.Context, tag string) ([]Script, error)
    
    // Versioning
    GetVersion(ctx context.Context, id uuid.UUID, version int) (Script, error)
    GetVersionHistory(ctx context.Context, id uuid.UUID) ([]ScriptVersion, error)
}

// FindParams follows IOTA SDK pattern
type FindParams struct {
    ID       uuid.UUID
    Name     string
    Type     ScriptType
    Enabled  *bool
    Tags     []string
    Limit    int
    Offset   int
    SortBy   SortBy
    Filters  []Filter
    Search   string
}

type SortBy struct {
    Field repo.SortField
    Order repo.SortOrder
}

type Filter struct {
    Field    string
    Operator repo.FilterOperator
    Value    interface{}
}

// Version history
type ScriptVersion struct {
    ID        uuid.UUID
    ScriptID  uuid.UUID
    Version   int
    Content   string
    CreatedAt time.Time
    CreatedBy uuid.UUID
}
```

#### 5. Create Domain Events
Create `modules/scripts/domain/aggregates/script/script_events.go`:
```go
package script

import (
    "time"
    "github.com/google/uuid"
)

// Base event
type ScriptEvent struct {
    ScriptID  uuid.UUID
    TenantID  uuid.UUID
    Timestamp time.Time
    ActorID   uuid.UUID
}

// ScriptCreatedEvent fired when a new script is created
type ScriptCreatedEvent struct {
    ScriptEvent
    Name       string
    Type       ScriptType
    Content    string
}

func NewScriptCreatedEvent(script Script, actorID uuid.UUID) *ScriptCreatedEvent {
    return &ScriptCreatedEvent{
        ScriptEvent: ScriptEvent{
            ScriptID:  script.ID(),
            TenantID:  script.TenantID(),
            Timestamp: time.Now(),
            ActorID:   actorID,
        },
        Name:    script.Name(),
        Type:    script.Type(),
        Content: script.Content(),
    }
}

// ScriptUpdatedEvent fired when script content is updated
type ScriptUpdatedEvent struct {
    ScriptEvent
    OldVersion int
    NewVersion int
    Changes    map[string]interface{}
}

// ScriptExecutedEvent fired when script is executed
type ScriptExecutedEvent struct {
    ScriptEvent
    ExecutionID uuid.UUID
    Duration    time.Duration
    Success     bool
    Error       string
}

// ScriptFailedEvent fired when script execution fails
type ScriptFailedEvent struct {
    ScriptEvent
    ExecutionID uuid.UUID
    Error       string
    StackTrace  string
}

// ScriptEnabledEvent fired when script is enabled
type ScriptEnabledEvent struct {
    ScriptEvent
}

// ScriptDisabledEvent fired when script is disabled
type ScriptDisabledEvent struct {
    ScriptEvent
    Reason string
}
```

### Testing Requirements

Create `modules/scripts/domain/aggregates/script/script_test.go`:
```go
package script_test

import (
    "testing"
    "github.com/google/uuid"
    "github.com/stretchr/testify/require"
)

func TestScriptCreation(t *testing.T) {
    tenantID := uuid.New()
    userID := uuid.New()
    
    t.Run("should create valid script", func(t *testing.T) {
        script, err := NewScript(
            tenantID,
            "Test Script",
            ScriptTypeFunction,
            "console.log('Hello')",
            userID,
        )
        
        require.NoError(t, err)
        require.NotNil(t, script)
        require.Equal(t, "Test Script", script.Name())
        require.Equal(t, ScriptTypeFunction, script.Type())
        require.Equal(t, 1, script.Version())
        require.True(t, script.Enabled())
    })
    
    t.Run("should validate required fields", func(t *testing.T) {
        _, err := NewScript(tenantID, "", ScriptTypeFunction, "content", userID)
        require.Error(t, err)
        require.Contains(t, err.Error(), "name is required")
        
        _, err = NewScript(tenantID, "name", ScriptTypeFunction, "", userID)
        require.Error(t, err)
        require.Contains(t, err.Error(), "content is required")
    })
}

func TestScriptImmutability(t *testing.T) {
    script, _ := NewScript(uuid.New(), "Test", ScriptTypeFunction, "code", uuid.New())
    
    // Get tags
    tags1 := script.Tags()
    tags1 = append(tags1, "modified")
    
    // Original should be unchanged
    tags2 := script.Tags()
    require.Empty(t, tags2)
    
    // Get metadata
    meta1 := script.Metadata()
    meta1["key"] = "value"
    
    // Original should be unchanged
    meta2 := script.Metadata()
    require.Empty(t, meta2)
}

func TestScriptOperations(t *testing.T) {
    userID := uuid.New()
    script, _ := NewScript(uuid.New(), "Test", ScriptTypeFunction, "v1", userID)
    
    t.Run("update content", func(t *testing.T) {
        updated, err := script.UpdateContent("v2", userID)
        require.NoError(t, err)
        
        // New instance created
        require.NotEqual(t, script, updated)
        
        // Version incremented
        require.Equal(t, 2, updated.Version())
        require.Equal(t, "v2", updated.Content())
        
        // Original unchanged
        require.Equal(t, 1, script.Version())
        require.Equal(t, "v1", script.Content())
    })
    
    t.Run("enable/disable", func(t *testing.T) {
        disabled := script.Disable(userID)
        require.False(t, disabled.Enabled())
        require.True(t, script.Enabled()) // Original unchanged
        
        enabled := disabled.Enable(userID)
        require.True(t, enabled.Enabled())
    })
}

func TestScriptType(t *testing.T) {
    t.Run("validation", func(t *testing.T) {
        require.True(t, ScriptTypeCron.IsValid())
        require.True(t, ScriptTypeHTTPEndpoint.IsValid())
        require.True(t, ScriptTypeEventHandler.IsValid())
        require.True(t, ScriptTypeFunction.IsValid())
        require.False(t, ScriptType("invalid").IsValid())
    })
    
    t.Run("requirements", func(t *testing.T) {
        require.True(t, ScriptTypeCron.RequiresSchedule())
        require.False(t, ScriptTypeFunction.RequiresSchedule())
        
        require.True(t, ScriptTypeHTTPEndpoint.RequiresHTTPRoute())
        require.False(t, ScriptTypeCron.RequiresHTTPRoute())
        
        require.True(t, ScriptTypeEventHandler.RequiresEventSubscription())
        require.False(t, ScriptTypeHTTPEndpoint.RequiresEventSubscription())
    })
}

func TestCronSchedule(t *testing.T) {
    t.Run("valid schedule", func(t *testing.T) {
        schedule, err := NewCronSchedule("0 * * * *", "UTC")
        require.NoError(t, err)
        require.Equal(t, "0 * * * *", schedule.Expression())
        
        // Test next run calculation
        from := time.Date(2024, 1, 1, 10, 30, 0, 0, time.UTC)
        next, err := schedule.NextRun(from)
        require.NoError(t, err)
        require.Equal(t, 11, next.Hour())
        require.Equal(t, 0, next.Minute())
    })
    
    t.Run("invalid schedule", func(t *testing.T) {
        _, err := NewCronSchedule("invalid", "UTC")
        require.Error(t, err)
    })
}
```

### Deliverables Checklist
- [ ] Script aggregate with immutable pattern
- [ ] ScriptType value object with validation
- [ ] CronSchedule value object
- [ ] Repository interface following IOTA patterns
- [ ] Complete set of domain events
- [ ] Unit tests with 100% coverage
- [ ] Documentation for domain model

## Task 2.2: Infrastructure Layer (Day 2)

### Objectives
- Create database schema with versioning support
- Implement repository with multi-tenant isolation
- Create domain/database mappers
- Add soft delete functionality
- Support script version history

### Detailed Steps

#### 1. Create Database Schema
Create `modules/scripts/infrastructure/persistence/schema/scripts-schema.sql`:
```sql
-- Scripts table
CREATE TABLE IF NOT EXISTS scripts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    tags TEXT[] DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    enabled BOOLEAN NOT NULL DEFAULT true,
    deleted_at TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL,
    updated_by UUID NOT NULL,
    
    -- Constraints
    CONSTRAINT scripts_tenant_name_unique UNIQUE (tenant_id, name),
    CONSTRAINT scripts_type_check CHECK (type IN ('cron', 'http_endpoint', 'event_handler', 'function'))
);

-- Indexes
CREATE INDEX idx_scripts_tenant_id ON scripts(tenant_id);
CREATE INDEX idx_scripts_type ON scripts(type);
CREATE INDEX idx_scripts_enabled ON scripts(enabled);
CREATE INDEX idx_scripts_tags ON scripts USING GIN(tags);
CREATE INDEX idx_scripts_deleted_at ON scripts(deleted_at);

-- Script versions table for history
CREATE TABLE IF NOT EXISTS script_versions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    version INTEGER NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID NOT NULL,
    
    -- Constraints
    CONSTRAINT script_versions_unique UNIQUE (script_id, version)
);

-- Indexes for versions
CREATE INDEX idx_script_versions_script_id ON script_versions(script_id);
CREATE INDEX idx_script_versions_created_at ON script_versions(created_at);

-- Cron schedules table
CREATE TABLE IF NOT EXISTS script_schedules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    expression VARCHAR(255) NOT NULL,
    timezone VARCHAR(50) NOT NULL DEFAULT 'UTC',
    next_run TIMESTAMP,
    last_run TIMESTAMP,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_script_schedules_script_id ON script_schedules(script_id);
CREATE INDEX idx_script_schedules_next_run ON script_schedules(next_run);

-- HTTP routes table
CREATE TABLE IF NOT EXISTS script_routes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    script_id UUID NOT NULL REFERENCES scripts(id) ON DELETE CASCADE,
    method VARCHAR(10) NOT NULL,
    path VARCHAR(255) NOT NULL,
    auth_required BOOLEAN NOT NULL DEFAULT true,
    rate_limit INTEGER,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT script_routes_unique UNIQUE (method, path)
);

CREATE INDEX idx_script_routes_script_id ON script_routes(script_id);
CREATE INDEX idx_script_routes_path ON script_routes(path);
```

#### 2. Create Database Models
Create `modules/scripts/infrastructure/persistence/models/models.go`:
```go
package models

import (
    "time"
    "database/sql/driver"
    "github.com/google/uuid"
    "github.com/lib/pq"
)

type Script struct {
    ID          uuid.UUID      `db:"id"`
    TenantID    uuid.UUID      `db:"tenant_id"`
    Name        string         `db:"name"`
    Description string         `db:"description"`
    Type        string         `db:"type"`
    Content     string         `db:"content"`
    Version     int            `db:"version"`
    Tags        pq.StringArray `db:"tags"`
    Metadata    JSONB          `db:"metadata"`
    Enabled     bool           `db:"enabled"`
    DeletedAt   *time.Time     `db:"deleted_at"`
    CreatedAt   time.Time      `db:"created_at"`
    UpdatedAt   time.Time      `db:"updated_at"`
    CreatedBy   uuid.UUID      `db:"created_by"`
    UpdatedBy   uuid.UUID      `db:"updated_by"`
}

type ScriptVersion struct {
    ID        uuid.UUID `db:"id"`
    ScriptID  uuid.UUID `db:"script_id"`
    Version   int       `db:"version"`
    Content   string    `db:"content"`
    Metadata  JSONB     `db:"metadata"`
    CreatedAt time.Time `db:"created_at"`
    CreatedBy uuid.UUID `db:"created_by"`
}

type ScriptSchedule struct {
    ID         uuid.UUID  `db:"id"`
    ScriptID   uuid.UUID  `db:"script_id"`
    Expression string     `db:"expression"`
    Timezone   string     `db:"timezone"`
    NextRun    *time.Time `db:"next_run"`
    LastRun    *time.Time `db:"last_run"`
    Enabled    bool       `db:"enabled"`
    CreatedAt  time.Time  `db:"created_at"`
    UpdatedAt  time.Time  `db:"updated_at"`
}

type ScriptRoute struct {
    ID           uuid.UUID `db:"id"`
    ScriptID     uuid.UUID `db:"script_id"`
    Method       string    `db:"method"`
    Path         string    `db:"path"`
    AuthRequired bool      `db:"auth_required"`
    RateLimit    *int      `db:"rate_limit"`
    CreatedAt    time.Time `db:"created_at"`
    UpdatedAt    time.Time `db:"updated_at"`
}

// JSONB type for PostgreSQL JSONB columns
type JSONB map[string]interface{}

func (j JSONB) Value() (driver.Value, error) {
    if j == nil {
        return nil, nil
    }
    return json.Marshal(j)
}

func (j *JSONB) Scan(value interface{}) error {
    if value == nil {
        *j = nil
        return nil
    }
    bytes, ok := value.([]byte)
    if !ok {
        return errors.New("type assertion to []byte failed")
    }
    return json.Unmarshal(bytes, j)
}
```

#### 3. Implement Repository
Create `modules/scripts/infrastructure/persistence/script_repository.go`:
```go
package persistence

import (
    "context"
    "fmt"
    "github.com/google/uuid"
    "github.com/jmoiron/sqlx"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/modules/scripts/infrastructure/persistence/models"
    "github.com/iota-uz/iota-sdk/pkg/composables"
)

type scriptRepository struct {
    db *sqlx.DB
}

func NewScriptRepository(db *sqlx.DB) script.Repository {
    return &scriptRepository{db: db}
}

func (r *scriptRepository) Create(ctx context.Context, script script.Script) (script.Script, error) {
    // Ensure tenant isolation
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, fmt.Errorf("failed to get tenant from context: %w", err)
    }
    
    if script.TenantID() != tenantID {
        return nil, fmt.Errorf("tenant mismatch")
    }
    
    // Convert domain to DB model
    dbScript := mapScriptToModel(script)
    
    // Insert script
    query := `
        INSERT INTO scripts (
            id, tenant_id, name, description, type, content, version,
            tags, metadata, enabled, created_at, updated_at, created_by, updated_by
        ) VALUES (
            :id, :tenant_id, :name, :description, :type, :content, :version,
            :tags, :metadata, :enabled, :created_at, :updated_at, :created_by, :updated_by
        ) RETURNING *`
    
    stmt, err := r.db.PrepareNamedContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()
    
    var created models.Script
    err = stmt.GetContext(ctx, &created, dbScript)
    if err != nil {
        return nil, err
    }
    
    // Create initial version record
    versionQuery := `
        INSERT INTO script_versions (script_id, version, content, created_by)
        VALUES ($1, $2, $3, $4)`
    
    _, err = r.db.ExecContext(ctx, versionQuery, 
        script.ID(), script.Version(), script.Content(), script.CreatedBy())
    if err != nil {
        return nil, err
    }
    
    return mapModelToScript(&created), nil
}

func (r *scriptRepository) Update(ctx context.Context, script script.Script) (script.Script, error) {
    // Ensure tenant isolation
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, err
    }
    
    // Check ownership
    existing, err := r.GetByID(ctx, script.ID())
    if err != nil {
        return nil, err
    }
    
    if existing.TenantID() != tenantID {
        return nil, fmt.Errorf("access denied")
    }
    
    // Update script
    dbScript := mapScriptToModel(script)
    query := `
        UPDATE scripts SET
            name = :name,
            description = :description,
            content = :content,
            version = :version,
            tags = :tags,
            metadata = :metadata,
            enabled = :enabled,
            updated_at = :updated_at,
            updated_by = :updated_by
        WHERE id = :id AND tenant_id = :tenant_id
        RETURNING *`
    
    stmt, err := r.db.PrepareNamedContext(ctx, query)
    if err != nil {
        return nil, err
    }
    defer stmt.Close()
    
    var updated models.Script
    err = stmt.GetContext(ctx, &updated, dbScript)
    if err != nil {
        return nil, err
    }
    
    // Create version record if content changed
    if existing.Content() != script.Content() {
        versionQuery := `
            INSERT INTO script_versions (script_id, version, content, created_by)
            VALUES ($1, $2, $3, $4)`
        
        _, err = r.db.ExecContext(ctx, versionQuery,
            script.ID(), script.Version(), script.Content(), script.UpdatedBy())
        if err != nil {
            return nil, err
        }
    }
    
    return mapModelToScript(&updated), nil
}

func (r *scriptRepository) Delete(ctx context.Context, id uuid.UUID) error {
    // Soft delete with tenant check
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return err
    }
    
    query := `
        UPDATE scripts 
        SET deleted_at = CURRENT_TIMESTAMP 
        WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
    
    result, err := r.db.ExecContext(ctx, query, id, tenantID)
    if err != nil {
        return err
    }
    
    rows, err := result.RowsAffected()
    if err != nil {
        return err
    }
    
    if rows == 0 {
        return fmt.Errorf("script not found")
    }
    
    return nil
}

func (r *scriptRepository) GetByID(ctx context.Context, id uuid.UUID) (script.Script, error) {
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, err
    }
    
    query := `
        SELECT * FROM scripts 
        WHERE id = $1 AND tenant_id = $2 AND deleted_at IS NULL`
    
    var dbScript models.Script
    err = r.db.GetContext(ctx, &dbScript, query, id, tenantID)
    if err != nil {
        return nil, err
    }
    
    return mapModelToScript(&dbScript), nil
}

func (r *scriptRepository) Find(ctx context.Context, params script.FindParams) ([]script.Script, error) {
    tenantID, err := composables.UseTenantID(ctx)
    if err != nil {
        return nil, err
    }
    
    query := `SELECT * FROM scripts WHERE tenant_id = $1 AND deleted_at IS NULL`
    args := []interface{}{tenantID}
    argCount := 1
    
    // Build dynamic query
    if params.Type != "" {
        argCount++
        query += fmt.Sprintf(" AND type = $%d", argCount)
        args = append(args, params.Type)
    }
    
    if params.Enabled != nil {
        argCount++
        query += fmt.Sprintf(" AND enabled = $%d", argCount)
        args = append(args, *params.Enabled)
    }
    
    if len(params.Tags) > 0 {
        argCount++
        query += fmt.Sprintf(" AND tags && $%d", argCount)
        args = append(args, pq.Array(params.Tags))
    }
    
    if params.Search != "" {
        argCount++
        query += fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", argCount, argCount)
        args = append(args, "%"+params.Search+"%")
    }
    
    // Sorting
    if params.SortBy.Field != "" {
        query += fmt.Sprintf(" ORDER BY %s %s", params.SortBy.Field, params.SortBy.Order)
    } else {
        query += " ORDER BY created_at DESC"
    }
    
    // Pagination
    if params.Limit > 0 {
        query += fmt.Sprintf(" LIMIT %d", params.Limit)
    }
    if params.Offset > 0 {
        query += fmt.Sprintf(" OFFSET %d", params.Offset)
    }
    
    rows, err := r.db.QueryContext(ctx, query, args...)
    if err != nil {
        return nil, err
    }
    defer rows.Close()
    
    var scripts []script.Script
    for rows.Next() {
        var dbScript models.Script
        err := rows.Scan(&dbScript)
        if err != nil {
            return nil, err
        }
        scripts = append(scripts, mapModelToScript(&dbScript))
    }
    
    return scripts, nil
}
```

#### 4. Create Mappers
Create `modules/scripts/infrastructure/persistence/script_mappers.go`:
```go
package persistence

import (
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/aggregates/script"
    "github.com/iota-uz/iota-sdk/modules/scripts/domain/value_objects"
    "github.com/iota-uz/iota-sdk/modules/scripts/infrastructure/persistence/models"
    "github.com/iota-uz/iota-sdk/pkg/mapping"
)

// Domain to DB
func mapScriptToModel(s script.Script) *models.Script {
    return &models.Script{
        ID:          s.ID(),
        TenantID:    s.TenantID(),
        Name:        s.Name(),
        Description: s.Description(),
        Type:        s.Type().String(),
        Content:     s.Content(),
        Version:     s.Version(),
        Tags:        s.Tags(),
        Metadata:    models.JSONB(s.Metadata()),
        Enabled:     s.Enabled(),
        CreatedAt:   s.CreatedAt(),
        UpdatedAt:   s.UpdatedAt(),
        CreatedBy:   s.CreatedBy(),
        UpdatedBy:   s.UpdatedBy(),
    }
}

// DB to Domain
func mapModelToScript(m *models.Script) script.Script {
    // Use factory method to ensure proper initialization
    s := script.FromRepository(
        m.ID,
        m.TenantID,
        m.Name,
        m.Description,
        value_objects.ScriptType(m.Type),
        m.Content,
        m.Version,
        m.Tags,
        mapping.JSONBToMap(m.Metadata),
        m.Enabled,
        m.CreatedAt,
        m.UpdatedAt,
        m.CreatedBy,
        m.UpdatedBy,
    )
    
    return s
}

func mapModelToScriptVersion(m *models.ScriptVersion) script.ScriptVersion {
    return script.ScriptVersion{
        ID:        m.ID,
        ScriptID:  m.ScriptID,
        Version:   m.Version,
        Content:   m.Content,
        CreatedAt: m.CreatedAt,
        CreatedBy: m.CreatedBy,
    }
}
```

### Testing Requirements

Create `modules/scripts/infrastructure/persistence/setup_test.go`:
```go
package persistence_test

import (
    "context"
    "testing"
    "github.com/iota-uz/iota-sdk/pkg/testutil"
)

var testDB *sqlx.DB

func TestMain(m *testing.M) {
    // Setup test database
    testDB = testutil.SetupTestDB()
    defer testDB.Close()
    
    // Run migrations
    testutil.RunMigrations(testDB, "schema/scripts-schema.sql")
    
    // Run tests
    os.Exit(m.Run())
}

func setupTestContext(tenantID, userID uuid.UUID) context.Context {
    ctx := context.Background()
    ctx = composables.WithTenantID(ctx, tenantID)
    ctx = composables.WithUserID(ctx, userID)
    return ctx
}
```

Create `modules/scripts/infrastructure/persistence/script_repository_test.go`:
```go
func TestScriptRepository(t *testing.T) {
    repo := NewScriptRepository(testDB)
    tenantID := uuid.New()
    userID := uuid.New()
    ctx := setupTestContext(tenantID, userID)
    
    t.Run("Create", func(t *testing.T) {
        script, _ := script.NewScript(
            tenantID,
            "Test Script",
            value_objects.ScriptTypeFunction,
            "console.log('test')",
            userID,
        )
        
        created, err := repo.Create(ctx, script)
        require.NoError(t, err)
        require.NotNil(t, created)
        require.Equal(t, script.Name(), created.Name())
        
        // Verify version created
        versions, err := repo.GetVersionHistory(ctx, created.ID())
        require.NoError(t, err)
        require.Len(t, versions, 1)
    })
    
    t.Run("Multi-tenant isolation", func(t *testing.T) {
        // Create script in tenant 1
        tenant1 := uuid.New()
        ctx1 := setupTestContext(tenant1, userID)
        script1, _ := script.NewScript(tenant1, "Script 1", value_objects.ScriptTypeFunction, "code", userID)
        created1, _ := repo.Create(ctx1, script1)
        
        // Try to access from tenant 2
        tenant2 := uuid.New()
        ctx2 := setupTestContext(tenant2, userID)
        _, err := repo.GetByID(ctx2, created1.ID())
        require.Error(t, err)
        
        // Find should not return other tenant's scripts
        scripts, err := repo.Find(ctx2, script.FindParams{})
        require.NoError(t, err)
        require.Empty(t, scripts)
    })
    
    t.Run("Soft delete", func(t *testing.T) {
        script, _ := script.NewScript(tenantID, "To Delete", value_objects.ScriptTypeFunction, "code", userID)
        created, _ := repo.Create(ctx, script)
        
        // Delete
        err := repo.Delete(ctx, created.ID())
        require.NoError(t, err)
        
        // Should not find
        _, err = repo.GetByID(ctx, created.ID())
        require.Error(t, err)
        
        // Should not appear in find results
        scripts, _ := repo.Find(ctx, script.FindParams{})
        for _, s := range scripts {
            require.NotEqual(t, created.ID(), s.ID())
        }
    })
    
    t.Run("Version history", func(t *testing.T) {
        script, _ := script.NewScript(tenantID, "Versioned", value_objects.ScriptTypeFunction, "v1", userID)
        created, _ := repo.Create(ctx, script)
        
        // Update content
        updated, _ := created.UpdateContent("v2", userID)
        saved, _ := repo.Update(ctx, updated)
        
        // Check versions
        versions, err := repo.GetVersionHistory(ctx, saved.ID())
        require.NoError(t, err)
        require.Len(t, versions, 2)
        require.Equal(t, "v1", versions[0].Content)
        require.Equal(t, "v2", versions[1].Content)
    })
}
```

### Deliverables Checklist
- [ ] Database schema with all tables
- [ ] Migration files ready
- [ ] Database models with JSONB support
- [ ] Repository implementation with multi-tenancy
- [ ] Domain/DB mappers using pkg/mapping
- [ ] Soft delete functionality
- [ ] Version history tracking
- [ ] Integration tests with real PostgreSQL
- [ ] Test coverage > 80%

## Success Criteria
1. Domain model follows IOTA's immutable pattern
2. All domain operations maintain immutability
3. Repository enforces tenant isolation
4. Soft delete preserves data integrity
5. Version history accurately tracks changes
6. All tests pass with real database
7. No cross-tenant data leakage

## Notes for Next Phase
- Domain events will be published by the service layer
- Script validation will be enhanced in service layer
- Consider adding script templates in future
- Version comparison/diff functionality may be needed
- Migration rollback procedures should be documented