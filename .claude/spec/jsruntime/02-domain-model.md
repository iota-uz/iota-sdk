# JavaScript Runtime - Domain Model

## Overview

The JavaScript Runtime domain model follows DDD principles with aggregates as interfaces, entities with immutable setters, value objects for type safety, and domain events for lifecycle tracking.

## Aggregates

### Script Aggregate

The Script aggregate is the root entity representing a user-defined JavaScript program with metadata, resource limits, and trigger configuration.

```go
// modules/jsruntime/domain/aggregates/script/script.go
package script

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
)

// Script is the aggregate root for user-defined JavaScript code
type Script interface {
	// Identity
	ID() uuid.UUID
	TenantID() uuid.UUID
	OrganizationID() uuid.UUID

	// Basic attributes
	Name() string
	Description() string
	Source() string
	Type() value_objects.ScriptType
	Status() value_objects.ScriptStatus

	// Resource management
	ResourceLimits() value_objects.ResourceLimits

	// Trigger configuration
	CronExpression() *value_objects.CronExpression
	HTTPPath() string
	HTTPMethods() []string
	EventTypes() []string

	// Metadata
	Metadata() map[string]string
	Tags() []string

	// Audit fields
	CreatedAt() time.Time
	UpdatedAt() time.Time
	CreatedBy() uint

	// Immutable setters (return new instance)
	SetName(name string) Script
	SetDescription(description string) Script
	SetSource(source string) Script
	SetStatus(status value_objects.ScriptStatus) Script
	SetResourceLimits(limits value_objects.ResourceLimits) Script
	SetCronExpression(expr *value_objects.CronExpression) Script
	SetHTTPPath(path string) Script
	SetHTTPMethods(methods []string) Script
	SetEventTypes(types []string) Script
	SetMetadata(metadata map[string]string) Script
	SetTags(tags []string) Script

	// Business methods
	CanExecute() bool
	Validate() error
	IsScheduled() bool
	IsHTTPEndpoint() bool
	IsEventTriggered() bool
}

// Option is a functional option for script construction
type Option func(*script)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(s *script) {
		s.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(s *script) {
		s.tenantID = tenantID
	}
}

func WithOrganizationID(orgID uuid.UUID) Option {
	return func(s *script) {
		s.organizationID = orgID
	}
}

func WithDescription(description string) Option {
	return func(s *script) {
		s.description = description
	}
}

func WithSource(source string) Option {
	return func(s *script) {
		s.source = source
	}
}

func WithType(scriptType value_objects.ScriptType) Option {
	return func(s *script) {
		s.scriptType = scriptType
	}
}

func WithStatus(status value_objects.ScriptStatus) Option {
	return func(s *script) {
		s.status = status
	}
}

func WithResourceLimits(limits value_objects.ResourceLimits) Option {
	return func(s *script) {
		s.resourceLimits = limits
	}
}

func WithCronExpression(expr *value_objects.CronExpression) Option {
	return func(s *script) {
		s.cronExpression = expr
	}
}

func WithHTTPPath(path string) Option {
	return func(s *script) {
		s.httpPath = path
	}
}

func WithHTTPMethods(methods []string) Option {
	return func(s *script) {
		s.httpMethods = methods
	}
}

func WithEventTypes(types []string) Option {
	return func(s *script) {
		s.eventTypes = types
	}
}

func WithMetadata(metadata map[string]string) Option {
	return func(s *script) {
		s.metadata = metadata
	}
}

func WithTags(tags []string) Option {
	return func(s *script) {
		s.tags = tags
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(s *script) {
		s.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(s *script) {
		s.updatedAt = t
	}
}

func WithCreatedBy(userID uint) Option {
	return func(s *script) {
		s.createdBy = userID
	}
}

// --- Constructor ---

func New(name string, source string, scriptType value_objects.ScriptType, opts ...Option) (Script, error) {
	s := &script{
		id:             uuid.New(),
		name:           name,
		source:         source,
		scriptType:     scriptType,
		status:         value_objects.ScriptStatusDraft,
		resourceLimits: value_objects.DefaultResourceLimits(),
		metadata:       make(map[string]string),
		tags:           []string{},
		httpMethods:    []string{},
		eventTypes:     []string{},
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
	}

	for _, opt := range opts {
		opt(s)
	}

	if err := s.Validate(); err != nil {
		return nil, err
	}

	return s, nil
}

// --- Private implementation ---

type script struct {
	id             uuid.UUID
	tenantID       uuid.UUID
	organizationID uuid.UUID
	name           string
	description    string
	source         string
	scriptType     value_objects.ScriptType
	status         value_objects.ScriptStatus
	resourceLimits value_objects.ResourceLimits
	cronExpression *value_objects.CronExpression
	httpPath       string
	httpMethods    []string
	eventTypes     []string
	metadata       map[string]string
	tags           []string
	createdAt      time.Time
	updatedAt      time.Time
	createdBy      uint
}

func (s *script) ID() uuid.UUID                                { return s.id }
func (s *script) TenantID() uuid.UUID                          { return s.tenantID }
func (s *script) OrganizationID() uuid.UUID                    { return s.organizationID }
func (s *script) Name() string                                 { return s.name }
func (s *script) Description() string                          { return s.description }
func (s *script) Source() string                               { return s.source }
func (s *script) Type() value_objects.ScriptType               { return s.scriptType }
func (s *script) Status() value_objects.ScriptStatus           { return s.status }
func (s *script) ResourceLimits() value_objects.ResourceLimits { return s.resourceLimits }
func (s *script) CronExpression() *value_objects.CronExpression {
	return s.cronExpression
}
func (s *script) HTTPPath() string              { return s.httpPath }
func (s *script) HTTPMethods() []string         { return s.httpMethods }
func (s *script) EventTypes() []string          { return s.eventTypes }
func (s *script) Metadata() map[string]string   { return s.metadata }
func (s *script) Tags() []string                { return s.tags }
func (s *script) CreatedAt() time.Time          { return s.createdAt }
func (s *script) UpdatedAt() time.Time          { return s.updatedAt }
func (s *script) CreatedBy() uint               { return s.createdBy }

// Immutable setters (return new instance)
func (s *script) SetName(name string) Script {
	result := *s
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetDescription(description string) Script {
	result := *s
	result.description = description
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetSource(source string) Script {
	result := *s
	result.source = source
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetStatus(status value_objects.ScriptStatus) Script {
	result := *s
	result.status = status
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetResourceLimits(limits value_objects.ResourceLimits) Script {
	result := *s
	result.resourceLimits = limits
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetCronExpression(expr *value_objects.CronExpression) Script {
	result := *s
	result.cronExpression = expr
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetHTTPPath(path string) Script {
	result := *s
	result.httpPath = path
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetHTTPMethods(methods []string) Script {
	result := *s
	result.httpMethods = methods
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetEventTypes(types []string) Script {
	result := *s
	result.eventTypes = types
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetMetadata(metadata map[string]string) Script {
	result := *s
	result.metadata = metadata
	result.updatedAt = time.Now()
	return &result
}

func (s *script) SetTags(tags []string) Script {
	result := *s
	result.tags = tags
	result.updatedAt = time.Now()
	return &result
}

// Business methods
func (s *script) CanExecute() bool {
	return s.status == value_objects.ScriptStatusActive
}

func (s *script) Validate() error {
	if s.name == "" {
		return serrors.E(serrors.KindValidation, "script name is required")
	}
	if s.source == "" {
		return serrors.E(serrors.KindValidation, "script source is required")
	}
	if s.tenantID == uuid.Nil {
		return serrors.E(serrors.KindValidation, "tenant ID is required")
	}

	// Type-specific validation
	if s.scriptType == value_objects.ScriptTypeScheduled && s.cronExpression == nil {
		return serrors.E(serrors.KindValidation, "cron expression required for scheduled scripts")
	}
	if s.scriptType == value_objects.ScriptTypeHTTP && s.httpPath == "" {
		return serrors.E(serrors.KindValidation, "HTTP path required for HTTP endpoint scripts")
	}
	if s.scriptType == value_objects.ScriptTypeEvent && len(s.eventTypes) == 0 {
		return serrors.E(serrors.KindValidation, "event types required for event-triggered scripts")
	}

	return nil
}

func (s *script) IsScheduled() bool {
	return s.scriptType == value_objects.ScriptTypeScheduled
}

func (s *script) IsHTTPEndpoint() bool {
	return s.scriptType == value_objects.ScriptTypeHTTP
}

func (s *script) IsEventTriggered() bool {
	return s.scriptType == value_objects.ScriptTypeEvent
}
```

