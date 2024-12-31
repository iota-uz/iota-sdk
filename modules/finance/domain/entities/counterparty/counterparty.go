package counterparty

import "time"

type Counterparty interface {
	ID() uint
	SetID(uint)

	TIN() string
	SetTIN(string)

	Name() string
	SetName(string)

	Type() Type
	SetType(Type)

	LegalType() LegalType
	SetLegalType(LegalType)

	LegalAddress() string
	SetLegalAddress(string)

	CreatedAt() time.Time
	UpdatedAt() time.Time
}
