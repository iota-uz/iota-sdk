package employee

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Option func(e *employee)

type Language interface {
	Primary() string
	Secondary() string
}

// --- Option setters ---

func WithID(id uint) Option {
	return func(e *employee) {
		e.id = id
	}
}

func WithTenantID(tenantID uuid.UUID) Option {
	return func(e *employee) {
		e.tenantID = tenantID
	}
}

func WithAvatarID(avatarID uint) Option {
	return func(e *employee) {
		e.avatarID = avatarID
	}
}

func WithBirthDate(birthDate time.Time) Option {
	return func(e *employee) {
		e.birthDate = birthDate
	}
}

func WithPassport(passport passport.Passport) Option {
	return func(e *employee) {
		e.passport = passport
	}
}

func WithResignationDate(resignationDate *time.Time) Option {
	return func(e *employee) {
		e.resignationDate = resignationDate
	}
}

func WithNotes(notes string) Option {
	return func(e *employee) {
		e.notes = notes
	}
}

func WithCreatedAt(createdAt time.Time) Option {
	return func(e *employee) {
		e.createdAt = createdAt
	}
}

func WithUpdatedAt(updatedAt time.Time) Option {
	return func(e *employee) {
		e.updatedAt = updatedAt
	}
}

type Employee interface {
	ID() uint
	TenantID() uuid.UUID
	FirstName() string
	LastName() string
	MiddleName() string
	Email() internet.Email
	Phone() string
	Salary() *money.Money
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
	UpdateName(firstName, lastName, middleName string) Employee
	MarkAsResigned(date time.Time) Employee

	// Timestamps
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

// --- Implementation ---

func NewWithID(
	id uint,
	tenantID uuid.UUID,
	firstName, lastName, middleName, phone string,
	email internet.Email,
	salary *money.Money,
	tin tax.Tin,
	pin tax.Pin,
	language Language,
	hireDate time.Time,
	opts ...Option,
) Employee {
	e := &employee{
		id:              id,
		tenantID:        tenantID,
		firstName:       firstName,
		lastName:        lastName,
		middleName:      middleName,
		email:           email,
		phone:           phone,
		salary:          salary,
		tin:             tin,
		pin:             pin,
		language:        language,
		avatarID:        0,
		hireDate:        hireDate,
		resignationDate: nil,
		notes:           "",
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func New(
	firstName, lastName, middleName, phone string,
	email internet.Email,
	salary *money.Money,
	tin tax.Tin,
	pin tax.Pin,
	language Language,
	hireDate time.Time,
	opts ...Option,
) Employee {
	e := &employee{
		id:              0,
		tenantID:        uuid.Nil, // Will be set in repository
		firstName:       firstName,
		lastName:        lastName,
		middleName:      middleName,
		email:           email,
		phone:           phone,
		salary:          salary,
		tin:             tin,
		pin:             pin,
		language:        language,
		avatarID:        0,
		hireDate:        hireDate,
		resignationDate: nil,
		notes:           "",
		createdAt:       time.Now(),
		updatedAt:       time.Now(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

type employee struct {
	id              uint
	tenantID        uuid.UUID
	firstName       string
	lastName        string
	middleName      string
	email           internet.Email
	phone           string
	salary          *money.Money
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

func (e *employee) TenantID() uuid.UUID {
	return e.tenantID
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

func (e *employee) Salary() *money.Money {
	return e.salary
}

func (e *employee) AvatarID() uint {
	return e.avatarID
}

func (e *employee) Email() internet.Email {
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

func (e *employee) UpdateName(firstName, lastName, middleName string) Employee {
	result := *e
	result.firstName = firstName
	result.lastName = lastName
	result.middleName = middleName
	result.updatedAt = time.Now()
	return &result
}

func (e *employee) MarkAsResigned(date time.Time) Employee {
	result := *e
	result.resignationDate = &date
	result.updatedAt = time.Now()
	return &result
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
