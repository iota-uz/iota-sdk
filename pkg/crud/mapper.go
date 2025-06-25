package crud

import (
	"context"
	"errors"
)

type Mapper[TEntity any] interface {
	ToEntities(ctx context.Context, values ...[]FieldValue) ([]TEntity, error)
	ToFieldValuesList(ctx context.Context, entities ...TEntity) ([][]FieldValue, error)
}

var ErrEmptyResult = errors.New("no entities returned from mapper")

type FlatMapper[TEntity any] interface {
	Mapper[TEntity]
	ToEntity(ctx context.Context, values []FieldValue) (TEntity, error)
	ToFieldValues(ctx context.Context, entity TEntity) ([]FieldValue, error)
}

func newFlatMapper[TEntity any](mapper Mapper[TEntity]) FlatMapper[TEntity] {
	return &flatMapper[TEntity]{Mapper: mapper}
}

type flatMapper[TEntity any] struct {
	Mapper[TEntity]
}

func (m *flatMapper[TEntity]) ToEntity(ctx context.Context, values []FieldValue) (TEntity, error) {
	var zero TEntity

	entities, err := m.ToEntities(ctx, values)
	if err != nil {
		return zero, err
	}
	if len(entities) == 0 {
		return zero, ErrEmptyResult
	}
	return entities[0], nil
}

func (m *flatMapper[TEntity]) ToFieldValues(ctx context.Context, entity TEntity) ([]FieldValue, error) {
	values, err := m.ToFieldValuesList(ctx, entity)
	if err != nil {
		return nil, err
	}
	if len(values) == 0 {
		return nil, ErrEmptyResult
	}
	return values[0], nil
}
