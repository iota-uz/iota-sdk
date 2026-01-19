package crud

import (
	"context"
	"encoding/json"
	"fmt"
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

	for _, rel := range rm.relations {
		mapper := rel.GetMapper()
		setOnParent := rel.GetSetOnParent()
		if mapper == nil || setOnParent == nil {
			continue
		}

		childFields := ExtractPrefixedFields(allFields, rel.GetAlias())
		if AllFieldsNull(childFields) {
			continue
		}

		childMapper, ok := mapper.(RelationEntityMapper)
		if !ok {
			return zero, serrors.E(op, serrors.Invalid, "relation %q mapper does not implement RelationEntityMapper", rel.GetAlias())
		}

		child, err := childMapper.MapEntity(ctx, childFields)
		if err != nil {
			return zero, serrors.E(op, "relation %q", rel.GetAlias(), err)
		}

		result := setOnParent(entity, child)
		typedResult, ok := result.(T)
		if !ok {
			return zero, serrors.E(op, serrors.Invalid, "relation %q setOnParent returned %T, expected %T", rel.GetAlias(), result, zero)
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

// parseHasManyJSON parses JSON array data into a slice of T.
// Handles nil, "null", "[]", []byte, and string inputs.
func parseHasManyJSON[T any](jsonData any) ([]T, error) {
	if jsonData == nil {
		return nil, nil
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
		return nil, fmt.Errorf("parseHasManyJSON: unexpected type %T", jsonData)
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
		return nil, fmt.Errorf("parseHasManyJSON: %w", err)
	}

	return items, nil
}
