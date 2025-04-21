package crud

import (
	"context"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
)

func NewSQLDataStoreAdapter[T any, ID any](
	tableName string,
) DataStore[T, ID] {
	return &sqlDataStoreAdapter[T, ID]{
		tableName: tableName,
	}
}

type sqlDataStoreAdapter[T any, ID any] struct {
	tableName string
}

func (s *sqlDataStoreAdapter[T, ID]) List(ctx context.Context, params FindParams) ([]T, error) {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return nil, err
	}
	rows, err := tx.Query(ctx, "SELECT * FROM "+s.tableName)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	entities, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return nil, err
	}
	return entities, nil
}

func (s *sqlDataStoreAdapter[T, ID]) Get(ctx context.Context, id ID) (T, error) {
	var zero T
	return zero, nil
}

func (s *sqlDataStoreAdapter[T, ID]) Create(ctx context.Context, entity T) (ID, error) {
	var zero ID
	return zero, nil
}

func (s *sqlDataStoreAdapter[T, ID]) Update(ctx context.Context, id ID, entity T) error {
	return nil
}

func (s *sqlDataStoreAdapter[T, ID]) Delete(ctx context.Context, id ID) error {
	return nil
}
