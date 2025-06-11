package counterparty

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
)

type Counterparty interface {
	ID() uuid.UUID
	SetID(uuid.UUID)

	Tin() tax.Tin
	SetTin(t tax.Tin)

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
