package mapping

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/google/uuid"
)

// MapViewModels maps entities to view models
func MapViewModels[T any, V any](entities []T, mapFunc func(T) V) []V {
	viewModels := make([]V, len(entities))
	for i, entity := range entities {
		viewModels[i] = mapFunc(entity)
	}
	return viewModels
}

// MapDBModels maps entities to db models
func MapDBModels[T any, V any](
	entities []T,
	mapFunc func(T) (V, error),
) ([]V, error) {
	viewModels := make([]V, len(entities))
	for i, entity := range entities {
		viewModel, err := mapFunc(entity)
		if err != nil {
			return nil, err
		}
		viewModels[i] = viewModel
	}
	return viewModels, nil
}

// Or is a utility function that returns the first non-zero value.
func Or[T any](args ...T) T {
	for _, arg := range args {
		if !reflect.ValueOf(arg).IsZero() {
			return arg
		}
	}
	return args[0]
}

// Pointer is a utility function that returns a pointer to the given value.
func Pointer[T any](v T) *T {
	if reflect.ValueOf(v).IsZero() {
		return nil
	}
	return &v
}

// Value is a utility function that returns the value of the given pointer.
func Value[T any](v *T) T {
	if v == nil {
		return reflect.Zero(reflect.TypeOf((*T)(nil)).Elem()).Interface().(T)
	}
	return *v
}

// ValueSlice is a utility function that returns a slice of values from a slice of pointers.
func ValueSlice[T any](v []*T) []T {
	values := make([]T, len(v))
	for i, val := range v {
		values[i] = *val
	}
	return values
}

// PointerSlice is a utility function that returns a slice of pointers from a slice of values.
func PointerSlice[T any](v []T) []*T {
	values := make([]*T, len(v))
	for i, val := range v {
		values[i] = &val
	}
	return values
}

func ValueToSQLNullString(s string) sql.NullString {
	return sql.NullString{
		String: s,
		Valid:  s != "",
	}
}

func ValueToSQLNullInt32(i int32) sql.NullInt32 {
	return sql.NullInt32{
		Int32: i,
		Valid: i != 0,
	}
}

func ValueToSQLNullInt64(i int64) sql.NullInt64 {
	return sql.NullInt64{
		Int64: i,
		Valid: i != 0,
	}
}

func ValueToSQLNullFloat64(f float64) sql.NullFloat64 {
	return sql.NullFloat64{
		Float64: f,
		Valid:   f != 0,
	}
}

func PointerToSQLNullString(s *string) sql.NullString {
	if s != nil {
		return sql.NullString{
			String: *s,
			Valid:  true,
		}
	}
	return sql.NullString{
		String: "",
		Valid:  false,
	}
}

func ValueToSQLNullTime(t time.Time) sql.NullTime {
	return sql.NullTime{
		Time:  t,
		Valid: t != time.Time{},
	}
}

func SQLNullTimeToPointer(v sql.NullTime) *time.Time {
	if v.Valid {
		return &v.Time
	}
	return nil
}

func PointerToSQLNullTime(t *time.Time) sql.NullTime {
	if t != nil {
		return sql.NullTime{
			Time:  *t,
			Valid: true,
		}
	}
	return sql.NullTime{
		Time:  time.Time{},
		Valid: false,
	}
}

func UUIDToSQLNullString(id uuid.UUID) sql.NullString {
	return sql.NullString{
		String: id.String(),
		Valid:  id != uuid.Nil,
	}
}