## Entities

### Execution Entity

The Execution entity represents a single run of a script with input, output, status, and metrics.

```go
// modules/jsruntime/domain/entities/execution/execution.go
package execution

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/jsruntime/domain/value_objects"
)

type Execution interface {
	// Identity
	ID() uuid.UUID
	ScriptID() uuid.UUID
	TenantID() uuid.UUID

	// Status
	Status() value_objects.ExecutionStatus

	// Trigger information
	TriggerType() value_objects.TriggerType
	TriggerData() value_objects.TriggerData

	// Input/Output
	Input() map[string]interface{}
	Output() interface{}
	Error() string

	// Metrics
	Metrics() value_objects.ExecutionMetrics

	// Timestamps
	StartedAt() time.Time
	CompletedAt() *time.Time

	// Immutable setters
	SetStatus(status value_objects.ExecutionStatus) Execution
	SetOutput(output interface{}) Execution
	SetError(err string) Execution
	SetMetrics(metrics value_objects.ExecutionMetrics) Execution
	SetCompletedAt(t time.Time) Execution

	// Business methods
	IsRunning() bool
	IsCompleted() bool
	IsFailed() bool
	Duration() time.Duration
}

type Option func(*execution)

// --- Option setters ---

func WithID(id uuid.UUID) Option {
	return func(e *execution) {
		e.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(e *execution) {
		e.tenantID = tenantID
	}
}

func WithStatus(status value_objects.ExecutionStatus) Option {
	return func(e *execution) {
		e.status = status
	}
}

func WithInput(input map[string]interface{}) Option {
	return func(e *execution) {
		e.input = input
	}
}

func WithOutput(output interface{}) Option {
	return func(e *execution) {
		e.output = output
	}
}

func WithError(err string) Option {
	return func(e *execution) {
		e.errorMsg = err
	}
}

func WithMetrics(metrics value_objects.ExecutionMetrics) Option {
	return func(e *execution) {
		e.metrics = metrics
	}
}

func WithStartedAt(t time.Time) Option {
	return func(e *execution) {
		e.startedAt = t
	}
}

func WithCompletedAt(t time.Time) Option {
	return func(e *execution) {
		e.completedAt = &t
	}
}

// --- Constructor ---

func New(scriptID uuid.UUID, triggerType value_objects.TriggerType, triggerData value_objects.TriggerData, opts ...Option) (Execution, error) {
	e := &execution{
		id:          uuid.New(),
		scriptID:    scriptID,
		status:      value_objects.ExecutionStatusPending,
		triggerType: triggerType,
		triggerData: triggerData,
		input:       make(map[string]interface{}),
		startedAt:   time.Now(),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e, nil
}

// --- Private implementation ---

type execution struct {
	id          uuid.UUID
	scriptID    uuid.UUID
	tenantID    uuid.UUID
	status      value_objects.ExecutionStatus
	triggerType value_objects.TriggerType
	triggerData value_objects.TriggerData
	input       map[string]interface{}
	output      interface{}
	errorMsg    string
	metrics     value_objects.ExecutionMetrics
	startedAt   time.Time
	completedAt *time.Time
}

func (e *execution) ID() uuid.UUID                                { return e.id }
func (e *execution) ScriptID() uuid.UUID                          { return e.scriptID }
func (e *execution) TenantID() uuid.UUID                          { return e.tenantID }
func (e *execution) Status() value_objects.ExecutionStatus        { return e.status }
func (e *execution) TriggerType() value_objects.TriggerType       { return e.triggerType }
func (e *execution) TriggerData() value_objects.TriggerData       { return e.triggerData }
func (e *execution) Input() map[string]interface{}               { return e.input }
func (e *execution) Output() interface{}                         { return e.output }
func (e *execution) Error() string                               { return e.errorMsg }
func (e *execution) Metrics() value_objects.ExecutionMetrics      { return e.metrics }
func (e *execution) StartedAt() time.Time                        { return e.startedAt }
func (e *execution) CompletedAt() *time.Time                     { return e.completedAt }

func (e *execution) SetStatus(status value_objects.ExecutionStatus) Execution {
	result := *e
	result.status = status
	return &result
}

func (e *execution) SetOutput(output interface{}) Execution {
	result := *e
	result.output = output
	return &result
}

func (e *execution) SetError(err string) Execution {
	result := *e
	result.errorMsg = err
	return &result
}

func (e *execution) SetMetrics(metrics value_objects.ExecutionMetrics) Execution {
	result := *e
	result.metrics = metrics
	return &result
}

func (e *execution) SetCompletedAt(t time.Time) Execution {
	result := *e
	result.completedAt = &t
	return &result
}

func (e *execution) IsRunning() bool {
	return e.status == value_objects.ExecutionStatusRunning
}

func (e *execution) IsCompleted() bool {
	return e.status == value_objects.ExecutionStatusCompleted
}

func (e *execution) IsFailed() bool {
	return e.status == value_objects.ExecutionStatusFailed ||
		e.status == value_objects.ExecutionStatusTimeout
}

func (e *execution) Duration() time.Duration {
	if e.completedAt != nil {
		return e.completedAt.Sub(e.startedAt)
	}
	return time.Since(e.startedAt)
}
```

