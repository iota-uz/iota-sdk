package tenant

import (
	"time"

	"github.com/google/uuid"
)

type Tenant struct {
	id            uuid.UUID
	name          string
	domain        string
	isActive      bool
	logoID        *int
	logoCompactID *int
	createdAt     time.Time
	updatedAt     time.Time
}

type Option func(*Tenant)

func WithID(id uuid.UUID) Option {
	return func(t *Tenant) {
		t.id = id
	}
}

func WithDomain(domain string) Option {
	return func(t *Tenant) {
		t.domain = domain
	}
}

func WithIsActive(isActive bool) Option {
	return func(t *Tenant) {
		t.isActive = isActive
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(t *Tenant) {
		t.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(t *Tenant) {
		t.updatedAt = updatedAt
	}
}

func WithLogoID(logoID *int) Option {
	return func(t *Tenant) {
		t.logoID = logoID
	}
}

func WithLogoCompactID(logoCompactID *int) Option {
	return func(t *Tenant) {
		t.logoCompactID = logoCompactID
	}
}

func New(name string, opts ...Option) *Tenant {
	t := &Tenant{
		id:        uuid.New(),
		name:      name,
		isActive:  true,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
	for _, opt := range opts {
		opt(t)
	}
	return t
}

func (t *Tenant) ID() uuid.UUID {
	return t.id
}

func (t *Tenant) Name() string {
	return t.name
}

func (t *Tenant) Domain() string {
	return t.domain
}

func (t *Tenant) IsActive() bool {
	return t.isActive
}

func (t *Tenant) CreatedAt() time.Time {
	return t.createdAt
}

func (t *Tenant) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t *Tenant) LogoID() *int {
	return t.logoID
}

func (t *Tenant) LogoCompactID() *int {
	return t.logoCompactID
}

func (t *Tenant) SetLogoID(logoID *int) {
	t.logoID = logoID
	t.updatedAt = time.Now()
}

func (t *Tenant) SetLogoCompactID(logoCompactID *int) {
	t.logoCompactID = logoCompactID
	t.updatedAt = time.Now()
}
