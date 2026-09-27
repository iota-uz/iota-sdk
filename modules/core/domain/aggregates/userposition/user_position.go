// Package userposition provides the user position aggregate: a tenant-scoped
// assignment of a user to a department with a multilingual title.
package userposition

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
)

// Status enumerates the lifecycle states a user position can be in.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

type Option func(p *userPosition)

// ---- Interface ----

// UserPosition assigns a user to a department within a tenant. A user may hold
// several positions; IsManager marks the position as a department head and
// IsPrimary marks the user's primary placement. Titles are multilingual.
type UserPosition interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	UserID() uint
	DepartmentID() uuid.UUID
	// Title resolves the position title in the requested locale, falling back
	// to the MultiLang default when the locale is missing.
	Title(locale string) string
	// TitleI18n returns the full multilingual title value.
	TitleI18n() models.MultiLang
	IsManager() bool
	IsPrimary() bool
	Status() Status
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetTitle(title models.MultiLang) UserPosition
	SetDepartmentID(departmentID uuid.UUID) UserPosition
	SetIsManager(isManager bool) UserPosition
	SetIsPrimary(isPrimary bool) UserPosition
	SetStatus(status Status) UserPosition
	SetTenantID(tenantID uuid.UUID) UserPosition
}

// ---- Options ----

func WithID(id uuid.UUID) Option {
	return func(p *userPosition) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *userPosition) {
		p.tenantID = tenantID
	}
}

func WithIsManager(isManager bool) Option {
	return func(p *userPosition) {
		p.isManager = isManager
	}
}

func WithIsPrimary(isPrimary bool) Option {
	return func(p *userPosition) {
		p.isPrimary = isPrimary
	}
}

func WithStatus(status Status) Option {
	return func(p *userPosition) {
		p.status = status
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(p *userPosition) {
		p.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(p *userPosition) {
		p.updatedAt = t
	}
}

// New constructs a UserPosition placing userID into departmentID with the given
// multilingual title.
func New(userID uint, departmentID uuid.UUID, title models.MultiLang, opts ...Option) UserPosition {
	p := &userPosition{
		id:           uuid.New(),
		tenantID:     uuid.Nil,
		userID:       userID,
		departmentID: departmentID,
		title:        title,
		isManager:    false,
		isPrimary:    false,
		status:       StatusActive,
		createdAt:    time.Now(),
		updatedAt:    time.Now(),
	}

	for _, opt := range opts {
		opt(p)
	}
	return p
}

// ---- Implementation ----

type userPosition struct {
	id           uuid.UUID
	tenantID     uuid.UUID
	userID       uint
	departmentID uuid.UUID
	title        models.MultiLang
	isManager    bool
	isPrimary    bool
	status       Status
	createdAt    time.Time
	updatedAt    time.Time
}

func (p *userPosition) ID() uuid.UUID {
	return p.id
}

func (p *userPosition) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *userPosition) UserID() uint {
	return p.userID
}

func (p *userPosition) DepartmentID() uuid.UUID {
	return p.departmentID
}

func (p *userPosition) Title(locale string) string {
	if p.title == nil {
		return ""
	}
	return p.title.GetWithFallback(locale)
}

func (p *userPosition) TitleI18n() models.MultiLang {
	return p.title
}

func (p *userPosition) IsManager() bool {
	return p.isManager
}

func (p *userPosition) IsPrimary() bool {
	return p.isPrimary
}

func (p *userPosition) Status() Status {
	return p.status
}

func (p *userPosition) CreatedAt() time.Time {
	return p.createdAt
}

func (p *userPosition) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *userPosition) SetTitle(title models.MultiLang) UserPosition {
	r := *p
	r.title = title
	r.updatedAt = time.Now()
	return &r
}

func (p *userPosition) SetDepartmentID(departmentID uuid.UUID) UserPosition {
	r := *p
	r.departmentID = departmentID
	r.updatedAt = time.Now()
	return &r
}

func (p *userPosition) SetIsManager(isManager bool) UserPosition {
	r := *p
	r.isManager = isManager
	r.updatedAt = time.Now()
	return &r
}

func (p *userPosition) SetIsPrimary(isPrimary bool) UserPosition {
	r := *p
	r.isPrimary = isPrimary
	r.updatedAt = time.Now()
	return &r
}

func (p *userPosition) SetStatus(status Status) UserPosition {
	r := *p
	r.status = status
	r.updatedAt = time.Now()
	return &r
}

func (p *userPosition) SetTenantID(tenantID uuid.UUID) UserPosition {
	r := *p
	r.tenantID = tenantID
	r.updatedAt = time.Now()
	return &r
}
