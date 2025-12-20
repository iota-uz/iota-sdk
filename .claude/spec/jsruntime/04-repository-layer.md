# JavaScript Runtime - Repository Layer

## Overview

The repository layer implements data access for JavaScript Runtime entities following IOTA SDK patterns with multi-tenant isolation, parameterized queries, and the `pkg/repo` dynamic query builder.

## Repository Interfaces

All repository interfaces are defined in the domain layer with domain types (no database types leak into domain).

### ScriptRepository Interface

```go
// modules/jsruntime/domain/repositories/script_repository.go
package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/aggregates/script"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
)

type ScriptRepository interface {
	// Count and retrieval
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]script.Script, error)
	GetByID(ctx context.Context, id uuid.UUID) (script.Script, error)
	GetByName(ctx context.Context, name string) (script.Script, error)
	GetPaginated(ctx context.Context, params FindParams) ([]script.Script, uint, error)

	// Type-specific retrieval
	GetByType(ctx context.Context, scriptType value_objects.ScriptType) ([]script.Script, error)
	GetByEventType(ctx context.Context, eventType string) ([]script.Script, error)
	GetByHTTPPath(ctx context.Context, path string) (script.Script, error)
	GetScheduled(ctx context.Context) ([]script.Script, error)

	// Mutation
	Create(ctx context.Context, script script.Script) error
	Update(ctx context.Context, script script.Script) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Existence checks
	Exists(ctx context.Context, id uuid.UUID) (bool, error)
	NameExists(ctx context.Context, name string) (bool, error)
	HTTPPathExists(ctx context.Context, path string) (bool, error)

	// Search
	Search(ctx context.Context, query string, limit int) ([]script.Script, error)
}

// Field represents sortable/filterable script fields
type Field string

const (
	FieldID          Field = "id"
	FieldName        Field = "name"
	FieldType        Field = "type"
	FieldStatus      Field = "status"
	FieldCreatedAt   Field = "created_at"
	FieldUpdatedAt   Field = "updated_at"
	FieldTenantID    Field = "tenant_id"
)

// SortBy defines sort direction
type SortBy string

const (
	SortByAsc  SortBy = "ASC"
	SortByDesc SortBy = "DESC"
)

// FindParams defines query parameters for pagination and filtering
type FindParams struct {
	Limit  uint
	Offset uint
	SortBy Field
	Order  SortBy

	// Filters
	Type   *value_objects.ScriptType
	Status *value_objects.ScriptStatus
	Tags   []string
}
```

### ExecutionRepository Interface

```go
// modules/jsruntime/domain/repositories/execution_repository.go
package repositories

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/entities/execution"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
)

type ExecutionRepository interface {
	// Count and retrieval
	Count(ctx context.Context) (uint, error)
	GetAll(ctx context.Context) ([]execution.Execution, error)
	GetByID(ctx context.Context, id uuid.UUID) (execution.Execution, error)
	GetByScriptID(ctx context.Context, scriptID uuid.UUID) ([]execution.Execution, error)
	GetPaginated(ctx context.Context, params ExecutionFindParams) ([]execution.Execution, uint, error)

	// Status-based retrieval
	GetPending(ctx context.Context) ([]execution.Execution, error)
	GetRunning(ctx context.Context) ([]execution.Execution, error)
	GetByStatus(ctx context.Context, status value_objects.ExecutionStatus) ([]execution.Execution, error)

	// Mutation
	Create(ctx context.Context, exec execution.Execution) error
	Update(ctx context.Context, exec execution.Execution) error
	Delete(ctx context.Context, id uuid.UUID) error

	// Cleanup
	DeleteOlderThan(ctx context.Context, cutoff time.Time) (int64, error)
}

type ExecutionFindParams struct {
	Limit  uint
	Offset uint
	SortBy ExecutionField
	Order  SortBy

	// Filters
	ScriptID    *uuid.UUID
	Status      *value_objects.ExecutionStatus
	TriggerType *value_objects.TriggerType
	StartedFrom *time.Time
	StartedTo   *time.Time
}

type ExecutionField string

const (
	ExecutionFieldID         ExecutionField = "id"
	ExecutionFieldScriptID   ExecutionField = "script_id"
	ExecutionFieldStatus     ExecutionField = "status"
	ExecutionFieldStartedAt  ExecutionField = "started_at"
	ExecutionFieldCompletedAt ExecutionField = "completed_at"
)
```

