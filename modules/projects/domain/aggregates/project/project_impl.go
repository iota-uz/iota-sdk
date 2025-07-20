package project

import (
	"time"

	"github.com/google/uuid"
)

type Option func(p *project)

func WithID(id uuid.UUID) Option {
	return func(p *project) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *project) {
		p.tenantID = tenantID
	}
}

func WithCounterpartyID(counterpartyID uuid.UUID) Option {
	return func(p *project) {
		p.counterpartyID = counterpartyID
	}
}

func WithDescription(description string) Option {
	return func(p *project) {
		p.description = description
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *project) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *project) {
		p.updatedAt = updatedAt
	}
}

func New(
	name string,
	counterpartyID uuid.UUID,
	opts ...Option,
) Project {
	p := &project{
		id:             uuid.New(),
		tenantID:       uuid.Nil,
		counterpartyID: counterpartyID,
		name:           name,
		description:    "",
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

type project struct {
	id             uuid.UUID
	tenantID       uuid.UUID
	counterpartyID uuid.UUID
	name           string
	description    string
	createdAt      time.Time
	updatedAt      time.Time
}

func (p *project) ID() uuid.UUID {
	return p.id
}

func (p *project) SetID(id uuid.UUID) {
	p.id = id
}

func (p *project) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *project) CounterpartyID() uuid.UUID {
	return p.counterpartyID
}

func (p *project) UpdateCounterpartyID(counterpartyID uuid.UUID) Project {
	res := *p
	res.counterpartyID = counterpartyID
	res.updatedAt = time.Now()
	return &res
}

func (p *project) Name() string {
	return p.name
}

func (p *project) UpdateName(name string) Project {
	res := *p
	res.name = name
	res.updatedAt = time.Now()
	return &res
}

func (p *project) Description() string {
	return p.description
}

func (p *project) UpdateDescription(description string) Project {
	res := *p
	res.description = description
	res.updatedAt = time.Now()
	return &res
}

func (p *project) CreatedAt() time.Time {
	return p.createdAt
}

func (p *project) UpdatedAt() time.Time {
	return p.updatedAt
}
