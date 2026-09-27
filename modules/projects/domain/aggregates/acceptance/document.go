// Package acceptance holds the documents a client signs to accept the work
// done on a project: acts, delivery notes and the like.
package acceptance

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Kind string

const (
	KindAct          Kind = "ACT"
	KindDeliveryNote Kind = "DELIVERY_NOTE"
	KindOther        Kind = "OTHER"
)

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusSigned    Status = "SIGNED"
	StatusCancelled Status = "CANCELLED"
)

// Document is an acceptance document of a project. Only a signed document
// recognises revenue.
type Document interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	ProjectID() uuid.UUID
	Kind() Kind
	Number() string
	Date() time.Time
	Amount() *money.Money
	Status() Status
	Description() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	UpdateStatus(Status) Document
}

type Option func(d *document)

func WithID(id uuid.UUID) Option {
	return func(d *document) {
		d.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(d *document) {
		d.tenantID = tenantID
	}
}

func WithStatus(status Status) Option {
	return func(d *document) {
		d.status = status
	}
}

func WithDescription(description string) Option {
	return func(d *document) {
		d.description = description
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(d *document) {
		d.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(d *document) {
		d.updatedAt = updatedAt
	}
}

func New(projectID uuid.UUID, kind Kind, number string, date time.Time, amount *money.Money, opts ...Option) Document {
	d := &document{
		id:          uuid.New(),
		tenantID:    uuid.Nil,
		projectID:   projectID,
		kind:        kind,
		number:      number,
		date:        date,
		amount:      amount,
		status:      StatusDraft,
		description: "",
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type document struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	projectID   uuid.UUID
	kind        Kind
	number      string
	date        time.Time
	amount      *money.Money
	status      Status
	description string
	createdAt   time.Time
	updatedAt   time.Time
}

func (d *document) ID() uuid.UUID        { return d.id }
func (d *document) TenantID() uuid.UUID  { return d.tenantID }
func (d *document) ProjectID() uuid.UUID { return d.projectID }
func (d *document) Kind() Kind           { return d.kind }
func (d *document) Number() string       { return d.number }
func (d *document) Date() time.Time      { return d.date }
func (d *document) Amount() *money.Money { return d.amount }
func (d *document) Status() Status       { return d.status }
func (d *document) Description() string  { return d.description }
func (d *document) CreatedAt() time.Time { return d.createdAt }
func (d *document) UpdatedAt() time.Time { return d.updatedAt }

func (d *document) UpdateStatus(status Status) Document {
	res := *d
	res.status = status
	res.updatedAt = time.Now()
	return &res
}