### VersionRepository Interface

```go
// modules/jsruntime/domain/repositories/version_repository.go
package repositories

import (
	"context"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/entities/version"
)

type VersionRepository interface {
	// Retrieval
	GetByScriptID(ctx context.Context, scriptID uuid.UUID) ([]version.Version, error)
	GetByScriptIDAndVersion(ctx context.Context, scriptID uuid.UUID, versionNumber int) (version.Version, error)
	GetLatestVersion(ctx context.Context, scriptID uuid.UUID) (version.Version, error)

	// Mutation
	Create(ctx context.Context, v version.Version) error

	// Utility
	GetNextVersionNumber(ctx context.Context, scriptID uuid.UUID) (int, error)
}
```

## PostgreSQL Implementation

### ScriptRepository Implementation

```go
// modules/jsruntime/infrastructure/persistence/script_repository.go
package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/aggregates/script"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/repositories"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrScriptNotFound = errors.New("script not found")
)

const (
	selectScriptQuery = `
		SELECT
			id,
			tenant_id,
			organization_id,
			name,
			description,
			source,
			type,
			status,
			resource_limits,
			cron_expression,
			http_path,
			http_methods,
			event_types,
			metadata,
			tags,
			created_at,
			updated_at,
			created_by
		FROM scripts
	`
	countScriptQuery = `SELECT COUNT(*) FROM scripts`
)

type ScriptRepository struct {
	pool     *pgxpool.Pool
	fieldMap map[repositories.Field]string
}

func NewScriptRepository(pool *pgxpool.Pool) repositories.ScriptRepository {
	return &ScriptRepository{
		pool: pool,
		fieldMap: map[repositories.Field]string{
			repositories.FieldID:        "id",
			repositories.FieldName:      "name",
			repositories.FieldType:      "type",
			repositories.FieldStatus:    "status",
			repositories.FieldCreatedAt: "created_at",
			repositories.FieldUpdatedAt: "updated_at",
			repositories.FieldTenantID:  "tenant_id",
		},
	}
}

// Count returns total script count for tenant
func (r *ScriptRepository) Count(ctx context.Context) (uint, error) {
	const op serrors.Op = "ScriptRepository.Count"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := countScriptQuery + " WHERE tenant_id = $1"

	var count uint
	err := tx.QueryRow(ctx, query, tenantID).Scan(&count)
	if err != nil {
		return 0, serrors.E(op, err)
	}

	return count, nil
}

// GetAll retrieves all scripts for tenant
func (r *ScriptRepository) GetAll(ctx context.Context) ([]script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetAll"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := selectScriptQuery + " WHERE tenant_id = $1 ORDER BY created_at DESC"

	rows, err := tx.Query(ctx, query, tenantID)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	scripts := make([]script.Script, 0)
	for rows.Next() {
		s, err := r.scanScript(rows)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		scripts = append(scripts, s)
	}

	if err := rows.Err(); err != nil {
		return nil, serrors.E(op, err)
	}

	return scripts, nil
}

// GetByID retrieves a script by ID
func (r *ScriptRepository) GetByID(ctx context.Context, id uuid.UUID) (script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetByID"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := selectScriptQuery + " WHERE id = $1 AND tenant_id = $2"

	row := tx.QueryRow(ctx, query, id, tenantID)
	s, err := r.scanScriptRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serrors.E(op, ErrScriptNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return s, nil
}

// GetByName retrieves a script by name
func (r *ScriptRepository) GetByName(ctx context.Context, name string) (script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetByName"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := selectScriptQuery + " WHERE name = $1 AND tenant_id = $2"

	row := tx.QueryRow(ctx, query, name, tenantID)
	s, err := r.scanScriptRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serrors.E(op, ErrScriptNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return s, nil
}

// GetPaginated retrieves paginated scripts with filters
func (r *ScriptRepository) GetPaginated(ctx context.Context, params repositories.FindParams) ([]script.Script, uint, error) {
	const op serrors.Op = "ScriptRepository.GetPaginated"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	// Build dynamic query using pkg/repo
	qb := repo.NewQueryBuilder()
	qb.AddCondition("tenant_id = ?", tenantID)

	if params.Type != nil {
		qb.AddCondition("type = ?", string(*params.Type))
	}
	if params.Status != nil {
		qb.AddCondition("status = ?", string(*params.Status))
	}
	if len(params.Tags) > 0 {
		qb.AddCondition("tags && ?", params.Tags)
	}

	// Count total
	countQuery := countScriptQuery + " WHERE " + qb.Conditions()
	var total uint
	err := tx.QueryRow(ctx, countQuery, qb.Args()...).Scan(&total)
	if err != nil {
		return nil, 0, serrors.E(op, err)
	}

	// Get paginated results
	sortField := r.fieldMap[params.SortBy]
	if sortField == "" {
		sortField = "created_at"
	}
	order := string(params.Order)
	if order == "" {
		order = "DESC"
	}

	query := fmt.Sprintf(
		"%s WHERE %s ORDER BY %s %s LIMIT $%d OFFSET $%d",
		selectScriptQuery,
		qb.Conditions(),
		sortField,
		order,
		len(qb.Args())+1,
		len(qb.Args())+2,
	)
	args := append(qb.Args(), params.Limit, params.Offset)

	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, serrors.E(op, err)
	}
	defer rows.Close()

	scripts := make([]script.Script, 0)
	for rows.Next() {
		s, err := r.scanScript(rows)
		if err != nil {
			return nil, 0, serrors.E(op, err)
		}
		scripts = append(scripts, s)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, serrors.E(op, err)
	}

	return scripts, total, nil
}

// GetByType retrieves scripts by type
func (r *ScriptRepository) GetByType(ctx context.Context, scriptType value_objects.ScriptType) ([]script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetByType"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := selectScriptQuery + " WHERE tenant_id = $1 AND type = $2 AND status = $3"

	rows, err := tx.Query(ctx, query, tenantID, string(scriptType), string(value_objects.ScriptStatusActive))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	scripts := make([]script.Script, 0)
	for rows.Next() {
		s, err := r.scanScript(rows)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		scripts = append(scripts, s)
	}

	return scripts, nil
}

// GetByEventType retrieves scripts subscribed to an event type
func (r *ScriptRepository) GetByEventType(ctx context.Context, eventType string) ([]script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetByEventType"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := `
		SELECT ` + selectScriptQuery[7:] + ` -- Skip "SELECT " prefix
		FROM scripts s
		JOIN script_event_subscriptions sub ON sub.script_id = s.id
		WHERE s.tenant_id = $1
		  AND sub.event_type = $2
		  AND sub.is_active = true
		  AND s.status = $3
	`

	rows, err := tx.Query(ctx, query, tenantID, eventType, string(value_objects.ScriptStatusActive))
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	scripts := make([]script.Script, 0)
	for rows.Next() {
		s, err := r.scanScript(rows)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		scripts = append(scripts, s)
	}

	return scripts, nil
}

// GetByHTTPPath retrieves a script by HTTP path
func (r *ScriptRepository) GetByHTTPPath(ctx context.Context, path string) (script.Script, error) {
	const op serrors.Op = "ScriptRepository.GetByHTTPPath"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := `
		SELECT ` + selectScriptQuery[7:] + ` -- Skip "SELECT " prefix
		FROM scripts s
		JOIN script_http_endpoints e ON e.script_id = s.id
		WHERE s.tenant_id = $1
		  AND e.http_path = $2
		  AND e.is_active = true
		  AND s.status = $3
	`

	row := tx.QueryRow(ctx, query, tenantID, path, string(value_objects.ScriptStatusActive))
	s, err := r.scanScriptRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, serrors.E(op, ErrScriptNotFound)
		}
		return nil, serrors.E(op, err)
	}

	return s, nil
}

// GetScheduled retrieves all scheduled scripts
func (r *ScriptRepository) GetScheduled(ctx context.Context) ([]script.Script, error) {
	return r.GetByType(ctx, value_objects.ScriptTypeScheduled)
}

// Create inserts a new script
func (r *ScriptRepository) Create(ctx context.Context, s script.Script) error {
	const op serrors.Op = "ScriptRepository.Create"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)

	query := `
		INSERT INTO scripts (
			id, tenant_id, organization_id, name, description, source, type, status,
			resource_limits, cron_expression, http_path, http_methods, event_types,
			metadata, tags, created_at, updated_at, created_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18
		)
	`

	limits := mapResourceLimitsToJSON(s.ResourceLimits())
	metadata := mapMetadataToJSON(s.Metadata())

	_, err := tx.Exec(ctx, query,
		s.ID(),
		s.TenantID(),
		s.OrganizationID(),
		s.Name(),
		s.Description(),
		s.Source(),
		string(s.Type()),
		string(s.Status()),
		limits,
		mapCronExpression(s.CronExpression()),
		s.HTTPPath(),
		s.HTTPMethods(),
		s.EventTypes(),
		metadata,
		s.Tags(),
		s.CreatedAt(),
		s.UpdatedAt(),
		s.CreatedBy(),
	)
	if err != nil {
		return serrors.E(op, err)
	}

	return nil
}

// Update modifies an existing script
func (r *ScriptRepository) Update(ctx context.Context, s script.Script) error {
	const op serrors.Op = "ScriptRepository.Update"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := `
		UPDATE scripts SET
			name = $1,
			description = $2,
			source = $3,
			type = $4,
			status = $5,
			resource_limits = $6,
			cron_expression = $7,
			http_path = $8,
			http_methods = $9,
			event_types = $10,
			metadata = $11,
			tags = $12,
			updated_at = $13
		WHERE id = $14 AND tenant_id = $15
	`

	limits := mapResourceLimitsToJSON(s.ResourceLimits())
	metadata := mapMetadataToJSON(s.Metadata())

	result, err := tx.Exec(ctx, query,
		s.Name(),
		s.Description(),
		s.Source(),
		string(s.Type()),
		string(s.Status()),
		limits,
		mapCronExpression(s.CronExpression()),
		s.HTTPPath(),
		s.HTTPMethods(),
		s.EventTypes(),
		metadata,
		s.Tags(),
		s.UpdatedAt(),
		s.ID(),
		tenantID,
	)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrScriptNotFound)
	}

	return nil
}

// Delete removes a script
func (r *ScriptRepository) Delete(ctx context.Context, id uuid.UUID) error {
	const op serrors.Op = "ScriptRepository.Delete"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := "DELETE FROM scripts WHERE id = $1 AND tenant_id = $2"

	result, err := tx.Exec(ctx, query, id, tenantID)
	if err != nil {
		return serrors.E(op, err)
	}

	if result.RowsAffected() == 0 {
		return serrors.E(op, ErrScriptNotFound)
	}

	return nil
}

// Exists checks if a script exists by ID
func (r *ScriptRepository) Exists(ctx context.Context, id uuid.UUID) (bool, error) {
	const op serrors.Op = "ScriptRepository.Exists"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := "SELECT EXISTS(SELECT 1 FROM scripts WHERE id = $1 AND tenant_id = $2)"

	var exists bool
	err := tx.QueryRow(ctx, query, id, tenantID).Scan(&exists)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return exists, nil
}

// NameExists checks if a script name exists
func (r *ScriptRepository) NameExists(ctx context.Context, name string) (bool, error) {
	const op serrors.Op = "ScriptRepository.NameExists"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := "SELECT EXISTS(SELECT 1 FROM scripts WHERE name = $1 AND tenant_id = $2)"

	var exists bool
	err := tx.QueryRow(ctx, query, name, tenantID).Scan(&exists)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return exists, nil
}

// HTTPPathExists checks if an HTTP path exists
func (r *ScriptRepository) HTTPPathExists(ctx context.Context, path string) (bool, error) {
	const op serrors.Op = "ScriptRepository.HTTPPathExists"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	query := "SELECT EXISTS(SELECT 1 FROM scripts WHERE http_path = $1 AND tenant_id = $2)"

	var exists bool
	err := tx.QueryRow(ctx, query, path, tenantID).Scan(&exists)
	if err != nil {
		return false, serrors.E(op, err)
	}

	return exists, nil
}

// Search performs full-text search on scripts
func (r *ScriptRepository) Search(ctx context.Context, query string, limit int) ([]script.Script, error) {
	const op serrors.Op = "ScriptRepository.Search"

	tx := composables.UseTx(ctx)
	pool := composables.UsePool(ctx, r.pool)
	tenantID := composables.UseTenantID(ctx)

	searchQuery := selectScriptQuery + `
		WHERE tenant_id = $1
		  AND to_tsvector('english', name || ' ' || coalesce(description, '')) @@ plainto_tsquery('english', $2)
		ORDER BY created_at DESC
		LIMIT $3
	`

	rows, err := tx.Query(ctx, searchQuery, tenantID, query, limit)
	if err != nil {
		return nil, serrors.E(op, err)
	}
	defer rows.Close()

	scripts := make([]script.Script, 0)
	for rows.Next() {
		s, err := r.scanScript(rows)
		if err != nil {
			return nil, serrors.E(op, err)
		}
		scripts = append(scripts, s)
	}

	return scripts, nil
}

// Helper functions for scanning
func (r *ScriptRepository) scanScript(rows pgx.Rows) (script.Script, error) {
	var row ScriptRow
	err := rows.Scan(
		&row.ID,
		&row.TenantID,
		&row.OrganizationID,
		&row.Name,
		&row.Description,
		&row.Source,
		&row.Type,
		&row.Status,
		&row.ResourceLimits,
		&row.CronExpression,
		&row.HTTPPath,
		&row.HTTPMethods,
		&row.EventTypes,
		&row.Metadata,
		&row.Tags,
		&row.CreatedAt,
		&row.UpdatedAt,
		&row.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	return MapRowToScript(row)
}

func (r *ScriptRepository) scanScriptRow(row pgx.Row) (script.Script, error) {
	var scriptRow ScriptRow
	err := row.Scan(
		&scriptRow.ID,
		&scriptRow.TenantID,
		&scriptRow.OrganizationID,
		&scriptRow.Name,
		&scriptRow.Description,
		&scriptRow.Source,
		&scriptRow.Type,
		&scriptRow.Status,
		&scriptRow.ResourceLimits,
		&scriptRow.CronExpression,
		&scriptRow.HTTPPath,
		&scriptRow.HTTPMethods,
		&scriptRow.EventTypes,
		&scriptRow.Metadata,
		&scriptRow.Tags,
		&scriptRow.CreatedAt,
		&scriptRow.UpdatedAt,
		&scriptRow.CreatedBy,
	)
	if err != nil {
		return nil, err
	}

	return MapRowToScript(scriptRow)
}
```

