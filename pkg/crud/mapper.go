package crud

import "context"

type Mapper[TEntity any] interface {
	ToEntities(ctx context.Context, values ...FieldValues) ([]TEntity, error)
	ToFieldValues(ctx context.Context, entities ...TEntity) ([]FieldValues, error)
}
