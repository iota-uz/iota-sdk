package crud

import "context"

type Mapper[TEntity any] interface {
	ToEntity(ctx context.Context, values []FieldValue) (TEntity, error)
	ToFieldValues(ctx context.Context, entity TEntity) ([]FieldValue, error)
}
