package employee

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/money"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"time"
)

type Language interface {
	Primary() string
	Secondary() string
}

// Employee represents an aggregate root entity in the domain.
type Employee interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Email() internet.Email
	Phone() string
	Salary() money.Amount
	AvatarID() uint
	HireDate() time.Time
	BirthDate() time.Time
	Language() Language
	Passport() passport.Passport
	Tin() tax.Tin
	Pin() tax.Pin
	Notes() string
	ResignationDate() *time.Time

	// Behavioral methods
	UpdateName(firstName, lastName, middleName string)
	MarkAsResigned(date time.Time)

	// Timestamps
	CreatedAt() time.Time
	UpdatedAt() time.Time
}
