package employee

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/email"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/tax"
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
	Email() email.Email
	Phone() string
	Salary() float64
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
