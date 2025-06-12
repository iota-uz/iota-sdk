package counterparty

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
)

type Option func(c *counterparty)

func WithID(id uuid.UUID) Option {
	return func(c *counterparty) {
		c.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(c *counterparty) {
		c.tenantID = tenantID
	}
}

func WithTin(tin tax.Tin) Option {
	return func(c *counterparty) {
		c.tin = tin
	}
}

func WithType(partyType Type) Option {
	return func(c *counterparty) {
		c.partyType = partyType
	}
}

func WithLegalType(legalType LegalType) Option {
	return func(c *counterparty) {
		c.legalType = legalType
	}
}

func WithLegalAddress(legalAddress string) Option {
	return func(c *counterparty) {
		c.legalAddress = legalAddress
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(c *counterparty) {
		c.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(c *counterparty) {
		c.updatedAt = updatedAt
	}
}

func New(
	name string,
	partyType Type,
	legalType LegalType,
	opts ...Option,
) Counterparty {
	c := &counterparty{
		id:        uuid.New(),
		tenantID:  uuid.Nil,
		tin:       tax.NilTin,
		name:      name,
		partyType: partyType,
		legalType: legalType,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}

	for _, opt := range opts {
		opt(c)
	}
	return c
}

type counterparty struct {
	id           uuid.UUID
	tenantID     uuid.UUID
	tin          tax.Tin
	name         string
	partyType    Type
	legalType    LegalType
	legalAddress string
	createdAt    time.Time
	updatedAt    time.Time
}

func (c *counterparty) ID() uuid.UUID {
	return c.id
}

func (c *counterparty) SetID(id uuid.UUID) {
	c.id = id
}

func (c *counterparty) TenantID() uuid.UUID {
	return c.tenantID
}

func (c *counterparty) Tin() tax.Tin {
	return c.tin
}

func (c *counterparty) SetTin(tin tax.Tin) {
	c.tin = tin
	c.updatedAt = time.Now()
}

func (c *counterparty) Name() string {
	return c.name
}

func (c *counterparty) SetName(name string) {
	c.name = name
	c.updatedAt = time.Now()
}

func (c *counterparty) Type() Type {
	return c.partyType
}

func (c *counterparty) SetType(partyType Type) {
	c.partyType = partyType
	c.updatedAt = time.Now()
}

func (c *counterparty) LegalType() LegalType {
	return c.legalType
}

func (c *counterparty) SetLegalType(legalType LegalType) {
	c.legalType = legalType
	c.updatedAt = time.Now()
}

func (c *counterparty) LegalAddress() string {
	return c.legalAddress
}

func (c *counterparty) SetLegalAddress(legalAddress string) {
	c.legalAddress = legalAddress
	c.updatedAt = time.Now()
}

func (c *counterparty) CreatedAt() time.Time {
	return c.createdAt
}

func (c *counterparty) UpdatedAt() time.Time {
	return c.updatedAt
}
