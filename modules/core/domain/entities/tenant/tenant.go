package tenant

import "time"

type Tenant struct {
	id        uint
	name      string
	domain    string
	isActive  bool
	createdAt time.Time
	updatedAt time.Time
}

type Option func(*Tenant)

func WithID(id uint) Option {
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

func New(name string, opts ...Option) *Tenant {
	t := &Tenant{
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

func (t *Tenant) ID() uint {
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
