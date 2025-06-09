package crud

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/jackc/pgx/v5"
)

func getPrimaryKey[T any]() string {
	v := reflect.TypeOf((*T)(nil)).Elem()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		if field.Tag.Get("sdk") == "primaryKey" {
			return field.Name
		}
	}
	panic(fmt.Sprintf("No primary key found for type %s", v.Name()))
}

func NewSQLDataStoreAdapter[T any, ID any](tableName string) DataStore[T, ID] {
	return &sqlDataStoreAdapter[T, ID]{
		tableName:  tableName,
		primaryKey: getPrimaryKey[T](),
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

	query := fmt.Sprintf("SELECT * FROM %s WHERE %s = $1", s.tableName, s.primaryKey)
	rows, err := tx.Query(ctx, query, id)
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
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return zero, err
	}

	// Get entity fields through reflection
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	entityType := entityValue.Type()

	columns := make([]string, 0, entityType.NumField())
	values := make([]interface{}, 0, entityType.NumField())
	placeholders := make([]string, 0, entityType.NumField())
	var primaryKeyField string

	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		// Skip if not DB field or is primary key with zero value
		if field.Tag.Get("db") == "-" {
			continue
		}

		fieldName := field.Tag.Get("db")
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		isPrimaryKey := field.Tag.Get("sdk") == "primaryKey"
		if isPrimaryKey {
			primaryKeyField = fieldName
		}

		// Skip if primary key is auto-increment and has zero value
		fieldValue := entityValue.Field(i)
		isZeroValue := reflect.DeepEqual(fieldValue.Interface(), reflect.Zero(fieldValue.Type()).Interface())
		if isPrimaryKey && isZeroValue {
			continue
		}

		columns = append(columns, fieldName)
		values = append(values, fieldValue.Interface())
		placeholders = append(placeholders, fmt.Sprintf("$%d", len(placeholders)+1))
	}

	query := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s) RETURNING %s",
		s.tableName,
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
		primaryKeyField,
	)

	var result ID
	err = tx.QueryRow(ctx, query, values...).Scan(&result)
	if err != nil {
		return zero, err
	}

	return result, nil
}

func (s *sqlDataStoreAdapter[T, ID]) Update(ctx context.Context, id ID, entity T) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	// Get entity fields through reflection
	entityValue := reflect.ValueOf(entity)
	if entityValue.Kind() == reflect.Ptr {
		entityValue = entityValue.Elem()
	}
	entityType := entityValue.Type()

	updates := make([]string, 0, entityType.NumField())
	values := make([]interface{}, 0, entityType.NumField())

	for i := 0; i < entityType.NumField(); i++ {
		field := entityType.Field(i)
		// Skip if not DB field or is primary key
		if field.Tag.Get("db") == "-" || field.Tag.Get("sdk") == "primaryKey" {
			continue
		}

		fieldName := field.Tag.Get("db")
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		updates = append(updates, fmt.Sprintf("%s = $%d", fieldName, len(values)+1))
		values = append(values, entityValue.Field(i).Interface())
	}

	// Add ID as the last parameter
	values = append(values, id)

	query := fmt.Sprintf(
		"UPDATE %s SET %s WHERE %s = $%d",
		s.tableName,
		strings.Join(updates, ", "),
		s.primaryKey,
		len(values),
	)

	_, err = tx.Exec(ctx, query, values...)
	return err
}

func (s *sqlDataStoreAdapter[T, ID]) Delete(ctx context.Context, id ID) error {
	tx, err := composables.UseTx(ctx)
	if err != nil {
		return err
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE %s = $1", s.tableName, s.primaryKey)
	_, err = tx.Exec(ctx, query, id)
	return err
}
