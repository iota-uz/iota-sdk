package counterparty

import "time"

func NewWithID(
	id uint,
	tin string,
	name string,
	partyType Type,
	legalType LegalType,
	legalAddress string,
	createdAt, updatedAt time.Time,
) Counterparty {
	return &counterparty{
		id:           id,
		tin:          tin,
		name:         name,
		partyType:    partyType,
		legalType:    legalType,
		legalAddress: legalAddress,
		createdAt:    createdAt,
		updatedAt:    updatedAt,
	}
}

func New(
	tin string,
	name string,
	partyType Type,
	legalType LegalType,
	legalAddress string,
) Counterparty {
	return NewWithID(
		0,
		tin,
		name,
		partyType,
		legalType,
		legalAddress,
		time.Now(),
		time.Now(),
	)
}

type counterparty struct {
	id           uint
	tin          string
	name         string
	partyType    Type
	legalType    LegalType
	legalAddress string
	createdAt    time.Time
	updatedAt    time.Time
}

func (c *counterparty) ID() uint {
	return c.id
}

func (c *counterparty) SetID(id uint) {
	c.id = id
}

func (c *counterparty) TIN() string {
	return c.tin
}

func (c *counterparty) SetTIN(tin string) {
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