### Version Entity

The Version entity provides an immutable audit trail of script source code changes.

```go
// modules/jsruntime/domain/entities/version/version.go
package version

import (
	"time"

	"github.com/google/uuid"
)

type Version interface {
	ID() uuid.UUID
	ScriptID() uuid.UUID
	TenantID() uuid.UUID
	VersionNumber() int
	Source() string
	ChangeDescription() string
	CreatedAt() time.Time
	CreatedBy() uint
}

type Option func(*version)

func WithID(id uuid.UUID) Option {
	return func(v *version) {
		v.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(v *version) {
		v.tenantID = tenantID
	}
}

func WithChangeDescription(desc string) Option {
	return func(v *version) {
		v.changeDescription = desc
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(v *version) {
		v.createdAt = t
	}
}

func WithCreatedBy(userID uint) Option {
	return func(v *version) {
		v.createdBy = userID
	}
}

func New(scriptID uuid.UUID, versionNumber int, source string, opts ...Option) (Version, error) {
	v := &version{
		id:            uuid.New(),
		scriptID:      scriptID,
		versionNumber: versionNumber,
		source:        source,
		createdAt:     time.Now(),
	}

	for _, opt := range opts {
		opt(v)
	}

	return v, nil
}

type version struct {
	id                uuid.UUID
	scriptID          uuid.UUID
	tenantID          uuid.UUID
	versionNumber     int
	source            string
	changeDescription string
	createdAt         time.Time
	createdBy         uint
}

func (v *version) ID() uuid.UUID            { return v.id }
func (v *version) ScriptID() uuid.UUID      { return v.scriptID }
func (v *version) TenantID() uuid.UUID      { return v.tenantID }
func (v *version) VersionNumber() int       { return v.versionNumber }
func (v *version) Source() string           { return v.source }
func (v *version) ChangeDescription() string { return v.changeDescription }
func (v *version) CreatedAt() time.Time     { return v.createdAt }
func (v *version) CreatedBy() uint          { return v.createdBy }
```

