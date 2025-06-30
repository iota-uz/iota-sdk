package upload

import (
	"context"

	"github.com/gabriel-vasile/mimetype"
	"github.com/iota-uz/iota-sdk/pkg/repo"
)

type Field int

const (
	FieldSize Field = iota
	FieldName
	FieldCreatedAt
	FieldUpdatedAt
)

type SortByField = repo.SortByField[Field]
type SortBy = repo.SortBy[Field]

type FindParams struct {
	ID       uint
	Hash     string
	Limit    int
	Offset   int
	SortBy   SortBy
	Search   string
	Type     UploadType
	Mimetype *mimetype.MIME
}

type Repository interface {
	Count(ctx context.Context) (int64, error)
	GetAll(ctx context.Context) ([]Upload, error)
	GetPaginated(ctx context.Context, params *FindParams) ([]Upload, error)
	GetByID(ctx context.Context, id uint) (Upload, error)
	GetByHash(ctx context.Context, hash string) (Upload, error)
	Exists(ctx context.Context, id uint) (bool, error)
	Create(ctx context.Context, data Upload) (Upload, error)
	Update(ctx context.Context, data Upload) error
	Delete(ctx context.Context, id uint) error
}
