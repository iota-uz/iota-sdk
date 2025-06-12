package payment_category

import (
	"time"

	"github.com/google/uuid"
)

type Option func(p *paymentCategory)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(p *paymentCategory) {
		p.id = id
	}
}

func WithName(name string) Option {
	return func(p *paymentCategory) {
		p.name = name
	}
}

func WithDescription(description string) Option {
	return func(p *paymentCategory) {
		p.description = description
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *paymentCategory) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *paymentCategory) {
		p.updatedAt = updatedAt
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *paymentCategory) {
		p.tenantID = tenantID
	}
}

// Interface
type PaymentCategory interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	Name() string
	Description() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	UpdateName(name string) PaymentCategory
	UpdateDescription(description string) PaymentCategory
}

// Implementation
func New(
	name string,
	opts ...Option,
) PaymentCategory {
	p := &paymentCategory{
		id:          uuid.New(),
		tenantID:    uuid.Nil,
		name:        name,
		description: "", // description is optional
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type paymentCategory struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	name        string
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

func (p *paymentCategory) ID() uuid.UUID {
	return p.id
}

func (p *paymentCategory) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *paymentCategory) Name() string {
	return p.name
}

func (p *paymentCategory) Description() string {
	return p.description
}

func (p *paymentCategory) CreatedAt() time.Time {
	return p.createdAt
}

func (p *paymentCategory) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *paymentCategory) UpdateName(name string) PaymentCategory {
	result := *p
	result.name = name
	result.updatedAt = time.Now()
	return &result
}

func (p *paymentCategory) UpdateDescription(description string) PaymentCategory {
	result := *p
	result.description = description
	result.updatedAt = time.Now()
	return &result
}