## Value Objects

### ScriptType Enum

```go
// modules/jsruntime/domain/value_objects/script_type.go
package value_objects

type ScriptType string

const (
	ScriptTypeScheduled ScriptType = "scheduled" // Cron-triggered
	ScriptTypeHTTP      ScriptType = "http"      // HTTP endpoint
	ScriptTypeEvent     ScriptType = "event"     // EventBus-triggered
	ScriptTypeOneOff    ScriptType = "oneoff"    // Manual execution
	ScriptTypeEmbedded  ScriptType = "embedded"  // Programmatic invocation
)

func (t ScriptType) IsValid() bool {
	switch t {
	case ScriptTypeScheduled, ScriptTypeHTTP, ScriptTypeEvent, ScriptTypeOneOff, ScriptTypeEmbedded:
		return true
	}
	return false
}
```

### ScriptStatus Enum

```go
// modules/jsruntime/domain/value_objects/script_status.go
package value_objects

type ScriptStatus string

const (
	ScriptStatusDraft    ScriptStatus = "draft"    // Being edited
	ScriptStatusActive   ScriptStatus = "active"   // Running and executable
	ScriptStatusPaused   ScriptStatus = "paused"   // Temporarily disabled
	ScriptStatusDisabled ScriptStatus = "disabled" // Permanently disabled
	ScriptStatusArchived ScriptStatus = "archived" // Historical record only
)

func (s ScriptStatus) IsValid() bool {
	switch s {
	case ScriptStatusDraft, ScriptStatusActive, ScriptStatusPaused, ScriptStatusDisabled, ScriptStatusArchived:
		return true
	}
	return false
}
```

