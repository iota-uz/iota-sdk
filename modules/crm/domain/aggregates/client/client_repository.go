package client

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type DateRange struct {
	From string
	To   string
}

type Field int

const (
	FirstName Field = iota
	LastName
	MiddleName
	PhoneNumber
	CreatedAt
	UpdatedAt
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	Limit     int
	Offset    int
	Query     string
	Field     string
	Search    string
	SortBy    SortBy
	CreatedAt DateRange
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Client, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Client, error)
	GetByID(ctx context.Context, id uint) (Client, error)
	GetByPhone(ctx context.Context, phoneNumber string) (Client, error)
	GetByContactValue(ctx context.Context, contactType ContactType, value string) (Client, error)
	Save(ctx context.Context, data Client) (Client, error) // Create or Update
	Delete(ctx context.Context, id uint) error
}
