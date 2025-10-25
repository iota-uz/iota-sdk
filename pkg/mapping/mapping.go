package mapping

import (
	"database/sql"
	"reflect"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
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

func SQLNullStringToUUID(ns sql.NullString) uuid.UUID {
	if !ns.Valid {
		return uuid.Nil
	}
	id, err := uuid.Parse(ns.String)
	if err != nil {
		return uuid.Nil
	}
	return id
}

func PointerToSQLNullInt32(i *int) sql.NullInt32 {
	if i != nil {
		return sql.NullInt32{
			Int32: int32(*i),
			Valid: true,
		}
	}
	return sql.NullInt32{
		Int32: 0,
		Valid: false,
	}
}

func SQLNullInt32ToPointer(v sql.NullInt32) *int {
	if v.Valid {
		val := int(v.Int32)
		return &val
	}
	return nil
}

// ToInterfaceSlice converts a slice of any type to a slice of interface{}.
// This is useful for libraries that expect []interface{} (e.g., chart libraries).
//
// Example:
//
//	floatData := []float64{1.5, 2.3, 3.7}
//	seriesData := mapping.ToInterfaceSlice(floatData)
//	// seriesData is []interface{}{1.5, 2.3, 3.7}
func ToInterfaceSlice[T any](values []T) []interface{} {
	result := make([]interface{}, len(values))
	for i, v := range values {
		result[i] = v
	}
	return result
}

// UUIDToNullUUID converts a UUID to uuid.NullUUID.
// Zero UUID (uuid.Nil) becomes NULL (Valid=false).
func UUIDToNullUUID(id uuid.UUID) uuid.NullUUID {
	return uuid.NullUUID{
		UUID:  id,
		Valid: id != uuid.Nil,
	}
}

// NullUUIDToUUID converts uuid.NullUUID to UUID.
// NULL (Valid=false) becomes uuid.Nil.
func NullUUIDToUUID(nid uuid.NullUUID) uuid.UUID {
	if nid.Valid {
		return nid.UUID
	}
	return uuid.Nil
}

// PointerToNullUUID converts a UUID pointer to uuid.NullUUID.
// nil pointer becomes NULL (Valid=false).
func PointerToNullUUID(id *uuid.UUID) uuid.NullUUID {
	if id != nil {
		return uuid.NullUUID{
			UUID:  *id,
			Valid: true,
		}
	}
	return uuid.NullUUID{
		Valid: false,
	}
}

// NullUUIDToPointer converts uuid.NullUUID to UUID pointer.
// NULL (Valid=false) becomes nil pointer.
func NullUUIDToPointer(nid uuid.NullUUID) *uuid.UUID {
	if nid.Valid {
		return &nid.UUID
	}
	return nil
}

// PointerToSQLNullInt64 converts an int pointer to sql.NullInt64.
// nil pointer becomes NULL (Valid=false).
func PointerToSQLNullInt64(i *int) sql.NullInt64 {
	if i != nil {
		return sql.NullInt64{
			Int64: int64(*i),
			Valid: true,
		}
	}
	return sql.NullInt64{
		Int64: 0,
		Valid: false,
	}
}

// SQLNullInt64ToPointer converts sql.NullInt64 to int pointer.
// NULL (Valid=false) becomes nil pointer.
func SQLNullInt64ToPointer(v sql.NullInt64) *int {
	if v.Valid {
		val := int(v.Int64)
		return &val
	}
	return nil
}

// SQLNullStringToString converts sql.NullString to string.
// NULL (Valid=false) becomes empty string.
func SQLNullStringToString(ns sql.NullString) string {
	if ns.Valid {
		return ns.String
	}
	return ""
}

// DecimalFromStringOrZero parses a decimal string with fallback to zero.
// Invalid strings return decimal.Zero instead of an error.
// Useful for database string-to-decimal conversions where zero is a safe default.
func DecimalFromStringOrZero(s string) decimal.Decimal {
	d, err := decimal.NewFromString(s)
	if err != nil {
		return decimal.Zero
	}
	return d
}

// UintToSQLNullInt64 converts a uint to sql.NullInt64.
// Zero uint becomes NULL (Valid=false), appropriate for optional foreign key references.
func UintToSQLNullInt64(val uint) sql.NullInt64 {
	if val == 0 {
		return sql.NullInt64{Valid: false}
	}
	return sql.NullInt64{Int64: int64(val), Valid: true}
}
