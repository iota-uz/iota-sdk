package crud

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// RelationMapper wraps a Mapper[T] and handles nested relations automatically.
// It extracts child fields, calls child mappers, and attaches results to the parent.
//
// Usage:
//
//	// VehicleGroup - leaf, no relations
//	vgMapper := NewRelationMapper(baseVgMapper, nil, vgMapOwn)
//
//	// VehicleType - has VehicleGroup relation
//	vtMapper := NewRelationMapper(baseVtMapper, []Relation{
//	    {Alias: "vg", Mapper: vgMapper, SetOnParent: withGroup},
//	}, vtMapOwn)
//
//	// Vehicle - has VehicleType relation (doesn't know about VehicleGroup)
//	vMapper := NewRelationMapper(baseVMapper, []Relation{
//	    {Alias: "vt", Mapper: vtMapper, SetOnParent: withVehicleType},
//	}, vMapOwn)
type RelationMapper[T any] struct {
	inner     Mapper[T]
	relations []RelationDescriptor
	mapOwn    func([]FieldValue) T
}

// NewRelationMapper creates a RelationMapper that wraps an existing mapper.
//
// Parameters:
//   - inner: the base mapper (can be nil if mapOwn handles everything)
//   - relations: child relations with Alias, Mapper (must be *RelationMapper), and SetOnParent
//   - mapOwn: function to map this entity's own fields (non-prefixed) to an entity
func NewRelationMapper[T any](
	inner Mapper[T],
	relations []RelationDescriptor,
	mapOwn func([]FieldValue) T,
) *RelationMapper[T] {
	return &RelationMapper[T]{
		inner:     inner,
		relations: relations,
		mapOwn:    mapOwn,
	}
}

func (rm *RelationMapper[T]) ToEntity(ctx context.Context, allFields []FieldValue) (T, error) {
	const op = serrors.Op("RelationMapper.ToEntity")
	var zero T

	if rm.mapOwn == nil {
		return zero, serrors.E(op, serrors.Invalid, "mapOwn function is nil")
	}

	entity := rm.mapOwn(allFields)

	// If mapOwn returns nil (e.g., when all fields are null for an interface type),
	// skip relation processing entirely and return the zero value
	if isNilEntity(entity) {
		return zero, nil
	}

	for _, rel := range rm.relations {
		alias := rel.GetAlias()
		setOnParent := rel.GetSetOnParent()
		if setOnParent == nil {
			continue
		}

		// Handle HasMany relations via JSON
		if rel.GetType() == HasMany {
			jsonFieldName := alias + "__json"
			var jsonData any
			for _, fv := range allFields {
				if fv.Field().Name() == jsonFieldName {
					jsonData = fv.Value()
					break
				}
			}
			if jsonData == nil {
				continue
			}

			// Parse JSON and call SetOnParent for each child
			// We use map[string]any since we don't know the concrete type
			items, err := parseHasManyJSON[map[string]any](jsonData)
			if err != nil {
				return zero, serrors.E(op, err, "parsing "+jsonFieldName)
			}

			for _, item := range items {
				result := setOnParent(entity, item)
				typedResult, ok := result.(T)
				if !ok {
					return zero, serrors.E(op, serrors.Invalid, "relation %q setOnParent returned unexpected type", alias)
				}
				entity = typedResult
			}
			continue
		}

		// Handle BelongsTo relations
		mapper := rel.GetMapper()
		if mapper == nil {
			continue
		}

		childFields := ExtractPrefixedFields(allFields, alias)
		// Check only non-prefixed fields (own fields) for null - nested relations like HasMany
		// have default non-null values (e.g., "[]" for empty arrays) that shouldn't affect this check
		if AllFieldsNull(ExtractNonPrefixedFields(childFields)) {
			continue
		}

		childMapper, ok := mapper.(RelationEntityMapper)
		if !ok {
			return zero, serrors.E(op, serrors.Invalid, "relation %q mapper does not implement RelationEntityMapper", alias)
		}

		child, err := childMapper.MapEntity(ctx, childFields)
		if err != nil {
			return zero, serrors.E(op, "relation %q", alias, err)
		}

		result := setOnParent(entity, child)
		typedResult, ok := result.(T)
		if !ok {
			return zero, serrors.E(op, serrors.Invalid, "relation %q setOnParent returned %T, expected %T", alias, result, zero)
		}
		entity = typedResult
	}

	return entity, nil
}

func (rm *RelationMapper[T]) ToEntities(ctx context.Context, rows ...[]FieldValue) ([]T, error) {
	result := make([]T, 0, len(rows))
	for _, row := range rows {
		entity, err := rm.ToEntity(ctx, row)
		if err != nil {
			return nil, err
		}
		result = append(result, entity)
	}
	return result, nil
}

func (rm *RelationMapper[T]) ToFieldValuesList(ctx context.Context, entities ...T) ([][]FieldValue, error) {
	if rm.inner == nil {
		return nil, nil
	}
	return rm.inner.ToFieldValuesList(ctx, entities...)
}

func (rm *RelationMapper[T]) ToFieldValues(ctx context.Context, entity T) ([]FieldValue, error) {
	fvsList, err := rm.ToFieldValuesList(ctx, entity)
	if err != nil {
		return nil, err
	}
	if len(fvsList) == 0 {
		return nil, nil
	}
	return fvsList[0], nil
}

func (rm *RelationMapper[T]) MapEntity(ctx context.Context, fields []FieldValue) (any, error) {
	return rm.ToEntity(ctx, fields)
}

var _ RelationEntityMapper = (*RelationMapper[any])(nil)

// isNilEntity checks if an entity value is nil.
// Handles both nil interfaces and nil pointers.
func isNilEntity[T any](entity T) bool {
	v := reflect.ValueOf(entity)
	if !v.IsValid() {
		return true
	}
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	}
	return false
}

// parseHasManyJSON parses JSON array data into a slice of T.
// Handles nil, "null", "[]", []byte, and string inputs.
func parseHasManyJSON[T any](jsonData any) ([]T, error) {
	const op = serrors.Op("parseHasManyJSON")

	if jsonData == nil {
		return nil, nil
	}

	// Handle already-parsed data (e.g., from pgx driver that auto-parses JSON)
	if items, ok := jsonData.([]T); ok {
		return items, nil
	}
	// Handle []any (common when JSON is auto-parsed)
	if anySlice, ok := jsonData.([]any); ok {
		result := make([]T, len(anySlice))
		for i, item := range anySlice {
			if typed, ok := item.(T); ok {
				result[i] = typed
			} else {
				return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("item %d has unexpected type %T", i, item))
			}
		}
		return result, nil
	}
	// Handle []map[string]any directly
	if mapSlice, ok := jsonData.([]map[string]any); ok {
		result := make([]T, len(mapSlice))
		for i, item := range mapSlice {
			if typed, ok := any(item).(T); ok {
				result[i] = typed
			} else {
				return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("item %d cannot be converted to target type", i))
			}
		}
		return result, nil
	}

	var data []byte

	switch v := jsonData.(type) {
	case []byte:
		if len(v) == 0 {
			return nil, nil
		}
		data = v
	case string:
		if v == "" {
			return nil, nil
		}
		data = []byte(v)
	default:
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("unexpected type %T", jsonData))
	}

	// Handle null and empty array
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" {
		return nil, nil
	}
	if trimmed == "[]" {
		return []T{}, nil
	}

	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, serrors.E(op, err)
	}

	return items, nil
}
