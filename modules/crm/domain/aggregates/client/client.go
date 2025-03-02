package client

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
	"github.com/iota-uz/iota-sdk/modules/core/domain/value_objects/passport"
)

type Client interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Phone() phone.Phone
	Address() string
	Email() string
	HourlyRate() float64
	DateOfBirth() *time.Time
	Gender() string
	Passport() passport.Passport
	PIN() string
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetPhone(number phone.Phone) Client
	SetName(firstName, lastName, middleName string) Client
	SetAddress(address string) Client
	SetEmail(email string) Client
	SetHourlyRate(rate float64) Client
	SetDateOfBirth(dob *time.Time) Client
	SetGender(gender string) Client
	SetPassport(p passport.Passport) Client
	SetPIN(pin string) Client
}

func New(
	firstName, lastName, middleName string,
	phoneNumber phone.Phone,
) (Client, error) {
	return &client{
		id:         0,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		phone:      phoneNumber,
		createdAt:  time.Now(),
		updatedAt:  time.Now(),
	}, nil
}

func NewWithID(
	id uint,
	firstName, lastName, middleName string,
	phoneNumber phone.Phone,
	createdAt, updatedAt time.Time,
) (Client, error) {
	return &client{
		id:         id,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		phone:      phoneNumber,
		createdAt:  createdAt,
		updatedAt:  updatedAt,
	}, nil
}

func NewComplete(
	id uint,
	firstName, lastName, middleName string,
	phoneNumber phone.Phone,
	address, email string,
	hourlyRate float64,
	dateOfBirth *time.Time,
	gender string,
	passportData passport.Passport,
	pin string,
	createdAt, updatedAt time.Time,
) (Client, error) {
	return &client{
		id:          id,
		firstName:   firstName,
		lastName:    lastName,
		middleName:  middleName,
		phone:       phoneNumber,
		address:     address,
		email:       email,
		hourlyRate:  hourlyRate,
		dateOfBirth: dateOfBirth,
		gender:      gender,
		passport:    passportData,
		pin:         pin,
		createdAt:   createdAt,
		updatedAt:   updatedAt,
	}, nil
}

type client struct {
	id          uint
	firstName   string
	lastName    string
	middleName  string
	phone       phone.Phone
	address     string
	email       string
	hourlyRate  float64
	dateOfBirth *time.Time
	gender      string
	passport    passport.Passport
	pin         string
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

func (c *client) Email() string {
	return c.email
}

func (c *client) HourlyRate() float64 {
	return c.hourlyRate
}

func (c *client) DateOfBirth() *time.Time {
	return c.dateOfBirth
}

func (c *client) Gender() string {
	return c.gender
}

func (c *client) Passport() passport.Passport {
	return c.passport
}

func (c *client) PIN() string {
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

func (c *client) SetEmail(email string) Client {
	result := *c
	result.email = email
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetHourlyRate(rate float64) Client {
	result := *c
	result.hourlyRate = rate
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetDateOfBirth(dob *time.Time) Client {
	result := *c
	result.dateOfBirth = dob
	result.updatedAt = time.Now()
	return &result
}

func (c *client) SetGender(gender string) Client {
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

func (c *client) SetPIN(pin string) Client {
	result := *c
	result.pin = pin
	result.updatedAt = time.Now()
	return &result
}
