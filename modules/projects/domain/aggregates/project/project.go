package project

import (
	"time"

	"github.com/google/uuid"
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

	CreatedAt() time.Time
	UpdatedAt() time.Time
}
