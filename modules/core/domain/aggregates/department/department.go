// Package department provides the department aggregate: a tenant-scoped,
// self-referential organizational unit with a multilingual name.
package department

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/crud/models"
)

// Status enumerates the lifecycle states a department can be in.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

type Option func(d *department)

// ---- Interface ----

// Department is a tenant-scoped organizational unit. Names are stored as a
// MultiLang jsonb value so each tenant can localise them independently.
// Departments may form a hierarchy through ParentID.
type Department interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	ParentID() *uuid.UUID
	Code() string
	// Name resolves the department name in the requested locale, falling back
	// to the MultiLang default when the locale is missing.
	Name(locale string) string
	// NameI18n returns the full multilingual name value.
	NameI18n() models.MultiLang
	Order() int
	Status() Status
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetCode(code string) Department
	SetName(name models.MultiLang) Department
	SetParentID(parentID *uuid.UUID) Department
	SetOrder(order int) Department
	SetStatus(status Status) Department
	SetTenantID(tenantID uuid.UUID) Department
}

// ---- Options ----

func WithID(id uuid.UUID) Option {
	return func(d *department) {
		d.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(d *department) {
		d.tenantID = tenantID
	}
}

func WithParentID(parentID *uuid.UUID) Option {
	return func(d *department) {
		d.parentID = parentID
	}
}

func WithOrder(order int) Option {
	return func(d *department) {
		d.order = order
	}
}

func WithStatus(status Status) Option {
	return func(d *department) {
		d.status = status
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(d *department) {
		d.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(d *department) {
		d.updatedAt = t
	}
}

// New constructs a Department. code is the tenant-unique identifier and name is
// the multilingual display name.
func New(code string, name models.MultiLang, opts ...Option) Department {
	d := &department{
		id:        uuid.New(),
		tenantID:  uuid.Nil,
		parentID:  nil,
		code:      code,
		name:      name,
		order:     0,
		status:    StatusActive,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(d)
	}
	return d
}

// ---- Implementation ----

type department struct {
	id        uuid.UUID
	tenantID  uuid.UUID
	parentID  *uuid.UUID
	code      string
	name      models.MultiLang
	order     int
	status    Status
	createdAt time.Time
	updatedAt time.Time
}

func (d *department) ID() uuid.UUID {
	return d.id
}

func (d *department) TenantID() uuid.UUID {
	return d.tenantID
}

func (d *department) ParentID() *uuid.UUID {
	return d.parentID
}

func (d *department) Code() string {
	return d.code
}

func (d *department) Name(locale string) string {
	if d.name == nil {
		return ""
	}
	return d.name.GetWithFallback(locale)
}

func (d *department) NameI18n() models.MultiLang {
	return d.name
}

func (d *department) Order() int {
	return d.order
}

func (d *department) Status() Status {
	return d.status
}

func (d *department) CreatedAt() time.Time {
	return d.createdAt
}

func (d *department) UpdatedAt() time.Time {
	return d.updatedAt
}

func (d *department) SetCode(code string) Department {
	r := *d
	r.code = code
	r.updatedAt = time.Now()
	return &r
}

func (d *department) SetName(name models.MultiLang) Department {
	r := *d
	r.name = name
	r.updatedAt = time.Now()
	return &r
}

func (d *department) SetParentID(parentID *uuid.UUID) Department {
	r := *d
	r.parentID = parentID
	r.updatedAt = time.Now()
	return &r
}

func (d *department) SetOrder(order int) Department {
	r := *d
	r.order = order
	r.updatedAt = time.Now()
	return &r
}

func (d *department) SetStatus(status Status) Department {
	r := *d
	r.status = status
	r.updatedAt = time.Now()
	return &r
}

func (d *department) SetTenantID(tenantID uuid.UUID) Department {
	r := *d
	r.tenantID = tenantID
	r.updatedAt = time.Now()
	return &r
}
