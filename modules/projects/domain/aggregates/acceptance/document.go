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
	KindProtocol     Kind = "PROTOCOL"
	KindOther        Kind = "OTHER"
)

// KindOption is a kind a form offers. Built-in kinds are labelled by the
// translation Projects.Acceptance.Kinds._<value>; an application adding its own
// kind sets Label instead.
type KindOption struct {
	Value Kind
	// Label is shown as is when it is set.
	Label string
}

// DefaultKinds are the kinds every installation offers.
func DefaultKinds() []KindOption {
	return []KindOption{
		{Value: KindAct, Label: ""},
		{Value: KindDeliveryNote, Label: ""},
		{Value: KindProtocol, Label: ""},
		{Value: KindOther, Label: ""},
	}
}

// ValidKind reports whether the value can be stored as a kind: upper-case
// letters, digits and underscores, starting with a letter.
func ValidKind(kind Kind) bool {
	value := string(kind)
	if value == "" || len(value) > 32 || value[0] < 'A' || value[0] > 'Z' {
		return false
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '_' {
			return false
		}
	}
	return true
}

type Status string

const (
	StatusDraft     Status = "DRAFT"
	StatusSigned    Status = "SIGNED"
	StatusCancelled Status = "CANCELLED"
)

// Document is an acceptance document of a project. Only a signed document
// recognises revenue. SignedOn is the day the client signed it; it is zero
// until then.
type Document interface {
	ID() uuid.UUID
	TenantID() uuid.UUID
	ProjectID() uuid.UUID
	Kind() Kind
	Number() string
	Date() time.Time
	SignedOn() time.Time
	Amount() *money.Money
	Status() Status
	Description() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	// UpdateStatus moves the document to the status. Signing a document that
	// has no signing day yet signs it today.
	UpdateStatus(Status) Document
	// Revise replaces what a person edits. The signing day decides the status
	// of a document that is not cancelled: a day signs it, none makes it a
	// draft again.
	Revise(Revision) Document
}

// Revision is what a person can change in a document.
type Revision struct {
	Kind        Kind
	Number      string
	Date        time.Time
	SignedOn    time.Time
	Amount      *money.Money
	Description string
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

// WithSignedOn sets the signing day. A draft with a signing day is signed.
func WithSignedOn(signedOn time.Time) Option {
	return func(d *document) {
		d.signedOn = signedOn
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
		signedOn:    time.Time{},
		amount:      amount,
		status:      StatusDraft,
		description: "",
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(d)
	}
	switch {
	case d.status == StatusDraft && !d.signedOn.IsZero():
		d.status = StatusSigned
	case d.status == StatusSigned && d.signedOn.IsZero():
		d.signedOn = d.date
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
	signedOn    time.Time
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
func (d *document) SignedOn() time.Time  { return d.signedOn }
func (d *document) Amount() *money.Money { return d.amount }
func (d *document) Status() Status       { return d.status }
func (d *document) Description() string  { return d.description }
func (d *document) CreatedAt() time.Time { return d.createdAt }
func (d *document) UpdatedAt() time.Time { return d.updatedAt }

func (d *document) UpdateStatus(status Status) Document {
	res := *d
	res.status = status
	if status == StatusSigned && res.signedOn.IsZero() {
		res.signedOn = today()
	}
	res.updatedAt = time.Now()
	return &res
}

func (d *document) Revise(revision Revision) Document {
	res := *d
	res.kind = revision.Kind
	res.number = revision.Number
	res.date = revision.Date
	res.signedOn = revision.SignedOn
	res.amount = revision.Amount
	res.description = revision.Description
	if res.status != StatusCancelled {
		res.status = StatusDraft
		if !revision.SignedOn.IsZero() {
			res.status = StatusSigned
		}
	}
	res.updatedAt = time.Now()
	return &res
}

// today is the current day as a date without a time of day.
func today() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}