## Mapper Functions

```go
// modules/jsruntime/infrastructure/persistence/mappers.go
package persistence

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/aggregates/script"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/entities/execution"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
)

// ScriptRow represents a database row from scripts table
type ScriptRow struct {
	ID             uuid.UUID
	TenantID       uuid.UUID
	OrganizationID sql.NullString
	Name           string
	Description    sql.NullString
	Source         string
	Type           string
	Status         string
	ResourceLimits []byte // JSONB
	CronExpression sql.NullString
	HTTPPath       sql.NullString
	HTTPMethods    []string
	EventTypes     []string
	Metadata       []byte // JSONB
	Tags           []string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	CreatedBy      sql.NullInt64
}

// MapRowToScript converts database row to domain entity
func MapRowToScript(row ScriptRow) (script.Script, error) {
	// Parse resource limits
	var limits value_objects.ResourceLimits
	if err := json.Unmarshal(row.ResourceLimits, &limits); err != nil {
		limits = value_objects.DefaultResourceLimits()
	}

	// Parse metadata
	var metadata map[string]string
	if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
		metadata = make(map[string]string)
	}

	// Parse cron expression
	var cronExpr *value_objects.CronExpression
	if row.CronExpression.Valid {
		expr, err := value_objects.NewCronExpression(row.CronExpression.String)
		if err == nil {
			cronExpr = expr
		}
	}

	// Parse organization ID
	var orgID uuid.UUID
	if row.OrganizationID.Valid {
		orgID = uuid.MustParse(row.OrganizationID.String)
	}

	// Build options
	opts := []script.Option{
		script.WithID(row.ID),
		script.WithTenantID(row.TenantID),
		script.WithDescription(row.Description.String),
		script.WithType(value_objects.ScriptType(row.Type)),
		script.WithStatus(value_objects.ScriptStatus(row.Status)),
		script.WithResourceLimits(limits),
		script.WithHTTPMethods(row.HTTPMethods),
		script.WithEventTypes(row.EventTypes),
		script.WithMetadata(metadata),
		script.WithTags(row.Tags),
		script.WithCreatedAt(row.CreatedAt),
		script.WithUpdatedAt(row.UpdatedAt),
	}

	if !orgID.IsNil() {
		opts = append(opts, script.WithOrganizationID(orgID))
	}
	if cronExpr != nil {
		opts = append(opts, script.WithCronExpression(cronExpr))
	}
	if row.HTTPPath.Valid {
		opts = append(opts, script.WithHTTPPath(row.HTTPPath.String))
	}
	if row.CreatedBy.Valid {
		opts = append(opts, script.WithCreatedBy(uint(row.CreatedBy.Int64)))
	}

	return script.New(row.Name, row.Source, value_objects.ScriptType(row.Type), opts...)
}

// Helper functions for domain to DB mapping
func mapResourceLimitsToJSON(limits value_objects.ResourceLimits) []byte {
	data, _ := json.Marshal(limits)
	return data
}

func mapMetadataToJSON(metadata map[string]string) []byte {
	data, _ := json.Marshal(metadata)
	return data
}

func mapCronExpression(expr *value_objects.CronExpression) sql.NullString {
	if expr == nil {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: expr.String(), Valid: true}
}
```

