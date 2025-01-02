package employee

import (
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/money"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/email"
)

func NewWithID(
	id uint,
	firstName, lastName, middleName, phone string,
	email email.Email,
	salary money.Amount,
	tin tax.Tin,
	pin tax.Pin,
	language Language,
	hireDate time.Time,
	resignationDate *time.Time,
	avatarID uint,
	notes string,
	createdAt, updatedAt time.Time,
) Employee {
	return &employee{
		id:              id,
		firstName:       firstName,
		lastName:        lastName,
		middleName:      middleName,
		email:           email,
		phone:           phone,
		salary:          salary,
		tin:             tin,
		pin:             pin,
		language:        language,
		avatarID:        avatarID,
		hireDate:        hireDate,
		resignationDate: resignationDate,
		notes:           notes,
		createdAt:       createdAt,
		updatedAt:       updatedAt,
	}
}

func New(
	firstName, lastName, middleName, phone string,
	email email.Email,
	salary money.Amount,
	tin tax.Tin,
	pin tax.Pin,
	language Language,
	hireDate time.Time,
	resignationDate *time.Time,
	avatarID uint,
	notes string,
) (Employee, error) {
	return &employee{
		id:              0,
		firstName:       firstName,
		lastName:        lastName,
		middleName:      middleName,
		email:           email,
		phone:           phone,
		salary:          salary,
		tin:             tin,
		pin:             pin,
		language:        language,
		avatarID:        avatarID,
		hireDate:        hireDate,
		resignationDate: resignationDate,
		notes:           notes,
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
	}, nil
}

type employee struct {
	id              uint
	firstName       string
	lastName        string
	middleName      string
	email           email.Email
	phone           string
	salary          money.Amount
	avatarID        uint
	language        Language
	tin             tax.Tin
	pin             tax.Pin
	passport        passport.Passport
	birthDate       time.Time
	hireDate        time.Time
	resignationDate *time.Time
	notes           string
	createdAt       time.Time
	updatedAt       time.Time
}

func (e *employee) ID() uint {
	return e.id
}

func (e *employee) FirstName() string {
	return e.firstName
}

func (e *employee) LastName() string {
	return e.lastName
}

func (e *employee) MiddleName() string {
	return e.middleName
}

func (e *employee) Phone() string {
	return e.phone
}

func (e *employee) Salary() money.Amount {
	return e.salary
}

func (e *employee) AvatarID() uint {
	return e.avatarID
}

func (e *employee) Email() email.Email {
	return e.email
}

func (e *employee) BirthDate() time.Time {
	return e.birthDate
}

func (e *employee) Language() Language {
	return e.language
}

func (e *employee) Tin() tax.Tin {
	return e.tin
}

func (e *employee) Pin() tax.Pin {
	return e.pin
}

func (e *employee) Passport() passport.Passport {
	return e.passport
}

func (e *employee) HireDate() time.Time {
	return e.hireDate
}

func (e *employee) ResignationDate() *time.Time {
	return e.resignationDate
}

func (e *employee) UpdateName(firstName, lastName, middleName string) {
	e.firstName = firstName
	e.lastName = lastName
	e.middleName = middleName
	e.updatedAt = time.Now()
}

func (e *employee) MarkAsResigned(date time.Time) {
	e.resignationDate = &date
	e.updatedAt = time.Now()
}

func (e *employee) Notes() string {
	return e.notes
}

func (e *employee) CreatedAt() time.Time {
	return e.createdAt
}

func (e *employee) UpdatedAt() time.Time {
	return e.updatedAt
}