### ExecutionStatus Enum

```go
// modules/jsruntime/domain/value_objects/execution_status.go
package value_objects

type ExecutionStatus string

const (
	ExecutionStatusPending   ExecutionStatus = "pending"   // Queued for execution
	ExecutionStatusRunning   ExecutionStatus = "running"   // Currently executing
	ExecutionStatusCompleted ExecutionStatus = "completed" // Successful completion
	ExecutionStatusFailed    ExecutionStatus = "failed"    // Error during execution
	ExecutionStatusTimeout   ExecutionStatus = "timeout"   // Exceeded time limit
	ExecutionStatusCancelled ExecutionStatus = "cancelled" // Manually stopped
)

func (s ExecutionStatus) IsValid() bool {
	switch s {
	case ExecutionStatusPending, ExecutionStatusRunning, ExecutionStatusCompleted,
		ExecutionStatusFailed, ExecutionStatusTimeout, ExecutionStatusCancelled:
		return true
	}
	return false
}
```

### TriggerType Enum

```go
// modules/jsruntime/domain/value_objects/trigger_type.go
package value_objects

type TriggerType string

const (
	TriggerTypeCron    TriggerType = "cron"    // Scheduled via cron
	TriggerTypeHTTP    TriggerType = "http"    // HTTP request
	TriggerTypeEvent   TriggerType = "event"   // Domain event
	TriggerTypeManual  TriggerType = "manual"  // User-initiated
	TriggerTypeAPI     TriggerType = "api"     // Programmatic call
)

func (t TriggerType) IsValid() bool {
	switch t {
	case TriggerTypeCron, TriggerTypeHTTP, TriggerTypeEvent, TriggerTypeManual, TriggerTypeAPI:
		return true
	}
	return false
}
```

### ResourceLimits Struct