## Acceptance Criteria

### Repository Interfaces
- [ ] All interfaces defined in domain layer (`modules/jsruntime/domain/repositories/`)
- [ ] Use domain types only (no database types in interface)
- [ ] Field enums for sortable/filterable columns
- [ ] FindParams struct for pagination and filtering
- [ ] SortBy options (ASC, DESC)

### ScriptRepository
- [ ] Implements all CRUD operations (Count, GetAll, GetByID, GetByName, Create, Update, Delete)
- [ ] GetPaginated with filtering (type, status, tags)
- [ ] Type-specific queries (GetByType, GetByEventType, GetByHTTPPath, GetScheduled)
- [ ] Existence checks (Exists, NameExists, HTTPPathExists)
- [ ] Full-text search (Search method)
- [ ] All queries include `tenant_id` for isolation
- [ ] Uses `composables.UseTx()` and `composables.UsePool()`
- [ ] Uses `composables.UseTenantID()` for automatic tenant scoping

### ExecutionRepository
- [ ] CRUD operations for executions
- [ ] Status-based retrieval (GetPending, GetRunning, GetByStatus)
- [ ] Pagination with filters (script ID, status, trigger type, date range)
- [ ] Cleanup method (DeleteOlderThan) for retention policy

### VersionRepository
- [ ] Retrieval by script ID and version number
- [ ] GetLatestVersion for current version
- [ ] GetNextVersionNumber for auto-increment
- [ ] Create only (no update/delete for immutable audit trail)

### Mappers
- [ ] MapRowToScript converts DB row to domain entity
- [ ] Handles nullable fields (sql.NullString, sql.NullInt64)
- [ ] Parses JSONB columns (resource_limits, metadata)
- [ ] Parses arrays (http_methods, event_types, tags)
- [ ] Uses functional options pattern for entity construction

### Error Handling
- [ ] All methods define `serrors.Op` for operation tracking
- [ ] Errors wrapped with `serrors.E(op, err)`
- [ ] ErrScriptNotFound returned when query returns no rows
- [ ] Proper error propagation through layers

### Performance
- [ ] Parameterized queries ($1, $2) - no SQL concatenation
- [ ] SQL as constants (no string building in methods)
- [ ] Uses `pkg/repo.QueryBuilder` for dynamic filters
- [ ] Leverages indexes from schema (tenant_id, type, status, paths)
- [ ] EXPLAIN ANALYZE run on complex queries during development
