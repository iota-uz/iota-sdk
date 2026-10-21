// Package project provides this package.
package project

import (
	"time"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/money"
)

type Project interface {
	ID() uuid.UUID
	SetID(uuid.UUID)

	TenantID() uuid.UUID

	CounterpartyID() uuid.UUID
	UpdateCounterpartyID(uuid.UUID) Project

	Name() string
	UpdateName(string) Project

	Description() string
	UpdateDescription(string) Project

	// Contract is the amount agreed with the client, nil when not set.
	Contract() *money.Money
	UpdateContract(*money.Money) Project

	CreatedAt() time.Time
	UpdatedAt() time.Time
}
