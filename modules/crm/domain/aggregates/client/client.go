package client

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/passport"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/general"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/internet"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/tax"
)

type Option func(c *client)

// --- Option setters ---

func WithID(id uint) Option {
	return func(c *client) {
		c.id = id
	}
}

func WithAddress(address string) Option {
	return func(c *client) {
		c.address = address
	}
}

func WithEmail(email internet.Email) Option {
	return func(c *client) {
		c.email = email
	}
}

func WithDateOfBirth(dob *time.Time) Option {
	return func(c *client) {
		c.dateOfBirth = dob
	}
}

func WithPassport(p passport.Passport) Option {
	return func(c *client) {
		c.passport = p
	}
}

func WithPin(pin tax.Pin) Option {
	return func(c *client) {
		c.pin = pin
	}
}

func WithGender(g general.Gender) Option {
	return func(c *client) {
		c.gender = g
	}
}

func WithCreatedAt(t time.Time) Option {
	return func(c *client) {
		c.createdAt = t
	}
}

func WithUpdatedAt(t time.Time) Option {
	return func(c *client) {
		c.updatedAt = t
	}
}

// --- Interface ---

type Client interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Phone() phone.Phone
	Address() string
	Email() internet.Email
	DateOfBirth() *time.Time
	Gender() general.Gender
	Passport() passport.Passport
	PIN() tax.Pin
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetPhone(number phone.Phone) Client
	SetName(firstName, lastName, middleName string) Client
	SetAddress(address string) Client
	SetEmail(email internet.Email) Client
	SetDateOfBirth(dob *time.Time) Client
	SetGender(gender general.Gender) Client
	SetPassport(p passport.Passport) Client
	SetPIN(pin tax.Pin) Client
}

func New(
	firstName, lastName, middleName string,
	phoneNumber phone.Phone,
	opts ...Option,
) (Client, error) {
	c := &client{
		id:         0,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		phone:      phoneNumber,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

type client struct {
	id          uint
	firstName   string
	lastName    string
	middleName  string
	phone       phone.Phone
	address     string
	email       internet.Email
	dateOfBirth *time.Time
	gender      general.Gender
	passport    passport.Passport
	pin         tax.Pin
	createdAt   time.Time
	updatedAt   time.Time
}

func (c *client) ID() uint {
	return c.id
}

func (c *client) FirstName() string {
	return c.firstName
}

func (c *client) LastName() string {
	return c.lastName
}

func (c *client) MiddleName() string {
	return c.middleName
}

func (c *client) Phone() phone.Phone {
	return c.phone
}

func (c *client) Address() string {
	return c.address
}

func (c *client) Email() internet.Email {
	return c.email
}

func (c *client) DateOfBirth() *time.Time {
	return c.dateOfBirth
}

func (c *client) Gender() general.Gender {
	return c.gender
}

func (c *client) Passport() passport.Passport {
	return c.passport
}

func (c *client) PIN() tax.Pin {
	return c.pin
}

func (c *client) CreatedAt() time.Time {
	return c.createdAt
}

func (c *client) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *client) SetPhone(number phone.Phone) Client {
	result := *c
	result.phone = number
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetName(firstName, lastName, middleName string) Client {
	result := *c
	result.firstName = firstName
	result.lastName = lastName
	result.middleName = middleName
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetAddress(address string) Client {
	result := *c
	result.address = address
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetEmail(email internet.Email) Client {
	result := *c
	result.email = email
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetDateOfBirth(dob *time.Time) Client {
	result := *c
	result.dateOfBirth = dob
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetGender(gender general.Gender) Client {
	result := *c
	result.gender = gender
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetPassport(p passport.Passport) Client {
	result := *c
	result.passport = p
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetPIN(pin tax.Pin) Client {
	result := *c
	result.pin = pin
	result.updatedAt = time.Now()
	return &result
}
