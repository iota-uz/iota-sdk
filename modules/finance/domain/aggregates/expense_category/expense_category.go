package category

import (
	"time"

	"github.com/google/uuid"
)

type Option func(e *expenseCategory)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(e *expenseCategory) {
		e.id = id
	}
}

func WithName(name string) Option {
	return func(e *expenseCategory) {
		e.name = name
	}
}

func WithDescription(description string) Option {
	return func(e *expenseCategory) {
		e.description = description
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(e *expenseCategory) {
		e.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(e *expenseCategory) {
		e.updatedAt = updatedAt
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(e *expenseCategory) {
		e.tenantID = tenantID
	}
}

// Interface
type ExpenseCategory interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	Name() string
	Description() string
	CreatedAt() time.Time
	UpdatedAt() time.Time
	UpdateName(name string) ExpenseCategory
	UpdateDescription(description string) ExpenseCategory
}

// Implementation
func New(
	name string,
	opts ...Option,
) ExpenseCategory {
	e := &expenseCategory{
		id:          uuid.New(),
		tenantID:    uuid.Nil,
		name:        name,
		description: "", // description is optional
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type expenseCategory struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	name        string
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

func (e *expenseCategory) ID() uuid.UUID {
	return e.id
}

func (e *expenseCategory) TenantID() uuid.UUID {
	return e.tenantID
}

func (e *expenseCategory) Name() string {
	return e.name
}

func (e *expenseCategory) Description() string {
	return e.description
}

func (e *expenseCategory) CreatedAt() time.Time {
	return e.createdAt
}

func (e *expenseCategory) UpdatedAt() time.Time {
	return e.updatedAt
}

// Update methods
func (e *expenseCategory) UpdateName(name string) ExpenseCategory {
	result := *e
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (e *expenseCategory) UpdateDescription(description string) ExpenseCategory {
	result := *e
	result.description = description
	result.updatedAt = time.Now()
	return &result
}
