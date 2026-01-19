package crud

import (
	"context"
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
			continue
		}

		child, err := childMapper.MapEntity(ctx, childFields)
		if err != nil {
			continue
		}

		entity = setOnParent(entity, child).(T)
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
