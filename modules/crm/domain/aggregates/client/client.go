package client

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/phone"
)

type Client interface {
	ID() uint
	FirstName() string
	LastName() string
	MiddleName() string
	Phone() phone.Phone
	CreatedAt() time.Time
	UpdatedAt() time.Time

	SetPhone(number phone.Phone) Client
	SetName(firstName, lastName, middleName string) Client
}
