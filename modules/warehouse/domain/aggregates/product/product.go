package product

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
)

type Option func(p *product)

// --- Option setters ---

func WithID(id uint) Option {
	return func(p *product) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *product) {
		p.tenantID = tenantID
	}
}

func WithPosition(position position.Position) Option {
	return func(p *product) {
		p.position = position
		if position != nil {
			p.positionID = position.ID()
		}
	}
}

func WithPositionID(positionID uint) Option {
	return func(p *product) {
		p.positionID = positionID
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *product) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *product) {
		p.updatedAt = updatedAt
	}
}

// --- Interface ---

type Product interface {
	ID() uint
	TenantID() uuid.UUID
	PositionID() uint
	Rfid() string
	Status() Status
	Position() position.Position
	CreatedAt() time.Time
	UpdatedAt() time.Time

	Events() []interface{}

	SetStatus(status Status) Product
	SetPosition(position position.Position) Product
}

// --- Implementation ---

func New(rfid string, status Status, opts ...Option) Product {
	p := &product{
		id:         0,
		tenantID:   uuid.Nil,
		positionID: 0,
		rfid:       rfid,
		status:     status,
		position:   nil,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
		events:     make([]interface{}, 0),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type product struct {
	id         uint
	tenantID   uuid.UUID
	positionID uint
	rfid       string
	status     Status
	position   position.Position
	createdAt  time.Time
	updatedAt  time.Time
	events     []interface{}
}

func (p *product) ID() uint {
	return p.id
}

func (p *product) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *product) PositionID() uint {
	return p.positionID
}

func (p *product) Rfid() string {
	return p.rfid
}

func (p *product) Status() Status {
	return p.status
}

func (p *product) Position() position.Position {
	return p.position
}

func (p *product) CreatedAt() time.Time {
	return p.createdAt
}

func (p *product) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *product) Events() []interface{} {
	return p.events
}

func (p *product) SetStatus(status Status) Product {
	result := *p
	result.status = status
	result.updatedAt = time.Now()
	return &result
}

func (p *product) SetPosition(position position.Position) Product {
	result := *p
	result.position = position
	if position != nil {
		result.positionID = position.ID()
	}
	result.updatedAt = time.Now()
	return &result
}
