package tenant

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
)

// Tenant is the public interface for tenant entity
type Tenant interface {
	ID() uuid.UUID
	Name() string
	Domain() string
	IsActive() bool
	CreatedAt() time.Time
	UpdatedAt() time.Time
	LogoID() *int
	LogoCompactID() *int
	Phone() phone.Phone
	Email() internet.Email

	// Immutable setters (return new Tenant instance)
	SetLogoID(logoID *int) Tenant
	SetLogoCompactID(logoCompactID *int) Tenant
	SetPhone(p phone.Phone) Tenant
	SetEmail(e internet.Email) Tenant
}

// tenant is the private implementation
type tenant struct {
	id            uuid.UUID
	name          string
	domain        string
	phone         phone.Phone
	email         internet.Email
	isActive      bool
	logoID        *int
	logoCompactID *int
	createdAt     time.Time
	updatedAt     time.Time
}

type Option func(*tenant)

func WithID(id uuid.UUID) Option {
	return func(t *tenant) {
		t.id = id
	}
}

func WithDomain(domain string) Option {
	return func(t *tenant) {
		t.domain = domain
	}
}

func WithIsActive(isActive bool) Option {
	return func(t *tenant) {
		t.isActive = isActive
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(t *tenant) {
		t.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(t *tenant) {
		t.updatedAt = updatedAt
	}
}

func WithLogoID(logoID *int) Option {
	return func(t *tenant) {
		t.logoID = logoID
	}
}

func WithLogoCompactID(logoCompactID *int) Option {
	return func(t *tenant) {
		t.logoCompactID = logoCompactID
	}
}

func WithPhone(p phone.Phone) Option {
	return func(t *tenant) {
		t.phone = p
	}
}

func WithEmail(e internet.Email) Option {
	return func(t *tenant) {
		t.email = e
	}
}

func New(name string, opts ...Option) Tenant {
	t := &tenant{
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

func (t *tenant) ID() uuid.UUID {
	return t.id
}

func (t *tenant) Name() string {
	return t.name
}

func (t *tenant) Domain() string {
	return t.domain
}

func (t *tenant) IsActive() bool {
	return t.isActive
}

func (t *tenant) CreatedAt() time.Time {
	return t.createdAt
}

func (t *tenant) UpdatedAt() time.Time {
	return t.updatedAt
}

func (t *tenant) LogoID() *int {
	return t.logoID
}

func (t *tenant) LogoCompactID() *int {
	return t.logoCompactID
}

func (t *tenant) Phone() phone.Phone {
	return t.phone
}

func (t *tenant) Email() internet.Email {
	return t.email
}

// SetLogoID returns a new Tenant with updated logoID (immutable)
func (t *tenant) SetLogoID(logoID *int) Tenant {
	return &tenant{
		id:            t.id,
		name:          t.name,
		domain:        t.domain,
		phone:         t.phone,
		email:         t.email,
		isActive:      t.isActive,
		logoID:        logoID,
		logoCompactID: t.logoCompactID,
		createdAt:     t.createdAt,
		updatedAt:     time.Now(),
	}
}

// SetLogoCompactID returns a new Tenant with updated logoCompactID (immutable)
func (t *tenant) SetLogoCompactID(logoCompactID *int) Tenant {
	return &tenant{
		id:            t.id,
		name:          t.name,
		domain:        t.domain,
		phone:         t.phone,
		email:         t.email,
		isActive:      t.isActive,
		logoID:        t.logoID,
		logoCompactID: logoCompactID,
		createdAt:     t.createdAt,
		updatedAt:     time.Now(),
	}
}

// SetPhone returns a new Tenant with updated phone (immutable)
func (t *tenant) SetPhone(p phone.Phone) Tenant {
	return &tenant{
		id:            t.id,
		name:          t.name,
		domain:        t.domain,
		phone:         p,
		email:         t.email,
		isActive:      t.isActive,
		logoID:        t.logoID,
		logoCompactID: t.logoCompactID,
		createdAt:     t.createdAt,
		updatedAt:     time.Now(),
	}
}

// SetEmail returns a new Tenant with updated email (immutable)
func (t *tenant) SetEmail(e internet.Email) Tenant {
	return &tenant{
		id:            t.id,
		name:          t.name,
		domain:        t.domain,
		phone:         t.phone,
		email:         e,
		isActive:      t.isActive,
		logoID:        t.logoID,
		logoCompactID: t.logoCompactID,
		createdAt:     t.createdAt,
		updatedAt:     time.Now(),
	}
}