```go
// modules/jsruntime/domain/value_objects/resource_limits.go
package value_objects

import "time"

type ResourceLimits struct {
	MaxExecutionTime     time.Duration // Max execution duration (default: 30s)
	MaxMemoryBytes       int64         // Max memory usage (default: 64MB)
	MaxConcurrentRuns    int           // Max parallel executions (default: 5)
	MaxAPICallsPerMinute int           // Rate limit for API calls (default: 60)
	MaxOutputSizeBytes   int64         // Max output size (default: 1MB)
}

func DefaultResourceLimits() ResourceLimits {
	return ResourceLimits{
		MaxExecutionTime:     30 * time.Second,
		MaxMemoryBytes:       64 * 1024 * 1024, // 64MB
		MaxConcurrentRuns:    5,
		MaxAPICallsPerMinute: 60,
		MaxOutputSizeBytes:   1024 * 1024, // 1MB
	}
}

func (r ResourceLimits) Validate() error {
	if r.MaxExecutionTime <= 0 {
		return serrors.E(serrors.KindValidation, "max execution time must be positive")
	}
	if r.MaxMemoryBytes <= 0 {
		return serrors.E(serrors.KindValidation, "max memory must be positive")
	}
	if r.MaxConcurrentRuns <= 0 {
		return serrors.E(serrors.KindValidation, "max concurrent runs must be positive")
	}
	return nil
}
```

### CronExpression Value Object

```go
// modules/jsruntime/domain/value_objects/cron_expression.go
package value_objects

import (
	"time"

	"github.com/robfig/cron/v3"
)

type CronExpression struct {
	expression string
	schedule   cron.Schedule
}

func NewCronExpression(expr string) (*CronExpression, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(expr)
	if err != nil {
		return nil, serrors.E(serrors.KindValidation, "invalid cron expression", err)
	}

	return &CronExpression{
		expression: expr,
		schedule:   schedule,
	}, nil
}

func (c *CronExpression) String() string {
	return c.expression
}

func (c *CronExpression) Next(t time.Time) time.Time {
	return c.schedule.Next(t)
}
```

### TriggerData Value Object

```go
// modules/jsruntime/domain/value_objects/trigger_data.go
package value_objects

type TriggerData struct {
	EventType   string                 `json:"event_type,omitempty"`
	HTTPMethod  string                 `json:"http_method,omitempty"`
	HTTPPath    string                 `json:"http_path,omitempty"`
	CronTrigger string                 `json:"cron_trigger,omitempty"`
	Payload     map[string]interface{} `json:"payload,omitempty"`
}
```

### ExecutionMetrics Struct

```go
// modules/jsruntime/domain/value_objects/execution_metrics.go
package value_objects

import "time"

type ExecutionMetrics struct {
	Duration          time.Duration `json:"duration"`
	MemoryUsedBytes   int64         `json:"memory_used_bytes"`
	APICallCount      int           `json:"api_call_count"`
	DatabaseQueryCount int          `json:"database_query_count"`
}
```

## Domain Events

```go
// modules/jsruntime/domain/events/script_events.go
package events

import (
	"time"

	"github.com/google/uuid"
)

type ScriptCreatedEvent struct {
	ScriptID       uuid.UUID `json:"script_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	Name           string    `json:"name"`
	Type           string    `json:"type"`
	CreatedBy      uint      `json:"created_by"`
	CreatedAt      time.Time `json:"created_at"`
}

func (e ScriptCreatedEvent) EventType() string {
	return "jsruntime.script.created"
}

type ScriptUpdatedEvent struct {
	ScriptID       uuid.UUID `json:"script_id"`
	TenantID       uuid.UUID `json:"tenant_id"`
	UpdatedBy      uint      `json:"updated_by"`
	UpdatedAt      time.Time `json:"updated_at"`
	VersionNumber  int       `json:"version_number"`
}

func (e ScriptUpdatedEvent) EventType() string {
	return "jsruntime.script.updated"
}

type ScriptDeletedEvent struct {
	ScriptID  uuid.UUID `json:"script_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	DeletedBy uint      `json:"deleted_by"`
	DeletedAt time.Time `json:"deleted_at"`
}

func (e ScriptDeletedEvent) EventType() string {
	return "jsruntime.script.deleted"
}

type ScriptStatusChangedEvent struct {
	ScriptID  uuid.UUID `json:"script_id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	OldStatus string    `json:"old_status"`
	NewStatus string    `json:"new_status"`
	ChangedBy uint      `json:"changed_by"`
	ChangedAt time.Time `json:"changed_at"`
}

