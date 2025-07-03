package position

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
)

type Option func(p *position)

// --- Option setters ---

func WithID(id uint) Option {
	return func(p *position) {
		p.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(p *position) {
		p.tenantID = tenantID
	}
}

func WithUnit(unit *unit.Unit) Option {
	return func(p *position) {
		p.unit = unit
		if unit != nil {
			p.unitID = unit.ID
		}
	}
}

func WithUnitID(unitID uint) Option {
	return func(p *position) {
		p.unitID = unitID
	}
}

func WithInStock(inStock uint) Option {
	return func(p *position) {
		p.inStock = inStock
	}
}

func WithImages(images []upload.Upload) Option {
	return func(p *position) {
		p.images = images
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(p *position) {
		p.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(p *position) {
		p.updatedAt = updatedAt
	}
}

// --- Interface ---

type Position interface {
	ID() uint
	TenantID() uuid.UUID
	Title() string
	Barcode() string
	UnitID() uint
	Unit() *unit.Unit
	InStock() uint
	Images() []upload.Upload
	CreatedAt() time.Time
	UpdatedAt() time.Time

	Events() []interface{}

	SetTitle(title string) Position
	SetBarcode(barcode string) Position
	SetUnit(unit *unit.Unit) Position
	SetInStock(inStock uint) Position
	SetImages(images []upload.Upload) Position
}

// --- Implementation ---

func New(title, barcode string, opts ...Option) Position {
	p := &position{
		id:        0,
		tenantID:  uuid.Nil,
		title:     title,
		barcode:   barcode,
		unitID:    0,
		unit:      nil,
		inStock:   0,
		images:    make([]upload.Upload, 0),
		createdAt: time.Now(),
		updatedAt: time.Now(),
		events:    make([]interface{}, 0),
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

type position struct {
	id        uint
	tenantID  uuid.UUID
	title     string
	barcode   string
	unitID    uint
	unit      *unit.Unit
	inStock   uint
	images    []upload.Upload
	createdAt time.Time
	updatedAt time.Time
	events    []interface{}
}

func (p *position) ID() uint {
	return p.id
}

func (p *position) TenantID() uuid.UUID {
	return p.tenantID
}

func (p *position) Title() string {
	return p.title
}

func (p *position) Barcode() string {
	return p.barcode
}

func (p *position) UnitID() uint {
	return p.unitID
}

func (p *position) Unit() *unit.Unit {
	return p.unit
}

func (p *position) InStock() uint {
	return p.inStock
}

func (p *position) Images() []upload.Upload {
	return p.images
}

func (p *position) CreatedAt() time.Time {
	return p.createdAt
}

func (p *position) UpdatedAt() time.Time {
	return p.updatedAt
}

func (p *position) Events() []interface{} {
	return p.events
}

func (p *position) SetTitle(title string) Position {
	result := *p
	result.title = title
	result.updatedAt = time.Now()
	return &result
}

func (p *position) SetBarcode(barcode string) Position {
	result := *p
	result.barcode = barcode
	result.updatedAt = time.Now()
	return &result
}

func (p *position) SetUnit(unit *unit.Unit) Position {
	result := *p
	result.unit = unit
	if unit != nil {
		result.unitID = unit.ID
	}
	result.updatedAt = time.Now()
	return &result
}

func (p *position) SetInStock(inStock uint) Position {
	result := *p
	result.inStock = inStock
	result.updatedAt = time.Now()
	return &result
}

func (p *position) SetImages(images []upload.Upload) Position {
	result := *p
	result.images = images
	result.updatedAt = time.Now()
	return &result
}
