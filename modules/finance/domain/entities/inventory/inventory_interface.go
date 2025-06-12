package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"

	ut "github.com/go-playground/universal-translator"
)

type Inventory interface {
	ID() uuid.UUID
	SetID(id uuid.UUID)
	TenantID() uuid.UUID
	UpdateTenantID(id uuid.UUID) Inventory
	Name() string
	UpdateName(name string) Inventory
	Description() string
	UpdateDescription(description string) Inventory
	Price() *money.Money
	UpdatePrice(price *money.Money) Inventory
	Quantity() int
	UpdateQuantity(quantity int) Inventory
	CreatedAt() time.Time
	UpdatedAt() time.Time
	Ok(l ut.Translator) (map[string]string, bool)
}
