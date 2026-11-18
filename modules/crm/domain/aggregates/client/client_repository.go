// Package client provides this package.
package client

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/listext"
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

// ListID is the stable public ID of the clients list; extension packages
// contribute columns to it. NativeEntityID is the native entity it lists.
const (
	ListID         = "crm.lists.clients"
	NativeEntityID = "crm.client"
)

type FindParams struct {
	Limit   int
	Offset  int
	Search  string
	SortBy  SortBy
	Filters []Filter
	// Extra holds filters and ordering on columns contributed by extension
	// packages (see pkg/listext).
	Extra listext.Request
}

// ExtensionReader is implemented by repositories that can read the columns
// extension packages contribute to the clients list.
type ExtensionReader interface {
	ExtensionView(ctx context.Context, params *FindParams, purpose listext.Purpose, ids []uint) (*listext.View, error)
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
