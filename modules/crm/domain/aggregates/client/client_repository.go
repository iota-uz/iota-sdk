package client

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	ID Field = iota
	FirstName
	LastName
	MiddleName
	PhoneNumber
	CreatedAt
	UpdatedAt
	TenantID
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]
type Filter = repo.FieldFilter[Field]

type FindParams struct {
	Limit   int
	Offset  int
	Search  string
	SortBy  SortBy
	Filters []Filter
}

type Repository interface {
	Count(ctx context.Context, params *FindParams) (int64, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)
	GetByID(ctx context.Context, id uint) (Client, error)
	GetByPhone(ctx context.Context, phoneNumber string) (Client, error)
	GetByContactValue(ctx context.Context, contactType ContactType, value string) (Client, error)
	Save(ctx context.Context, data Client) (Client, error) // Create or Update
	Delete(ctx context.Context, id uint) error
}
