package client

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
)

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

type client struct {
	id         uint
	firstName  string
	lastName   string
	middleName string
	phone      phone.Phone
	createdAt  time.Time
	updatedAt  time.Time
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

func (c *client) CreatedAt() time.Time {
	return c.createdAt
}

func (c *client) UpdatedAt() time.Time {
	return c.updatedAt
}

func (c *client) SetPhone(number phone.Phone) Client {
	return &client{
		id:         c.id,
		firstName:  c.firstName,
		lastName:   c.lastName,
		middleName: c.middleName,
		phone:      number,
		createdAt:  c.createdAt,
		updatedAt:  time.Now(),
	}
}

func (c *client) SetName(firstName, lastName, middleName string) Client {
	return &client{
		id:         c.id,
		firstName:  firstName,
		lastName:   lastName,
		middleName: middleName,
		phone:      c.phone,
		createdAt:  c.createdAt,
		updatedAt:  time.Now(),
	}
}
