package crud

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
)

func NewSQLDataStoreAdapter[T any, ID any](
	tableName string,
	primaryKey string,
) DataStore[T, ID] {
	return &sqlDataStoreAdapter[T, ID]{
		tableName:  tableName,
		primaryKey: primaryKey,
	}
}

type sqlDataStoreAdapter[T any, ID any] struct {
	tableName  string
	primaryKey string
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return zero, err
	}

	rows, err := tx.Query(ctx, fmt.Sprintf("SELECT * FROM %s", s.tableName)+fmt.Sprintf(" WHERE %s = $1", s.primaryKey), id)
	if err != nil {
		return zero, err
	}
	defer rows.Close()

	entities, err := pgx.CollectRows(rows, pgx.RowToStructByName[T])
	if err != nil {
		return zero, err
	}
	if len(entities) == 0 {
		return zero, pgx.ErrNoRows
	}
	return entities[0], nil
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
