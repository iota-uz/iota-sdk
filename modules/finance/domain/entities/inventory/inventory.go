package inventory

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"

	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

type Option func(i *inventory)

// Option setters
func WithID(id uuid.UUID) Option {
	return func(i *inventory) {
		i.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(i *inventory) {
		i.tenantID = tenantID
	}
}

func WithName(name string) Option {
	return func(i *inventory) {
		i.name = name
	}
}

func WithDescription(description string) Option {
	return func(i *inventory) {
		i.description = description
	}
}

func WithPrice(price *money.Money) Option {
	return func(i *inventory) {
		i.price = price
	}
}

func WithQuantity(quantity int) Option {
	return func(i *inventory) {
		i.quantity = quantity
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(i *inventory) {
		i.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(i *inventory) {
		i.updatedAt = updatedAt
	}
}

func New(
	name string,
	price *money.Money,
	quantity int,
	opts ...Option,
) Inventory {
	i := &inventory{
		id:          uuid.Nil,
		tenantID:    uuid.Nil,
		name:        name,
		description: "",
		price:       price,
		quantity:    quantity,
		createdAt:   time.Now(),
		updatedAt:   time.Now(),
	}
	for _, opt := range opts {
		opt(i)
	}
	return i
}

type inventory struct {
	id          uuid.UUID
	tenantID    uuid.UUID
	name        string
	description string
	price       *money.Money
	quantity    int
	createdAt   time.Time
	updatedAt   time.Time
}

func (i *inventory) ID() uuid.UUID {
	return i.id
}

func (i *inventory) SetID(id uuid.UUID) {
	i.id = id
}

func (i *inventory) TenantID() uuid.UUID {
	return i.tenantID
}

func (i *inventory) UpdateTenantID(id uuid.UUID) Inventory {
	result := *i
	result.tenantID = id
	return &result
}

func (i *inventory) Name() string {
	return i.name
}

func (i *inventory) UpdateName(name string) Inventory {
	result := *i
	result.name = name
	return &result
}

func (i *inventory) Description() string {
	return i.description
}

func (i *inventory) UpdateDescription(description string) Inventory {
	result := *i
	result.description = description
	return &result
}

func (i *inventory) Price() *money.Money {
	return i.price
}

func (i *inventory) UpdatePrice(price *money.Money) Inventory {
	result := *i
	result.price = price
	return &result
}

func (i *inventory) Quantity() int {
	return i.quantity
}

func (i *inventory) UpdateQuantity(quantity int) Inventory {
	result := *i
	result.quantity = quantity
	return &result
}

func (i *inventory) CreatedAt() time.Time {
	return i.createdAt
}

func (i *inventory) UpdatedAt() time.Time {
	return i.updatedAt
}

func (i *inventory) Ok(l ut.Translator) (map[string]string, bool) {
	errors := map[string]string{}
	errs := constants.Validate.Struct(i)
	if errs == nil {
		return errors, true
	}
	for _, err := range errs.(validator.ValidationErrors) {
		errors[err.Field()] = err.Translate(l)
	}
	return errors, len(errors) == 0
}