func (e ScriptStatusChangedEvent) EventType() string {
	return "jsruntime.script.status_changed"
}
```

```go
// modules/jsruntime/domain/events/execution_events.go
package events

import (
	"time"

	"github.com/google/uuid"
)

type ExecutionStartedEvent struct {
	ExecutionID uuid.UUID `json:"execution_id"`
	ScriptID    uuid.UUID `json:"script_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	TriggerType string    `json:"trigger_type"`
	StartedAt   time.Time `json:"started_at"`
}

func (e ExecutionStartedEvent) EventType() string {
	return "jsruntime.execution.started"
}

type ExecutionCompletedEvent struct {
	ExecutionID uuid.UUID     `json:"execution_id"`
	ScriptID    uuid.UUID     `json:"script_id"`
	TenantID    uuid.UUID     `json:"tenant_id"`
	Duration    time.Duration `json:"duration"`
	CompletedAt time.Time     `json:"completed_at"`
}

func (e ExecutionCompletedEvent) EventType() string {
	return "jsruntime.execution.completed"
}

type ExecutionFailedEvent struct {
	ExecutionID uuid.UUID `json:"execution_id"`
	ScriptID    uuid.UUID `json:"script_id"`
	TenantID    uuid.UUID `json:"tenant_id"`
	Error       string    `json:"error"`
	FailedAt    time.Time `json:"failed_at"`
}

func (e ExecutionFailedEvent) EventType() string {
	return "jsruntime.execution.failed"
}

type ExecutionTimeoutEvent struct {
	ExecutionID uuid.UUID     `json:"execution_id"`
	ScriptID    uuid.UUID     `json:"script_id"`
	TenantID    uuid.UUID     `json:"tenant_id"`
	Duration    time.Duration `json:"duration"`
	TimeoutAt   time.Time     `json:"timeout_at"`
}

func (e ExecutionTimeoutEvent) EventType() string {
	return "jsruntime.execution.timeout"
}
```

## Acceptance Criteria

### Script Aggregate
- [ ] Implements all getter methods for fields
- [ ] All setters return new instance (immutability)
- [ ] `Validate()` enforces business rules (name, source, tenant ID required)
- [ ] Type-specific validation (cron for scheduled, path for HTTP, events for event-triggered)
- [ ] `CanExecute()` returns true only when status is Active
- [ ] Constructor uses functional options pattern
- [ ] Private struct, public interface
- [ ] No external dependencies in domain layer

### Execution Entity
- [ ] Tracks execution lifecycle (pending → running → completed/failed)
- [ ] Stores input, output, error, metrics
- [ ] Immutable setters for status, output, error, metrics
- [ ] `Duration()` calculates time from start to completion (or current time if running)
- [ ] Business methods for status checks (IsRunning, IsCompleted, IsFailed)

### Version Entity
- [ ] Immutable audit trail (no setters, only getters)
- [ ] Version number increments on each change
- [ ] Stores complete source code snapshot
- [ ] Change description for human-readable audit

### Value Objects
- [ ] ScriptType enum with 5 types (Scheduled, HTTP, Event, OneOff, Embedded)
- [ ] ScriptStatus enum with 5 states (Draft, Active, Paused, Disabled, Archived)
- [ ] ExecutionStatus enum with 6 states
- [ ] ResourceLimits with sensible defaults (30s timeout, 64MB memory, 5 concurrent)
- [ ] CronExpression validated using `robfig/cron` parser
- [ ] All enums implement `IsValid()` method

### Domain Events
- [ ] Events for script lifecycle (created, updated, deleted, status changed)
- [ ] Events for execution lifecycle (started, completed, failed, timeout)
- [ ] All events include tenant ID for multi-tenant event filtering
- [ ] Event types follow naming convention: `jsruntime.{entity}.{action}`
